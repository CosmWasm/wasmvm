package runtime

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	// Import the Wasm runtime package (e.g., wasmer-go or similar) and CosmWasm types
	// Assuming a Wasm runtime interface is available for loading modules and calling exports
	// example import for WASM engine
	// Also assume we have access to types.Env, types.MessageInfo, types.Response, etc.
	"github.com/CosmWasm/wasmvm/v2/types" // example import path for CosmWasm Go types (Env, MessageInfo, Response, etc.)
)

// LegacyEnv is used for compatibility with CosmWasm 0.x contracts that use `init` entry point.
// It combines Env and MessageInfo into a single structure as older contracts expect.
type LegacyEnv struct {
	Block    types.BlockInfo    `json:"block"`
	Contract types.ContractInfo `json:"contract"`
	Message  LegacyMessageInfo  `json:"message"`
}

// LegacyMessageInfo holds sender and sent funds for legacy `init` calls [oai_citation_attribution:0‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20copy%20memory%20to%2Ffrom%20host%2C,fn%20deallocate%28pointer%3A%20u32).
type LegacyMessageInfo struct {
	Sender    string       `json:"sender"`
	SentFunds []types.Coin `json:"sent_funds"`
}

// Instantiate will create a new contract instance (constructor), matching CosmWasm behavior for both 1.x and 2.x.
func (vm *VM) Instantiate(codeID CodeID, env types.Env, info types.MessageInfo, initMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.Response, uint64, error) {
	// Debug: start instantiation
	fmt.Printf("Instantiating contract code %X with gas limit %d\n", codeID, gasLimit)

	// Marshal environment and info to JSON (as required by CosmWasm VM input) [oai_citation_attribution:1‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=envBin%2C%20err%20%3A%3D%20json) [oai_citation_attribution:2‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=infoBin%2C%20err%20%3A%3D%20json).
	envBytes, err := json.Marshal(env)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to serialize Env: %w", err)
	}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to serialize MessageInfo: %w", err)
	}
	// Debug: show JSON sizes
	fmt.Printf("Serialized Env (%d bytes), MessageInfo (%d bytes), InitMsg (%d bytes)\n", len(envBytes), len(infoBytes), len(initMsg))

	// Load the WASM module for the given codeID (from cache or persistent storage).
	module, err := vm.getModuleForCode(codeID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load module: %w", err)
	}

	// Set up the Wasm instance with required imports (storage, API, querier, gas metering).
	importObj := vm.buildImportObject(store, api, querier, gasMeter, gasLimit)
	instance, err := module.Instantiate(importObj)
	if err != nil {
		// If instantiation fails (e.g., memory allocation issues or incompatible module), return error.
		return nil, 0, fmt.Errorf("failed to instantiate Wasm module: %w", err)
	}
	defer instance.Close() // ensure instance is cleaned up

	// Check for required interface version exports (ensures contract compatibility) [oai_citation_attribution:3‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20signal%20for%201,%28%29).
	// CosmWasm 1.0+ contracts export `interface_version_8`. CosmWasm 2.x may export `requires_cosmwasm_2_0` if built with that feature.
	if instance.Exports.Exists("interface_version_8") {
		fmt.Println("Contract reports interface_version_8 (CosmWasm 1.x)")
	}
	if instance.Exports.Exists("requires_cosmwasm_2_0") {
		fmt.Println("Contract requires CosmWasm 2.0 compatibility")
		// If our VM does not support CosmWasm 2.0 features, we should error. (Assume vm.supports2x indicates support)
		if !vm.supports2x {
			return nil, 0, fmt.Errorf("contract requires CosmWasm 2.x features not supported by VM")
		}
	}

	// Determine entry function: prefer "instantiate", fallback to "init" for older contracts [oai_citation_attribution:4‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20creates%20an%20initial%20state,u32).
	funcName := "instantiate"
	if !instance.Exports.Exists(funcName) {
		if instance.Exports.Exists("init") {
			funcName = "init"
			// If using legacy "init", combine env and info into LegacyEnv structure
			legacyEnv := LegacyEnv{
				Block:    env.Block,
				Contract: env.Contract,
				Message: LegacyMessageInfo{
					Sender:    info.Sender,
					SentFunds: info.Funds,
				},
			}
			envBytes, err = json.Marshal(legacyEnv)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to serialize LegacyEnv: %w", err)
			}
			// Recalculate, legacy env now holds what was separate info
			infoBytes = nil // not used in call
			fmt.Printf("Legacy Env serialized (%d bytes) for init call\n", len(envBytes))
		} else {
			instance.Close()
			return nil, 0, fmt.Errorf("Wasm contract does not have an instantiate or init entry point")
		}
	}
	// Fetch the exported functions for allocate, deallocate, and instantiate/init.
	allocateFn, allocErr := instance.Exports.GetFunction("allocate")
	deallocateFn, deallocErr := instance.Exports.GetFunction("deallocate")
	entryFn, entryErr := instance.Exports.GetFunction(funcName)
	if allocErr != nil || deallocErr != nil || entryErr != nil {
		instance.Close()
		return nil, 0, fmt.Errorf("missing required exports (allocate/deallocate or %s): %v %v %v", funcName, allocErr, deallocErr, entryErr)
	}

	// Helper to allocate and write data into Wasm memory, returning the region pointer.
	writeToWasm := func(data []byte) (uint32, error) {
		// Request the contract to allocate a region of the needed size [oai_citation_attribution:5‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=%2F%2F%2F%20allocate%20reserves%20the%20given,memory%20and%20returns%20a%20pointer) [oai_citation_attribution:6‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=extern%20,u32).
		res, err := allocateFn(len(data))
		if err != nil {
			return 0, fmt.Errorf("allocate failed: %w", err)
		}
		// The allocate function returns an Offset (u32 pointer) to a Region in memory [oai_citation_attribution:7‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20copy%20memory%20to%2Ffrom%20host%2C,fn%20deallocate%28pointer%3A%20u32).
		regionPtr := res.(uint32)
		if regionPtr == 0 {
			return 0, fmt.Errorf("allocate returned 0 (out of memory)")
		}
		// Access the instance's linear memory to fill the allocated region.
		memory, memErr := instance.Exports.GetMemory("memory")
		if memErr != nil {
			return 0, fmt.Errorf("unable to access memory: %w", memErr)
		}
		memData := memory.Data() // []byte of linear memory
		// Read the Region struct (12 bytes) at regionPtr [oai_citation_attribution:8‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/memory.rs#:~:text=pub%20offset%3A%20u32%2C).
		if regionPtr+12 > uint32(len(memData)) {
			return 0, fmt.Errorf("region pointer out of memory bounds")
		}
		offset := binary.LittleEndian.Uint32(memData[regionPtr : regionPtr+4])
		capacity := binary.LittleEndian.Uint32(memData[regionPtr+4 : regionPtr+8])
		// Write data into the allocated region (up to 'capacity' bytes).
		if capacity < uint32(len(data)) {
			return 0, fmt.Errorf("allocated region too small: capacity %d, required %d", capacity, len(data))
		}
		copy(memData[offset:offset+uint32(len(data))], data)
		// Set the region's length field to the actual data length [oai_citation_attribution:9‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/memory.rs#:~:text=let%20vector%20%3D%20unsafe%20) [oai_citation_attribution:10‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/memory.rs#:~:text=self).
		binary.LittleEndian.PutUint32(memData[regionPtr+8:regionPtr+12], uint32(len(data)))
		// Debug: log the allocated region info
		fmt.Printf("Allocated %d bytes in wasm memory at offset %d (region ptr 0x%X)\n", len(data), offset, regionPtr)
		return regionPtr, nil
	}

	// Allocate and write env, info (if used), and initMsg into the contract's memory.
	envPtr, err := writeToWasm(envBytes)
	if err != nil {
		instance.Close()
		return nil, 0, fmt.Errorf("writing Env to wasm memory failed: %w", err)
	}
	var infoPtr uint32
	if infoBytes != nil {
		infoPtr, err = writeToWasm(infoBytes)
		if err != nil {
			// Clean up allocated env
			_, _ = deallocateFn(envPtr)
			instance.Close()
			return nil, 0, fmt.Errorf("writing Info to wasm memory failed: %w", err)
		}
	}
	msgPtr, err := writeToWasm(initMsg)
	if err != nil {
		// Clean up allocated env and info
		_, _ = deallocateFn(envPtr)
		if infoBytes != nil {
			_, _ = deallocateFn(infoPtr)
		}
		instance.Close()
		return nil, 0, fmt.Errorf("writing InitMsg to wasm memory failed: %w", err)
	}

	// Call the contract's instantiate (or init) function with the appropriate pointers [oai_citation_attribution:11‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20creates%20an%20initial%20state,u32).
	var ret interface{}
	if funcName == "init" {
		// Legacy init takes 2 parameters: env_ptr and msg_ptr (no separate info) [oai_citation_attribution:12‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20creates%20an%20initial%20state,u32).
		ret, err = entryFn(envPtr, msgPtr)
	} else {
		// Standard instantiate takes 3 parameters: env_ptr, info_ptr, msg_ptr [oai_citation_attribution:13‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20creates%20an%20initial%20state,u32).
		ret, err = entryFn(envPtr, infoPtr, msgPtr)
	}

	// Immediately free the input Regions (we manage their lifecycle manually) [oai_citation_attribution:14‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=%2F%2F%2F%20deallocate%20expects%20a%20pointer,a%20Region%20created%20with%20allocate) [oai_citation_attribution:15‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=extern%20,).
	_, _ = deallocateFn(envPtr)
	if infoBytes != nil && infoPtr != 0 {
		_, _ = deallocateFn(infoPtr)
	}
	_, _ = deallocateFn(msgPtr)

	if err != nil {
		// The contract execution failed (possibly out-of-gas or runtime error).
		// Ensure to consume all gas that was used before the error.
		used := gasMeter.GasConsumed() // gas consumed so far
		return nil, used, fmt.Errorf("contract instantiate failed: %w", err)
	}

	// The contract returned a pointer to a Region containing the result (ContractResult<Response>) [oai_citation_attribution:16‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=%2F%2F%20creates%20an%20initial%20state,u32).
	resultPtr := ret.(uint32)
	if resultPtr == 0 {
		// A null pointer indicates an immediate failure (should not happen as errors are returned as Region).
		used := gasMeter.GasConsumed()
		return nil, used, fmt.Errorf("contract returned null response")
	}

	// Read the response Region from memory to get the result data.
	memory, _ := instance.Exports.GetMemory("memory")
	memData := memory.Data()
	if resultPtr+12 > uint32(len(memData)) {
		used := gasMeter.GasConsumed()
		return nil, used, fmt.Errorf("response region pointer out of bounds")
	}
	resultOffset := binary.LittleEndian.Uint32(memData[resultPtr : resultPtr+4])
	resultLength := binary.LittleEndian.Uint32(memData[resultPtr+8 : resultPtr+12])
	if resultOffset+resultLength > uint32(len(memData)) {
		used := gasMeter.GasConsumed()
		return nil, used, fmt.Errorf("response data out of memory bounds")
	}
	responseData := make([]byte, resultLength)
	copy(responseData, memData[resultOffset:resultOffset+resultLength])
	// Free the response region now that we have copied it [oai_citation_attribution:17‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=%2F%2F%2F%20deallocate%20expects%20a%20pointer,a%20Region%20created%20with%20allocate) [oai_citation_attribution:18‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=extern%20,).
	_, _ = deallocateFn(resultPtr)

	// Debug: log the raw response data size
	fmt.Printf("Contract instantiate returned %d bytes\n", resultLength)

	// Deserialize the response data into a Response struct (or error message) [oai_citation_attribution:19‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=gasForDeserialization%20%3A%3D%20deserCost) [oai_citation_attribution:20‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=err%20%3A%3D%20json).
	var contractResult types.ContractResult // assume this can hold success or error
	// Charge gas for deserializing the result data, to match CosmWasm gas accounting [oai_citation_attribution:21‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=gasForDeserialization%20%3A%3D%20deserCost).
	gasForDeserialization := vm.deserializationCost.Multiply(uint64(len(responseData))).Ceil() // Ceil or Floor depending on policy
	// Ensure we have enough remaining gas for deserialization
	if gasMeter.GasConsumed()+gasForDeserialization > gasLimit {
		used := gasMeter.GasConsumed()
		return nil, used, fmt.Errorf("insufficient gas for result deserialization (result size %d bytes)", len(responseData))
	}
	gasMeter.ConsumeGas(gasForDeserialization, "CosmWasm: deserialize result") // deduct the gas
	// Now perform the JSON deserialization
	if err := json.Unmarshal(responseData, &contractResult); err != nil {
		used := gasMeter.GasConsumed()
		return nil, used, fmt.Errorf("failed to decode contract response: %w", err)
	}

	// If the contractResult is an error, return it as an error.
	if contractResult.Err != "" {
		// The contract explicitly returned an error message
		used := gasMeter.GasConsumed()
		return nil, used, fmt.Errorf("contract error: %s", contractResult.Err)
	}

	// For a successful instantiate, extract the Response (with data, attributes, messages, etc.).
	res := contractResult.Ok // assuming Ok field holds *types.Response

	// Safety check: ensure any SubMessages in the response have reasonable payload size [oai_citation_attribution:22‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=const%20ReplyPayloadMaxBytes%20%3D%20128%20,1024%20%2F%2F%20128%20KiB).
	const ReplyPayloadMaxBytes = 128 * 1024 // 128 KiB max payload size for SubMsg reply
	for i, sub := range res.SubMessages {
		if len(sub.Msg.Payload) > ReplyPayloadMaxBytes {
			used := gasMeter.GasConsumed()
			return nil, used, fmt.Errorf("submessage %d payload size %d exceeds limit %d bytes", i, len(sub.Msg.Payload), ReplyPayloadMaxBytes)
		}
	}

	// Determine total gas used. CosmWasm gas includes VM execution plus our added deserialization cost.
	usedGas := gasMeter.GasConsumed()
	fmt.Printf("Instantiate complete. Gas used: %d\n", usedGas)
	return res, usedGas, nil
}
