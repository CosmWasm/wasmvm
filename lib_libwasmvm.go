// This file contains the part of the API that is exposed when libwasmvm
// is available (i.e. cgo is enabled and nolink_libwasmvm is not set).

package cosmwasm

import (
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// VM is the main entry point to this library.
// You should create an instance with its own subdirectory to manage state inside,
// and call it for all cosmwasm code related actions.
type VM struct {
	cache      api.Cache
	printDebug bool
}

// NewVM creates a new VM.
//
// `dataDir` is a base directory for Wasm blobs and various caches.
// `supportedCapabilities` is a list of capabilities supported by the chain.
// `memoryLimit` is the memory limit of each contract execution (in MiB)
// `printDebug` is a flag to enable/disable printing debug logs from the contract to STDOUT. This should be false in production environments.
// `cacheSize` sets the size in MiB of an in-memory cache for e.g. module caching. Set to 0 to disable.
// `deserCost` sets the gas cost of deserializing one byte of data.
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
// This allows for more fine-grained control over the VM's behavior compared to NewVM and
// can be extended more easily in the future.
func NewVMWithConfig(config types.VMConfig, printDebug bool) (*VM, error) {
	cache, err := api.InitCache(config)
	if err != nil {
		return nil, err
	}
	return &VM{cache: cache, printDebug: printDebug}, nil
}

// Cleanup should be called when no longer using this instances.
// It frees resources in libwasmvm (the Rust part) and releases a lock in the base directory.
func (vm *VM) Cleanup() {
	api.ReleaseCache(vm.cache)
}

// StoreCode will compile the Wasm code, and store the resulting compiled module
// as well as the original code. Both can be referenced later via Checksum.
// This must be done one time for given code, after which it can be
// instatitated many times, and each instance called many times.
//
// For example, the code for all ERC-20 contracts should be the same.
// This function stores the code for that contract only once, but it can
// be instantiated with custom inputs in the future.
//
// Returns both the checksum, as well as the gas cost of compilation (in CosmWasm Gas) or an error.
func (vm *VM) StoreCode(code WasmCode, gasLimit uint64) (Checksum, uint64, error) {
	gasCost := compileCost(code)
	if gasLimit < gasCost {
		return nil, gasCost, types.OutOfGasError{}
	}

	checksum, err := api.StoreCode(vm.cache, code, true)
	return checksum, gasCost, err
}

// SimulateStoreCode is the same as StoreCode but does not actually store the code.
// This is useful for simulating all the validations happening in StoreCode without actually
// writing anything to disk.
func (vm *VM) SimulateStoreCode(code WasmCode, gasLimit uint64) (Checksum, uint64, error) {
	gasCost := compileCost(code)
	if gasLimit < gasCost {
		return nil, gasCost, types.OutOfGasError{}
	}

	checksum, err := api.StoreCode(vm.cache, code, false)
	return checksum, gasCost, err
}

// StoreCodeUnchecked is the same as StoreCode but skips static validation checks.
// Use this for adding code that was checked before, particularly in the case of state sync.
func (vm *VM) StoreCodeUnchecked(code WasmCode) (Checksum, error) {
	checksum, err := api.StoreCodeUnchecked(vm.cache, code)
	return checksum, err
}

func (vm *VM) RemoveCode(checksum Checksum) error {
	return api.RemoveCode(vm.cache, checksum)
}

// GetCode will load the original Wasm code for the given checksum.
// This will only succeed if that checksum was previously returned from
// a call to StoreCode.
//
// This can be used so that the (short) checksum is stored in the iavl tree
// and the larger binary blobs (wasm and compiled modules) are all managed
// by libwasmvm/cosmwasm-vm (Rust part).
func (vm *VM) GetCode(checksum Checksum) (WasmCode, error) {
	return api.GetCode(vm.cache, checksum)
}

// Pin pins a code to an in-memory cache, such that is
// always loaded quickly when executed.
// Pin is idempotent.
func (vm *VM) Pin(checksum Checksum) error {
	return api.Pin(vm.cache, checksum)
}

// Unpin removes the guarantee of a contract to be pinned (see Pin).
// After calling this, the code may or may not remain in memory depending on
// the implementor's choice.
// Unpin is idempotent.
func (vm *VM) Unpin(checksum Checksum) error {
	return api.Unpin(vm.cache, checksum)
}

// Returns a report of static analysis of the wasm contract (uncompiled).
// This contract must have been stored in the cache previously (via Create).
// Only info currently returned is if it exposes all ibc entry points, but this may grow later
func (vm *VM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error) {
	return api.AnalyzeCode(vm.cache, checksum)
}

// GetMetrics some internal metrics for monitoring purposes.
func (vm *VM) GetMetrics() (*types.Metrics, error) {
	return api.GetMetrics(vm.cache)
}

// GetPinnedMetrics returns some internal metrics of pinned contracts for monitoring purposes.
// The order of entries is non-deterministic and the values are node-specific. Don't use this in consensus-critical contexts.
func (vm *VM) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	return api.GetPinnedMetrics(vm.cache)
}

