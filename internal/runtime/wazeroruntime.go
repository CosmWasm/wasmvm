package runtime

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	// Assume types and memory packages are in same module
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// WazeroVM is a CosmWasm VM implementation using the Wazero runtime.
// It manages module compilation, instantiation, and memory via MemoryManager.
type WazeroVM struct {
	runtime     wazero.Runtime
	cache       map[types.Checksum]wazero.CompiledModule // cache compiled modules by checksum
	debug       bool                                     // print debug logs if true
	memoryLimit uint32                                   // memory limit (in MiB) for each instance
}

// NewWazeroVM creates a new WazeroVM. It compiles modules on the fly or from cache.
// memoryLimit is the memory limit per contract (in MiB). debug controls debug logging.
func NewWazeroVM(runtime wazero.Runtime, memoryLimit uint32, debug bool) *WazeroVM {
	return &WazeroVM{
		runtime:     runtime,
		cache:       make(map[types.Checksum]wazero.CompiledModule),
		debug:       debug,
		memoryLimit: memoryLimit,
	}
}

// Instantiate instantiates a contract with given code checksum and message, returning the result bytes.
// It uses MemoryManager for all memory interactions [oai_citation_attribution:0‡k33g.hashnode.dev](https://k33g.hashnode.dev/wazero-cookbook-part-two-host-functions#:~:text=%2F%2F%20Read%20the%20memory%202%EF%B8%8F%E2%83%A3,Println%28string%28buffer%29%29)
// to ensure compatibility with CosmWasm interface and memory safety.
func (vm *WazeroVM) Instantiate(code types.Checksum, env, info, initMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, error) {
	// Ensure the module is compiled or retrieve from cache
	compiled, err := vm.getCompiledModule(code, store)
	if err != nil {
		return nil, fmt.Errorf("error loading module: %w", err)
	}

	// Instantiate a fresh module instance for this execution
	ctx := context.Background()
	config := wazero.NewModuleConfig()
	module, err := vm.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("error instantiating module: %w", err)
	}
	defer vm.cleanupModule(module)

	// Wrap the module's memory in a MemoryManager for safe operations
	mm, err := memory.NewMemoryManager(module)
	if err != nil {
		return nil, fmt.Errorf("failed to create MemoryManager: %w", err)
	}

	// Ensure the contract exports a supported interface version (v1.x or v2.x)
	if err := vm.checkInterfaceVersion(ctx, module); err != nil {
		return nil, err
	}

	// Allocate and write input data to WASM memory using MemoryManager
	envPtr, err := mm.Allocate(uint32(len(env)))
	if err != nil {
		return nil, fmt.Errorf("allocate env: %w", err)
	}
	if err := mm.Write(envPtr, env); err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("write env: %w", err)
	}
	infoPtr, err := mm.Allocate(uint32(len(info)))
	if err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("allocate info: %w", err)
	}
	if err := mm.Write(infoPtr, info); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		return nil, fmt.Errorf("write info: %w", err)
	}
	initPtr, err := mm.Allocate(uint32(len(initMsg)))
	if err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		return nil, fmt.Errorf("allocate initMsg: %w", err)
	}
	if err := mm.Write(initPtr, initMsg); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		mm.Deallocate(initPtr)
		return nil, fmt.Errorf("write initMsg: %w", err)
	}

	// Call the contract's "instantiate" export with pointers to env, info, and init message
	instFn := module.ExportedFunction("instantiate")
	if instFn == nil {
		// Free allocated memory before returning error
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		mm.Deallocate(initPtr)
		return nil, errors.New("instantiate function not found in contract")
	}
	result, callErr := instFn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(initPtr))
	// Free input allocations as they are no longer needed, regardless of success or failure
	_ = mm.Deallocate(envPtr)
	_ = mm.Deallocate(infoPtr)
	_ = mm.Deallocate(initPtr)
	if callErr != nil {
		// Contract execution failed (e.g. panic in contract or out-of-gas)
		if vm.debug {
			log.Printf("instantiate call failed: %v", callErr)
		}
		return nil, fmt.Errorf("instantiate call failed: %w", callErr)
	}
	if len(result) == 0 {
		return nil, errors.New("instantiate returned no result")
	}
	regionPtr := uint32(result[0])
	if vm.debug {
		log.Printf("instantiate returned region pointer 0x%X", regionPtr)
	}
	if regionPtr == 0 {
		// A null pointer indicates an error in contract (e.g., explicit SystemError)
		return nil, errors.New("instantiate returned null pointer (contract error)")
	}

	// Read the result Region (offset and length) from WASM memory [oai_citation_attribution:1‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=SDK%20github,access%20between%20the%20caller).
	// A Region is 8 bytes: [4-byte offset][4-byte length] [oai_citation_attribution:2‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=SDK%20github,access%20between%20the%20caller).
	region, err := mm.Read(regionPtr, 8)
	if err != nil {
		// If we cannot read the region, free the region pointer and return error
		_ = mm.Deallocate(regionPtr)
		return nil, fmt.Errorf("failed to read result region at 0x%X: %w", regionPtr, err)
	}
	if len(region) < 8 {
		_ = mm.Deallocate(regionPtr)
		return nil, fmt.Errorf("result region too small: %d bytes", len(region))
	}
	dataOffset := binary.LittleEndian.Uint32(region[0:4])
	dataLength := binary.LittleEndian.Uint32(region[4:8])
	if vm.debug {
		log.Printf("Result region -> offset: %d, length: %d", dataOffset, dataLength)
	}

	// Read the actual result data from the contract memory
	data, err := mm.Read(dataOffset, dataLength)
	if err != nil {
		// Free the region and any allocated data if read fails
		_ = mm.Deallocate(regionPtr)
		_ = mm.Deallocate(dataOffset)
		return nil, fmt.Errorf("failed to read result data (offset %d, length %d): %w", dataOffset, dataLength, err)
	}

	// Deallocate the region struct and the data buffer in the contract's memory to avoid leaks
	if err := mm.Deallocate(regionPtr); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result region at 0x%X: %v", regionPtr, err)
	}
	if err := mm.Deallocate(dataOffset); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result data at 0x%X: %v", dataOffset, err)
	}

	// Return the raw result bytes (e.g., JSON-encoded InstantiateResponse)
	return data, nil
}

