package cosmwasm

import (
	"encoding/json"
	"fmt"

	"github.com/confio/go-cosmwasm/api"
	"github.com/confio/go-cosmwasm/types"
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
	cache api.Cache
}

// NewWasmer creates an new binding, with the given dataDir where
// it can store raw wasm and the pre-compile cache
func NewWasmer(dataDir string, cacheSize uint64) (*Wasmer, error) {
	cache, err := api.InitCache(dataDir, cacheSize)
	if err != nil {
		return nil, err
	}
	return &Wasmer{cache: cache}, nil
}

// Cleanup should be called when no longer using this to free resources on the rust-side
func (w *Wasmer)Cleanup() {
	api.ReleaseCache(w.cache)
}

// Create will compile the wasm code, and store the resulting pre-compile
// as well as the original code. Both can be referenced later via ContractID
// This must be done one time for given code, after which it can be
// instatitated many times, and each instance called many times.
//
// TODO: return gas cost? Add gas limit??? there is no metering here...
func (w *Wasmer) Create(contract WasmCode) (ContractID, error) {
	return api.Create(w.cache, contract)
}

// GetCode will load the original wasm code for the given contract id.
// This will only succeed if that contract id was previously returned from
// a call to Create.
//
// This can be used so that the (short) contract id (hash) is stored in the iavl tree
// and the larger binary blobs (wasm and pre-compiles) are all managed by the
// rust library
func (w *Wasmer) GetCode(contract ContractID) (WasmCode, error) {
	return api.GetCode(w.cache, contract)
}

// Instantiate will create a new instance of a contract using the given contractID.
// storage should be set with a PrefixedKVStore that this contract can safely access.
//
// Under the hood, we may recompile the wasm, use a cached native compile, or even use a cached instance
// for performance.
//
// TODO: clarify which errors are returned? vm failure. out of gas. contract unauthorized.
// TODO: add callback for querying into other modules
func (w *Wasmer) Instantiate(contract ContractID, params types.Params, userMsg []byte, store KVStore, gasLimit int64) (*types.Result, error) {
	paramBin, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	data, err := api.Instantiate(w.cache, contract, paramBin, userMsg, store, gasLimit)
	if err != nil {
		return nil, err
	}

	var resp types.CosmosResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Err != "" {
		return nil, fmt.Errorf(resp.Err)
	}
	return &resp.Ok, nil
}

// Handle calls a given instance of the contract. Since the only difference between the instances is the data in their
// local storage, and their address in the outside world, we need no InstanceID.
// The caller is responsible for passing the correct `store` (which must have been initialized exactly once),
// and setting the params with relevent info on this instance (address, balance, etc)
//
// TODO: add callback for querying into other modules
func (w *Wasmer) Handle(contract ContractID, params types.Params, userMsg []byte, store KVStore, gasLimit int64) (*types.Result, error) {
	paramBin, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	data, err := api.Handle(w.cache, contract, paramBin, userMsg, store, gasLimit)
	if err != nil {
		return nil, err
	}

	var resp types.CosmosResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Err != "" {
		return nil, fmt.Errorf(resp.Err)
	}
	return &resp.Ok, nil
}

// Query allows a client to execute a contract-specific query. If the result is not empty, it should be
// valid json-encoded data to return to the client.
// The meaning of path and data can be determined by the contract. Path is the suffix of the abci.QueryRequest.Path
func (w *Wasmer) Query(contract ContractID, path []byte, data []byte, store KVStore, gasLimit int64) ([]byte, error) {
	panic("unimplemented!")
	// TODO
	return nil, nil
}
