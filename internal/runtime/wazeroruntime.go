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

type RuntimeEnvironment struct {
	DB      types.KVStore
	API     types.GoAPI
	Querier types.Querier
	Gas     types.GasMeter
	GasUsed types.Gas

	// Iterator management
	iteratorsMutex sync.RWMutex
	iterators      map[uint64]map[uint64]types.Iterator
	nextIterID     uint64
	nextCallID     uint64
}

// Constants for memory management
const (
	// Memory page size in WebAssembly (64KB)
	wasmPageSize = 65536

	// Size of a Region struct in bytes (3x4 bytes)
	regionSize = 12
)

// Region describes some data allocated in Wasm's linear memory.
// This matches cosmwasm_std::memory::Region.
type Region struct {
	Offset   uint32 // Beginning of the region in linear memory
	Capacity uint32 // Number of bytes available in this region
	Length   uint32 // Number of bytes used in this region
}

// memoryManager handles WASM memory allocation and deallocation
type memoryManager struct {
	memory api.Memory
	module api.Module
}

func newMemoryManager(memory api.Memory, module api.Module) *memoryManager {
	return &memoryManager{
		memory: memory,
		module: module,
	}
}

// writeToMemory writes data to WASM memory and returns the pointer and size
func (m *memoryManager) writeToMemory(data []byte) (uint32, uint32, error) {
	if data == nil {
		return 0, 0, nil
	}

	// Get the allocate function
	allocate := m.module.ExportedFunction("allocate")
	if allocate == nil {
		return 0, 0, fmt.Errorf("allocate function not found in WASM module")
	}

	// Allocate memory for the data
	size := uint32(len(data))
	results, err := allocate.Call(context.Background(), uint64(size))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate memory: %w", err)
	}
	ptr := uint32(results[0])

	// Ensure the allocated memory is properly aligned
	if ptr%4 != 0 {
		return 0, 0, fmt.Errorf("memory not properly aligned: ptr=%d", ptr)
	}

	// Write the data to memory
	if !m.memory.Write(ptr, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory at ptr=%d size=%d", ptr, size)
	}

	return ptr, size, nil
}

// readFromMemory reads data from WASM memory
func (m *memoryManager) readFromMemory(ptr, size uint32) ([]byte, error) {
	if ptr == 0 || size == 0 {
		return nil, nil
	}

	// Ensure we're not reading out of bounds
	memSize := uint64(m.memory.Size())
	if uint64(ptr)+uint64(size) > memSize {
		return nil, fmt.Errorf("memory access out of bounds: ptr=%d size=%d memSize=%d", ptr, size, memSize)
	}

	data, ok := m.memory.Read(ptr, size)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at ptr=%d size=%d", ptr, size)
	}
	return data, nil
}

// readRegion reads a Region struct from memory and validates it
func (m *memoryManager) readRegion(ptr uint32) (*Region, error) {
	// Read the Region struct (12 bytes total)
	data, ok := m.memory.Read(ptr, regionSize)
	if !ok {
		return nil, fmt.Errorf("failed to read Region struct at ptr=%d", ptr)
	}

	// Parse the Region struct (little-endian)
	region := &Region{
		Offset:   binary.LittleEndian.Uint32(data[0:4]),
		Capacity: binary.LittleEndian.Uint32(data[4:8]),
		Length:   binary.LittleEndian.Uint32(data[8:12]),
	}

	// Validate the region
	if region.Length > region.Capacity {
		return nil, fmt.Errorf("invalid Region: length %d exceeds capacity %d", region.Length, region.Capacity)
	}

	// Ensure we're not reading out of bounds
	memSize := uint64(m.memory.Size()) * wasmPageSize
	if uint64(region.Offset)+uint64(region.Capacity) > memSize {
		return nil, fmt.Errorf("invalid Region: out of memory bounds (offset=%d, capacity=%d, memSize=%d)", region.Offset, region.Capacity, memSize)
	}

	return region, nil
}

