package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// Assume these are your runtime-level interfaces:
// - db is a KVStore for contract storage
// - api is GoAPI for address manipulations
// - querier is a Querier for external queries
type RuntimeEnvironment struct {
	DB      types.KVStore
	API     *types.GoAPI
	Querier types.Querier
}

// Example host function: Get key from DB
func hostGet(ctx context.Context, mod api.Module, keyPtr uint32, keyLen uint32) (dataPtr uint32, dataLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment) // Assume you set this in config
	mem := mod.Memory()

	keyBytes, ok := mem.Read(keyPtr, keyLen)
	if !ok {
		panic("failed to read key from memory")
	}

	value := env.DB.Get(keyBytes)
	if len(value) == 0 {
		// Return (0,0) for no data
		return 0, 0
	}

	// Write the value back to memory. In a real scenario, you need an allocator.
	// For simplicity, assume we have a fixed offset for result writes:
	offset := uint32(2048) // Just an example offset
	if !mem.Write(offset, value) {
		panic("failed to write value to memory")
	}
	return offset, uint32(len(value))
}

// Example host function: Set key in DB
func hostSet(ctx context.Context, mod api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	key, ok := mem.Read(keyPtr, keyLen)
	if !ok {
		panic("failed to read key from memory")
	}
	val, ok2 := mem.Read(valPtr, valLen)
	if !ok2 {
		panic("failed to read value from memory")
	}

	env.DB.Set(key, val)
}

// Example host function: HumanizeAddress
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) (resPtr, resLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	addrBytes, ok := mem.Read(addrPtr, addrLen)
	if !ok {
		panic("failed to read address from memory")
	}

	human, _, err := env.API.HumanizeAddress(addrBytes)
	if err != nil {
		// On error, you might return 0,0 or handle differently
		return 0, 0
	}

	offset := uint32(4096) // Some offset for writing back
	if !mem.Write(offset, []byte(human)) {
		panic("failed to write humanized address")
	}
	return offset, uint32(len(human))
}

// Example host function: QueryExternal
func hostQueryExternal(ctx context.Context, mod api.Module, reqPtr, reqLen, gasLimit uint32) (resPtr, resLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	req, ok := mem.Read(reqPtr, reqLen)
	if !ok {
		panic("failed to read query request")
	}

	res := types.RustQuery(env.Querier, req, uint64(gasLimit))
	serialized, err := json.Marshal(res)
	if err != nil {
		// handle error, maybe return 0,0
		return 0, 0
	}

	offset := uint32(8192) // Another offset
	if !mem.Write(offset, serialized) {
		panic("failed to write query response")
	}
	return offset, uint32(len(serialized))
}

// RegisterHostFunctions registers all host functions into a module named "env".
// The wasm code must import them from "env".
func RegisterHostFunctions(r wazero.Runtime, env *RuntimeEnvironment) (wazero.CompiledModule, error) {
	// Build a module that exports these functions
	hostModBuilder := r.NewHostModuleBuilder("env")

	// Example: Registering hostGet as "db_get"
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostGet).
		Export("db_get")

	// Similarly for hostSet
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostSet).
		Export("db_set")

	// For humanize address
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostHumanizeAddress).
		Export("api_humanize_address")

	// For queries
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostQueryExternal).
		Export("querier_query")

	// Compile the host module
	compiled, err := hostModBuilder.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to compile host module: %w", err)
	}

	return compiled, nil
}

// When you instantiate a contract, you can do something like:
//
// compiledHost, err := RegisterHostFunctions(runtime, env)
// if err != nil {
//   ...
// }
// _, err = runtime.InstantiateModule(ctx, compiledHost, wazero.NewModuleConfig())
// if err != nil {
//   ...
// }
//
// Then, instantiate your contract module which imports "env" module's functions.