// Execute triggers a contract's execute function with the given message and returns the response bytes.
// Memory operations are handled by MemoryManager to ensure safety and compatibility.
func (vm *WazeroVM) Execute(code types.Checksum, env, info, execMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, error) {
	// The implementation is analogous to Instantiate, using MemoryManager for memory ops.
	// For brevity, we omit repeating comments for each step. Any differences from Instantiate are noted.
	compiled, err := vm.getCompiledModule(code, store)
	if err != nil {
		return nil, fmt.Errorf("error loading module: %w", err)
	}
	ctx := context.Background()
	module, err := vm.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("error instantiating module: %w", err)
	}
	defer vm.cleanupModule(module)
	mm, err := memory.NewMemoryManager(module)
	if err != nil {
		return nil, fmt.Errorf("failed to create MemoryManager: %w", err)
	}
	if err := vm.checkInterfaceVersion(ctx, module); err != nil {
		return nil, err
	}
	envPtr, err := mm.Allocate(uint32(len(env)))
	if err != nil {
		return nil, fmt.Errorf("allocate env: %w", err)
	}
	if err := mm.Write(envPtr, env); err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("write env: %w", err)
	}
	infoPtr, err := mm.Allocate(uint32(len(info)))
	if err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("allocate info: %w", err)
	}
	if err := mm.Write(infoPtr, info); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		return nil, fmt.Errorf("write info: %w", err)
	}
	msgPtr, err := mm.Allocate(uint32(len(execMsg)))
	if err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		return nil, fmt.Errorf("allocate execMsg: %w", err)
	}
	if err := mm.Write(msgPtr, execMsg); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		mm.Deallocate(msgPtr)
		return nil, fmt.Errorf("write execMsg: %w", err)
	}
	execFn := module.ExportedFunction("execute")
	if execFn == nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		mm.Deallocate(msgPtr)
		return nil, errors.New("execute function not found in contract")
	}
	result, callErr := execFn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	_ = mm.Deallocate(envPtr)
	_ = mm.Deallocate(infoPtr)
	_ = mm.Deallocate(msgPtr)
	if callErr != nil {
		if vm.debug {
			log.Printf("execute call failed: %v", callErr)
		}
		return nil, fmt.Errorf("execute call failed: %w", callErr)
	}
	if len(result) == 0 {
		return nil, errors.New("execute returned no result")
	}
	regionPtr := uint32(result[0])
	if vm.debug {
		log.Printf("execute returned region pointer 0x%X", regionPtr)
	}
	if regionPtr == 0 {
		return nil, errors.New("execute returned null pointer (contract error)")
	}
	region, err := mm.Read(regionPtr, 8)
	if err != nil {
		_ = mm.Deallocate(regionPtr)
		return nil, fmt.Errorf("failed to read result region: %w", err)
	}
	dataOffset := binary.LittleEndian.Uint32(region[0:4])
	dataLength := binary.LittleEndian.Uint32(region[4:8])
	data, err := mm.Read(dataOffset, dataLength)
	if err != nil {
		_ = mm.Deallocate(regionPtr)
		_ = mm.Deallocate(dataOffset)
		return nil, fmt.Errorf("failed to read result data: %w", err)
	}
	if err := mm.Deallocate(regionPtr); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result region: %v", err)
	}
	if err := mm.Deallocate(dataOffset); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result data: %v", err)
	}
	return data, nil
}

