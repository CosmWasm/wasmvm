package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/CosmWasm/wasmvm/v2/types"
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

// RuntimeEnvironment holds all execution context for the contract
type RuntimeEnvironment struct {
	DB       types.KVStore
	API      types.GoAPI
	Querier  types.Querier
	Gas      types.GasMeter
	gasLimit uint64 // Maximum gas that can be used
	gasUsed  uint64 // Current gas usage

	// Iterator management
	iteratorsMutex sync.RWMutex
	iterators      map[uint64]map[uint64]types.Iterator
	nextIterID     uint64
	nextCallID     uint64
}

func NewWazeroRuntime() (*WazeroRuntime, error) {
	// Create a new wazero runtime with memory configuration
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(maxMemoryPages). // Set max memory to 128 MiB (2048 * 64KB)
		WithMemoryCapacityFromMax(true).      // Eagerly allocate memory to ensure it's initialized
		WithDebugInfoEnabled(true)            // Enable debug info

	r := wazero.NewRuntimeWithConfig(context.Background(), runtimeConfig)

	// Create mock implementations
	kvStore := &MockKVStore{}
	api := NewMockGoAPI()
	querier := &MockQuerier{}

	return &WazeroRuntime{
		runtime:         r,
		codeCache:       make(map[string][]byte),
		compiledModules: make(map[string]wazero.CompiledModule),
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
		return nil, errors.New("Null/Nil argument: wasm")
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

func (w *WazeroRuntime) StoreCode(wasm []byte, persist bool) ([]byte, error) {
	if wasm == nil {
		return nil, errors.New("Null/Nil argument: wasm")
	}

	if len(wasm) == 0 {
		return nil, errors.New("Wasm bytecode could not be deserialized")
	}

	compiled, err := w.runtime.CompileModule(context.Background(), wasm)
	if err != nil {
		return nil, errors.New("Wasm bytecode could not be deserialized")
	}

	// Here is where we do the static checks
	if err := w.analyzeForValidation(compiled); err != nil {
		compiled.Close(context.Background())
		return nil, fmt.Errorf("static validation failed: %w", err)
	}

	sum := sha256.Sum256(wasm)
	csHex := hex.EncodeToString(sum[:])

	if !persist {
		// just close the compiled module
		compiled.Close(context.Background())
		return sum[:], nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.compiledModules[csHex]; exists {
		compiled.Close(context.Background())
		return sum[:], nil
	}

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
		"ibc_destination_callback",
	}

	var entrypoints []string
	for name := range exports {
		entrypoints = append(entrypoints, name)
		for _, ibcFn := range ibcFunctions {
			if name == ibcFn {
				hasIBCEntryPoints = true
				break
			}
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

	return &types.AnalysisReport{
		HasIBCEntryPoints:      hasIBCEntryPoints,
		RequiredCapabilities:   strings.Join(capabilities, ","),
		ContractMigrateVersion: migrateVersion,
		Entrypoints:            entrypoints,
	}, nil
}

// parseParams extracts and validates the common parameters passed to contract functions
func (w *WazeroRuntime) parseParams(otherParams []interface{}) (*types.GasMeter, types.KVStore, *types.GoAPI, *types.Querier, uint64, bool, error) {
	if len(otherParams) < 6 {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("missing required parameters")
	}

	gasMeter, ok := otherParams[0].(*types.GasMeter)
	if !ok {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("invalid gas meter parameter")
	}

	store, ok := otherParams[1].(types.KVStore)
	if !ok {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("invalid store parameter")
	}

	api, ok := otherParams[2].(*types.GoAPI)
	if !ok {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("invalid api parameter")
	}

	querier, ok := otherParams[3].(*types.Querier)
	if !ok {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("invalid querier parameter")
	}

	gasLimit, ok := otherParams[4].(uint64)
	if !ok {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("invalid gas limit parameter")
	}

	printDebug, ok := otherParams[5].(bool)
	if !ok {
		return nil, nil, nil, nil, 0, false, fmt.Errorf("invalid printDebug parameter")
	}

	return gasMeter, store, api, querier, gasLimit, printDebug, nil
}

func (w *WazeroRuntime) Instantiate(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Parse input parameters and create gas state
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to parse params: %w", err)
	}

	// Create gas state for tracking memory operations
	gasState := NewGasState(gasLimit)

	// Initialize runtime environment
	runtimeEnv := &RuntimeEnvironment{
		DB:         store,
		API:        *api,
		Querier:    *querier,
		Gas:        *gasMeter,
		gasLimit:   gasLimit,
		gasUsed:    0,
		iterators:  make(map[uint64]map[uint64]types.Iterator),
		nextCallID: 1,
	}

	// Create context with environment
	ctx := context.WithValue(context.Background(), envKey, runtimeEnv)

	// Register host functions
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(ctx)

	// Create and instantiate environment module
	envModule, err := w.runtime.InstantiateModule(ctx, hostModule,
		wazero.NewModuleConfig().WithName("env").WithStartFunctions())
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Get the contract module
	w.mu.Lock()
	csHex := hex.EncodeToString(checksum)
	compiledModule, ok := w.compiledModules[csHex]
	if !ok {
		w.mu.Unlock()
		return nil, types.GasReport{}, fmt.Errorf("module not found for checksum: %x", checksum)
	}
	w.mu.Unlock()

	// Create and instantiate contract module
	contractModule, err := w.runtime.InstantiateModule(ctx, compiledModule,
		wazero.NewModuleConfig().WithName("contract").WithStartFunctions())
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer contractModule.Close(ctx)

	// Get contract memory
	memory := contractModule.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("contract module has no memory")
	}

	// Initialize memory with one page to avoid null pointer issues
	if memory.Size() == 0 {
		if _, ok := memory.Grow(1); !ok {
			return nil, types.GasReport{}, fmt.Errorf("failed to initialize memory with one page")
		}
	}

	// Initialize memory manager
	memManager := newMemoryManager(memory, gasState)

	// Validate and prepare input data
	if err := validateInputData(env, info, msg); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("invalid input data: %w", err)
	}

	// Write input data to memory
	envPtr, infoPtr, msgPtr, err := writeInputData(memManager, env, info, msg, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write input data: %w", err)
	}

	if printDebug {
		fmt.Printf("Memory layout before instantiate:\n")
		fmt.Printf("- env: ptr=0x%x, size=%d\n", envPtr, len(env))
		fmt.Printf("- info: ptr=0x%x, size=%d\n", infoPtr, len(info))
		fmt.Printf("- msg: ptr=0x%x, size=%d\n", msgPtr, len(msg))
	}

	// Get instantiate function
	instantiate := contractModule.ExportedFunction("instantiate")
	if instantiate == nil {
		return nil, types.GasReport{}, fmt.Errorf("instantiate function not exported")
	}

	// Charge gas for instantiation
	if err := gasState.ConsumeGas(gasState.config.Instantiate, "contract instantiation"); err != nil {
		return nil, types.GasReport{}, err
	}

	// Call instantiate function
	ret, err := instantiate.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("instantiate call failed: %w", err)
	}

	// Validate return value
	if len(ret) != 1 {
		return nil, types.GasReport{}, fmt.Errorf("expected 1 return value, got %d", len(ret))
	}

	// Read and validate result
	resultPtr := uint32(ret[0])
	result, err := readAndValidateResult(memory, resultPtr, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read result: %w", err)
	}

	// Create gas report
	gasReport := types.GasReport{
		UsedInternally: runtimeEnv.gasUsed,
		UsedExternally: gasState.GetGasUsed(),
		Remaining:      gasLimit - (runtimeEnv.gasUsed + gasState.GetGasUsed()),
		Limit:          gasLimit,
	}

	if printDebug {
		fmt.Printf("Gas report:\n")
		fmt.Printf("- Used internally: %d\n", gasReport.UsedInternally)
		fmt.Printf("- Used externally: %d\n", gasReport.UsedExternally)
		fmt.Printf("- Remaining: %d\n", gasReport.Remaining)
		fmt.Printf("- Limit: %d\n", gasReport.Limit)
	}

	return result, gasReport, nil
}

