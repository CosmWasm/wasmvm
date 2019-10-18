package cosmwasm

import (
	"encoding/json"

	"github.com/confio/go-cosmwasm/api"
)

// ContractID represents an ID for a contract, must be generated from this library
type ContractID []byte

// WasmCode is an alias for raw bytes of the wasm compiled code
type WasmCode []byte

// KVStore is a reference to some sub-kvstore that is valid for one instance of a contract
type KVStore = api.KVStore

// Wasmer is the main entry point to this library.
// You should create an instance with it's own subdirectory to manage state inside,
// and call it for all cosmwasm contract related actions.
type Wasmer struct {
	dataDir string
}

// NewWasmer creates an new binding, with the given dataDir where
// it can store raw wasm and the pre-compile cache
func NewWasmer(dataDir string) *Wasmer {
	// TODO: at least double-check this dir exists and we can write to this dir (or panic?)
	return &Wasmer{dataDir: dataDir}
}

// Create will compile the wasm code, and store the resulting pre-compile
// as well as the original code. Both can be referenced later via ContractID
// This must be done one time for given code, after which it can be
// instatitated many times, and each instance called many times.
//
// TODO: return gas cost? Add gas limit??? there is no metering here...
func (w *Wasmer) Create(contract WasmCode) (ContractID, error) {
	return api.Create(w.dataDir, contract)
}

// Instantiate will create a new instance of a contract using the given contractID.
// storage should be set with a PrefixedKVStore that this contract can safely access.
//
// Under the hood, we may recompile the wasm, use a cached native compile, or even use a cached instance
// for performance.
//
// TODO: clarify which errors are returned? vm failure. out of gas. contract unauthorized.
func (w *Wasmer) Instantiate(contract ContractID, params Params, userMsg []byte, store KVStore, gasLimit int64) (*Result, error) {
	paramBin, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	data, err := api.Instantiate(w.dataDir, contract, paramBin, userMsg, store, gasLimit)
	if err != nil {
		return nil, err
	}
	var res Result
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Handle calls a given instance of the contract. Since the only difference between the instances is the data in their
// local storage, and their address in the outside world, we need no InstanceID.
// The caller is responsible for passing the correct `store` (which must have been initialized exactly once),
// and setting the params with relevent info on this instance (address, balance, etc)
func (w *Wasmer) Handle(contract ContractID, params Params, userMsg []byte, store KVStore, gasLimit int64) (*Result, error) {
	paramBin, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	data, err := api.Handle(w.dataDir, contract, paramBin, userMsg, store, gasLimit)
	if err != nil {
		return nil, err
	}
	var res Result
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Query allows a client to execute a contract-specific query. If the result is not empty, it should be
// valid json-encoded data to return to the client.
// The meaning of path and data can be determined by the contract. Path is the suffix of the abci.QueryRequest.Path
func (w *Wasmer) Query(contract ContractID, path []byte, data []byte, store KVStore, gasLimit int64) ([]byte, error) {
	panic("unimplemented!")
	// TODO
	return nil, nil
}