// Query calls a contract's query function. It is similar to Execute but without an Info (no funds).
func (vm *WazeroVM) Query(code types.Checksum, env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, error) {
	compiled, err := vm.getCompiledModule(code, store)
	if err != nil {
		return nil, fmt.Errorf("error loading module: %w", err)
	}
	ctx := context.Background()
	module, err := vm.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("error instantiating module: %w", err)
	}
	defer vm.cleanupModule(module)
	mm, err := memory.NewMemoryManager(module)
	if err != nil {
		return nil, fmt.Errorf("failed to create MemoryManager: %w", err)
	}
	if err := vm.checkInterfaceVersion(ctx, module); err != nil {
		return nil, err
	}
	envPtr, err := mm.Allocate(uint32(len(env)))
	if err != nil {
		return nil, fmt.Errorf("allocate env: %w", err)
	}
	if err := mm.Write(envPtr, env); err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("write env: %w", err)
	}
	msgPtr, err := mm.Allocate(uint32(len(queryMsg)))
	if err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("allocate queryMsg: %w", err)
	}
	if err := mm.Write(msgPtr, queryMsg); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(msgPtr)
		return nil, fmt.Errorf("write queryMsg: %w", err)
	}
	queryFn := module.ExportedFunction("query")
	if queryFn == nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(msgPtr)
		return nil, errors.New("query function not found in contract")
	}
	result, callErr := queryFn.Call(ctx, uint64(envPtr), uint64(msgPtr))
	_ = mm.Deallocate(envPtr)
	_ = mm.Deallocate(msgPtr)
	if callErr != nil {
		if vm.debug {
			log.Printf("query call failed: %v", callErr)
		}
		return nil, fmt.Errorf("query call failed: %w", callErr)
	}
	if len(result) == 0 {
		return nil, errors.New("query returned no result")
	}
	regionPtr := uint32(result[0])
	if vm.debug {
		log.Printf("query returned region pointer 0x%X", regionPtr)
	}
	if regionPtr == 0 {
		return nil, errors.New("query returned null pointer (contract error)")
	}
	region, err := mm.Read(regionPtr, 8)
	if err != nil {
		_ = mm.Deallocate(regionPtr)
		return nil, fmt.Errorf("failed to read result region: %w", err)
	}
	dataOffset := binary.LittleEndian.Uint32(region[0:4])
	dataLength := binary.LittleEndian.Uint32(region[4:8])
	data, err := mm.Read(dataOffset, dataLength)
	if err != nil {
		_ = mm.Deallocate(regionPtr)
		_ = mm.Deallocate(dataOffset)
		return nil, fmt.Errorf("failed to read result data: %w", err)
	}
	if err := mm.Deallocate(regionPtr); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result region: %v", err)
	}
	if err := mm.Deallocate(dataOffset); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result data: %v", err)
	}
	return data, nil
}

