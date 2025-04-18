//go:build cgo && !nolink_libwasmvm

// This file contains the part of the API that is exposed when libwasmvm
// is available (i.e. cgo is enabled and nolink_libwasmvm is not set).

package cosmwasm

import (
	"encoding/json"
	"errors"
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

// VMConfig contains the configuration for VM operations
type VMConfig struct {
	Checksum  types.Checksum
	Env       types.Env
	Info      types.MessageInfo
	Msg       []byte
	Store     KVStore
	GoAPI     GoAPI
	Querier   Querier
	GasMeter  GasMeter
	GasLimit  uint64
	DeserCost types.UFraction
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
func (vm *VM) StoreCode(code WasmCode, gasLimit uint64) (types.Checksum, uint64, error) {
	gasCost := compileCost(code)
	if gasLimit < gasCost {
		return types.Checksum{}, gasCost, types.OutOfGasError{}
	}

	checksumBytes, err := api.StoreCode(vm.cache, code, true)
	if err != nil {
		return types.Checksum{}, gasCost, err
	}
	checksum, err := types.NewChecksum(checksumBytes)
	if err != nil {
		return types.Checksum{}, gasCost, err
	}
	return checksum, gasCost, nil
}

// SimulateStoreCode is the same as StoreCode but does not actually store the code.
// This is useful for simulating all the validations happening in StoreCode without actually
// writing anything to disk.
func (vm *VM) SimulateStoreCode(code WasmCode, gasLimit uint64) (types.Checksum, uint64, error) {
	gasCost := compileCost(code)
	if gasLimit < gasCost {
		return types.Checksum{}, gasCost, types.OutOfGasError{}
	}

	checksumBytes, err := api.StoreCode(vm.cache, code, false)
	if err != nil {
		return types.Checksum{}, gasCost, err
	}
	checksum, err := types.NewChecksum(checksumBytes)
	if err != nil {
		return types.Checksum{}, gasCost, err
	}
	return checksum, gasCost, nil
}

// StoreCodeUnchecked is the same as StoreCode but skips static validation checks and charges no gas.
// Use this for adding code that was checked before, particularly in the case of state sync.
func (vm *VM) StoreCodeUnchecked(code WasmCode) (types.Checksum, error) {
	checksumBytes, err := api.StoreCodeUnchecked(vm.cache, code)
	if err != nil {
		return types.Checksum{}, err
	}
	return types.NewChecksum(checksumBytes)
}

// RemoveCode removes a code from the VM
func (vm *VM) RemoveCode(checksum types.Checksum) error {
	return api.RemoveCode(vm.cache, checksum.Bytes())
}

// GetCode will load the original Wasm code for the given checksum.
// This will only succeed if that checksum was previously returned from
// a call to StoreCode.
//
// This can be used so that the (short) checksum is stored in the iavl tree
// and the larger binary blobs (wasm and compiled modules) are all managed
// by libwasmvm/cosmwasm-vm (Rust part).
func (vm *VM) GetCode(checksum types.Checksum) (WasmCode, error) {
	return api.GetCode(vm.cache, checksum.Bytes())
}

// Pin pins a code to an in-memory cache, such that is
// always loaded quickly when executed.
// Pin is idempotent.
func (vm *VM) Pin(checksum types.Checksum) error {
	return api.Pin(vm.cache, checksum.Bytes())
}

// Unpin removes the guarantee of a contract to be pinned (see Pin).
// After calling this, the code may or may not remain in memory depending on
// the implementor's choice.
// Unpin is idempotent.
func (vm *VM) Unpin(checksum types.Checksum) error {
	return api.Unpin(vm.cache, checksum.Bytes())
}

// AnalyzeCode analyzes a code in the VM
func (vm *VM) AnalyzeCode(checksum types.Checksum) (*types.AnalysisReport, error) {
	return api.AnalyzeCode(vm.cache, checksum.Bytes())
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
func (vm *VM) Instantiate(params api.ContractCallParams) ([]byte, types.ContractResult, types.GasReport, error) {
	// Pass params to api.Instantiate
	resBytes, gasReport, err := api.Instantiate(params)
	if err != nil {
		return nil, types.ContractResult{}, gasReport, err
	}

	// Unmarshal the result
	var result types.ContractResult
	err = json.Unmarshal(resBytes, &result)
	if err != nil {
		return resBytes, types.ContractResult{}, gasReport, err
	}

	return resBytes, result, gasReport, nil
}

// Execute calls a given contract. Since the only difference between contracts with the same Checksum is the
// data in their local storage, and their address in the outside world, we need no ContractID here.
// (That is a detail for the external, sdk-facing, side).
//
// The caller is responsible for passing the correct `store` (which must have been initialized exactly once),
// and setting the env with relevant info on this instance (address, balance, etc).
func (vm *VM) Execute(params api.ContractCallParams) ([]byte, types.ContractResult, types.GasReport, error) {
	// Call the API with the params
	resBytes, gasReport, err := api.Execute(params)
	if err != nil {
		return nil, types.ContractResult{}, gasReport, err
	}

	// Unmarshal the result
	var result types.ContractResult
	err = json.Unmarshal(resBytes, &result)
	if err != nil {
		return resBytes, types.ContractResult{}, gasReport, err
	}

	return resBytes, result, gasReport, nil
}

// Query allows a client to execute a contract-specific query. If the result is not empty, it should be
// valid json-encoded data to return to the client.
// The meaning of path and data can be determined by the code. Path is the suffix of the abci.QueryRequest.Path.
func (vm *VM) Query(params api.ContractCallParams) ([]byte, types.QueryResult, types.GasReport, error) {
	// Call the API with the params
	resBytes, gasReport, err := api.Query(params)
	if err != nil {
		return nil, types.QueryResult{}, gasReport, err
	}

	// Unmarshal the query result
	var result types.QueryResult
	err = json.Unmarshal(resBytes, &result)
	if err != nil {
		return resBytes, types.QueryResult{}, gasReport, err
	}

	return resBytes, result, gasReport, nil
}

// Migrate will migrate an existing contract to a new code binary.
// This takes storage of the data from the original contract and the Checksum of the new contract that should
// replace it. This allows it to run a migration step if needed, or return an error if unable to migrate
// the given data.
//
// MigrateMsg has some data on how to perform the migration.
func (vm *VM) Migrate(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.Migrate(params)
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
func (vm *VM) MigrateWithInfo(codeID []byte, env []byte, msg []byte, migrateInfo []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.MigrateWithInfoParams{
		ContractCallParams: api.ContractCallParams{
			Cache:      vm.cache,
			Checksum:   codeID,
			Env:        env,
			Msg:        msg,
			GasMeter:   gasMeter,
			Store:      store,
			API:        goapi,
			Querier:    querier,
			GasLimit:   gasLimit,
			PrintDebug: printDebug,
		},
		MigrateInfo: migrateInfo,
	}
	return api.MigrateWithInfo(params)
}

// Sudo allows native Go modules to make privileged (sudo) calls on the contract.
// The contract can expose entry points that cannot be triggered by any transaction, but only via
// native Go modules, and delegate the access control to the system.
//
// These work much like Migrate (same scenario) but allows custom apps to extend the privileged entry points
// without forking cosmwasm-vm.
func (vm *VM) Sudo(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.Sudo(params)
}

// Reply allows the native Go wasm modules to make a privileged call to return the result
// of executing a SubMsg.
//
// These work much like Sudo (same scenario) but focuses on one specific case (and one message type).
func (vm *VM) Reply(codeID []byte, env []byte, reply []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        reply,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.Reply(params)
}

// IBCChannelOpen is available on IBC-enabled contracts and is a hook to call into
// during the handshake pahse.
func (vm *VM) IBCChannelOpen(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCChannelOpen(params)
}

// IBCChannelConnect is available on IBC-enabled contracts and is a hook to call into
// during the handshake pahse.
func (vm *VM) IBCChannelConnect(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCChannelConnect(params)
}

// IBCChannelClose is available on IBC-enabled contracts and is a hook to call into
// at the end of the channel lifetime.
func (vm *VM) IBCChannelClose(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCChannelClose(params)
}

// IBCPacketReceive is available on IBC-enabled contracts and is called when an incoming
// packet is received on a channel belonging to this contract.
func (vm *VM) IBCPacketReceive(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCPacketReceive(params)
}

// IBCPacketAck is available on IBC-enabled contracts and is called when an
// the response for an outgoing packet (previously sent by this contract)
// is received.
func (vm *VM) IBCPacketAck(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCPacketAck(params)
}

// IBCPacketTimeout is available on IBC-enabled contracts and is called when an
// outgoing packet (previously sent by this contract) will provably never be executed.
// Usually handled like ack returning an error.
func (vm *VM) IBCPacketTimeout(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCPacketTimeout(params)
}

// IBCSourceCallback is available on IBC-enabled contracts with the corresponding entrypoint
// and should be called when the response (ack or timeout) for an outgoing callbacks-enabled packet
// (previously sent by this contract) is received.
func (vm *VM) IBCSourceCallback(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBCSourceCallback(params)
}

// IBCDestinationCallback is available on IBC-enabled contracts with the corresponding entrypoint
// and should be called when an incoming callbacks-enabled IBC packet is received.
func (vm *VM) IBCDestinationCallback(
	checksum types.Checksum,
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

	// First instantiate the contract
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBin,
		Info:       nil, // Info is not needed for instantiation in this case
		Msg:        msgBin,
		GasMeter:   &gasMeter,
		Store:      store,
		API:        &goapi,
		Querier:    &querier,
		GasLimit:   gasLimit,
		PrintDebug: vm.printDebug,
	}

	data, gasReport, err := api.Instantiate(params)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	// Deserialize the response into a ContractResult
	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return nil, gasReport.UsedInternally, err
	}

	// Convert ContractResult to IBCBasicResult
	var ibcResult types.IBCBasicResult
	if result.Err != "" {
		ibcResult.Err = result.Err
	} else if result.Ok != nil {
		ibcResult.Ok = &types.IBCBasicResponse{
			Messages:   result.Ok.Messages,
			Attributes: result.Ok.Attributes,
			Events:     result.Ok.Events,
		}
	}

	return &ibcResult, gasReport.UsedInternally, nil
}

// IBC2PacketReceive is available on IBC-enabled contracts and is called when an incoming
// packet is received on a channel belonging to this contract
func (vm *VM) IBC2PacketReceive(codeID []byte, env []byte, msg []byte, gasMeter *GasMeter, store KVStore, goapi *GoAPI, querier *Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   codeID,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}
	return api.IBC2PacketReceive(params)
}

func compileCost(code WasmCode) uint64 {
	// CostPerByte is how much CosmWasm gas is charged *per byte* for compiling WASM code.
	// Benchmarks and numbers (in SDK Gas) were discussed in:
	// https://github.com/CosmWasm/wasmd/pull/634#issuecomment-938056803
	const costPerByte = 3 * 140_000

	return costPerByte * uint64(len(code))
}

// hasSubMessages is an interface for contract results that can contain sub-messages.
type hasSubMessages interface {
	SubMessages() []types.SubMsg
}

// make sure the types implement the interface
// cannot put these next to the types, as the interface is private.
var (
	_ hasSubMessages = (*types.IBCBasicResult)(nil)
	_ hasSubMessages = (*types.IBCReceiveResult)(nil)
	_ hasSubMessages = (*types.ContractResult)(nil)
)

// DeserializeResponse deserializes a response
func DeserializeResponse(gasLimit uint64, deserCost types.UFraction, gasReport *types.GasReport, data []byte, response any) error {
	if len(data) == 0 {
		return errors.New("empty response data")
	}

	gasForDeserialization := deserCost.Mul(uint64(len(data))).Floor()
	if gasForDeserialization > gasLimit {
		return errors.New("gas limit exceeded for deserialization")
	}

	if err := json.Unmarshal(data, response); err != nil {
		return fmt.Errorf("failed to deserialize response: %w", err)
	}

	if gasReport != nil {
		gasReport.UsedInternally += gasForDeserialization
		gasReport.Remaining -= gasForDeserialization
	}

	return nil
}