// Helper functions for the Instantiate implementation

func validateInputData(env, info, msg []byte) error {
	if len(env) == 0 {
		return fmt.Errorf("empty environment data")
	}
	if len(info) == 0 {
		return fmt.Errorf("empty info data")
	}
	if len(msg) == 0 {
		return fmt.Errorf("empty message data")
	}

	// Validate env is proper JSON
	var envData map[string]interface{}
	if err := json.Unmarshal(env, &envData); err != nil {
		return fmt.Errorf("invalid environment JSON: %w", err)
	}

	// Validate info is proper JSON
	var infoData map[string]interface{}
	if err := json.Unmarshal(info, &infoData); err != nil {
		return fmt.Errorf("invalid info JSON: %w", err)
	}

	// Validate msg is proper JSON
	var msgData map[string]interface{}
	if err := json.Unmarshal(msg, &msgData); err != nil {
		return fmt.Errorf("invalid message JSON: %w", err)
	}

	// Re-marshal each JSON to ensure consistent formatting
	cleanEnv, err := json.Marshal(envData)
	if err != nil {
		return fmt.Errorf("failed to re-marshal environment JSON: %w", err)
	}
	env = cleanEnv

	cleanInfo, err := json.Marshal(infoData)
	if err != nil {
		return fmt.Errorf("failed to re-marshal info JSON: %w", err)
	}
	info = cleanInfo

	cleanMsg, err := json.Marshal(msgData)
	if err != nil {
		return fmt.Errorf("failed to re-marshal message JSON: %w", err)
	}
	msg = cleanMsg

	return nil
}

