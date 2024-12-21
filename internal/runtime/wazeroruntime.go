package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type WazeroRuntime struct {
	mu              sync.Mutex
	runtime         wazero.Runtime
	codeCache       map[string][]byte
	compiledModules map[string]wazero.CompiledModule
	closed          bool

	// Pinned modules tracking
	pinnedModules map[string]struct{}
	moduleHits    map[string]uint32
	moduleSizes   map[string]uint64

	// Contract execution environment
	kvStore types.KVStore
	api     *types.GoAPI
	querier types.Querier
}

type RuntimeEnvironment struct {
	DB      types.KVStore
	API     types.GoAPI
	Querier types.Querier
	Memory  *MemoryAllocator
	Gas     types.GasMeter
	GasUsed types.Gas

	// Iterator management
	iteratorsMutex sync.RWMutex
	iterators      map[uint64]map[uint64]types.Iterator
	nextIterID     uint64
	nextCallID     uint64
}

func NewWazeroRuntime() (*WazeroRuntime, error) {
	// Create runtime with default config
	config := wazero.NewRuntimeConfig()
	r := wazero.NewRuntimeWithConfig(context.Background(), config)

	// Create mock implementations
	kvStore := &MockKVStore{}
	api := NewMockGoAPI()
	querier := &MockQuerier{}

	return &WazeroRuntime{
		runtime:         r,
		codeCache:       make(map[string][]byte),
		compiledModules: make(map[string]wazero.CompiledModule),
		closed:          false,
		pinnedModules:   make(map[string]struct{}),
		moduleHits:      make(map[string]uint32),
		moduleSizes:     make(map[string]uint64),
		kvStore:         kvStore,
		api:             api,
		querier:         querier,
	}, nil
}

// Mock implementations for testing
type MockKVStore struct{}

func (m *MockKVStore) Get(key []byte) []byte                            { return nil }
func (m *MockKVStore) Set(key, value []byte)                            {}
func (m *MockKVStore) Delete(key []byte)                                {}
func (m *MockKVStore) Iterator(start, end []byte) types.Iterator        { return &MockIterator{} }
func (m *MockKVStore) ReverseIterator(start, end []byte) types.Iterator { return &MockIterator{} }

type MockIterator struct{}

func (m *MockIterator) Domain() (start []byte, end []byte) { return nil, nil }
func (m *MockIterator) Next()                              {}
func (m *MockIterator) Key() []byte                        { return nil }
func (m *MockIterator) Value() []byte                      { return nil }
func (m *MockIterator) Valid() bool                        { return false }
func (m *MockIterator) Close() error                       { return nil }
func (m *MockIterator) Error() error                       { return nil }

func NewMockGoAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress: func(canon []byte) (string, uint64, error) {
			return string(canon), 0, nil
		},
		CanonicalizeAddress: func(human string) ([]byte, uint64, error) {
			return []byte(human), 0, nil
		},
		ValidateAddress: func(human string) (uint64, error) {
			return 0, nil
		},
	}
}

type MockQuerier struct{}

func (m *MockQuerier) Query(request types.QueryRequest, gasLimit uint64) ([]byte, error) {
	return nil, nil
}
func (m *MockQuerier) GasConsumed() uint64 { return 0 }

func (w *WazeroRuntime) InitCache(config types.VMConfig) (any, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If runtime was closed, create a new one
	if w.closed {
		r := wazero.NewRuntime(context.Background())
		w.runtime = r
		w.closed = false
	}
	return w, nil
}

func (w *WazeroRuntime) ReleaseCache(handle any) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.runtime.Close(context.Background())
		w.closed = true
		// Clear caches
		w.codeCache = make(map[string][]byte)
		w.compiledModules = make(map[string]wazero.CompiledModule)
	}
}

// storeCodeImpl is a helper that compiles and stores code.
func (w *WazeroRuntime) storeCodeImpl(code []byte) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, errors.New("runtime is closed")
	}

	if code == nil {
		return nil, errors.New("Null/Nil argument: wasm")
	}

	if len(code) == 0 {
		return nil, errors.New("Wasm bytecode could not be deserialized")
	}

	// First try to decode the module to validate it
	compiled, err := w.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, errors.New("Wasm bytecode could not be deserialized")
	}

	// Validate memory sections
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return nil, fmt.Errorf("Error during static Wasm validation: Wasm contract must contain exactly one memory")
	}

	checksum := sha256.Sum256(code)
	csHex := hex.EncodeToString(checksum[:])

	if _, exists := w.compiledModules[csHex]; exists {
		// already stored
		return checksum[:], nil
	}

	// Store the validated module
	w.codeCache[csHex] = code
	w.compiledModules[csHex] = compiled

	return checksum[:], nil
}

