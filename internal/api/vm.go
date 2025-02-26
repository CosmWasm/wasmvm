package api

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

type VM struct {
	runtime WasmVMRuntime
}

// NewVM creates a new VM with the specified runtime (Wasmer or Wazero).
func NewVM(dataDir string, supportedCapabilities map[string]struct{}, memoryLimit uint32, logger types.InfoLogger, runtimeType string) (*VM, error) {
	var runtime WasmVMRuntime
	switch runtimeType {
	case "wasmer":
		runtime = newWasmerRuntime() // Existing Wasmer implementation
	case "wazero":
		runtime = newWazeroRuntime() // New Wazero implementation
	default:
		return nil, fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	vm := &VM{runtime: runtime}
	if err := vm.runtime.NewVM(dataDir, supportedCapabilities, memoryLimit, logger); err != nil {
		return nil, err
	}
	return vm, nil
}

// Delegate all methods to the runtime
func (vm *VM) Close() error {
	return vm.runtime.Close()
}

func (vm *VM) StoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error) {
	return vm.runtime.StoreCode(code, gasLimit)
}

func (vm *VM) SimulateStoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error) {
	return vm.runtime.SimulateStoreCode(code, gasLimit)
}

func (vm *VM) GetCode(checksum types.Checksum) ([]byte, error) {
	return vm.runtime.GetCode(checksum)
}

func (vm *VM) RemoveCode(checksum types.Checksum) error {
	return vm.runtime.RemoveCode(checksum)
}

func (vm *VM) Pin(checksum types.Checksum) error {
	return vm.runtime.Pin(checksum)
}

func (vm *VM) Unpin(checksum types.Checksum) error {
	return vm.runtime.Unpin(checksum)
}

func (vm *VM) AnalyzeCode(checksum types.Checksum) (*types.AnalysisReport, error) {
	return vm.runtime.AnalyzeCode(checksum)
}

func (vm *VM) Instantiate(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return vm.runtime.Instantiate(checksum, env, info, msg, store, api, querier, gasMeter, gasLimit, deserCost)
}

func (vm *VM) Execute(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return vm.runtime.Execute(checksum, env, info, msg, store, api, querier, gasMeter, gasLimit, deserCost)
}

func (vm *VM) Query(checksum types.Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	return vm.runtime.Query(checksum, env, queryMsg, store, api, querier, gasMeter, gasLimit)
}

func (vm *VM) Migrate(checksum types.Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, ga