// writeRegion writes a Region struct to memory
func (m *memoryManager) writeRegion(ptr uint32, region *Region) error {
	// Validate the region before writing
	if region.Length > region.Capacity {
		return fmt.Errorf("invalid Region: length %d exceeds capacity %d", region.Length, region.Capacity)
	}

	// Ensure we're not writing out of bounds
	memSize := uint64(m.memory.Size()) * wasmPageSize
	if uint64(region.Offset)+uint64(region.Capacity) > memSize {
		return fmt.Errorf("invalid Region: out of memory bounds (offset=%d, capacity=%d, memSize=%d)", region.Offset, region.Capacity, memSize)
	}

	// Create the Region struct bytes (little-endian)
	data := make([]byte, regionSize)
	binary.LittleEndian.PutUint32(data[0:4], region.Offset)
	binary.LittleEndian.PutUint32(data[4:8], region.Capacity)
	binary.LittleEndian.PutUint32(data[8:12], region.Length)

	// Write the Region struct
	if !m.memory.Write(ptr, data) {
		return fmt.Errorf("failed to write Region struct at ptr=%d", ptr)
	}

	return nil
}

func NewWazeroRuntime() (*WazeroRuntime, error) {
	// Create a new wazero runtime with memory configuration
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(512).      // Set max memory to 32 MiB (512 * 64KB)
		WithMemoryCapacityFromMax(true) // Eagerly allocate memory

	r := wazero.NewRuntimeWithConfig(context.Background(), runtimeConfig)

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

	// Validate query message format
	var queryMsg map[string]interface{}
	if err := json.Unmarshal(query, &queryMsg); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("invalid query message format: %w", err)
	}

	// Ensure query message is properly formatted
	if len(queryMsg) != 1 {
		return nil, types.GasReport{}, fmt.Errorf("query message must have exactly one field")
	}

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

// serializeEnvForContract serializes the environment based on the contract version
func serializeEnvForContract(env []byte, checksum []byte, w *WazeroRuntime) ([]byte, error) {
	// We'll try to parse it as a strongly typed Env first.
	// If that works, we adapt to a 1.0+ shape (numeric block height/time).
	var typedEnv types.Env
	if err := json.Unmarshal(env, &typedEnv); err == nil {
		// Convert to raw map so we can adjust fields.
		var rawEnv map[string]interface{}
		if err := json.Unmarshal(env, &rawEnv); err != nil {
			return nil, fmt.Errorf("failed to unmarshal env into raw map: %w", err)
		}

		// If there's a "block" section, set `height` and `time` as numeric
		if block, ok := rawEnv["block"].(map[string]interface{}); ok {
			block["height"] = typedEnv.Block.Height // store as integer
			block["time"] = typedEnv.Block.Time     // store as integer
			// chain_id presumably remains a string
		}

		// If there's a "transaction" section, set the transaction index as numeric (if present)
		if txn, ok := rawEnv["transaction"].(map[string]interface{}); ok {
			if typedEnv.Transaction != nil {
				txn["index"] = typedEnv.Transaction.Index
			}
		}

		// Re-serialize with the numeric fields
		adaptedEnv, err := json.Marshal(rawEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal adapted env: %w", err)
		}
		return adaptedEnv, nil
	}

	// If we *couldn’t* parse typedEnv, then we just handle it as raw JSON
	// but still enforce numeric shape for block.height/time and transaction.index
	// by heuristics. Example below:
	var rawEnv map[string]interface{}
	d := json.NewDecoder(strings.NewReader(string(env)))
	d.UseNumber() // preserve numeric precision
	if err := d.Decode(&rawEnv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal env: %w", err)
	}

	// Try to enforce numeric `height` and `time` in "block"
	if block, ok := rawEnv["block"].(map[string]interface{}); ok {
		// parse block.height
		if hval, ok := block["height"]; ok {
			if num, ok := hval.(json.Number); ok {
				if i64, err := num.Int64(); err == nil {
					block["height"] = i64 // store as integer
				} else {
					return nil, fmt.Errorf("unable to parse block.height as integer: %w", err)
				}
			}
		}
		// parse block.time
		if tval, ok := block["time"]; ok {
			if num, ok := tval.(json.Number); ok {
				if i64, err := num.Int64(); err == nil {
					block["time"] = i64 // store as integer
				} else {
					return nil, fmt.Errorf("unable to parse block.time as integer: %w", err)
				}
			}
		}
	}

	// parse transaction.index as numeric if present
	if txn, ok := rawEnv["transaction"].(map[string]interface{}); ok {
		if idxVal, ok := txn["index"]; ok {
			if num, ok := idxVal.(json.Number); ok {
				if i64, err := num.Int64(); err == nil {
					txn["index"] = i64 // store as integer
				} else {
					return nil, fmt.Errorf("unable to parse transaction.index: %w", err)
				}
			}
		}
	}

	// Now re-serialize
	adaptedEnv, err := json.Marshal(rawEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal adapted env: %w", err)
	}
	return adaptedEnv, nil
}

