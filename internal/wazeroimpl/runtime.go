package wazeroimpl

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/sys/unix"

	"github.com/CosmWasm/wasmvm/v3/types"
)

// Cache manages a wazero runtime, compiled modules, and on-disk code storage.
type Cache struct {
	runtime wazero.Runtime
	modules map[string]wazero.CompiledModule
	// raw stores the original Wasm bytes by checksum hex
	raw map[string][]byte
	// lockfile holds the exclusive lock on BaseDir
	lockfile *os.File
	// baseDir is the root directory for on-disk cache
	baseDir string
}

// locateData allocates memory in the given module using its "allocate" export
// and writes the provided data slice there. It returns the pointer and length
// of the written data within the module's linear memory. Any allocation or
// write failure results in a panic, as this indicates the guest module does
// not follow the expected CosmWasm ABI.
func locateData(ctx context.Context, mod api.Module, data []byte) (uint32, uint32) {
	if len(data) == 0 {
		return 0, 0
	}

	alloc := mod.ExportedFunction("allocate")
	if alloc == nil {
		panic("guest module does not export an 'allocate' function required by CosmWasm ABI")
	}

	// Call allocate with the size (i32). The function returns a pointer (i32).
	res, err := alloc.Call(ctx, uint64(len(data)))
	if err != nil {
		panic(fmt.Sprintf("allocate() failed: %v", err))
	}
	if len(res) == 0 {
		panic("allocate() returned no results")
	}

	ptr := uint32(res[0])

	mem := mod.Memory()
	if ok := mem.Write(ptr, data); !ok {
		panic("failed to write data into guest memory")
	}

	return ptr, uint32(len(data))
}

// RemoveCode removes stored Wasm and compiled module for the given checksum.
func (c *Cache) RemoveCode(checksum types.Checksum) error {
	key := hex.EncodeToString(checksum)
	if _, ok := c.raw[key]; !ok {
		return fmt.Errorf("code '%s' not found", key)
	}
	// Remove on-disk Wasm file if persisted
	if c.baseDir != "" {
		filePath := filepath.Join(c.baseDir, "code", key+".wasm")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove wasm file: %w", err)
		}
	}
	delete(c.raw, key)
	delete(c.modules, key)
	return nil
}

// GetCode returns the original Wasm bytes for the given checksum.
func (c *Cache) GetCode(checksum types.Checksum) ([]byte, error) {
	key := hex.EncodeToString(checksum)
	data, ok := c.raw[key]
	if !ok {
		return nil, fmt.Errorf("code '%s' not found", key)
	}
	return append([]byte(nil), data...), nil
}

// InitCache creates a new wazero Runtime with memory limits similar to api.InitCache.
func InitCache(config types.VMConfig) (*Cache, error) {
	// Prepare in-memory storage, lockfile handle, and base directory
	raw := make(map[string][]byte)
	var lf *os.File
	base := config.Cache.BaseDir
	if base != "" {
		// Create base and code directories
		if strings.Contains(base, ":") && runtime.GOOS != "windows" {
			return nil, fmt.Errorf("invalid base directory: %s", base)
		}
		if err := os.MkdirAll(base, 0o755); err != nil {
			return nil, fmt.Errorf("could not create base directory: %w", err)
		}
		codeDir := filepath.Join(base, "code")
		if err := os.MkdirAll(codeDir, 0o755); err != nil {
			return nil, fmt.Errorf("could not create code directory: %w", err)
		}
		// Acquire exclusive lock
		lockPath := filepath.Join(base, "exclusive.lock")
		var err error
		lf, err = os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE, 0o666)
		if err != nil {
			return nil, fmt.Errorf("could not open exclusive.lock: %w", err)
		}
		_, err = lf.WriteString("exclusive lock for wazero VM\n")
		if err != nil {
			lf.Close()
			return nil, fmt.Errorf("error writing to exclusive.lock: %w", err)
		}
		if err := unix.Flock(int(lf.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			lf.Close()
			return nil, fmt.Errorf("could not lock exclusive.lock; is another VM running? %w", err)
		}
		// Pre-load existing Wasm blobs
		patterns := filepath.Join(codeDir, "*.wasm")
		files, err := filepath.Glob(patterns)
		if err != nil {
			lf.Close()
			return nil, fmt.Errorf("failed scanning code directory: %w", err)
		}
		for _, p := range files {
			data, err := os.ReadFile(p)
			if err != nil {
				lf.Close()
				return nil, fmt.Errorf("failed reading existing code %s: %w", p, err)
			}
			name := filepath.Base(p)
			key := strings.TrimSuffix(name, ".wasm")
			raw[key] = data
		}
	}

	ctx := context.Background()
	limitBytes := *(*uint32)(unsafe.Pointer(&config.Cache.InstanceMemoryLimitBytes))
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(limitBytes/65536))
	return &Cache{
		runtime:  r,
		modules:  make(map[string]wazero.CompiledModule),
		raw:      raw,
		lockfile: lf,
		baseDir:  base,
	}, nil
}

