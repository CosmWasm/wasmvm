package cosmwasm

// import "github.com/confio/go-cosmwasm/api"

// ContractID represents an ID for a contract, must be generated from this library
type ContractID []byte

// WasmCode is an alias for raw bytes of the wasm compiled code
type WasmCode []byte

// StorageCallback is a reference to some sub-kvstore that is valid for one instance of a contract
type StorageCallback interface {
	Get(key []byte) []byte
	Set(key, value []byte)
}

// Wasmer is the main entry point to this library.
// You should create an instance with it's own subdirectory to manage state inside,
// and call it for all cosmwasm contract related actions.
type Wasmer struct {
	// TODO
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
	// TODO
	return nil, nil
}

// Instantiate will create a new instance of a contract using the given contractID.
// storage should be set with a PrefixedKVStore that this contract can safely access.
//
// Under the hood, we may recompile the wasm, use a cached native compile, or even use a cached instance
// for performance.
//
// TODO: use a real struct for params and json encode it here
// TODO: use a real struct for res and json encode it here
//
// TODO: is there a way to simplify the arguments here? `gasAvailable *int64` and modify in place???
// TODO: clarify which errors are returned? vm failure. out of gas. contract unauthorized.
func (w *Wasmer) Instantiate(contract ContractID, params Params, userMsg []byte, storage StorageCallback, gasLimit int64) (res *Result, err error) {
	return nil, nil
}

// Handle calls a given instance of the contract. Since the only difference between the instances is the data in their
// local storage, and their address in the outside world, we need no InstanceID.
// The caller is responsible for passing the correct `storage` (which must have been initialized exactly once),
// and setting the params with relevent info on this instance (address, balance, etc)
func (w *Wasmer) Handle(contract ContractID, params Params, userMsg []byte, storage StorageCallback, gasLimit int64) (res *Result, err error) {
	return nil, nil
}
