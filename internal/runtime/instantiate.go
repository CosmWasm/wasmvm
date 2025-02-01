package runtime

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/tetratelabs/wazero"

	"github.com/CosmWasm/wasmvm/v2/types"
)

func (w *WazeroRuntime) Instantiate(
	checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{},
) ([]byte, types.GasReport, error) {
	fmt.Println("\n=== Contract Instantiation Start ===")
	fmt.Printf("Checksum: %x\n", checksum)
	fmt.Printf("Input sizes - env: %d, info: %d, msg: %d\n", len(env), len(info), len(msg))
	fmt.Printf("Message content: %s\n", string(msg))

	// Parse extra parameters (gas meter, store, API, etc.)
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to parse params: %w", err)
	}
	gasState := NewGasState(gasLimit)
	fmt.Printf("Gas state initialized with limit: %d\n", gasLimit)

	runtimeEnv := &RuntimeEnvironment{
		DB:         store,
		API:        *api,
		Querier:    *querier,
		Gas:        *gasMeter,
		gasLimit:   gasLimit,
		gasUsed:    0,
		iterators:  make(map[uint64]map[uint64]types.Iterator),
		nextCallID: 1,
	}
	ctx := context.WithValue(context.Background(), envKey, runtimeEnv)

	// Retrieve the compiled module.
	w.mu.Lock()
	module, ok := w.compiledModules[hex.EncodeToString(checksum)]
	w.mu.Unlock()
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("module not found for checksum: %x", checksum)
	}

	// Register host functions and log the registration.
	fmt.Println("\n=== Registering Host Functions ===")
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer func() {
		if closeErr := hostModule.Close(ctx); closeErr != nil {
			fmt.Printf("WARNING: Failed to close host module: %v\n", closeErr)
		}
	}()

	// Instantiate the host module as "env".
	fmt.Println("\n=== Instantiating Environment Module ===")
	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, wazero.NewModuleConfig().WithName("env"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer func() {
		if closeErr := envModule.Close(ctx); closeErr != nil {
			fmt.Printf("WARNING: Failed to close env module: %v\n", closeErr)
		}
	}()

	// Instantiate the contract module.
	fmt.Println("\n=== Instantiating Contract Module ===")
	contractModule, err := w.runtime.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithName("contract"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer func() {
		if closeErr := contractModule.Close(ctx); closeErr != nil {
			fmt.Printf("WARNING: Failed to close contract module: %v\n", closeErr)
		}
	}()

	// Access the contract memory and log its size.
	memory := contractModule.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("contract module has no memory")
	}
	fmt.Printf("\n=== Memory Setup ===\n")
	fmt.Printf("Initial memory size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/wasmPageSize)

	// Prepare regions for env, info, msg; log the starting offset.
	memManager := newMemoryManager(memory, gasState)
	envRegion, infoRegion, msgRegion, err := memManager.prepareRegions(env, info, msg)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to prepare memory regions: %w", err)
	}

	// Write the region structs to memory and log their pointers.
	envPtr, infoPtr, msgPtr, err := memManager.writeRegions(envRegion, infoRegion, msgRegion)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write regions to memory: %w", err)
	}
	fmt.Println("\n=== Final Memory Layout ===")
	fmt.Printf("Env data: offset=0x%x, length=%d\n", envRegion.Offset, envRegion.Length)
	fmt.Printf("Info data: offset=0x%x, length=%d\n", infoRegion.Offset, infoRegion.Length)
	fmt.Printf("Msg data: offset=0x%x, length=%d\n", msgRegion.Offset, msgRegion.Length)
	fmt.Printf("Region pointers: env=0x%x, info=0x%x, msg=0x%x\n", envPtr, infoPtr, msgPtr)

	// (Optional) Read back each region to log its contents:
	if debugData, ok := memory.Read(envRegion.Offset, envRegion.Length); ok {
		fmt.Printf("[DEBUG] Env region data: %s\n", string(debugData))
	}
	if debugData, ok := memory.Read(infoRegion.Offset, infoRegion.Length); ok {
		fmt.Printf("[DEBUG] Info region data: %s\n", string(debugData))
	}
	if debugData, ok := memory.Read(msgRegion.Offset, msgRegion.Length); ok {
		fmt.Printf("[DEBUG] Msg region data: %s\n", string(debugData))
	}

	// Look up the exported "instantiate" function.
	instantiate := contractModule.ExportedFunction("instantiate")
	if instantiate == nil {
		return nil, types.GasReport{}, fmt.Errorf("instantiate function not exported")
	}

	// Charge gas for instantiation.
	if err := gasState.ConsumeGas(gasState.config.Instantiate, "contract instantiation"); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("out of gas during instantiation: %w", err)
	}

	fmt.Printf("\n=== Calling instantiate ===\n")
	fmt.Printf("Calling with region pointers: env=0x%x, info=0x%x, msg=0x%x\n", envPtr, infoPtr, msgPtr)

	// Call the contract’s instantiate function.
	ret, err := instantiate.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	if err != nil {
		// Dump some memory to help diagnose the abort.
		dumpMemoryDebug(memory, envPtr, infoPtr, msgPtr)
		return nil, types.GasReport{}, fmt.Errorf("instantiate call failed: %w", err)
	}
	if len(ret) != 1 {
		return nil, types.GasReport{}, fmt.Errorf("expected 1 return value, got %d", len(ret))
	}

	// Read back the result from memory.
	resultPtr := uint32(ret[0])
	result, err := w.readFunctionResult(memory, resultPtr, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read function result: %w", err)
	}
	fmt.Printf("[DEBUG] Instantiate result (hex): %x\n", result)

	gasReport := types.GasReport{
		UsedInternally: runtimeEnv.gasUsed,
		UsedExternally: gasState.GetGasUsed(),
		Remaining:      gasLimit - (runtimeEnv.gasUsed + gasState.GetGasUsed()),
		Limit:          gasLimit,
	}
	fmt.Printf("\n=== Instantiation Complete ===\n")
	fmt.Printf("Gas used: %d, Gas remaining: %d\n", gasReport.UsedInternally+gasReport.UsedExternally, gasReport.Remaining)

	return result, gasReport, nil
}
