package cosmwasm

import (
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/api"
	"github.com/CosmWasm/wasmvm/types"
)

// CodeID represents an ID for a given wasm code blob, must be generated from this library
type CodeID []byte

// WasmCode is an alias for raw bytes of the wasm compiled code
type WasmCode []byte

// KVStore is a reference to some sub-kvstore that is valid for one instance of a code
type KVStore = api.KVStore

// GoAPI is a reference to some "precompiles", go callbacks
type GoAPI = api.GoAPI

// Querier lets us make read-only queries on other modules
type Querier = types.Querier

// GasMeter is a read-only version of the sdk gas meter
type GasMeter = api.GasMeter

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
// `supportedFeatures` is a comma separated list of features suppored by the chain.
// `memoryLimit` is the memory limit of each contract execution (in MiB)
// `printDebug` is a flag to enable/disable printing debug logs from the contract to STDOUT. This should be false in production environments.
// `cacheSize` sets the size in MiB of an in-memory cache for e.g. module caching. Set to 0 to disable.
func NewVM(dataDir string, supportedFeatures string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error) {
	cache, err := api.InitCache(dataDir, supportedFeatures, cacheSize, memoryLimit)
	if err != nil {
		return nil, err
	}
	return &VM{cache: cache, printDebug: printDebug}, nil
}

// Cleanup should be called when no longer using this to free resources on the rust-side
func (vm *VM) Cleanup() {
	api.ReleaseCache(vm.cache)
}

// Create will compile the wasm code, and store the resulting pre-compile
// as well as the original code. Both can be referenced later via CodeID
// This must be done one time for given code, after which it can be
// instatitated many times, and each instance called many times.
//
// For example, the code for all ERC-20 contracts should be the same.
// This function stores the code for that contract only once, but it can
// be instantiated with custom inputs in the future.
//
// TODO: return gas cost? Add gas limit??? there is no metering here...
func (vm *VM) Create(code WasmCode) (CodeID, error) {
	return api.Create(vm.cache, code)
}

// GetCode will load the original wasm code for the given code id.
// This will only succeed if that code id was previously returned from
// a call to Create.
//
// This can be used so that the (short) code id (hash) is stored in the iavl tree
// and the larger binary blobs (wasm and pre-compiles) are all managed by the
// rust library
func (vm *VM) GetCode(code CodeID) (WasmCode, error) {
	return api.GetCode(vm.cache, code)
}

