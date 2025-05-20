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

	// db_read
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		keyPtr := uint32(stack[0])
		keyLen := uint32(stack[1])
		outPtr := uint32(stack[2])
		mem := m.Memory()
		key, _ := mem.Read(keyPtr, keyLen)
		value := store.Get(key)
		if value == nil {
			_ = mem.WriteUint32Le(outPtr, 0)
			return
		}
		_ = mem.WriteUint32Le(outPtr, uint32(len(value)))
		mem.Write(outPtr+4, value)
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).Export("db_read")

	// db_write
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		keyPtr := uint32(stack[0])
		keyLen := uint32(stack[1])
		valPtr := uint32(stack[2])
		valLen := uint32(stack[3])
		mem := m.Memory()
		key, _ := mem.Read(keyPtr, keyLen)
		val, _ := mem.Read(valPtr, valLen)
		store.Set(key, val)
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).Export("db_write")

	// db_remove
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		keyPtr := uint32(stack[0])
		keyLen := uint32(stack[1])
		mem := m.Memory()
		key, _ := mem.Read(keyPtr, keyLen)
		store.Delete(key)
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).Export("db_remove")

	// query_external - simplified: returns 0 length
	// canonicalize_address: input human string -> canonical bytes
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		inputPtr := uint32(stack[0])
		inputLen := uint32(stack[1])
		outPtr := uint32(stack[2])
		errPtr := uint32(stack[3])
		gasPtr := uint32(stack[4])
		mem := m.Memory()
		input := mem.Read(inputPtr, inputLen)
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
		input := mem.Read(inputPtr, inputLen)
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
		input := string(mem.Read(inputPtr, inputLen))
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
		_, err = fn.Call(ctx)
	}
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
		_, err = fn.Call(ctx)
	}
	return err
}