func writeInputData(mm *memoryManager, env, info, msg []byte, printDebug bool) (envPtr, infoPtr, msgPtr uint32, err error) {
	// Write environment data
	envPtr, _, err = mm.writeAlignedData(env, printDebug)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write env data: %w", err)
	}

	// Write info data
	infoPtr, _, err = mm.writeAlignedData(info, printDebug)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write info data: %w", err)
	}

	// Write message data
	msgPtr, _, err = mm.writeAlignedData(msg, printDebug)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write msg data: %w", err)
	}

	return envPtr, infoPtr, msgPtr, nil
}

func readAndValidateResult(memory api.Memory, resultPtr uint32, printDebug bool) ([]byte, error) {
	// Validate result pointer
	if resultPtr == 0 {
		return nil, fmt.Errorf("null result pointer")
	}

	// Read result region
	resultRegion, err := readResultRegionInternal(memory, resultPtr, printDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to read result region: %w", err)
	}

	// Read result data
	data, err := readRegionData(memory, resultRegion, printDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to read result data: %w", err)
	}

	// Validate result is proper JSON if it looks like JSON
	if len(data) > 0 && data[0] == '{' {
		var js map[string]interface{}
		if err := json.Unmarshal(data, &js); err != nil {
			return nil, fmt.Errorf("invalid result JSON: %w", err)
		}
		// Re-marshal to ensure consistent formatting
		data, err = json.Marshal(js)
		if err != nil {
			return nil, fmt.Errorf("failed to re-marshal result JSON: %w", err)
		}
	}

	return data, nil
}