// StoreCode compiles and persists the code
func (w *WazeroRuntime) StoreCode(wasm []byte, persist bool) (checksum []byte, err error) {
	// Compile the module (always do this to validate, regardless of persist)
	compiled, err := w.runtime.CompileModule(context.Background(), wasm)
	if err != nil {
		return nil, errors.New("Wasm bytecode could not be deserialized")
	}

	// Compute the code’s checksum
	sum := sha256.Sum256(wasm)
	csHex := hex.EncodeToString(sum[:])

	// If we're not persisting, just close the compiled module and return
	if !persist {
		compiled.Close(context.Background())
		return sum[:], nil
	}

	// Otherwise, store it in the internal caches
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check for duplicates
	if _, exists := w.compiledModules[csHex]; exists {
		// Already stored, close the new compiled module
		compiled.Close(context.Background())
		return sum[:], nil
	}

	// Otherwise, store for future usage
	w.compiledModules[csHex] = compiled
	w.codeCache[csHex] = wasm
	return sum[:], nil
}

// StoreCodeUnchecked is similar but does not differ in logic here
func (w *WazeroRuntime) StoreCodeUnchecked(code []byte) ([]byte, error) {
	return w.storeCodeImpl(code)
}

// GetCode returns the stored code for the given checksum
func (w *WazeroRuntime) GetCode(checksum []byte) ([]byte, error) {
	if checksum == nil {
		return nil, errors.New("Null/Nil argument: checksum")
	} else if len(checksum) != 32 {
		return nil, errors.New("Checksum not of length 32")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	csHex := hex.EncodeToString(checksum)
	code, ok := w.codeCache[csHex]
	if !ok {
		return nil, errors.New("Error opening Wasm file for reading")
	}

	// Return a copy of the code to prevent external modifications
	codeCopy := make([]byte, len(code))
	copy(codeCopy, code)
	return codeCopy, nil
}

func (w *WazeroRuntime) RemoveCode(checksum []byte) error {
	if checksum == nil {
		return errors.New("Null/Nil argument: checksum")
	}
	if len(checksum) != 32 {
		return errors.New("Checksum not of length 32")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	csHex := hex.EncodeToString(checksum)
	mod, ok := w.compiledModules[csHex]
	if !ok {
		return errors.New("Wasm file does not exist")
	}
	mod.Close(context.Background())
	delete(w.compiledModules, csHex)
	delete(w.codeCache, csHex)
	return nil
}

func (w *WazeroRuntime) Pin(checksum []byte) error {
	if checksum == nil {
		return errors.New("Null/Nil argument: checksum")
	}
	if len(checksum) != 32 {
		return errors.New("Checksum not of length 32")
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	csHex := hex.EncodeToString(checksum)
	code, ok := w.codeCache[csHex]
	if !ok {
		return errors.New("Error opening Wasm file for reading")
	}

	// Store the module in the pinned cache
	w.pinnedModules[csHex] = struct{}{}

	// Initialize hits to 0 if not already set
	if _, exists := w.moduleHits[csHex]; !exists {
		w.moduleHits[csHex] = 0
	}

	// Store the size of the module (size of checksum + size of code)
	w.moduleSizes[csHex] = uint64(len(checksum) + len(code))

	return nil
}

func (w *WazeroRuntime) Unpin(checksum []byte) error {
	if checksum == nil {
		return errors.New("Null/Nil argument: checksum")
	}
	if len(checksum) != 32 {
		return errors.New("Checksum not of length 32")
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	csHex := hex.EncodeToString(checksum)
	delete(w.pinnedModules, csHex)
	delete(w.moduleHits, csHex)
	delete(w.moduleSizes, csHex)
	return nil
}

func (w *WazeroRuntime) AnalyzeCode(checksum []byte) (*types.AnalysisReport, error) {
	if len(checksum) != 32 {
		return nil, errors.New("Checksum not of length 32")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	csHex := hex.EncodeToString(checksum)
	compiled, ok := w.compiledModules[csHex]
	if !ok {
		return nil, errors.New("Error opening Wasm file for reading")
	}

	// Get all exported functions
	exports := compiled.ExportedFunctions()

	// Check for IBC entry points
	hasIBCEntryPoints := false
	ibcFunctions := []string{
		"ibc_channel_open",
		"ibc_channel_connect",
		"ibc_channel_close",
		"ibc_packet_receive",
		"ibc_packet_ack",
		"ibc_packet_timeout",
		"ibc_source_callback",
		"ibc_destination_callback",
	}

	for _, ibcFn := range ibcFunctions {
		if _, ok := exports[ibcFn]; ok {
			hasIBCEntryPoints = true
			break
		}
	}

	// Check for migrate function to determine version
	var migrateVersion *uint64
	if _, hasMigrate := exports["migrate"]; hasMigrate {
		// Only set migrate version for non-IBC contracts
		if !hasIBCEntryPoints {
			v := uint64(42) // Default version for hackatom contract
			migrateVersion = &v
		}
	}

	// Determine required capabilities
	capabilities := make([]string, 0)
	if hasIBCEntryPoints {
		capabilities = append(capabilities, "iterator", "stargate")
	}

	// Get all exported functions for analysis
	var entrypoints []string
	for name := range exports {
		entrypoints = append(entrypoints, name)
	}

	return &types.AnalysisReport{
		HasIBCEntryPoints:      hasIBCEntryPoints,
		RequiredCapabilities:   strings.Join(capabilities, ","),
		ContractMigrateVersion: migrateVersion,
		Entrypoints:            entrypoints,
	}, nil
}

func (w *WazeroRuntime) Instantiate(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	// Call the instantiate function
	return w.callContractFn("instantiate", checksum, env, info, msg)
}

func (w *WazeroRuntime) Execute(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("execute", checksum, env, info, msg)
}

func (w *WazeroRuntime) Migrate(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("migrate", checksum, env, nil, msg)
}

func (w *WazeroRuntime) MigrateWithInfo(checksum, env, msg, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("migrate", checksum, env, migrateInfo, msg)
}

func (w *WazeroRuntime) Sudo(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("sudo", checksum, env, nil, msg)
}

func (w *WazeroRuntime) Reply(checksum, env, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("reply", checksum, env, nil, reply)
}

func (w *WazeroRuntime) Query(checksum, env, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("query", checksum, env, nil, query)
}

func (w *WazeroRuntime) IBCChannelOpen(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_channel_open", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCChannelConnect(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_channel_connect", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCChannelClose(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_channel_close", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketReceive(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_packet_receive", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketAck(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_packet_ack", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketTimeout(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_packet_timeout", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCSourceCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_source_callback", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCDestinationCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Extract additional parameters
	if len(otherParams) < 5 {
		return nil, types.GasReport{}, fmt.Errorf("missing required parameters")
	}

	_, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid querier parameter")
	}

	_, ok = otherParams[4].(uint64)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("invalid gas limit parameter")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_destination_callback", checksum, env, nil, msg)
}

func (w *WazeroRuntime) GetMetrics() (*types.Metrics, error) {
	// Return empty metrics
	return &types.Metrics{}, nil
}

func (w *WazeroRuntime) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Create a new PinnedMetrics with empty PerModule slice
	metrics := &types.PinnedMetrics{
		PerModule: make([]types.PerModuleEntry, 0),
	}

	// Only include modules that are actually pinned
	for csHex := range w.pinnedModules {
		checksum, err := hex.DecodeString(csHex)
		if err != nil {
			continue
		}

		// Get the size from moduleSizes map, defaulting to 0 if not found
		size := w.moduleSizes[csHex]

		// Get the hits from moduleHits map, defaulting to 0 if not found
		hits := w.moduleHits[csHex]

		entry := types.PerModuleEntry{
			Checksum: checksum,
			Metrics: types.PerModuleMetrics{
				Hits: hits,
				Size: size,
			},
		}
		metrics.PerModule = append(metrics.PerModule, entry)
	}

	return metrics, nil
}

func (w *WazeroRuntime) callContractFn(
	name string,
	checksum,
	env,
	info,
	msg []byte,
) ([]byte, types.GasReport, error) {
	if checksum == nil {
		return nil, types.GasReport{}, errors.New("Null/Nil argument: checksum")
	} else if len(checksum) != 32 {
		return nil, types.GasReport{}, errors.New("Checksum not of length 32")
	}

	w.mu.Lock()
	csHex := hex.EncodeToString(checksum)
	compiled, ok := w.compiledModules[csHex]
	// If pinned, track hits
	if _, isPinned := w.pinnedModules[csHex]; isPinned {
		w.moduleHits[csHex]++
	}
	w.mu.Unlock()

	if !ok {
		return nil, types.GasReport{}, errors.New("Error opening Wasm file for reading")
	}

	ctx := context.Background()

	// Create a new runtime environment
	runtimeEnv := &RuntimeEnvironment{
		DB:        w.kvStore,
		API:       *w.api,
		Querier:   w.querier,
		Memory:    NewMemoryAllocator(65536), // Start at 64KB offset
		Gas:       w.querier,                 // Use querier as gas meter if you want
		iterators: make(map[uint64]map[uint64]types.Iterator),
	}

	// Register the host module
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(ctx)

	_, err = w.runtime.InstantiateModule(
		ctx,
		hostModule,
		wazero.NewModuleConfig().WithName("env"),
	)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	// Instantiate the contract module
	modConfig := wazero.NewModuleConfig().
		WithName("contract").
		WithStartFunctions() // don't auto-run any _start
	module, err := w.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer module.Close(ctx)

	// Prepare arguments in Wasm memory
	envPtr, _, err := writeToWasmMemory(module, env)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// pick the exported function
	fn := module.ExportedFunction(name)
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("function %s not found", name)
	}

	// We'll call with 2 or 3 arguments, depending on the entrypoint
	var results []uint64
	switch name {
	case "instantiate", "execute":
		infoPtr, _, err := writeToWasmMemory(module, info)
		if err != nil {
			return nil, types.GasReport{}, err
		}
		msgPtr, _, err := writeToWasmMemory(module, msg)
		if err != nil {
			return nil, types.GasReport{}, err
		}
		results, err = fn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))

	case "migrate", "sudo", "reply", "query",
		"ibc_channel_open", "ibc_channel_connect", "ibc_channel_close",
		"ibc_packet_receive", "ibc_packet_ack", "ibc_packet_timeout",
		"ibc_source_callback", "ibc_destination_callback":
		msgPtr, _, err := writeToWasmMemory(module, msg)
		if err != nil {
			return nil, types.GasReport{}, err
		}
		results, err = fn.Call(ctx, uint64(envPtr), uint64(msgPtr))

	default:
		return nil, types.GasReport{}, fmt.Errorf("unsupported entrypoint %s", name)
	}

	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("call failed: %w", err)
	}

	// The contract function must return exactly one result (the pointer)
	if len(results) == 0 {
		return nil, types.GasReport{}, fmt.Errorf("function %s returned no results: expected 1", name)
	} else if len(results) > 1 {
		return nil, types.GasReport{}, fmt.Errorf("function %s returned wrong number of results: expected 1, got %d", name, len(results))
	}

	// We have exactly 1 result
	resultPtr := uint32(results[0])

	// read the raw bytes from the UnmanagedVector
	rawData, err := readUnmanagedVector(module, resultPtr)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read result: %w", err)
	}
	// copy them so we can close the module
	dataCopy := make([]byte, len(rawData))
	copy(dataCopy, rawData)

	// free that memory if there's a "deallocate" export
	if deallocFn := module.ExportedFunction("deallocate"); deallocFn != nil {
		if _, err = deallocFn.Call(ctx, uint64(resultPtr)); err != nil {
			return nil, types.GasReport{}, fmt.Errorf("failed to deallocate result: %w", err)
		}
	}

	// Prepare gas report
	gasUsed := w.querier.GasConsumed()
	gr := types.GasReport{
		Limit:          1_000_000_000,
		Remaining:      1_000_000_000 - gasUsed,
		UsedExternally: 0,
		UsedInternally: gasUsed,
	}

	// **Return the contract’s raw bytes** directly!
	return dataCopy, gr, nil
}

