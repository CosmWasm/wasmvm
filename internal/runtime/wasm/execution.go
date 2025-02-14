package wasm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	// Assume types package defines Env, MessageInfo, QueryRequest, Reply, etc.
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

// Instantiate compiles (if needed) and instantiates a contract, calling its "instantiate" method.
func (vm *WazeroVM) Instantiate(checksum Checksum, env types.Env, info types.MessageInfo, initMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	// Marshal env and info to JSON (as the contract expects JSON input) [oai_citation_attribution:14‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=func%20%28vm%20) [oai_citation_attribution:15‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=infoBin%2C%20err%20%3A%3D%20json).
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal Env: %w", err)
	}
	infoBz, err := json.Marshal(info)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal MessageInfo: %w", err)
	}
	// Execute the contract call
	resBz, gasUsed, execErr := vm.callContract(checksum, "instantiate", envBz, infoBz, initMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		// If an error occurred in execution, return the error with gas used so far [oai_citation_attribution:16‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=data%2C%20gasReport%2C%20err%20%3A%3D%20api,printDebug).
		return nil, gasUsed, execErr
	}
	// Deserialize the contract's response (JSON) into a ContractResult struct [oai_citation_attribution:17‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=var%20result%20types).
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize instantiate result: %w", err)
	}
	return &result, gasUsed, nil
}

// Execute calls a contract's "execute" entry point with the given message.
func (vm *WazeroVM) Execute(checksum Checksum, env types.Env, info types.MessageInfo, execMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBz, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "execute", envBz, infoBz, execMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize execute result: %w", err)
	}
	return &result, gasUsed, nil
}

// Query calls a contract's "query" entry point. Query has no MessageInfo (no funds or sender).
func (vm *WazeroVM) Query(checksum Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	// For queries, no info, so we pass only env and msg.
	resBz, gasUsed, execErr := vm.callContract(checksum, "query", envBz, nil, queryMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize query result: %w", err)
	}
	return &result, gasUsed, nil
}

// Migrate calls a contract's "migrate" entry point with given migrate message.
func (vm *WazeroVM) Migrate(checksum Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "migrate", envBz, nil, migrateMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize migrate result: %w", err)
	}
	return &result, gasUsed, nil
}

// Sudo calls the contract's "sudo" entry point (privileged call from the chain).
func (vm *WazeroVM) Sudo(checksum Checksum, env types.Env, sudoMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "sudo", envBz, nil, sudoMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize sudo result: %w", err)
	}
	return &result, gasUsed, nil
}

// Reply calls the contract's "reply" entry point to handle a SubMsg reply.
func (vm *WazeroVM) Reply(checksum Checksum, env types.Env, reply types.Reply, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	replyBz, err := json.Marshal(reply)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "reply", envBz, nil, replyBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize reply result: %w", err)
	}
	return &result, gasUsed, nil
}