// Instantiate will create a new contract based on the given codeID.
// We can set the initMsg (contract "genesis") here, and it then receives
// an account and address and can be invoked (Execute) many times.
//
// Storage should be set with a PrefixedKVStore that this code can safely access.
//
// Under the hood, we may recompile the wasm, use a cached native compile, or even use a cached instance
// for performance.
func (vm *VM) Instantiate(
	code CodeID,
	env types.Env,
	info types.MessageInfo,
	initMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.InitResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBin, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.Instantiate(vm.cache, code, envBin, infoBin, initMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.InitResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// Execute calls a given contract. Since the only difference between contracts with the same CodeID is the
// data in their local storage, and their address in the outside world, we need no ContractID here.
// (That is a detail for the external, sdk-facing, side).
//
// The caller is responsible for passing the correct `store` (which must have been initialized exactly once),
// and setting the env with relevent info on this instance (address, balance, etc)
func (vm *VM) Execute(
	code CodeID,
	env types.Env,
	info types.MessageInfo,
	executeMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.HandleResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBin, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.Handle(vm.cache, code, envBin, infoBin, executeMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.HandleResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// Query allows a client to execute a contract-specific query. If the result is not empty, it should be
// valid json-encoded data to return to the client.
// The meaning of path and data can be determined by the code. Path is the suffix of the abci.QueryRequest.Path
func (vm *VM) Query(
	code CodeID,
	env types.Env,
	queryMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) ([]byte, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.Query(vm.cache, code, envBin, queryMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.QueryResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// Migrate will migrate an existing contract to a new code binary.
// This takes storage of the data from the original contract and the CodeID of the new contract that should
// replace it. This allows it to run a migration step if needed, or return an error if unable to migrate
// the given data.
//
// MigrateMsg has some data on how to perform the migration.
func (vm *VM) Migrate(
	code CodeID,
	env types.Env,
	info types.MessageInfo,
	migrateMsg []byte,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.MigrateResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBin, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.Migrate(vm.cache, code, envBin, infoBin, migrateMsg, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.MigrateResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// IBCChannelOpen is available on IBC-enabled contracts and is a hook to call into
// during the handshake pahse
func (vm *VM) IBCChannelOpen(
	code CodeID,
	env types.Env,
	channel types.IBCChannel,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return 0, err
	}
	chanBin, err := json.Marshal(channel)
	if err != nil {
		return 0, err
	}
	data, gasUsed, err := api.IBCChannelOpen(vm.cache, code, envBin, chanBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return gasUsed, err
	}

	var resp types.IBCChannelOpenResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return gasUsed, err
	}
	if resp.Err != "" {
		return gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return gasUsed, nil
}

// IBCChannelConnect is available on IBC-enabled contracts and is a hook to call into
// during the handshake pahse
func (vm *VM) IBCChannelConnect(
	code CodeID,
	env types.Env,
	channel types.IBCChannel,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.IBCBasicResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	chanBin, err := json.Marshal(channel)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.IBCChannelConnect(vm.cache, code, envBin, chanBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.IBCBasicResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// IBCChannelClose is available on IBC-enabled contracts and is a hook to call into
// at the end of the channel lifetime
func (vm *VM) IBCChannelClose(
	code CodeID,
	env types.Env,
	channel types.IBCChannel,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.IBCBasicResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	chanBin, err := json.Marshal(channel)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.IBCChannelClose(vm.cache, code, envBin, chanBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.IBCBasicResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// IBCPacketReceive is available on IBC-enabled contracts and is called when an incoming
// packet is received on a channel belonging to this contract
func (vm *VM) IBCPacketReceive(
	code CodeID,
	env types.Env,
	packet types.IBCPacket,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.IBCReceiveResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	packetBin, err := json.Marshal(packet)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.IBCPacketReceive(vm.cache, code, envBin, packetBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.IBCReceiveResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// IBCPacketAck is available on IBC-enabled contracts and is called when an
// the response for an outgoing packet (previously sent by this contract)
// is received
func (vm *VM) IBCPacketAck(
	code CodeID,
	env types.Env,
	ack types.IBCAcknowledgement,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.IBCBasicResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	ackBin, err := json.Marshal(ack)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.IBCPacketAck(vm.cache, code, envBin, ackBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.IBCBasicResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}

// IBCPacketTimeout is available on IBC-enabled contracts and is called when an
// outgoing packet (previously sent by this contract) will provably never be executed.
// Usually handled like ack returning an error
func (vm *VM) IBCPacketTimeout(
	code CodeID,
	env types.Env,
	packet types.IBCPacket,
	store KVStore,
	goapi GoAPI,
	querier Querier,
	gasMeter GasMeter,
	gasLimit uint64,
) (*types.IBCBasicResponse, uint64, error) {
	envBin, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	packetBin, err := json.Marshal(packet)
	if err != nil {
		return nil, 0, err
	}
	data, gasUsed, err := api.IBCPacketTimeout(vm.cache, code, envBin, packetBin, &gasMeter, store, &goapi, &querier, gasLimit, vm.printDebug)
	if err != nil {
		return nil, gasUsed, err
	}

	var resp types.IBCBasicResult
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, gasUsed, err
	}
	if resp.Err != "" {
		return nil, gasUsed, fmt.Errorf("%s", resp.Err)
	}
	return resp.Ok, gasUsed, nil
}