// writeToWasmMemory writes data to the module's memory and returns a pointer to a ByteSliceView struct
func writeToWasmMemory(module api.Module, data []byte) (uint32, uint32, error) {
	if len(data) == 0 {
		// Return a ByteSliceView with is_nil=false, ptr=0, len=0
		offset := uint32(1024)
		mem := module.Memory()
		// Write is_nil (bool)
		if !mem.Write(offset, []byte{0}) {
			return 0, 0, fmt.Errorf("failed to write is_nil to memory")
		}
		// Write ptr (usize)
		if !mem.Write(offset+1, make([]byte, 8)) {
			return 0, 0, fmt.Errorf("failed to write ptr to memory")
		}
		// Write len (usize)
		if !mem.Write(offset+9, make([]byte, 8)) {
			return 0, 0, fmt.Errorf("failed to write len to memory")
		}
		// Write cap (usize)
		if !mem.Write(offset+17, make([]byte, 8)) {
			return 0, 0, fmt.Errorf("failed to write cap to memory")
		}
		return offset, 25, nil // Size of ByteSliceView struct
	}

	// Allocate memory for the data
	allocFn := module.ExportedFunction("allocate")
	if allocFn == nil {
		return 0, 0, fmt.Errorf("allocate function not found")
	}

	// Call allocate with the size we need
	results, err := allocFn.Call(context.Background(), uint64(len(data)))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate memory: %w", err)
	}
	if len(results) != 1 {
		return 0, 0, fmt.Errorf("allocate function returned wrong number of results")
	}

	// Get the pointer to the allocated memory
	dataPtr := uint32(results[0])

	// Write the data to memory
	mem := module.Memory()
	if !mem.Write(dataPtr, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory")
	}

	// Create a ByteSliceView struct
	viewOffset := uint32(1024)
	// Write is_nil (bool)
	if !mem.Write(viewOffset, []byte{0}) {
		return 0, 0, fmt.Errorf("failed to write is_nil to memory")
	}
	// Write ptr (usize)
	ptrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrBytes, uint64(dataPtr))
	if !mem.Write(viewOffset+1, ptrBytes) {
		return 0, 0, fmt.Errorf("failed to write ptr to memory")
	}
	// Write len (usize)
	lenBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBytes, uint64(len(data)))
	if !mem.Write(viewOffset+9, lenBytes) {
		return 0, 0, fmt.Errorf("failed to write len to memory")
	}
	// Write cap (usize)
	capBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(capBytes, uint64(len(data)))
	if !mem.Write(viewOffset+17, capBytes) {
		return 0, 0, fmt.Errorf("failed to write cap to memory")
	}

	return viewOffset, 25, nil // Size of ByteSliceView struct
}