// callContract is an internal helper to instantiate the Wasm module and call a specified entry point.
func (vm *WazeroVM) callContract(checksum Checksum, entrypoint string, env []byte, info []byte, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, uint64, error) {
	ctx := context.Background()
	// Attach the execution context (store, api, querier, gasMeter) to ctx for host functions.
	instCtx := instanceContext{store: store, api: api, querier: querier, gasMeter: gasMeter, gasLimit: gasLimit}
	ctx = context.WithValue(ctx, instanceContextKey{}, &instCtx)

	// Ensure we have a compiled module for this code (maybe from cache) [oai_citation_attribution:18‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=Contract%20pinning%20is%20a%20feature,33x%20faster).
	codeHash := [32]byte{}
	copy(codeHash[:], checksum) // convert to array key
	compiled, err := vm.getCompiledModule(codeHash)
	if err != nil {
		return nil, 0, fmt.Errorf("loading module: %w", err)
	}
	// Instantiate a new module instance for this execution.
	module, err := vm.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, 0, fmt.Errorf("instantiating module: %w", err)
	}
	defer module.Close(ctx) // ensure instance is closed after execution

	// Allocate and write input data (env, info, msg) into the module's memory.
	mem := module.Memory()
	// Helper to allocate a region and copy data into it, returning the Region pointer.
	allocData := func(data []byte) (uint32, error) {
		if data == nil {
			return 0, nil
		}
		allocFn := module.ExportedFunction("allocate")
		if allocFn == nil {
			return 0, fmt.Errorf("allocate function not found in module")
		}
		// Request a region for data
		allocRes, err := allocFn.Call(ctx, uint64(len(data)))
		if err != nil || len(allocRes) == 0 {
			return 0, fmt.Errorf("allocate failed: %v", err)
		}
		regionPtr := uint32(allocRes[0])
		// The Region struct is stored at regionPtr [oai_citation_attribution:19‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=). It contains a pointer to allocated memory.
		// Read the offset of the allocated buffer from the Region (first 4 bytes).
		offset, ok := mem.ReadUint32Le(regionPtr)
		if !ok {
			return 0, fmt.Errorf("failed to read allocated region offset")
		}
		// Write the data into the allocated buffer.
		if !mem.Write(uint32(offset), data) {
			return 0, fmt.Errorf("failed to write data into wasm memory")
		}
		// Set the region's length field (third 4 bytes of Region struct) to data length.
		if !mem.WriteUint32Le(regionPtr+8, uint32(len(data))) {
			return 0, fmt.Errorf("failed to write region length")
		}
		return regionPtr, nil
	}
	envPtr, err := allocData(env)
	if err != nil {
		return nil, 0, err
	}
	infoPtr, err := allocData(info)
	if err != nil {
		return nil, 0, err
	}
	msgPtr, err := allocData(msg)
	if err != nil {
		return nil, 0, err
	}

	// Call the contract's entrypoint function.
	fn := module.ExportedFunction(entrypoint)
	if fn == nil {
		return nil, 0, fmt.Errorf("entry point %q not found in contract", entrypoint)
	}
	// Prepare arguments as (env_ptr, info_ptr, msg_ptr) or (env_ptr, msg_ptr) depending on entrypoint [oai_citation_attribution:20‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=,to%20extend%20their%20functionality) [oai_citation_attribution:21‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=extern%20,u32).
	args := []uint64{uint64(envPtr)}
	if info != nil {
		args = append(args, uint64(infoPtr))
	}
	args = append(args, uint64(msgPtr))
	// Execute the contract function. This will trigger host function calls (db_read, etc.) as needed.
	results, err := fn.Call(ctx, args...)
	// Compute gas used internally by subtracting remaining gas from gasLimit.
	gasUsed := gasLimit
	if instCtx.gasMeter != nil {
		// Use GasConsumed difference (querier gas usage accounted separately).
		gasUsed = instCtx.gasMeter.GasConsumed()
	}
	if err != nil {
		// If the execution trapped (e.g., out of gas or contract panic), determine error.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Context cancellation (treat as out of gas for consistency).
			return nil, gasUsed, OutOfGasError{}
		}
		// Wazero traps on out-of-gas would manifest as a panic/exit error [oai_citation_attribution:22‡github.com](https://github.com/tetratelabs/wazero/blob/main/RATIONALE.md#:~:text=Currently%2C%20the%20only%20portable%20way,code%20isn%27t%20executed%20after%20it). We assume any runtime error means out of gas if gas is exhausted.
		if gasUsed >= gasLimit {
			return nil, gasUsed, OutOfGasError{}
		}
		// Otherwise, return the error as a generic VM error.
		return nil, gasUsed, fmt.Errorf("contract execution error: %w", err)
	}
	// The contract returns a pointer to a Region with the result data (or 0 if no data) [oai_citation_attribution:23‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=extern%20,u32).
	var data []byte
	if len(results) > 0 {
		resultPtr := uint32(results[0])
		if resultPtr != 0 {
			// Read region pointer for result
			resOffset, ok := mem.ReadUint32Le(resultPtr)
			resLength, ok2 := mem.ReadUint32Le(resultPtr + 8)
			if ok && ok2 {
				data, _ = mem.Read(resOffset, resLength)
			}
		}
	}
	// We do not explicitly call deallocate for result region, as the whole module instance will be closed and memory freed.
	return data, gasUsed, nil
}