// Close releases the runtime and the directory lock.
func (c *Cache) Close(ctx context.Context) error {
	if c.runtime != nil {
		c.runtime.Close(ctx)
	}
	if c.lockfile != nil {
		c.lockfile.Close()
	}
	return nil
}

// Compile stores a compiled module under the given checksum.
func (c *Cache) Compile(ctx context.Context, checksum types.Checksum, wasm []byte) error {
	key := hex.EncodeToString(checksum)
	// Persist Wasm blob to disk if enabled
	if c.baseDir != "" {
		codeDir := filepath.Join(c.baseDir, "code")
		if err := os.MkdirAll(codeDir, 0o755); err != nil {
			return fmt.Errorf("could not create code directory: %w", err)
		}
		filePath := filepath.Join(codeDir, key+".wasm")
		if err := os.WriteFile(filePath, wasm, 0o644); err != nil {
			return fmt.Errorf("failed to write wasm file: %w", err)
		}
	}
	// Store raw Wasm bytes in memory
	c.raw[key] = append([]byte(nil), wasm...)
	// Compile module
	mod, err := c.runtime.CompileModule(ctx, wasm)
	if err != nil {
		return err
	}
	c.modules[key] = mod
	return nil
}

// getModule returns the compiled module for the checksum.
func (c *Cache) getModule(checksum types.Checksum) (wazero.CompiledModule, bool) {
	mod, ok := c.modules[hex.EncodeToString(checksum)]
	return mod, ok
}

