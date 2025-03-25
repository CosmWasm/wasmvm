package api

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// WasmVMRuntime defines the interface for Wasm VM runtimes (e.g., Wasmer, Wazero).
type WasmVMRuntime interface {
	// NewVM initializes a new VM instance with the given configuration.
	NewVM(dataDir string, supportedCapabilities map[string]struct{}, memoryLimit uint32, logger types.InfoLogger) error

	// Close releases all resources held by the VM.
	Close() error

	// StoreCode compiles and stores Wasm code, returning its checksum and gas used.
	StoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error)

	// SimulateStoreCode estimates gas for storing code without persisting it.
	SimulateStoreCode(code []byte, gasLimit uint64) (types.Checksum, uint64, error)

	// GetCode retrieves the Wasm code for a given checksum.
	GetCode(checksum types.Checksum) ([]byte, error)

	// RemoveCode removes code and its cached compiled module.
	RemoveCode(checksum types.Checksum) error

	// Pin ensures a module remains in memory (not evicted by cache).
	Pin(checksum types.Checksum) error

	// Unpin allows a module to be evicted from cache.
	Unpin(checksum types.Checksum) error

	// AnalyzeCode performs static analysis on the Wasm code.
	AnalyzeCode(checksum types.Checksum) (*types.AnalysisReport, error)

	// Instantiate calls the contract's instantiate entry point.
	Instantiate(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error)

	// Execute calls the contract's execute entry point.
	Execute(checksum types.Checksum, env types.Env, info types.MessageInfo, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error)

	// Query calls the contract's query entry point.
	Query(checksum types.Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)

	// Migrate calls the contract's migrate entry point.
	Migrate(checksum types.Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)

	// Sudo calls the contract's sudo entry point.
	Sudo(checksum types.Checksum, env types.Env, sudoMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)

	// Reply calls the contract's reply entry point.
	Reply(checksum types.Checksum, env types.Env, reply types.Reply, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error)

	// IBCChannelOpen calls the contract's IBC channel open entry point.
	IBCChannelOpen(checksum types.Checksum, env types.Env, msg types.IBCChannelOpenMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCChannelOpenResult, uint64, error)

	// IBCChannelConnect calls the contract's IBC channel connect entry point.
	IBCChannelConnect(checksum types.Checksum, env types.Env, msg types.IBCChannelConnectMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)

	// IBCChannelClose calls the contract's IBC channel close entry point.
	IBCChannelClose(checksum types.Checksum, env types.Env, msg types.IBCChannelCloseMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)

	// IBCPacketReceive calls the contract's IBC packet receive entry point.
	IBCPacketReceive(checksum types.Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCReceiveResult, uint64, error)

	// IBCPacketAck calls the contract's IBC packet acknowledge entry point.
	IBCPacketAck(checksum types.Checksum, env types.Env, msg types.IBCPacketAckMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)

	// IBCPacketTimeout calls the contract's IBC packet timeout entry point.
	IBCPacketTimeout(checksum types.Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error)

	// GetMetrics returns cache usage metrics.
	GetMetrics() (*types.Metrics, error)

	// GetPinnedMetrics returns detailed metrics for pinned contracts.
	GetPinnedMetrics() (*types.PinnedMetrics, error)
}