// getCompiledModule returns a compiled module for the given checksum, compiling or retrieving from cache as needed.
func (vm *WazeroVM) getCompiledModule(codeHash [32]byte) (wazero.CompiledModule, error) {
	// Fast path: check caches under read lock.
	vm.cacheMu.RLock()
	if item, ok := vm.pinned[codeHash]; ok {
		vm.hitsPinned++ // pinned cache hit
		item.hits++
		compiled := item.compiled
		vm.cacheMu.RUnlock()
		vm.logger.Debug().Str("checksum", hex.EncodeToString(codeHash[:])).Msg("Using pinned contract module from cache")
		return compiled, nil
	}
	if item, ok := vm.memoryCache[codeHash]; ok {
		vm.hitsMemory++ // LRU cache hit
		item.hits++
		// Move this item to most-recently-used position in LRU order
		// (We'll do simple reorder: remove and append at end).
		// Find and remove from cacheOrder slice:
		for i, hash := range vm.cacheOrder {
			if hash == codeHash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.cacheOrder = append(vm.cacheOrder, codeHash)
		compiled := item.compiled
		vm.cacheMu.RUnlock()
		vm.logger.Debug().Str("checksum", hex.EncodeToString(codeHash[:])).Msg("Using cached module from LRU cache")
		return compiled, nil
	}
	vm.cacheMu.RUnlock()

	// Cache miss: compile the module.
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// Double-check if another goroutine compiled it while we were waiting.
	if item, ok := vm.pinned[codeHash]; ok {
		vm.hitsPinned++
		item.hits++
		return item.compiled, nil
	}
	if item, ok := vm.memoryCache[codeHash]; ok {
		vm.hitsMemory++
		item.hits++
		// promote in LRU order
		for i, hash := range vm.cacheOrder {
			if hash == codeHash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.cacheOrder = append(vm.cacheOrder, codeHash)
		return item.compiled, nil
	}
	// Not in any cache yet: compile the Wasm code.
	code, ok := vm.codeStore[codeHash]
	if !ok {
		vm.logger.Error().Msg("Wasm code bytes not found for checksum")
		return nil, fmt.Errorf("code %x not found", codeHash)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}
	vm.misses++ // cache miss (compiled new module)
	// Add to memory cache (un-pinned by default). Evict LRU if over capacity.
	size := uint64(len(code))
	vm.memoryCache[codeHash] = &cacheItem{compiled: compiled, size: size, hits: 0}
	vm.cacheOrder = append(vm.cacheOrder, codeHash)
	if len(vm.memoryCache) > vm.cacheSize {
		// evict least recently used (front of cacheOrder)
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background()) // free the compiled module
			delete(vm.memoryCache, oldest)
			vm.logger.Debug().Str("checksum", hex.EncodeToString(oldest[:])).Msg("Evicted module from cache (LRU)")
		}
	}
	vm.logger.Info().
		Str("checksum", hex.EncodeToString(codeHash[:])).
		Uint64("size_bytes", size).
		Msg("Compiled new contract module and cached")
	return compiled, nil
}

// instanceContext carries environment references for host functions.
type instanceContext struct {
	store    types.KVStore
	api      types.GoAPI
	querier  types.Querier
	gasMeter types.GasMeter
	gasLimit uint64
}

// instanceContextKey is used as context key for instanceContext.
type instanceContextKey struct{}

// Helper to retrieve instanceContext from a context.
func getInstanceContext(ctx context.Context) *instanceContext {
	val := ctx.Value(instanceContextKey{})
	if val == nil {
		return nil
	}
	return val.(*instanceContext)
}