// registerHost builds an env module with callbacks for the given state.
func (c *Cache) registerHost(ctx context.Context, store types.KVStore, apiImpl *types.GoAPI, q *types.Querier, gm types.GasMeter) (api.Module, error) {
	builder := c.runtime.NewHostModuleBuilder("env")
	// ---------------------------------------------------------------------
	// Helper functions required by CosmWasm contracts – **legacy** (v0.10-0.16)
	// ABI. These minimal stubs are sufficient for the reflect.wasm contract to
	// instantiate and run in tests. More complete, modern variants will be added
	// in later milestones.

	// debug(msg_ptr) – prints UTF-8 string [len|bytes]
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		mem := m.Memory()
		// message length is stored in little-endian u32 at ptr
		b, _ := mem.Read(ptr, 4)
		l := binary.LittleEndian.Uint32(b)
		data, _ := mem.Read(ptr+4, l)
		_ = data // silenced; could log.Printf if desired
	}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{}).Export("debug")

	// abort(msg_ptr)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		mem := m.Memory()
		b, _ := mem.Read(ptr, 4)
		l := binary.LittleEndian.Uint32(b)
		data, _ := mem.Read(ptr+4, l)
		panic(string(data))
	}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{}).Export("abort")

	// db_read(key_ptr) -> i32 (data_ptr)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		keyPtr := uint32(stack[0])
		mem := m.Memory()
		// length-prefixed key (4 byte little-endian length)
		lenBytes, _ := mem.Read(keyPtr, 4)
		keyLen := binary.LittleEndian.Uint32(lenBytes)
		key, _ := mem.Read(keyPtr+4, keyLen)
		val := store.Get(key)
		if val == nil {
			stack[0] = 0
			return
		}
		buf := make([]byte, 4+len(val))
		binary.LittleEndian.PutUint32(buf, uint32(len(val)))
		copy(buf[4:], val)
		ptr, _ := locateData(ctx, m, buf)
		stack[0] = uint64(ptr)
	}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("db_read")

	// db_write(key_ptr, value_ptr)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		keyPtr := uint32(stack[0])
		valPtr := uint32(stack[1])
		mem := m.Memory()
		// length-prefixed key
		lenB, _ := mem.Read(keyPtr, 4)
		keyLen := binary.LittleEndian.Uint32(lenB)
		key, _ := mem.Read(keyPtr+4, keyLen)
		// length-prefixed value
		valLenB, _ := mem.Read(valPtr, 4)
		valLen := binary.LittleEndian.Uint32(valLenB)
		val, _ := mem.Read(valPtr+4, valLen)
		store.Set(key, val)
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).Export("db_write")

	// db_remove(key_ptr)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		keyPtr := uint32(stack[0])
		mem := m.Memory()
		lenB, _ := mem.Read(keyPtr, 4)
		keyLen := binary.LittleEndian.Uint32(lenB)
		key, _ := mem.Read(keyPtr+4, keyLen)
		store.Delete(key)
	}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{}).Export("db_remove")

	// addr_validate(human_ptr) -> i32 (0 = valid)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		// we consider all addresses valid in stub; return 0
		stack[0] = 0
	}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("addr_validate")

	// addr_canonicalize(human_ptr, out_ptr) -> i32 (0 = OK)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("addr_canonicalize")

	// addr_humanize(canonical_ptr, out_ptr) -> i32 (0 = OK)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("addr_humanize")

	// query_chain(request_ptr) -> i32 (response_ptr)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		// not implemented: return 0 meaning empty response
		stack[0] = 0
	}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("query_chain")

	// secp256k1_verify, secp256k1_recover_pubkey, ed25519_verify, ed25519_batch_verify – stubs that return 0 (false)
	stubVerify := func(name string, paramCount int) {
		params := make([]api.ValueType, paramCount)
		for i := range params {
			params[i] = api.ValueTypeI32
		}
		builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			stack[0] = 0
		}), params, []api.ValueType{api.ValueTypeI32}).Export(name)
	}
	stubVerify("secp256k1_verify", 3)
	// secp256k1_recover_pubkey returns ptr (i64 in legacy ABI)
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).Export("secp256k1_recover_pubkey")

	stubVerify("secp256k1_verify", 3)
	stubVerify("ed25519_verify", 3)
	stubVerify("ed25519_batch_verify", 3)
	stubVerify("ed25519_verify", 3)
	stubVerify("ed25519_batch_verify", 3)

	// query_external - simplified: returns 0 length
	// canonicalize_address: input human string -> canonical bytes
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		inputPtr := uint32(stack[0])
		inputLen := uint32(stack[1])
		outPtr := uint32(stack[2])
		errPtr := uint32(stack[3])
		gasPtr := uint32(stack[4])
		mem := m.Memory()
		input, _ := mem.Read(inputPtr, inputLen)
		// call GoAPI
		canonical, usedGas, err := apiImpl.CanonicalizeAddress(string(input))
		// write gas
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, usedGas)
		mem.Write(gasPtr, buf)
		if err != nil {
			mem.WriteUint32Le(errPtr, uint32(len(err.Error())))
			mem.Write(errPtr+4, []byte(err.Error()))
			return
		}
		mem.WriteUint32Le(outPtr, uint32(len(canonical)))
		mem.Write(outPtr+4, canonical)
	}), []api.ValueType{
		api.ValueTypeI32, api.ValueTypeI32,
		api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32,
	}, []api.ValueType{}).Export("canonicalize_address")
	// humanize_address: input canonical bytes -> human string
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		inputPtr := uint32(stack[0])
		inputLen := uint32(stack[1])
		outPtr := uint32(stack[2])
		errPtr := uint32(stack[3])
		gasPtr := uint32(stack[4])
		mem := m.Memory()
		input, _ := mem.Read(inputPtr, inputLen)
		human, usedGas, err := apiImpl.HumanizeAddress(input)
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, usedGas)
		mem.Write(gasPtr, buf)
		if err != nil {
			mem.WriteUint32Le(errPtr, uint32(len(err.Error())))
			mem.Write(errPtr+4, []byte(err.Error()))
			return
		}
		mem.WriteUint32Le(outPtr, uint32(len(human)))
		mem.Write(outPtr+4, []byte(human))
	}), []api.ValueType{
		api.ValueTypeI32, api.ValueTypeI32,
		api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32,
	}, []api.ValueType{}).Export("humanize_address")
	// validate_address: input human string -> error only
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		inputPtr := uint32(stack[0])
		inputLen := uint32(stack[1])
		errPtr := uint32(stack[2])
		gasPtr := uint32(stack[3])
		mem := m.Memory()
		tmp, _ := mem.Read(inputPtr, inputLen)
		input := string(tmp)
		usedGas, err := apiImpl.ValidateAddress(input)
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, usedGas)
		mem.Write(gasPtr, buf)
		if err != nil {
			msg := err.Error()
			mem.WriteUint32Le(errPtr, uint32(len(msg)))
			mem.Write(errPtr+4, []byte(msg))
		}
	}), []api.ValueType{
		api.ValueTypeI32, api.ValueTypeI32,
		api.ValueTypeI32, api.ValueTypeI32,
	}, []api.ValueType{}).Export("validate_address")

	return builder.Instantiate(ctx)
}

