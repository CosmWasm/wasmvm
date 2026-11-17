//go:build wasmer
// +build wasmer

package wasmer

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// WasmerRuntime wraps the Wasmer-based VM to implement the WasmVMRuntime interface.
type WasmerRuntime struct {
	vm VMInterface // Use an interface to avoid import cycles with internal/api
}

// VMInterface defines the methods required from the VM to avoid direct imports.
type VMInterface interface {
	Close() error
	StoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error)
	SimulateStoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error)
	GetCode(checksum types.Checksum) ([]byte, error)
	RemoveCode(checksum types.Checksum) error
	Pin(checksum types.Checksum) error
	Unpin(checksum types.Checksum) error
	AnalyzeCode(checksum types.Checksum) (*types.AnalysisReport, error)
	Instantiate(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error)
	Execute(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error)
	Query(checksum types.Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)
	Migrate(checksum types.Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)
	Sudo(checksum types.Checksum, env types.Env, sudoMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)
	Reply(checksum types.Checksum, env types.Env, reply types.Reply, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)
	IBCChannelOpen(checksum types.Checksum, env types.Env, msg types.IBCChannelOpenMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCChannelOpenResult, uint64, error)
	IBCChannelConnect(checksum types.Checksum, env types.Env, msg types.IBCChannelConnectMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)
	IBCChannelClose(checksum types.Checksum, env types.Env, msg types.IBCChannelCloseMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)
	IBCPacketReceive(checksum types.Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCReceiveResult, uint64, error)
	IBCPacketAck(checksum types.Checksum, env types.Env, msg types.IBCPacketAckMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)
	IBCPacketTimeout(checksum types.Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)
	GetMetrics() (*types.Metrics, error)
	GetPinnedMetrics() (*types.PinnedMetrics, error)
}

// NewWasmerRuntime creates a new instance of WasmerRuntime with the provided VMInterface.
func NewWasmerRuntime(vm VMInterface) *WasmerRuntime {
	return &WasmerRuntime{vm: vm}
}

// Close releases resources held by the VM.
func (r *WasmerRuntime) Close() error {
	return r.vm.Close()
}

// StoreCode stores Wasm code and returns its checksum and gas cost.
func (r *WasmerRuntime) StoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error) {
	return r.vm.StoreCode(code, gasLimit)
}

// SimulateStoreCode estimates the gas cost of storing Wasm code without persisting it.
func (r *WasmerRuntime) SimulateStoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error) {
	return r.vm.SimulateStoreCode(code, gasLimit)
}

// GetCode retrieves the Wasm code associated with the given checksum.
func (r *WasmerRuntime) GetCode(checksum types.Checksum) ([]byte, error) {
	return r.vm.GetCode(checksum)
}

// RemoveCode deletes the Wasm code associated with the given checksum from the cache.
func (r *WasmerRuntime) RemoveCode(checksum types.Checksum) error {
	return r.vm.RemoveCode(checksum)
}

// Pin marks the Wasm code with the given checksum as pinned in the cache.
func (r *WasmerRuntime) Pin(checksum types.Checksum) error {
	return r.vm.Pin(checksum)
}

// Unpin removes the pinned status from the Wasm code with the given checksum.
func (r *WasmerRuntime) Unpin(checksum types.Checksum) error {
	return r.vm.Unpin(checksum)
}

// AnalyzeCode performs static analysis on the Wasm code identified by the checksum.
func (r *WasmerRuntime) AnalyzeCode(checksum types.Checksum) (*types.AnalysisReport, error) {
	return r.vm.AnalyzeCode(checksum)
}

// Instantiate creates a new instance of a Wasm contract.
func (r *WasmerRuntime) Instantiate(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return r.vm.Instantiate(checksum, env, info, msg, store, api, querier, gasMeter, gasLimit, deserCost)
}

// Execute runs a contract method.
func (r *WasmerRuntime) Execute(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	return r.vm.Execute(checksum, env, info, msg, store, api, querier, gasMeter, gasLimit, deserCost)
}

// Query calls the contract's query function with the provided message.
func (r *WasmerRuntime) Query(checksum types.Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	return r.vm.Query(checksum, env, queryMsg, store, api, querier, gasMeter, gasLimit)
}

// Migrate calls the contract's migrate function with the provided message.
func (r *WasmerRuntime) Migrate(checksum types.Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	return r.vm.Migrate(checksum, env, migrateMsg, store, api, querier, gasMeter, gasLimit)
}

// Sudo calls the contract's sudo function with the provided message.
func (r *WasmerRuntime) Sudo(checksum types.Checksum, env types.Env, sudoMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	return r.vm.Sudo(checksum, env, sudoMsg, store, api, querier, gasMeter, gasLimit)
}

// Reply calls the contract's reply function with the provided reply message.
func (r *WasmerRuntime) Reply(checksum types.Checksum, env types.Env, reply types.Reply, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	return r.vm.Reply(checksum, env, reply, store, api, querier, gasMeter, gasLimit)
}

// IBCChannelOpen handles the IBC channel open operation for the contract.
func (r *WasmerRuntime) IBCChannelOpen(checksum types.Checksum, env types.Env, msg types.IBCChannelOpenMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCChannelOpenResult, uint64, error) {
	return r.vm.IBCChannelOpen(checksum, env, msg, store, api, querier, gasMeter, gasLimit)
}

// IBCChannelConnect handles the IBC channel connect operation for the contract.
func (r *WasmerRuntime) IBCChannelConnect(checksum types.Checksum, env types.Env, msg types.IBCChannelConnectMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	return r.vm.IBCChannelConnect(checksum, env, msg, store, api, querier, gasMeter, gasLimit)
}

// IBCChannelClose handles the IBC channel close operation for the contract.
func (r *WasmerRuntime) IBCChannelClose(checksum types.Checksum, env types.Env, msg types.IBCChannelCloseMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	return r.vm.IBCChannelClose(checksum, env, msg, store, api, querier, gasMeter, gasLimit)
}

// IBCPacketReceive handles the IBC packet receive operation for the contract.
func (r *WasmerRuntime) IBCPacketReceive(checksum types.Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCReceiveResult, uint64, error) {
	return r.vm.IBCPacketReceive(checksum, env, msg, store, api, querier, gasMeter, gasLimit)
}

// IBCPacketAck handles the IBC packet acknowledgement operation for the contract.
func (r *WasmerRuntime) IBCPacketAck(checksum types.Checksum, env types.Env, msg types.IBCPacketAckMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	return r.vm.IBCPacketAck(checksum, env, msg, store, api, querier, gasMeter, gasLimit)
}

// IBCPacketTimeout handles the IBC packet timeout operation for the contract.
func (r *WasmerRuntime) IBCPacketTimeout(checksum types.Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	return r.vm.IBCPacketTimeout(checksum, env, msg, store, api, querier, gasMeter, gasLimit)
}

// GetMetrics retrieves general metrics about the VM's cache.
func (r *WasmerRuntime) GetMetrics() (*types.Metrics, error) {
	return r.vm.GetMetrics()
}

// GetPinnedMetrics retrieves metrics specific to pinned Wasm modules.
func (r *WasmerRuntime) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	return r.vm.GetPinnedMetrics()
}