func (w *WazeroRuntime) Execute(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("execute", checksum, env, info, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) Migrate(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("migrate", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) MigrateWithInfo(checksum, env, msg, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("migrate", checksum, env, migrateInfo, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) Sudo(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("sudo", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) Reply(checksum, env, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("reply", checksum, env, nil, reply, gasMeter, store, api, querier, gasLimit, printDebug)
}

// ByteSliceView represents a view into a Go byte slice without copying
type ByteSliceView struct {
	IsNil bool
	Data  []byte
}

func NewByteSliceView(data []byte) ByteSliceView {
	if data == nil {
		return ByteSliceView{
			IsNil: true,
			Data:  nil,
		}
	}
	return ByteSliceView{
		IsNil: false,
		Data:  data,
	}
}

func (w *WazeroRuntime) Query(checksum, env, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Create ByteSliceView for query to avoid unnecessary copying
	queryView := NewByteSliceView(query)
	defer func() {
		// Clear the view when done
		queryView.Data = nil
	}()

	// Create gas state for tracking memory operations
	gasState := NewGasState(gasLimit)

	// Account for memory view creation
	if !queryView.IsNil {
		gasState.ConsumeGas(uint64(len(queryView.Data))*DefaultGasConfig().PerByte, "query memory view")
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	// Create runtime environment with gas tracking
	runtimeEnv := &RuntimeEnvironment{
		DB:         store,
		API:        *api,
		Querier:    *querier,
		Gas:        *gasMeter,
		gasLimit:   gasState.GetGasLimit() - gasState.GetGasUsed(), // Adjust gas limit for memory operations
		gasUsed:    gasState.GetGasUsed(),
		iterators:  make(map[uint64]map[uint64]types.Iterator),
		nextCallID: 1,
	}

	// Register host functions
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(context.Background())

	// Get the module
	w.mu.Lock()
	module, ok := w.compiledModules[hex.EncodeToString(checksum)]
	if !ok {
		w.mu.Unlock()
		return nil, types.GasReport{}, fmt.Errorf("module not found for checksum %x", checksum)
	}
	w.mu.Unlock()

	// Create new module instance with host functions
	ctx := context.Background()
	moduleConfig := wazero.NewModuleConfig().
		WithName("env").
		WithStartFunctions()

	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, moduleConfig.WithName("env"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Create contract module instance
	contractModule, err := w.runtime.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithName("contract").WithStartFunctions())
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer contractModule.Close(ctx)

	// Initialize memory manager
	memory := contractModule.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("module has no memory")
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory initialization:\n")
		fmt.Printf("- Initial size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/wasmPageSize)
	}

	mm := newMemoryManager(memory, gasState)
	// Calculate total memory needed for data and Region structs
	envDataSize := uint32(len(env))
	envPagesNeeded := (envDataSize + wasmPageSize - 1) / wasmPageSize
	envAllocSize := envPagesNeeded * wasmPageSize

	queryDataSize := uint32(len(query))
	queryPagesNeeded := (queryDataSize + wasmPageSize - 1) / wasmPageSize
	queryAllocSize := queryPagesNeeded * wasmPageSize

	// Add space for Region structs (12 bytes each, aligned to page size)
	regionStructSize := uint32(24) // 2 Region structs * 12 bytes each
	regionPagesNeeded := (regionStructSize + wasmPageSize - 1) / wasmPageSize
	regionAllocSize := regionPagesNeeded * wasmPageSize

	// Ensure we have enough memory for everything
	totalSize := envAllocSize + queryAllocSize + regionAllocSize
	if totalSize > mm.size {
		pagesToGrow := (totalSize - mm.size + wasmPageSize - 1) / wasmPageSize
		if _, ok := mm.memory.Grow(pagesToGrow); !ok {
			return nil, types.GasReport{}, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Write data to memory
	envPtr, _, err := mm.writeToMemory(env, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env to memory: %w", err)
	}

	queryPtr, _, err := mm.writeToMemory(query, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write query to memory: %w", err)
	}

	// Create Region structs
	envRegion := &Region{
		Offset:   envPtr,
		Capacity: envAllocSize,
		Length:   envDataSize,
	}

	queryRegion := &Region{
		Offset:   queryPtr,
		Capacity: queryAllocSize,
		Length:   queryDataSize,
	}

	// Write Region structs to memory
	envRegionBytes := envRegion.ToBytes()
	envRegionPtr, _, err := mm.writeToMemory(envRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env region to memory: %w", err)
	}

	queryRegionBytes := queryRegion.ToBytes()
	queryRegionPtr, _, err := mm.writeToMemory(queryRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write query region to memory: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory layout before function call:\n")
		fmt.Printf("- Environment: ptr=0x%x, size=%d, region_ptr=0x%x\n", envPtr, len(env), envRegionPtr)
		fmt.Printf("- Query: ptr=0x%x, size=%d, region_ptr=0x%x\n", queryPtr, len(query), queryRegionPtr)
	}

	// Get the query function
	fn := contractModule.ExportedFunction("query")
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("query function not found")
	}

	// Call query function with Region struct pointers
	results, err := fn.Call(ctx, uint64(envRegionPtr), uint64(queryRegionPtr))
	if err != nil {
		if printDebug {
			fmt.Printf("\n[DEBUG] ====== Function Call Failed ======\n")
			fmt.Printf("Error: %v\n", err)

			// Try to read and deserialize memory at various locations
			if memory != nil {
				// Try the env region
				if envRegionData, ok := memory.Read(envRegionPtr, 12); ok {
					fmt.Printf("\nEnvironment Region:\n")
					offset := binary.LittleEndian.Uint32(envRegionData[0:4])
					length := binary.LittleEndian.Uint32(envRegionData[8:12])
					if data, err := readMemoryAndDeserialize(memory, offset, length); err == nil {
						fmt.Printf("Data: %s\n", data)
					}
				}

				// Try reading around the error location
				errPtr := uint32(1047844) // Common error location
				if data, err := readMemoryAndDeserialize(memory, errPtr-100, 200); err == nil {
					fmt.Printf("\nAround error location (offset=%d):\n%s\n", errPtr, data)
				}

				// Try reading the first page of memory
				if data, err := readMemoryAndDeserialize(memory, 0, 256); err == nil {
					fmt.Printf("\nFirst 256 bytes of memory:\n%s\n", data)
				}
			}

			fmt.Printf("=====================================\n\n")
		}
		return nil, types.GasReport{}, fmt.Errorf("query call failed: %w", err)
	}

	if len(results) != 1 {
		if printDebug {
			fmt.Printf("[DEBUG] Unexpected number of results: got %d, want 1\n", len(results))
		}
		return nil, types.GasReport{}, fmt.Errorf("expected 1 result, got %d", len(results))
	}

	// Read result from memory
	resultPtr := uint32(results[0])
	if printDebug {
		fmt.Printf("[DEBUG] Reading result from memory at ptr=0x%x\n", resultPtr)
	}

	resultData, ok := memory.Read(resultPtr, 8)
	if !ok {
		if printDebug {
			fmt.Printf("[DEBUG] Failed to read result data from memory\n")
		}
		return nil, types.GasReport{}, fmt.Errorf("failed to read result from memory")
	}

	dataPtr := binary.LittleEndian.Uint32(resultData[0:4])
	dataLen := binary.LittleEndian.Uint32(resultData[4:8])

	if printDebug {
		fmt.Printf("[DEBUG] Result points to: ptr=0x%x, len=%d\n", dataPtr, dataLen)
	}

	data, ok := memory.Read(dataPtr, dataLen)
	if !ok {
		if printDebug {
			fmt.Printf("[DEBUG] Failed to read data from memory\n")
		}
		return nil, types.GasReport{}, fmt.Errorf("failed to read data from memory")
	}

	if printDebug {
		fmt.Printf("[DEBUG] Function completed successfully\n")
		if len(data) < 1024 {
			fmt.Printf("[DEBUG] Result data: %s\n", string(data))
		} else {
			fmt.Printf("[DEBUG] Result data too large to display (len=%d)\n", len(data))
		}
	}

	gasReport := types.GasReport{
		UsedInternally: runtimeEnv.gasUsed,
		UsedExternally: gasState.GetGasUsed(),
		Remaining:      gasLimit - (runtimeEnv.gasUsed + gasState.GetGasUsed()),
		Limit:          gasLimit,
	}

	if printDebug {
		fmt.Printf("[DEBUG] Gas report:\n")
		fmt.Printf("- Used internally: %d\n", gasReport.UsedInternally)
		fmt.Printf("- Used externally: %d\n", gasReport.UsedExternally)
		fmt.Printf("- Remaining: %d\n", gasReport.Remaining)
		fmt.Printf("- Limit: %d\n", gasReport.Limit)
		fmt.Printf("=====================[END DEBUG]=====================\n\n")
	}

	return data, gasReport, nil
}

func (w *WazeroRuntime) IBCChannelOpen(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_channel_open", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCChannelConnect(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_channel_connect", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCChannelClose(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_channel_close", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCPacketReceive(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_packet_receive", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCPacketAck(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_packet_ack", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCPacketTimeout(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_packet_timeout", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCSourceCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_source_callback", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func (w *WazeroRuntime) IBCDestinationCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	return w.callContractFn("ibc_destination_callback", checksum, env, nil, msg, gasMeter, store, api, querier, gasLimit, printDebug)
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

// serializeEnvForContract serializes and validates the environment for the contract
func serializeEnvForContract(env []byte, printDebug bool) ([]byte, error) {
	// First unmarshal into a map to preserve field order
	var rawEnv map[string]interface{}
	if err := json.Unmarshal(env, &rawEnv); err != nil {
		return nil, fmt.Errorf("failed to deserialize environment: %w", err)
	}

	// Also unmarshal into a typed struct to validate the data
	var typedEnv types.Env
	if err := json.Unmarshal(env, &typedEnv); err != nil {
		return nil, fmt.Errorf("failed to deserialize environment: %w", err)
	}

	// Validate required fields
	if typedEnv.Block.Height == 0 {
		return nil, fmt.Errorf("block height is required")
	}
	if typedEnv.Block.ChainID == "" {
		return nil, fmt.Errorf("chain id is required")
	}
	if typedEnv.Contract.Address == "" {
		return nil, fmt.Errorf("contract address is required")
	}

	// If we got here, the original JSON is valid, so return it
	return env, nil
}

// readFunctionResult safely reads the result of a function call from memory
func (w *WazeroRuntime) readFunctionResult(memory api.Memory, resultPtr uint32, printDebug bool) ([]byte, error) {
	if printDebug {
		fmt.Printf("\n=== Reading Function Result ===\n")
		fmt.Printf("Result pointer: 0x%x\n", resultPtr)
	}

	// Validate result pointer is not null
	if resultPtr == 0 {
		return nil, fmt.Errorf("null result pointer")
	}

	// Ensure result pointer is within memory bounds
	if resultPtr >= uint32(memory.Size()) {
		return nil, fmt.Errorf("result pointer out of bounds: ptr=0x%x, memory_size=%d",
			resultPtr, memory.Size())
	}

	// Ensure result pointer is aligned
	if resultPtr%alignmentSize != 0 {
		return nil, fmt.Errorf("unaligned result pointer: %d must be aligned to %d", resultPtr, alignmentSize)
	}

	// Read and validate the result Region
	resultRegion, err := readResultRegionInternal(memory, resultPtr, printDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to read result region: %w", err)
	}

	// Validate region is not null
	if resultRegion == nil {
		return nil, fmt.Errorf("null result region")
	}

	// Additional validation of region fields
	if resultRegion.Length > resultRegion.Capacity {
		return nil, fmt.Errorf("invalid region: length %d exceeds capacity %d",
			resultRegion.Length, resultRegion.Capacity)
	}

	if resultRegion.Capacity == 0 {
		return nil, fmt.Errorf("invalid region: zero capacity")
	}

	// Read the actual data from the region
	data, err := readRegionData(memory, resultRegion, printDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to read result data: %w", err)
	}

	// Validate JSON response
	if len(data) > 0 && data[0] == '{' {
		var js interface{}
		if err := json.Unmarshal(data, &js); err != nil {
			if printDebug {
				fmt.Printf("[DEBUG] JSON validation failed: %v\n", err)
				// Print the problematic section
				errPos := 0
				if serr, ok := err.(*json.SyntaxError); ok {
					errPos = int(serr.Offset)
				}
				start := errPos - 20
				if start < 0 {
					start = 0
				}
				end := errPos + 20
				if end > len(data) {
					end = len(data)
				}
				fmt.Printf("[DEBUG] JSON error context: %q\n", string(data[start:end]))
				fmt.Printf("[DEBUG] Full data: %s\n", string(data))
			}
			return nil, fmt.Errorf("invalid JSON response: %w", err)
		}

		// Re-marshal to ensure consistent formatting
		cleanData, err := json.Marshal(js)
		if err != nil {
			return nil, fmt.Errorf("failed to re-marshal JSON response: %w", err)
		}
		data = cleanData
	}

	if printDebug {
		fmt.Printf("=== End Reading Function Result ===\n\n")
	}

	return data, nil
}

// repairJSONResponse attempts to fix common JSON corruption patterns
func repairJSONResponse(data []byte, printDebug bool) []byte {
	s := string(data)

	// Common corruption pattern: missing comma between array end and next field
	s = strings.ReplaceAll(s, `]"`, `],"`)

	// Common corruption pattern: truncated field names
	s = strings.ReplaceAll(s, `"hta":`, `"data":`)

	// Common corruption pattern: missing quotes around field names
	s = strings.ReplaceAll(s, `{instruction:`, `{"instruction":`)
	s = strings.ReplaceAll(s, `{seed:`, `{"seed":`)

	// Common corruption pattern: missing quotes around string values
	s = strings.ReplaceAll(s, `:instruction}`, `:"instruction"}`)
	s = strings.ReplaceAll(s, `:seed}`, `:"seed"}`)

	// Only return repaired data if it's valid JSON
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(s), &js); err != nil {
		if printDebug {
			fmt.Printf("[DEBUG] Repair attempt failed: %v\n", err)
		}
		return nil
	}

	return []byte(s)
}

func (w *WazeroRuntime) callContractFn(name string, checksum, env, info, msg []byte, gasMeter *types.GasMeter, store types.KVStore, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool) ([]byte, types.GasReport, error) {
	if printDebug {
		fmt.Printf("\n=====================[callContractFn DEBUG]=====================\n")
		fmt.Printf("[DEBUG] Function call: %s\n", name)
		fmt.Printf("[DEBUG] Checksum: %x\n", checksum)
		fmt.Printf("[DEBUG] Gas limit: %d\n", gasLimit)
		if env != nil {
			fmt.Printf("[DEBUG] Input sizes: env=%d", len(env))
		}
		if info != nil {
			fmt.Printf(", info=%d", len(info))
		}
		if msg != nil {
			fmt.Printf(", msg=%d", len(msg))
			fmt.Printf("\n[DEBUG] Message content: %s\n", string(msg))
		}
		fmt.Printf("\n")
	}

	// Create gas state for tracking memory operations
	gasState := NewGasState(gasLimit)

	// Create runtime environment with gas tracking
	runtimeEnv := &RuntimeEnvironment{
		DB:         store,
		API:        *api,
		Querier:    *querier,
		Gas:        *gasMeter,
		gasLimit:   gasState.GetGasLimit() - gasState.GetGasUsed(), // Adjust gas limit for memory operations
		gasUsed:    gasState.GetGasUsed(),
		iterators:  make(map[uint64]map[uint64]types.Iterator),
		nextCallID: 1,
	}

	// Create context with environment
	ctx := context.WithValue(context.Background(), envKey, runtimeEnv)

	// Register host functions
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(ctx)

	// Get the module
	w.mu.Lock()
	module, ok := w.compiledModules[hex.EncodeToString(checksum)]
	if !ok {
		w.mu.Unlock()
		return nil, types.GasReport{}, fmt.Errorf("module not found for checksum %x", checksum)
	}
	w.mu.Unlock()

	// Create new module instance with host functions
	moduleConfig := wazero.NewModuleConfig().
		WithName("env").
		WithStartFunctions()

	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, moduleConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Create contract module instance
	contractModule, err := w.runtime.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithName("contract").WithStartFunctions())
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer contractModule.Close(ctx)

	// Initialize memory manager
	memory := contractModule.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("module has no memory")
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory initialization:\n")
		fmt.Printf("- Initial size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/wasmPageSize)
	}

	memManager := newMemoryManager(memory, gasState)

	// Calculate total memory needed for data and Region structs
	envDataSize := uint32(len(env))
	envPagesNeeded := (envDataSize + wasmPageSize - 1) / wasmPageSize
	envAllocSize := envPagesNeeded * wasmPageSize

	var msgDataSize, msgPagesNeeded, msgAllocSize uint32
	if msg != nil {
		msgDataSize = uint32(len(msg))
		msgPagesNeeded = (msgDataSize + wasmPageSize - 1) / wasmPageSize
		msgAllocSize = msgPagesNeeded * wasmPageSize
	}

	// Add space for Region structs (12 bytes each, aligned to page size)
	regionStructSize := uint32(24) // 2 Region structs * 12 bytes each
	regionPagesNeeded := (regionStructSize + wasmPageSize - 1) / wasmPageSize
	regionAllocSize := regionPagesNeeded * wasmPageSize

	// Ensure we have enough memory for everything
	totalSize := envAllocSize + msgAllocSize + regionAllocSize
	currentSize := memory.Size()
	if totalSize > currentSize {
		pagesToGrow := (totalSize - currentSize + wasmPageSize - 1) / wasmPageSize
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages (from %d to %d)\n",
				pagesToGrow, currentSize/wasmPageSize, (currentSize+pagesToGrow*wasmPageSize)/wasmPageSize)
		}
		if _, ok := memory.Grow(pagesToGrow); !ok {
			return nil, types.GasReport{}, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
	}

	// Prepare regions for input data
	var envRegion, infoRegion, msgRegion *Region
	var envPtr, infoPtr, msgPtr uint32

	if printDebug {
		fmt.Printf("[DEBUG] Message data before prepareRegions: %s\n", string(msg))
	}

	if name == "query" {
		envRegion, _, msgRegion, err = memManager.prepareRegions(env, nil, msg)
	} else {
		envRegion, infoRegion, msgRegion, err = memManager.prepareRegions(env, info, msg)
	}
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to prepare regions: %w", err)
	}

	// Write the regions to memory with alignment checks
	if printDebug && msgRegion != nil {
		data, ok := memory.Read(msgRegion.Offset, msgRegion.Length)
		if ok {
			fmt.Printf("[DEBUG] Message data in memory: %s\n", string(data))
		}
	}

	// Ensure proper alignment for all pointers
	alignmentSize := uint32(8) // 8-byte alignment

	if name == "query" {
		envPtr, _, msgPtr, err = memManager.writeRegions(envRegion, nil, msgRegion)
		if err != nil {
			return nil, types.GasReport{}, fmt.Errorf("failed to write regions: %w", err)
		}

		// Align pointers
		envPtr = ((envPtr + alignmentSize - 1) / alignmentSize) * alignmentSize
		if msgPtr != 0 {
			msgPtr = ((msgPtr + alignmentSize - 1) / alignmentSize) * alignmentSize
		}
	} else {
		envPtr, infoPtr, msgPtr, err = memManager.writeRegions(envRegion, infoRegion, msgRegion)
		if err != nil {
			return nil, types.GasReport{}, fmt.Errorf("failed to write regions: %w", err)
		}

		// Align pointers
		envPtr = ((envPtr + alignmentSize - 1) / alignmentSize) * alignmentSize
		if infoPtr != 0 {
			infoPtr = ((infoPtr + alignmentSize - 1) / alignmentSize) * alignmentSize
		}
		if msgPtr != 0 {
			msgPtr = ((msgPtr + alignmentSize - 1) / alignmentSize) * alignmentSize
		}
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory layout before function call:\n")
		fmt.Printf("- Environment: ptr=0x%x, size=%d, region_ptr=0x%x\n", envRegion.Offset, len(env), envPtr)
		if infoRegion != nil {
			fmt.Printf("- Info: ptr=0x%x, size=%d, region_ptr=0x%x\n", infoRegion.Offset, len(info), infoPtr)
		}
		if msgRegion != nil {
			fmt.Printf("- Message: ptr=0x%x, size=%d, region_ptr=0x%x\n", msgRegion.Offset, len(msg), msgPtr)
		}
	}

	// Get the function
	fn := contractModule.ExportedFunction(name)
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("%s function not found", name)
	}

	// Call function with appropriate arguments
	var results []uint64
	if name == "query" {
		results, err = fn.Call(ctx, uint64(envPtr), uint64(msgPtr))
	} else {
		results, err = fn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	}

	if err != nil {
		if printDebug {
			fmt.Printf("\n[DEBUG] ====== Function Call Failed ======\n")
			fmt.Printf("Error: %v\n", err)
			dumpMemoryDebug(memory, envPtr, infoPtr, msgPtr)
			fmt.Printf("=====================================\n\n")
		}
		return nil, types.GasReport{}, fmt.Errorf("%s call failed: %w", name, err)
	}

	if len(results) != 1 {
		if printDebug {
			fmt.Printf("[DEBUG] Unexpected number of results: got %d, want 1\n", len(results))
		}
		return nil, types.GasReport{}, fmt.Errorf("expected 1 result, got %d", len(results))
	}

	// Read result from memory
	resultPtr := uint32(results[0])
	if printDebug {
		fmt.Printf("[DEBUG] Reading result from memory at ptr=0x%x\n", resultPtr)
	}

	// Read and validate result using safe method
	data, err := w.readFunctionResult(memory, resultPtr, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read function result: %w", err)
	}

	gasReport := types.GasReport{
		UsedInternally: runtimeEnv.gasUsed,
		UsedExternally: gasState.GetGasUsed(),
		Remaining:      gasLimit - (runtimeEnv.gasUsed + gasState.GetGasUsed()),
		Limit:          gasLimit,
	}

	if printDebug {
		fmt.Printf("[DEBUG] Gas report:\n")
		fmt.Printf("- Used internally: %d\n", gasReport.UsedInternally)
		fmt.Printf("- Used externally: %d\n", gasReport.UsedExternally)
		fmt.Printf("- Remaining: %d\n", gasReport.Remaining)
		fmt.Printf("- Limit: %d\n", gasReport.Limit)
		fmt.Printf("=====================[END DEBUG]=====================\n\n")
	}

	return data, gasReport, nil
}

// Helper function for debug memory dumps
func dumpMemoryDebug(memory api.Memory, envPtr, infoPtr, msgPtr uint32) {
	// Try the env region
	if envRegionData, ok := memory.Read(envPtr, regionStructSize); ok {
		fmt.Printf("\nEnvironment Region at 0x%x:\n", envPtr)
		if region, err := RegionFromBytes(envRegionData, ok); err == nil {
			if data, ok := memory.Read(region.Offset, region.Length); ok {
				fmt.Printf("Data: %s\n", string(data))
			}
		}
	}

	// Try the info region if it exists
	if infoPtr != 0 {
		if infoRegionData, ok := memory.Read(infoPtr, regionStructSize); ok {
			fmt.Printf("\nInfo Region at 0x%x:\n", infoPtr)
			if region, err := RegionFromBytes(infoRegionData, ok); err == nil {
				if data, ok := memory.Read(region.Offset, region.Length); ok {
					fmt.Printf("Data: %s\n", string(data))
				}
			}
		}
	}

	// Try the msg region if it exists
	if msgPtr != 0 {
		if msgRegionData, ok := memory.Read(msgPtr, regionStructSize); ok {
			fmt.Printf("\nMessage Region at 0x%x:\n", msgPtr)
			if region, err := RegionFromBytes(msgRegionData, ok); err == nil {
				if data, ok := memory.Read(region.Offset, region.Length); ok {
					fmt.Printf("Data: %s\n", string(data))
				}
			}
		}
	}

	// Dump start of memory
	if data, ok := memory.Read(0, 256); ok {
		fmt.Printf("\nFirst 256 bytes:\n%x\n", data)
	}
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

func readResultRegionInternal(memory api.Memory, resultPtr uint32, printDebug bool) (*Region, error) {
	// Validate result pointer
	if resultPtr == 0 {
		return nil, fmt.Errorf("null result pointer")
	}

	// Ensure pointer is aligned
	if resultPtr%alignmentSize != 0 {
		return nil, fmt.Errorf("unaligned result pointer: %d must be aligned to %d", resultPtr, alignmentSize)
	}

	// Read region data
	regionData, err := readMemory(memory, resultPtr, regionStructSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read region data: %w", err)
	}

	// Parse region
	region, err := RegionFromBytes(regionData, true)
	if err != nil {
		return nil, fmt.Errorf("invalid region data: %w", err)
	}

	// Validate region against memory size
	if err := region.Validate(memory.Size()); err != nil {
		return nil, fmt.Errorf("invalid result region: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Result region: offset=0x%x, capacity=%d, length=%d\n",
			region.Offset, region.Capacity, region.Length)
	}

	return region, nil
}