// callContractFn is the high-level dispatcher for all CosmWasm entry points.
//   - "query", "sudo", "reply" each expect 2 parameters: env + msg
//   - "instantiate", "execute", "migrate" each expect 3 parameters: env + info + msg
//
// Any mismatch leads to memory corruption in the Wasm, causing invalid JSON parse errors.
//
// Here, `info` must be non-nil if the contract function is one of the 3-argument entry points.
// For "migrate" you might also handle fallback for 2-argument signatures, depending on your contract.
// callContractFn is revised to not double-free env/info/msg pointers.
func (w *WazeroRuntime) callContractFn(
	name string,
	checksum []byte,
	env []byte,
	info []byte, // nil except for instantiate/execute/migrate
	msg []byte,
) ([]byte, types.GasReport, error) {
	// 1) Basic validations
	if checksum == nil {
		return nil, types.GasReport{}, errors.New("Null/Nil argument: checksum")
	} else if len(checksum) != 32 {
		return nil, types.GasReport{},
			fmt.Errorf("invalid argument: checksum must be 32 bytes, got %d", len(checksum))
	}

	// 2) Lookup compiled code
	w.mu.Lock()
	csHex := hex.EncodeToString(checksum)
	compiled, ok := w.compiledModules[csHex]
	if _, pinned := w.pinnedModules[csHex]; pinned {
		w.moduleHits[csHex]++
	}
	w.mu.Unlock()
	if !ok {
		return nil, types.GasReport{},
			fmt.Errorf("code for %s not found in compiled modules", csHex)
	}

	// 3) Adapt environment for contract version
	adaptedEnv, err := serializeEnvForContract(env, checksum, w)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to adapt environment: %w", err)
	}

	ctx := context.Background()

	// 4) Register and instantiate the host module "env"
	hm, err := RegisterHostFunctions(w.runtime, &RuntimeEnvironment{
		DB:        w.kvStore,
		API:       *w.api,
		Querier:   w.querier,
		Gas:       w.querier,
		iterators: make(map[uint64]map[uint64]types.Iterator),
	})
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hm.Close(ctx)

	_, err = w.runtime.InstantiateModule(ctx, hm, wazero.NewModuleConfig().WithName("env"))
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to instantiate host module: %w", err)
	}

	// 5) Instantiate the contract module
	modConfig := wazero.NewModuleConfig().
		WithName("contract").
		WithStartFunctions() // optional
	module, err := w.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to instantiate contract: %w", err)
	}
	defer module.Close(ctx)

	// 6) Create memory manager
	memory := module.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("no memory section in module")
	}
	mm := newMemoryManager(memory, module)

	//------------------------------------------------
	// a) Write env → Region pointer (envRegionPtr)
	//------------------------------------------------
	envPtr, envSize, err := mm.writeToMemory(adaptedEnv)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to write env: %w", err)
	}

	envRegion := &Region{
		Offset:   envPtr,
		Capacity: envSize,
		Length:   envSize,
	}
	envRegionPtr, err := allocateInContract(ctx, module, regionSize)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to allocate env region: %w", err)
	}
	if err := mm.writeRegion(envRegionPtr, envRegion); err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to write env region: %w", err)
	}

	//------------------------------------------------
	// b) Write msg → Region pointer (msgRegionPtr)
	//------------------------------------------------
	msgPtr, msgSize, err := mm.writeToMemory(msg)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to write msg: %w", err)
	}

	msgRegion := &Region{
		Offset:   msgPtr,
		Capacity: msgSize,
		Length:   msgSize,
	}
	msgRegionPtr, err := allocateInContract(ctx, module, regionSize)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to allocate msg region: %w", err)
	}
	if err := mm.writeRegion(msgRegionPtr, msgRegion); err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to write msg region: %w", err)
	}

	//------------------------------------------------
	// c) Possibly write info → Region pointer
	//------------------------------------------------
	var callParams []uint64
	switch name {
	case "instantiate", "execute", "migrate":
		if info == nil {
			return nil, types.GasReport{},
				fmt.Errorf("%s requires a non-nil info parameter", name)
		}
		infoPtr, infoSize, err2 := mm.writeToMemory(info)
		if err2 != nil {
			return nil, types.GasReport{},
				fmt.Errorf("failed to write info: %w", err2)
		}

		infoRegion := &Region{
			Offset:   infoPtr,
			Capacity: infoSize,
			Length:   infoSize,
		}
		infoRegionPtr, err2 := allocateInContract(ctx, module, regionSize)
		if err2 != nil {
			return nil, types.GasReport{},
				fmt.Errorf("failed to allocate info region: %w", err2)
		}
		if err2 = mm.writeRegion(infoRegionPtr, infoRegion); err2 != nil {
			return nil, types.GasReport{},
				fmt.Errorf("failed to write info region: %w", err2)
		}

		callParams = []uint64{
			uint64(envRegionPtr),
			uint64(infoRegionPtr),
			uint64(msgRegionPtr),
		}
		fmt.Printf("Calling instantiate with:\n")
		fmt.Printf("  envRegionPtr: %d\n", envRegionPtr)
		fmt.Printf("  infoRegionPtr: %d\n", infoRegionPtr)
		fmt.Printf("  msgRegionPtr: %d\n", msgRegionPtr)
	case "query", "sudo", "reply":
		callParams = []uint64{
			uint64(envRegionPtr),
			uint64(msgRegionPtr),
		}
	default:
		return nil, types.GasReport{},
			fmt.Errorf("unknown function name: %s", name)
	}

	//------------------------------------------------
	// d) Invoke the wasm function
	//------------------------------------------------
	fn := module.ExportedFunction(name)
	if fn == nil {
		return nil, types.GasReport{},
			fmt.Errorf("function %q not found in contract", name)
	}
	results, callErr := fn.Call(ctx, callParams...)
	if callErr != nil {
		return nil, types.GasReport{},
			fmt.Errorf("call to %s failed: %w", name, callErr)
	}
	if len(results) != 1 {
		return nil, types.GasReport{},
			fmt.Errorf("function %s returned %d results (wanted 1)", name, len(results))
	}

	//------------------------------------------------
	// e) The single result is the "result region"
	//------------------------------------------------
	resultRegionPtr := uint32(results[0])
	resultRegion, err := mm.readRegion(resultRegionPtr)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to read result region: %w", err)
	}
	resultBytes, err := mm.readFromMemory(resultRegion.Offset, resultRegion.Length)
	if err != nil {
		return nil, types.GasReport{},
			fmt.Errorf("failed to read result data: %w", err)
	}

	//------------------------------------------------
	// f) Construct GasReport
	//------------------------------------------------
	gasUsed := w.querier.GasConsumed()
	gr := types.GasReport{
		Limit:          1_000_000_000,
		Remaining:      1_000_000_000 - gasUsed,
		UsedExternally: 0,
		UsedInternally: gasUsed,
	}

	return resultBytes, gr, nil
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
