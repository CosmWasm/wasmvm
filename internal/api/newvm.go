package api

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/wasmer"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func NewVMWithRuntime(runtimeType string, dataDir string, capabilities map[string]struct{}, memoryLimit uint32, logger types.InfoLogger) (WasmVMRuntime, error) {
	switch runtimeType {
	case "wasmer":
		// Create the VM instance (implementation details omitted for brevity)
		vm, err := wasmer.NewVM(dataDir, capabilities, memoryLimit, logger)
		if err != nil {
			return nil, err
		}
		return wasmer.NewWasmerRuntime(vm), nil
	default:
		return nil, fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}
}
