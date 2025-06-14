//go:build wazero

package cosmwasm

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v3/internal/wazeroimpl"
	"github.com/CosmWasm/wasmvm/v3/types"
)

// VM implements a very small subset of the cosmwasm VM using the wazero runtime.
type VM struct {
	cache      *wazeroimpl.Cache
	printDebug bool
}

// NewVM creates a new wazero based VM.
func NewVM(dataDir string, supportedCapabilities []string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error) {
	return NewVMWithConfig(types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  dataDir,
			AvailableCapabilities:    supportedCapabilities,
			MemoryCacheSizeBytes:     types.NewSizeMebi(cacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(memoryLimit),
		},
	}, printDebug)
}

// NewVMWithConfig creates a new VM with a custom configuration.
func NewVMWithConfig(config types.VMConfig, printDebug bool) (*VM, error) {
	cache, err := wazeroimpl.InitCache(config)
	if err != nil {
		return nil, err
	}
	return &VM{cache: cache, printDebug: printDebug}, nil
}

// Cleanup releases resources used by this VM.
func (vm *VM) Cleanup() {
	_ = vm.cache.Close(context.Background())
}

// StoreCode compiles the given wasm code and stores it under its checksum.
func (vm *VM) StoreCode(code WasmCode, gasLimit uint64) (Checksum, uint64, error) {
	checksum, err := CreateChecksum(code)
	if err != nil {
		return nil, 0, err
	}
	if err := vm.cache.Compile(context.Background(), checksum, code); err != nil {
		return nil, 0, err
	}
	return checksum, 0, nil
}

// SimulateStoreCode behaves like StoreCode but does not persist anything.
// It compiles the module to ensure validity, then removes it.
func (vm *VM) SimulateStoreCode(code WasmCode, gasLimit uint64) (Checksum, uint64, error) {
	checksum, err := CreateChecksum(code)
	if err != nil {
		return nil, 0, err
	}
	// Compile to cache
	if err := vm.cache.Compile(context.Background(), checksum, code); err != nil {
		return nil, 0, err
	}
	// Remove to avoid persisting
	if err := vm.cache.RemoveCode(checksum); err != nil {
		return nil, 0, err
	}
	return checksum, 0, nil
}

// StoreCodeUnchecked is currently not implemented in the wazero runtime.
func (vm *VM) StoreCodeUnchecked(code WasmCode) (Checksum, error) {
	checksum, err := CreateChecksum(code)
	if err != nil {
		return nil, err
	}
	if err := vm.cache.Compile(context.Background(), checksum, code); err != nil {
		return nil, err
	}
	return checksum, nil
}

// RemoveCode deletes stored Wasm and compiled module for the given checksum.
func (vm *VM) RemoveCode(checksum Checksum) error {
	return vm.cache.RemoveCode(checksum)
}

// GetCode returns the original Wasm bytes for the given checksum.
func (vm *VM) GetCode(checksum Checksum) (WasmCode, error) {
	return vm.cache.GetCode(checksum)
}

func (vm *VM) Pin(checksum Checksum) error {
	return nil
}

func (vm *VM) Unpin(checksum Checksum) error {
	return nil
}

func (vm *VM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error) {
	return nil, fmt.Errorf("AnalyzeCode not supported in wazero VM")
}

func (vm *VM) GetMetrics() (*types.Metrics, error) {
	return nil, fmt.Errorf("GetMetrics not supported in wazero VM")
}

func (vm *VM) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	return nil, fmt.Errorf("GetPinnedMetrics not supported in wazero VM")
}

func (vm *VM) Instantiate(checksum Checksum, env types.Env, info types.MessageInfo, initMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	if err := vm.cache.Instantiate(context.Background(), checksum, nil, nil, nil, store, &goapi, &querier, gasMeter); err != nil {
		return nil, 0, err
	}
	return &types.ContractResult{}, 0, nil
}

func (vm *VM) Execute(checksum Checksum, env types.Env, info types.MessageInfo, executeMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	if err := vm.cache.Execute(context.Background(), checksum, nil, nil, nil, store, &goapi, &querier, gasMeter); err != nil {
		return nil, 0, err
	}
	return &types.ContractResult{}, 0, nil
}

func (vm *VM) Query(checksum Checksum, env types.Env, queryMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.QueryResult, uint64, error) {
	return nil, 0, fmt.Errorf("Query not supported in wazero VM")
}

func (vm *VM) Migrate(checksum Checksum, env types.Env, migrateMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return nil, 0, fmt.Errorf("Migrate not supported in wazero VM")
}

func (vm *VM) MigrateWithInfo(checksum Checksum, env types.Env, migrateMsg []byte, migrateInfo types.MigrateInfo, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return nil, 0, fmt.Errorf("MigrateWithInfo not supported in wazero VM")
}

func (vm *VM) Sudo(checksum Checksum, env types.Env, sudoMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return nil, 0, fmt.Errorf("Sudo not supported in wazero VM")
}

func (vm *VM) Reply(checksum Checksum, env types.Env, reply types.Reply, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return nil, 0, fmt.Errorf("Reply not supported in wazero VM")
}

func (vm *VM) IBCChannelOpen(checksum Checksum, env types.Env, msg types.IBCChannelOpenMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCChannelOpenResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCChannelOpen not supported in wazero VM")
}

func (vm *VM) IBCChannelConnect(checksum Checksum, env types.Env, msg types.IBCChannelConnectMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCChannelConnect not supported in wazero VM")
}

func (vm *VM) IBCChannelClose(checksum Checksum, env types.Env, msg types.IBCChannelCloseMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCChannelClose not supported in wazero VM")
}

func (vm *VM) IBCPacketReceive(checksum Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCReceiveResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCPacketReceive not supported in wazero VM")
}

func (vm *VM) IBCPacketAck(checksum Checksum, env types.Env, msg types.IBCPacketAckMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCPacketAck not supported in wazero VM")
}

func (vm *VM) IBCPacketTimeout(checksum Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCPacketTimeout not supported in wazero VM")
}

func (vm *VM) IBCSourceCallback(checksum Checksum, env types.Env, msg types.IBCSourceCallbackMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCSourceCallback not supported in wazero VM")
}

func (vm *VM) IBCDestinationCallback(checksum Checksum, env types.Env, msg types.IBCDestinationCallbackMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBCDestinationCallback not supported in wazero VM")
}

func (vm *VM) IBC2PacketAck(checksum Checksum, env types.Env, msg types.IBC2AcknowledgeMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBC2PacketAck not supported in wazero VM")
}

func (vm *VM) IBC2PacketReceive(checksum Checksum, env types.Env, msg types.IBC2PacketReceiveMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCReceiveResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBC2PacketReceive not supported in wazero VM")
}

func (vm *VM) IBC2PacketTimeout(checksum Checksum, env types.Env, msg types.IBC2PacketTimeoutMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBC2PacketTimeout not supported in wazero VM")
}

func (vm *VM) IBC2PacketSend(checksum Checksum, env types.Env, msg types.IBC2PacketSendMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResult, uint64, error) {
	return nil, 0, fmt.Errorf("IBC2PacketSend not supported in wazero VM")
}