// Instantiate loads and runs the contract's instantiate function.
func (c *Cache) Instantiate(ctx context.Context, checksum types.Checksum, env, info, msg []byte, store types.KVStore, apiImpl *types.GoAPI, q *types.Querier, gm types.GasMeter) error {
	compiled, ok := c.getModule(checksum)
	if !ok {
		return fmt.Errorf("module not found")
	}
	_, err := c.registerHost(ctx, store, apiImpl, q, gm)
	if err != nil {
		return err
	}
	mod, err := c.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return err
	}
	if fn := mod.ExportedFunction("instantiate"); fn != nil {
		paramCount := len(fn.Definition().ParamTypes())
		if paramCount == 6 {
			// CosmWasm v1+ ABI (ptr,len pairs)
			envPtr, envLen := uint32(0), uint32(0)
			infoPtr, infoLen := uint32(0), uint32(0)
			msgPtr, msgLen := uint32(0), uint32(0)
			if len(env) > 0 {
				envPtr, envLen = locateData(ctx, mod, env)
			}
			if len(info) > 0 {
				infoPtr, infoLen = locateData(ctx, mod, info)
			}
			if len(msg) > 0 {
				msgPtr, msgLen = locateData(ctx, mod, msg)
			}
			_, err = fn.Call(ctx, uint64(envPtr), uint64(envLen), uint64(infoPtr), uint64(infoLen), uint64(msgPtr), uint64(msgLen))
		} else if paramCount == 3 {
			// Legacy ABI: env_ptr, info_ptr, msg_ptr (each data = len|bytes)
			wrap := func(b []byte) []byte {
				buf := make([]byte, 4+len(b))
				binary.LittleEndian.PutUint32(buf, uint32(len(b)))
				copy(buf[4:], b)
				return buf
			}
			envPtr, _ := locateData(ctx, mod, wrap(env))
			infoPtr, _ := locateData(ctx, mod, wrap(info))
			msgPtr, _ := locateData(ctx, mod, wrap(msg))
			_, err = fn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
		} else {
			err = fmt.Errorf("unsupported instantiate param count %d", paramCount)
		}
	}
	_ = mod.Close(ctx)
	return err
}

// Execute runs the contract's execute function.
func (c *Cache) Execute(ctx context.Context, checksum types.Checksum, env, info, msg []byte, store types.KVStore, apiImpl *types.GoAPI, q *types.Querier, gm types.GasMeter) error {
	compiled, ok := c.getModule(checksum)
	if !ok {
		return fmt.Errorf("module not found")
	}
	_, err := c.registerHost(ctx, store, apiImpl, q, gm)
	if err != nil {
		return err
	}
	mod, err := c.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return err
	}
	if fn := mod.ExportedFunction("execute"); fn != nil {
		paramCount := len(fn.Definition().ParamTypes())
		if paramCount == 6 {
			envPtr, envLen := uint32(0), uint32(0)
			infoPtr, infoLen := uint32(0), uint32(0)
			msgPtr, msgLen := uint32(0), uint32(0)
			if len(env) > 0 {
				envPtr, envLen = locateData(ctx, mod, env)
			}
			if len(info) > 0 {
				infoPtr, infoLen = locateData(ctx, mod, info)
			}
			if len(msg) > 0 {
				msgPtr, msgLen = locateData(ctx, mod, msg)
			}
			_, err = fn.Call(ctx, uint64(envPtr), uint64(envLen), uint64(infoPtr), uint64(infoLen), uint64(msgPtr), uint64(msgLen))
		} else if paramCount == 3 {
			wrap := func(b []byte) []byte {
				buf := make([]byte, 4+len(b))
				binary.LittleEndian.PutUint32(buf, uint32(len(b)))
				copy(buf[4:], b)
				return buf
			}
			envPtr, _ := locateData(ctx, mod, wrap(env))
			infoPtr, _ := locateData(ctx, mod, wrap(info))
			msgPtr, _ := locateData(ctx, mod, wrap(msg))
			_, err = fn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
		} else {
			err = fmt.Errorf("unsupported execute param count %d", paramCount)
		}
	}
	_ = mod.Close(ctx)
	return err
}
