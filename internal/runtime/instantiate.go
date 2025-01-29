package runtime

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

func (w *WazeroRuntime) Instantiate(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	fmt.Printf("\n=== Contract Instantiation Start ===\n")
	fmt.Printf("=== Initial State ===\n")
	fmt.Printf("Checksum: %x\n", checksum)
	fmt.Printf("Input sizes - env: %d, info: %d, msg: %d\n", len(env), len(info), len(msg))
	fmt.Printf("Message content: %s\n", string(msg))

	// Parse input parameters and create gas state
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to parse params: %w", err)
	}

	// Create gas state for tracking memory operations
	gasState := NewGasState(gasLimit)
	fmt.Printf("Gas state initialized with limit: %d\n", gasLimit)

	// Initialize runtime environment
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

	// Create context with environment
	ctx := context.WithValue(context.Background(), envKey, runtimeEnv)

	// Get the module
	w.mu.Lock()
	module, ok := w.compiledModules[hex.EncodeToString(checksum)]
	if !ok {
		w.mu.Unlock()
		return nil, types.GasReport{}, fmt.Errorf("module not found for checksum: %x", checksum)
	}
	w.mu.Unlock()

	// Register host functions
	fmt.Printf("\n=== Registering Host Functions ===\n")
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer func() {
		if closeErr := hostModule.Close(ctx); closeErr != nil {
			fmt.Printf("WARNING: Failed to close host module: %v\n", closeErr)
		}
	}()

	// Create and instantiate environment module first
	fmt.Printf("\n=== Instantiating Environment Module ===\n")
	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, wazero.NewModuleConfig().WithName("env"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer func() {
		if closeErr := envModule.Close(ctx); closeErr != nil {
			fmt.Printf("WARNING: Failed to close env module: %v\n", closeErr)
		}
	}()

	// Then create and instantiate contract module
	fmt.Printf("\n=== Instantiating Contract Module ===\n")
	contractModule, err := w.runtime.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithName("contract"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer func() {
		if closeErr := contractModule.Close(ctx); closeErr != nil {
			fmt.Printf("WARNING: Failed to close contract module: %v\n", closeErr)
		}
	}()

	// Get contract memory
	memory := contractModule.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("contract module has no memory")
	}
	fmt.Printf("\n=== Memory Setup ===\n")
	fmt.Printf("Initial size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/wasmPageSize)

	// Initialize memory with one page if empty
	if memory.Size() == 0 {
		if _, ok := memory.Grow(1); !ok {
			return nil, types.GasReport{}, fmt.Errorf("failed to initialize memory with one page")
		}
	}

	// Prepare regions for input data
	memManager := newMemoryManager(memory, gasState)
	envRegion, infoRegion, msgRegion, err := memManager.prepareRegions(env, info, msg)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to prepare memory regions: %w", err)
	}

	// Write the regions to memory with alignment checks
	envPtr, infoPtr, msgPtr, err := memManager.writeRegions(envRegion, infoRegion, msgRegion)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write regions to memory: %w", err)
	}

	fmt.Printf("\n=== Memory Layout ===\n")
	fmt.Printf("env  data: offset=0x%x, size=%d\n", envRegion.Offset, envRegion.Length)
	fmt.Printf("info data: offset=0x%x, size=%d\n", infoRegion.Offset, infoRegion.Length)
	fmt.Printf("msg  data: offset=0x%x, size=%d\n", msgRegion.Offset, msgRegion.Length)
	fmt.Printf("Region pointers:\n")
	fmt.Printf("env:  ptr=0x%x\n", envPtr)
	fmt.Printf("info: ptr=0x%x\n", infoPtr)
	fmt.Printf("msg:  ptr=0x%x\n", msgPtr)

	// Get instantiate function
	instantiate := contractModule.ExportedFunction("instantiate")
	if instantiate == nil {
		return nil, types.GasReport{}, fmt.Errorf("instantiate function not exported")
	}

	// Charge gas for instantiation
	if err := gasState.ConsumeGas(gasState.config.Instantiate, "contract instantiation"); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("out of gas during instantiation: %w", err)
	}

	fmt.Printf("\n=== Calling instantiate ===\n")
	fmt.Printf("Parameters: envPtr=0x%x, infoPtr=0x%x, msgPtr=0x%x\n", envPtr, infoPtr, msgPtr)

	// Call instantiate function
	ret, err := instantiate.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	if err != nil {
		if printDebug {
			dumpMemoryDebug(memory, envPtr, infoPtr, msgPtr)
		}
		return nil, types.GasReport{}, fmt.Errorf("instantiate call failed: %w", err)
	}

	if len(ret) != 1 {
		return nil, types.GasReport{}, fmt.Errorf("expected 1 return value, got %d", len(ret))
	}

	// Read and validate result
	resultPtr := uint32(ret[0])
	result, err := w.readFunctionResult(memory, resultPtr, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read function result: %w", err)
	}

	gasReport := types.GasReport{
		UsedInternally: runtimeEnv.gasUsed,
		UsedExternally: gasState.GetGasUsed(),
		Remaining:      gasLimit - (runtimeEnv.gasUsed + gasState.GetGasUsed()),
		Limit:          gasLimit,
	}

	fmt.Printf("\n=== Instantiation Complete ===\n")
	fmt.Printf("Gas used: %d\n", gasReport.UsedInternally+gasReport.UsedExternally)
	fmt.Printf("Gas remaining: %d\n", gasReport.Remaining)

	return result, gasReport, nil
}
