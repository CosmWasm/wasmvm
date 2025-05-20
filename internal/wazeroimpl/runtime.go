package wazeroimpl

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/CosmWasm/wasmvm/v3/types"
)

// Cache manages a wazero runtime and compiled modules.
type Cache struct {
	runtime wazero.Runtime
	modules map[string]wazero.CompiledModule
}

// InitCache creates a new wazero Runtime with memory limits similar to api.InitCache.
func InitCache(config types.VMConfig) (*Cache, error) {
	if base := config.Cache.BaseDir; base != "" {
		if strings.Contains(base, ":") && runtime.GOOS != "windows" {
			return nil, fmt.Errorf("could not create base directory")
		}
		if err := os.MkdirAll(base, 0o700); err != nil {
			return nil, fmt.Errorf("could not create base directory: %w", err)
		}
	}

	ctx := context.Background()
	limitBytes := *(*uint32)(unsafe.Pointer(&config.Cache.InstanceMemoryLimitBytes))
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(limitBytes/65536))
	return &Cache{
		runtime: r,
		modules: make(map[string]wazero.CompiledModule),
	}, nil
}

// Close releases all resources of the runtime.
func (c *Cache) Close(ctx context.Context) error {
	if c.runtime != nil {
		return c.runtime.Close(ctx)
	}
	return nil
}

// Compile stores a compiled module under the given checksum.
func (c *Cache) Compile(ctx context.Context, checksum types.Checksum, wasm []byte) error {
	mod, err := c.runtime.CompileModule(ctx, wasm)
	if err != nil {
		return err
	}
	c.modules[hex.EncodeToString(checksum)] = mod
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
	builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		resPtr := uint32(stack[2])
		_ = m.Memory().WriteUint32Le(resPtr, 0)
	}), []api.ValueType{api.ValueTypeI64, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).Export("query_external")

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