// Instantiate will create a new contract based on the given Checksum.
// We can set the initMsg (contract "genesis") here, and it then receives
// an account and address and can be invoked (Execute) many times.
//
// Storage should be set with a PrefixedKVStore that this code can safely access.
//
// Under the hood, we may recompile the wasm, use a cached native compile, or even use a cached instance
// for performance.
func (vm *VM) Instantiate(
	checksum Checksum,
	env types.Env,
	info types.MessageInfo,
	initMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.ContractResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBin, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.Instantiate(vm.cache, checksum, envBin, infoBin, initMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// Execute calls a given contract. Since the only difference between contracts with the same Checksum is the
// data in their local storage, and their address in the outside world, we need no ContractID here.
// (That is a detail for the external, sdk-facing, side).
//
// The caller is responsible for passing the correct `store` (which must have been initialized exactly once),
// and setting the env with relevant info on this instance (address, balance, etc)
func (vm *VM) Execute(
	checksum Checksum,
	env types.Env,
	info types.MessageInfo,
	executeMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.ContractResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBin, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.Execute(vm.cache, checksum, envBin, infoBin, executeMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// Query allows a client to execute a contract-specific query. If the result is not empty, it should be
// valid json-encoded data to return to the client.
// The meaning of path and data can be determined by the code. Path is the suffix of the abci.QueryRequest.Path
func (vm *VM) Query(
	checksum Checksum,
	env types.Env,
	queryMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.QueryResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.Query(vm.cache, checksum, envBin, queryMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.QueryResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// Migrate will migrate an existing contract to a new code binary.
// This takes storage of the data from the original contract and the Checksum of the new contract that should
// replace it. This allows it to run a migration step if needed, or return an error if unable to migrate
// the given data.
//
// MigrateMsg has some data on how to perform the migration.
func (vm *VM) Migrate(
	checksum Checksum,
	env types.Env,
	migrateMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.ContractResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.Migrate(vm.cache, checksum, envBin, migrateMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// MigrateWithInfo will migrate an existing contract to a new code binary.
// This takes storage of the data from the original contract and the Checksum of the new contract that should
// replace it. This allows it to run a migration step if needed, or return an error if unable to migrate
// the given data.
//
// MigrateMsg has some data on how to perform the migration.
//
// MigrateWithInfo takes one more argument - `migateInfo`. It consist of an additional data
// related to the on-chain current contract's state version.
func (vm *VM) MigrateWithInfo(
	checksum Checksum,
	env types.Env,
	migrateMsg []byte,
	migrateInfo types.MigrateInfo,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.ContractResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}

	migrateBin, err := json.Marshal(migrateInfo)
	if err != nil {
		return nil, 0, err
	}

	data, gasReport, err := api.MigrateWithInfo(vm.cache, checksum, envBin, migrateMsg, migrateBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// Sudo allows native Go modules to make privileged (sudo) calls on the contract.
// The contract can expose entry points that cannot be triggered by any transaction, but only via
// native Go modules, and delegate the access control to the system.
//
// These work much like Migrate (same scenario) but allows custom apps to extend the privileged entry points
// without forking cosmwasm-vm.
func (vm *VM) Sudo(
	checksum Checksum,
	env types.Env,
	sudoMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.ContractResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.Sudo(vm.cache, checksum, envBin, sudoMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// Reply allows the native Go wasm modules to make a privileged call to return the result
// of executing a SubMsg.
//
// These work much like Sudo (same scenario) but focuses on one specific case (and one message type)
func (vm *VM) Reply(
	checksum Checksum,
	env types.Env,
	reply types.Reply,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.ContractResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	replyBin, err := json.Marshal(reply)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.Reply(vm.cache, checksum, envBin, replyBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCChannelOpen is available on IBC-enabled contracts and is a hook to call into
// during the handshake pahse
func (vm *VM) IBCChannelOpen(
	checksum Checksum,
	env types.Env,
	msg types.IBCChannelOpenMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCChannelOpenResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCChannelOpen(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCChannelOpenResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCChannelConnect is available on IBC-enabled contracts and is a hook to call into
// during the handshake pahse
func (vm *VM) IBCChannelConnect(
	checksum Checksum,
	env types.Env,
	msg types.IBCChannelConnectMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCBasicResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCChannelConnect(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCBasicResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCChannelClose is available on IBC-enabled contracts and is a hook to call into
// at the end of the channel lifetime
func (vm *VM) IBCChannelClose(
	checksum Checksum,
	env types.Env,
	msg types.IBCChannelCloseMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCBasicResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCChannelClose(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCBasicResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCPacketReceive is available on IBC-enabled contracts and is called when an incoming
// packet is received on a channel belonging to this contract
func (vm *VM) IBCPacketReceive(
	checksum Checksum,
	env types.Env,
	msg types.IBCPacketReceiveMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCReceiveResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCPacketReceive(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCReceiveResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCPacketAck is available on IBC-enabled contracts and is called when an
// the response for an outgoing packet (previously sent by this contract)
// is received
func (vm *VM) IBCPacketAck(
	checksum Checksum,
	env types.Env,
	msg types.IBCPacketAckMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCBasicResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCPacketAck(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCBasicResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCPacketTimeout is available on IBC-enabled contracts and is called when an
// outgoing packet (previously sent by this contract) will provably never be executed.
// Usually handled like ack returning an error
func (vm *VM) IBCPacketTimeout(
	checksum Checksum,
	env types.Env,
	msg types.IBCPacketTimeoutMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCBasicResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCPacketTimeout(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCBasicResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCSourceCallback is available on IBC-enabled contracts with the corresponding entrypoint
// and should be called when the response (ack or timeout) for an outgoing callbacks-enabled packet
// (previously sent by this contract) is received.
func (vm *VM) IBCSourceCallback(
	checksum Checksum,
	env types.Env,
	msg types.IBCSourceCallbackMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCBasicResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCSourceCallback(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCBasicResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

// IBCDestinationCallback is available on IBC-enabled contracts with the corresponding entrypoint
// and should be called when an incoming callbacks-enabled IBC packet is received.
func (vm *VM) IBCDestinationCallback(
	checksum Checksum,
	env types.Env,
	msg types.IBCDestinationCallbackMsg,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
	deserCost types.UFraction,
) (*types.IBCBasicResult, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBin, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	data, gasReport, err := api.IBCDestinationCallback(vm.cache, checksum, envBin, msgBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	var result types.IBCBasicResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}
	return &result, gasReport.UsedInternally, nil
}

func compileCost(code WasmCode) uint64 {
	// CostPerByte is how much CosmWasm gas is charged *per byte* for compiling WASM code.
	// Benchmarks and numbers (in SDK Gas) were discussed in:
	// https://github.com/CosmWasm/wasmd/pull/634#issuecomment-938056803
	const CostPerByte uint64 = 3 * 140_000

	return CostPerByte * uint64(len(code))
}

// hasSubMessages is an interface for contract results that can contain sub-messages.
type hasSubMessages interface {
	SubMessages() []types.SubMsg
}

// make sure the types implement the interface
// cannot put these next to the types, as the interface is private
var (
	_ hasSubMessages = (*types.IBCBasicResult)(nil)
	_ hasSubMessages = (*types.IBCReceiveResult)(nil)
	_ hasSubMessages = (*types.ContractResult)(nil)
)

func DeserializeResponse(gasLimit uint64, deserCost types.UFraction, gasReport *types.GasReport, data []byte, response any) error {
	gasForDeserialization := deserCost.Mul(uint64(len(data))).Floor()
	if gasLimit < gasForDeserialization+gasReport.UsedInternally {
		return fmt.Errorf("Insufficient gas left to deserialize contract execution result (%d bytes)", len(data))
	}
	gasReport.UsedInternally += gasForDeserialization
	gasReport.Remaining -= gasForDeserialization

	err := json.Unmarshal(data, response)
	if err != nil {
		return err
	}

	// All responses that have sub-messages need their payload size to be checked
	const ReplyPayloadMaxBytes = 128 * 1024 // 128 KiB
	if response, ok := response.(hasSubMessages); ok {
		for i, m := range response.SubMessages() {
			// each payload needs to be below maximum size
			if len(m.Payload) > ReplyPayloadMaxBytes {
				return fmt.Errorf("reply contains submessage at index %d with payload larger than %d bytes: %d bytes", i, ReplyPayloadMaxBytes, len(m.Payload))
			}
		}
	}

	return nil
}