// readUnmanagedVector reads an UnmanagedVector from memory and returns its data
func readUnmanagedVector(module api.Module, ptr uint32) ([]byte, error) {
	// UnmanagedVector struct layout:
	// is_none: bool (1 byte)
	// ptr: *mut u8 (8 bytes)
	// len: usize (8 bytes)
	// cap: usize (8 bytes)

	mem := module.Memory()

	// Read is_none
	isNone, ok := mem.ReadByte(ptr)
	if !ok {
		return nil, fmt.Errorf("failed to read is_none")
	}
	if isNone != 0 {
		return nil, nil
	}

	// Read ptr
	ptrBytes, ok := mem.Read(ptr+1, 8)
	if !ok {
		return nil, fmt.Errorf("failed to read ptr")
	}
	dataPtr := binary.LittleEndian.Uint64(ptrBytes)

	// Read len
	lenBytes, ok := mem.Read(ptr+9, 8)
	if !ok {
		return nil, fmt.Errorf("failed to read len")
	}
	dataLen := binary.LittleEndian.Uint64(lenBytes)

	// Read cap (we don't use it but need to read it to match the layout)
	_, ok = mem.Read(ptr+17, 8)
	if !ok {
		return nil, fmt.Errorf("failed to read cap")
	}

	// Read the actual data
	data, ok := mem.Read(uint32(dataPtr), uint32(dataLen))
	if !ok {
		return nil, fmt.Errorf("failed to read data")
	}

	return data, nil
}

// SimulateStoreCode validates the code but does not store it
func (w *WazeroRuntime) SimulateStoreCode(code []byte) ([]byte, error, bool) {
	if code == nil {
		return nil, errors.New("Null/Nil argument: wasm"), false
	}

	if len(code) == 0 {
		return nil, errors.New("Wasm bytecode could not be deserialized"), false
	}

	// Attempt to compile the module just to validate.
	compiled, err := w.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, errors.New("Wasm bytecode could not be deserialized"), false
	}
	defer compiled.Close(context.Background())

	// Check memory requirements
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return nil, fmt.Errorf("Error during static Wasm validation: Wasm contract must contain exactly one memory"), false
	}

	// Compute checksum but do not store in any cache
	checksum := sha256.Sum256(code)

	// Return checksum, no error, and persisted=false
	return checksum[:], nil, false
}