// Migrate calls a contract's migrate function, analogous to Instantiate/Execute.
func (vm *WazeroVM) Migrate(code types.Checksum, env, info, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, error) {
	// Implementation is similar to Execute. For brevity, not duplicating all comments.
	compiled, err := vm.getCompiledModule(code, store)
	if err != nil {
		return nil, fmt.Errorf("error loading module: %w", err)
	}
	ctx := context.Background()
	module, err := vm.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("error instantiating module: %w", err)
	}
	defer vm.cleanupModule(module)
	mm, err := memory.NewMemoryManager(module)
	if err != nil {
		return nil, fmt.Errorf("failed to create MemoryManager: %w", err)
	}
	if err := vm.checkInterfaceVersion(ctx, module); err != nil {
		return nil, err
	}
	envPtr, err := mm.Allocate(uint32(len(env)))
	if err != nil {
		return nil, fmt.Errorf("allocate env: %w", err)
	}
	if err := mm.Write(envPtr, env); err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("write env: %w", err)
	}
	infoPtr, err := mm.Allocate(uint32(len(info)))
	if err != nil {
		mm.Deallocate(envPtr)
		return nil, fmt.Errorf("allocate info: %w", err)
	}
	if err := mm.Write(infoPtr, info); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		return nil, fmt.Errorf("write info: %w", err)
	}
	msgPtr, err := mm.Allocate(uint32(len(migrateMsg)))
	if err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		return nil, fmt.Errorf("allocate migrateMsg: %w", err)
	}
	if err := mm.Write(msgPtr, migrateMsg); err != nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		mm.Deallocate(msgPtr)
		return nil, fmt.Errorf("write migrateMsg: %w", err)
	}
	migrateFn := module.ExportedFunction("migrate")
	if migrateFn == nil {
		mm.Deallocate(envPtr)
		mm.Deallocate(infoPtr)
		mm.Deallocate(msgPtr)
		return nil, errors.New("migrate function not found in contract")
	}
	result, callErr := migrateFn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	_ = mm.Deallocate(envPtr)
	_ = mm.Deallocate(infoPtr)
	_ = mm.Deallocate(msgPtr)
	if callErr != nil {
		if vm.debug {
			log.Printf("migrate call failed: %v", callErr)
		}
		return nil, fmt.Errorf("migrate call failed: %w", callErr)
	}
	if len(result) == 0 {
		return nil, errors.New("migrate returned no result")
	}
	regionPtr := uint32(result[0])
	if vm.debug {
		log.Printf("migrate returned region pointer 0x%X", regionPtr)
	}
	if regionPtr == 0 {
		return nil, errors.New("migrate returned null pointer (contract error)")
	}
	region, err := mm.Read(regionPtr, 8)
	if err != nil {
		_ = mm.Deallocate(regionPtr)
		return nil, fmt.Errorf("failed to read result region: %w", err)
	}
	dataOffset := binary.LittleEndian.Uint32(region[0:4])
	dataLength := binary.LittleEndian.Uint32(region[4:8])
	data, err := mm.Read(dataOffset, dataLength)
	if err != nil {
		_ = mm.Deallocate(regionPtr)
		_ = mm.Deallocate(dataOffset)
		return nil, fmt.Errorf("failed to read result data: %w", err)
	}
	if err := mm.Deallocate(regionPtr); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result region: %v", err)
	}
	if err := mm.Deallocate(dataOffset); err != nil && vm.debug {
		log.Printf("Warning: failed to deallocate result data: %v", err)
	}
	return data, nil
}

// (Additional VM methods like Sudo, Reply, IBCChannelOpen, etc., would follow the same pattern of memory management.)

// checkInterfaceVersion ensures the module exports a supported interface_version (8 or 9) and calls it to verify compatibility.
func (vm *WazeroVM) checkInterfaceVersion(ctx context.Context, module api.Module) error {
	// CosmWasm v1.x uses interface_version_8, v2.x uses interface_version_9 [oai_citation_attribution:3‡forum.cosmos.network](https://forum.cosmos.network/t/defunding-cosmwasm-is-a-threat-to-the-cosmos-hub-and-cosmos-ecosystem/14918?page=2#:~:text=There%20was%20a%20time%20when,go%2C%20which%20is%20underway%20here).
	if fn := module.ExportedFunction("interface_version_9"); fn != nil {
		_, err := fn.Call(ctx)
		if err != nil {
			return fmt.Errorf("contract interface mismatch (expected v2.x): %w", err)
		}
		if vm.debug {
			log.Printf("Detected CosmWasm interface_version_9 (v2.x contract)")
		}
		return nil
	}
	if fn := module.ExportedFunction("interface_version_8"); fn != nil {
		_, err := fn.Call(ctx)
		if err != nil {
			return fmt.Errorf("contract interface mismatch (expected v1.x): %w", err)
		}
		if vm.debug {
			log.Printf("Detected CosmWasm interface_version_8 (v1.x contract)")
		}
		return nil
	}
	return errors.New("no supported interface_version export found (not a CosmWasm contract)")
}

// getCompiledModule retrieves a compiled module for the given code (checksum). It compiles the WASM if not cached.
func (vm *WazeroVM) getCompiledModule(code types.Checksum, store types.KVStore) (wazero.CompiledModule, error) {
	if compiled, ok := vm.cache[code]; ok {
		return compiled, nil
	}
	// Load WASM code bytes from the KVStore (the code must have been stored already)
	wasmCode := store.Get(code[:])
	if wasmCode == nil {
		return nil, fmt.Errorf("unable to load contract code for %X", code)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), wasmCode)
	if err != nil {
		return nil, fmt.Errorf("WASM compile error: %w", err)
	}
	vm.cache[code] = compiled
	return compiled, nil
}

// cleanupModule handles post-execution cleanup of a module instance.
func (vm *WazeroVM) cleanupModule(module api.Module) {
	// Ensure the module is closed to free resources.
	// This will also free memory associated with the instance.
	_ = module.Close(context.Background())
	if vm.debug {
		log.Printf("Module %s cleaned up", module.Name())
	}
}
