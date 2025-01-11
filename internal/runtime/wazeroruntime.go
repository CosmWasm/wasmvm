package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

	// Gas tracking
	gasLimit uint64
	gasUsed  uint64

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

// Region describes data allocated in Wasm's linear memory
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// memoryManager handles WASM memory allocation and deallocation
type memoryManager struct {
	memory api.Memory
	module api.Module
	size   uint32
}

func newMemoryManager(memory api.Memory, module api.Module) *memoryManager {
	if memory == nil {
		panic("memory cannot be nil")
	}

	// Get initial memory size in bytes
	memBytes := memory.Size()

	// Ensure memory is initialized with at least one page
	if memBytes == 0 {
		if _, ok := memory.Grow(1); !ok {
			panic("failed to initialize memory with minimum size")
		}
		memBytes = memory.Size()
	}

	// Ensure we have enough memory pages (at least 1MB)
	const minMemoryBytes = 1024 * 1024 // 1MB
	if memBytes < minMemoryBytes {
		pagesNeeded := (minMemoryBytes - memBytes + wasmPageSize - 1) / wasmPageSize
		if _, ok := memory.Grow(uint32(pagesNeeded)); !ok {
			panic(fmt.Sprintf("failed to grow memory to minimum size: needs %d more pages", pagesNeeded))
		}
		memBytes = memory.Size()
	}

	// Verify memory size after initialization
	if memBytes == 0 {
		panic("memory not properly initialized after growth")
	}

	// Initialize first page with zeros to ensure clean state
	zeroMem := make([]byte, wasmPageSize)
	if !memory.Write(0, zeroMem) {
		panic("failed to initialize first memory page")
	}

	return &memoryManager{
		memory: memory,
		module: module,
		size:   memBytes,
	}
}

// validateMemorySize checks if the memory size is within acceptable limits
func (m *memoryManager) validateMemorySize(memSizeBytes uint32) error {
	// Get current memory size in bytes
	currentSize := m.memory.Size()

	if currentSize == 0 {
		return fmt.Errorf("memory not properly initialized")
	}

	// Check if memory size is reasonable (max 64MB)
	const maxMemorySize = 64 * 1024 * 1024 // 64MB
	if memSizeBytes > maxMemorySize {
		return fmt.Errorf("memory size %d bytes exceeds maximum allowed size of %d bytes", memSizeBytes, maxMemorySize)
	}

	// Ensure memory size is page-aligned
	if memSizeBytes%wasmPageSize != 0 {
		return fmt.Errorf("memory size %d bytes is not page-aligned (page size: %d bytes)", memSizeBytes, wasmPageSize)
	}

	// Verify first page is properly initialized
	firstPage, ok := m.memory.Read(0, wasmPageSize)
	if !ok || len(firstPage) != wasmPageSize {
		return fmt.Errorf("failed to read first memory page")
	}

	// Create initial memory region for validation
	region := &Region{
		Offset:   wasmPageSize, // Start after first page
		Capacity: currentSize - wasmPageSize,
		Length:   0,
	}

	// Validate the region
	if err := validateRegion(region); err != nil {
		return fmt.Errorf("memory region validation failed: %w", err)
	}

	return nil
}

// validateMemoryRegion performs validation checks on a memory region
func validateMemoryRegion(region *Region) error {
	// Check for zero offset (required by Rust implementation)
	if region.Offset == 0 {
		return fmt.Errorf("region offset cannot be zero")
	}

	// Check if length exceeds capacity
	if region.Length > region.Capacity {
		return fmt.Errorf("region length (%d) exceeds capacity (%d)", region.Length, region.Capacity)
	}

	// Check for potential overflow
	if region.Capacity > (math.MaxUint32 - region.Offset) {
		return fmt.Errorf("region capacity (%d) would overflow when added to offset (%d)", region.Capacity, region.Offset)
	}

	return nil
}

// validateRegion performs validation checks on a memory region
func validateRegion(region *Region) error {
	if region == nil {
		return fmt.Errorf("region is nil")
	}
	if region.Offset == 0 {
		return fmt.Errorf("region offset is zero")
	}
	if region.Capacity == 0 {
		return fmt.Errorf("region capacity is zero")
	}
	if region.Length > region.Capacity {
		return fmt.Errorf("region length %d exceeds capacity %d", region.Length, region.Capacity)
	}

	// Check for potential overflow in offset + capacity
	if region.Offset > math.MaxUint32-region.Capacity {
		return fmt.Errorf("region would overflow memory bounds: offset=%d, capacity=%d", region.Offset, region.Capacity)
	}

	// Enforce a maximum region size of 64MB to prevent excessive allocations
	const maxRegionSize = 64 * 1024 * 1024 // 64MB
	if region.Capacity > maxRegionSize {
		return fmt.Errorf("region capacity %d exceeds maximum allowed size of %d", region.Capacity, maxRegionSize)
	}

	return nil
}

// writeToMemory writes data to WASM memory and returns the pointer and size
func (mm *memoryManager) writeToMemory(data []byte, printDebug bool) (uint32, uint32, error) {
	if mm.memory == nil {
		return 0, 0, fmt.Errorf("memory not initialized")
	}

	if mm.size == 0 {
		return 0, 0, fmt.Errorf("memory size is 0 - memory not properly initialized")
	}

	// Calculate total size needed (data + Region struct)
	totalSize := uint32(len(data)) + regionSize

	// Check if we need to grow memory
	currentBytes := mm.size
	neededBytes := totalSize

	if neededBytes > currentBytes {
		pagesToGrow := (neededBytes - currentBytes + uint32(wasmPageSize) - 1) / uint32(wasmPageSize)
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages for allocation of %d bytes\n", pagesToGrow, totalSize)
		}
		if _, ok := mm.memory.Grow(pagesToGrow); !ok {
			return 0, 0, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Allocate memory for data
	dataPtr, err := allocateInContract(context.Background(), mm.module, uint32(len(data)))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate memory for data: %v", err)
	}

	// Write data to memory
	if err := writeMemory(mm.memory, dataPtr, data); err != nil {
		return 0, 0, fmt.Errorf("failed to write data to memory: %v", err)
	}

	// Create Region struct
	regionBytes := make([]byte, regionSize)
	binary.LittleEndian.PutUint32(regionBytes[0:4], dataPtr)
	binary.LittleEndian.PutUint32(regionBytes[4:8], uint32(len(data)))
	binary.LittleEndian.PutUint32(regionBytes[8:12], uint32(len(data)))

	// Allocate memory for Region struct
	regionPtr, err := allocateInContract(context.Background(), mm.module, regionSize)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate memory for Region struct: %v", err)
	}

	// Write Region struct to memory
	if err := writeMemory(mm.memory, regionPtr, regionBytes); err != nil {
		return 0, 0, fmt.Errorf("failed to write Region struct to memory: %v", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Wrote %d bytes at ptr=0x%x, Region struct at ptr=0x%x\n", len(data), dataPtr, regionPtr)
	}

	return regionPtr, uint32(len(data)), nil
}

func NewWazeroRuntime() (*WazeroRuntime, error) {
	// Create a new wazero runtime with memory configuration
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(1024).      // Set max memory to 64 MiB (1024 * 64KB)
		WithMemoryCapacityFromMax(true). // Eagerly allocate memory to ensure it's initialized
		WithDebugInfoEnabled(true)       // Enable debug info

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

func (w *WazeroRuntime) Instantiate(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	// Set the contract execution environment
	w.kvStore = store
	w.api = api
	w.querier = *querier

	// Call the instantiate function
	return w.callContractFn("instantiate", checksum, env, info, msg, gasMeter, store, api, querier, gasLimit, printDebug)
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

func (w *WazeroRuntime) Query(checksum, env, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	gasMeter, store, api, querier, gasLimit, printDebug, err := w.parseParams(otherParams)
	if err != nil {
		return nil, types.GasReport{}, err
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

	return w.callContractFn("query", checksum, env, nil, query, gasMeter, store, api, querier, gasLimit, printDebug)
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
func serializeEnvForContract(env []byte, _ []byte, _ *WazeroRuntime) ([]byte, error) {
	// First unmarshal into a raw map to preserve the exact string format of numbers
	var rawEnv map[string]interface{}
	if err := json.Unmarshal(env, &rawEnv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal env: %w", err)
	}

	// Now unmarshal into typed struct for validation
	var typedEnv types.Env
	if err := json.Unmarshal(env, &typedEnv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal env: %w", err)
	}

	// Validate Block fields
	if typedEnv.Block.Height == 0 {
		return nil, fmt.Errorf("block height cannot be 0")
	}
	if typedEnv.Block.Time == 0 {
		return nil, fmt.Errorf("block time cannot be 0")
	}
	if typedEnv.Block.ChainID == "" {
		return nil, fmt.Errorf("chain_id cannot be empty")
	}

	// Validate Contract fields
	if typedEnv.Contract.Address == "" {
		return nil, fmt.Errorf("contract address cannot be empty")
	}

	// Get the original block data to preserve number formats
	rawBlock, _ := rawEnv["block"].(map[string]interface{})

	// Create a map preserving the original time format
	envMap := map[string]interface{}{
		"block": map[string]interface{}{
			"height":   rawBlock["height"],
			"time":     rawBlock["time"], // Preserve original time format
			"chain_id": typedEnv.Block.ChainID,
		},
		"contract": map[string]interface{}{
			"address": typedEnv.Contract.Address,
		},
	}

	// Add Transaction if present
	if typedEnv.Transaction != nil {
		txMap := map[string]interface{}{
			"index": typedEnv.Transaction.Index,
		}
		if typedEnv.Transaction.Hash != "" {
			txMap["hash"] = typedEnv.Transaction.Hash
		}
		envMap["transaction"] = txMap
	}

	// Re-serialize with proper types
	adaptedEnv, err := json.Marshal(envMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal adapted env: %w", err)
	}

	return adaptedEnv, nil
}

func (w *WazeroRuntime) callContractFn(
	name string,
	checksum []byte,
	env []byte,
	info []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	if printDebug {
		fmt.Printf("\n=====================[callContractFn DEBUG]=====================\n")
		fmt.Printf("[DEBUG] callContractFn: name=%s checksum=%x\n", name, checksum)
		fmt.Printf("[DEBUG] len(env)=%d, len(info)=%d, len(msg)=%d\n", len(env), len(info), len(msg))
	}

	// 1) Basic validations
	if checksum == nil {
		const errStr = "[callContractFn] Error: Null/Nil argument: checksum"
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	} else if len(checksum) != 32 {
		errStr := fmt.Sprintf("[callContractFn] Error: invalid argument: checksum must be 32 bytes, got %d", len(checksum))
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// 2) Lookup compiled code
	w.mu.Lock()
	csHex := hex.EncodeToString(checksum)
	compiled, ok := w.compiledModules[csHex]
	if _, pinned := w.pinnedModules[csHex]; pinned {
		w.moduleHits[csHex]++
		if printDebug {
			fmt.Printf("[DEBUG] pinned module hits incremented for %s -> total hits = %d\n", csHex, w.moduleHits[csHex])
		}
	}
	w.mu.Unlock()
	if !ok {
		errStr := fmt.Sprintf("[callContractFn] Error: code for %s not found in compiled modules", csHex)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// 3) Adapt environment for contract version
	adaptedEnv, err := serializeEnvForContract(env, checksum, w)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error in serializeEnvForContract: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, fmt.Errorf("failed to adapt environment: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Original env: %s\n", string(env))
		fmt.Printf("[DEBUG] Adapted env: %s\n", string(adaptedEnv))
		fmt.Printf("[DEBUG] Message: %s\n", string(msg))
		fmt.Printf("[DEBUG] Function name: %s\n", name)
		fmt.Printf("[DEBUG] Gas limit: %d\n", gasLimit)
	}

	ctx := context.Background()

	// 4) Register and instantiate the host module "env"
	if printDebug {
		fmt.Println("[DEBUG] Registering host functions ...")
	}
	runtimeEnv := &RuntimeEnvironment{
		DB:        store,
		API:       *api,
		Querier:   *querier,
		Gas:       *gasMeter,
		gasLimit:  gasLimit,
		gasUsed:   0,
		iterators: make(map[uint64]map[uint64]types.Iterator),
	}
	hm, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to register host functions: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}
	defer func() {
		if printDebug {
			fmt.Println("[DEBUG] Closing host module ...")
		}
		hm.Close(ctx)
	}()

	// Instantiate the env module
	if printDebug {
		fmt.Println("[DEBUG] Instantiating 'env' module ...")
	}
	envConfig := wazero.NewModuleConfig().
		WithName("env").
		WithStartFunctions()
	envModule, err := w.runtime.InstantiateModule(ctx, hm, envConfig)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to instantiate env module: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}
	defer func() {
		if printDebug {
			fmt.Println("[DEBUG] Closing 'env' module ...")
		}
		envModule.Close(ctx)
	}()

	// 5) Instantiate the contract module with memory configuration
	if printDebug {
		fmt.Println("[DEBUG] Instantiating contract module ...")
	}

	// Configure module with memory
	modConfig := wazero.NewModuleConfig().
		WithName("contract").
		WithStartFunctions()
	// Initialize module with memory
	module, err := w.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to instantiate contract: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}
	defer func() {
		if printDebug {
			fmt.Println("[DEBUG] Closing contract module ...")
		}
		module.Close(ctx)
	}()

	// Get memory and verify it was initialized
	memory := module.Memory()
	if memory == nil {
		const errStr = "[callContractFn] Error: no memory section in module"
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// Get initial memory size in bytes
	initialBytes := memory.Size()
	if printDebug {
		fmt.Printf("[DEBUG] Initial memory size: %d bytes (%d pages)\n", initialBytes, initialBytes/uint32(wasmPageSize))
	}

	// Ensure we have enough initial memory (at least 1MB)
	const minBytes = 16 * wasmPageSize // 1MB = 16 * 64KB pages
	if initialBytes < minBytes {
		// Calculate required pages, rounding up
		newPages := (minBytes - initialBytes + uint32(wasmPageSize) - 1) / uint32(wasmPageSize)
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages to reach minimum size\n", newPages)
		}

		// Try to grow memory with proper error handling
		if grown, ok := memory.Grow(newPages); !ok {
			if printDebug {
				fmt.Printf("[DEBUG] Failed to grow memory. Current size: %d, Attempted growth: %d pages\n",
					initialBytes/uint32(wasmPageSize), newPages)
			}
			return nil, types.GasReport{}, fmt.Errorf("failed to grow memory to minimum size: needs %d more pages", newPages)
		} else {
			if printDebug {
				fmt.Printf("[DEBUG] Successfully grew memory by %d pages\n", grown)
			}
		}

		// Verify the new size
		newSize := memory.Size()
		if newSize < minBytes {
			return nil, types.GasReport{}, fmt.Errorf("memory growth failed: got %d bytes, need %d bytes", newSize, minBytes)
		}
		initialBytes = newSize
	}

	// Verify memory size after growth
	if initialBytes == 0 {
		const errStr = "[callContractFn] Error: memory not properly initialized"
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory initialized with %d bytes (%d pages)\n", initialBytes, initialBytes/uint32(wasmPageSize))
	}

	// Create memory manager with verified memory and add validation
	mm := newMemoryManager(memory, module)
	if err := mm.validateMemorySize(initialBytes); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("memory validation failed: %w", err)
	}

	// Pre-allocate some memory for the contract
	preAllocSize := uint32(1024) // Pre-allocate 1KB
	if _, err := allocateInContract(context.Background(), module, preAllocSize); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to pre-allocate memory: %v", err)
	}

	// Write environment data to memory
	if printDebug {
		fmt.Printf("[DEBUG] Writing environment to memory (size=%d) ...\n", len(adaptedEnv))
		fmt.Printf("[DEBUG] Environment content: %s\n", string(adaptedEnv))
		var prettyEnv interface{}
		if err := json.Unmarshal(adaptedEnv, &prettyEnv); err != nil {
			fmt.Printf("[DEBUG] Failed to parse env as JSON: %v\n", err)
		} else {
			prettyJSON, _ := json.MarshalIndent(prettyEnv, "", "  ")
			fmt.Printf("[DEBUG] Parsed env structure:\n%s\n", string(prettyJSON))
		}
	}
	envPtr, _, err := mm.writeToMemory(adaptedEnv, printDebug)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to write env: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Writing msg to memory (size=%d) ...\n", len(msg))
	}
	msgPtr, _, err := mm.writeToMemory(msg, printDebug)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to write msg: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	var callParams []uint64
	switch name {
	case "instantiate", "execute", "migrate":
		if info == nil {
			errStr := fmt.Sprintf("[callContractFn] Error: %s requires a non-nil info parameter", name)
			fmt.Println(errStr)
			return nil, types.GasReport{}, errors.New(errStr)
		}
		if printDebug {
			fmt.Printf("[DEBUG] Writing info to memory (size=%d) ...\n", len(info))
		}
		infoPtr, _, err := mm.writeToMemory(info, printDebug)
		if err != nil {
			errStr := fmt.Sprintf("[callContractFn] Error: failed to write info: %v", err)
			fmt.Println(errStr)
			return nil, types.GasReport{}, errors.New(errStr)
		}
		callParams = []uint64{uint64(envPtr), uint64(infoPtr), uint64(msgPtr)}
		if printDebug {
			fmt.Printf("[DEBUG] Instantiate/Execute/Migrate params: env=%d, info=%d, msg=%d\n", envPtr, infoPtr, msgPtr)
		}

	case "query", "sudo", "reply":
		callParams = []uint64{uint64(envPtr), uint64(msgPtr)}
		if printDebug {
			fmt.Printf("[DEBUG] Query/Sudo/Reply params: env=%d, msg=%d\n", envPtr, msgPtr)
		}

	default:
		errStr := fmt.Sprintf("[callContractFn] Error: unknown function name: %s", name)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// Call the contract function
	fn := module.ExportedFunction(name)
	if fn == nil {
		errStr := fmt.Sprintf("[callContractFn] Error: function %q not found in contract", name)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] about to call function '%s' with callParams=%v\n", name, callParams)
	}
	results, err := fn.Call(ctx, callParams...)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: call to %s failed: %v", name, err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if len(results) != 1 {
		errStr := fmt.Sprintf("[callContractFn] Error: function %s returned %d results (wanted 1)", name, len(results))
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] results from contract call: %#v\n", results)
	}

	// Read result from memory
	resultPtr := uint32(results[0])
	if printDebug {
		memPages := memory.Size() // This is in bytes, not pages
		actualPages := memPages / uint32(wasmPageSize)
		totalBytes := memPages // This is already in bytes
		fmt.Printf("[DEBUG] Memory info:\n")
		fmt.Printf("  - Raw size: %d bytes\n", memPages)
		fmt.Printf("  - Pages: %d\n", actualPages)
		fmt.Printf("  - Page size: %d bytes\n", wasmPageSize)
		fmt.Printf("  - Total size: %d bytes\n", totalBytes)
		fmt.Printf("  - Result pointer: 0x%x (%d)\n", resultPtr, resultPtr)
	}

	// Try to read the raw memory first
	rawData, ok := memory.Read(resultPtr, regionSize)
	if !ok {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to read raw memory at ptr=%d", resultPtr)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] raw memory at result ptr: % x\n", rawData)

		// Add more context around the result pointer
		contextStart := uint32(0)
		if resultPtr > 16 {
			contextStart = resultPtr - 16
		}
		contextData, ok := memory.Read(contextStart, 48) // 16 bytes before, 16 after
		if ok {
			fmt.Printf("[DEBUG] memory context around result (from %d): % x\n", contextStart, contextData)
		}
	}

	// Try to parse as Region struct
	resultRegion := &Region{
		Offset:   binary.LittleEndian.Uint32(rawData[0:4]),
		Capacity: binary.LittleEndian.Uint32(rawData[4:8]),
		Length:   binary.LittleEndian.Uint32(rawData[8:12]),
	}

	if printDebug {
		fmt.Printf("[DEBUG] parsed region: Offset=0x%x, Capacity=%d, Length=%d\n",
			resultRegion.Offset, resultRegion.Capacity, resultRegion.Length)
	}

	// Try to read the actual data, but be defensive about length
	var resultData []byte
	if resultRegion.Length > 1024*1024 { // Cap at 1MB
		resultRegion.Length = 1024 * 1024
	}

	resultData, ok = memory.Read(resultRegion.Offset, resultRegion.Length)
	if !ok {
		// Try reading a smaller amount if that failed
		resultRegion.Length = 1024 // Try 1KB
		resultData, ok = memory.Read(resultRegion.Offset, resultRegion.Length)
		if !ok {
			errStr := fmt.Sprintf("[callContractFn] Error: failed to read result data at offset=%d length=%d",
				resultRegion.Offset, resultRegion.Length)
			fmt.Println(errStr)
			return nil, types.GasReport{}, errors.New(errStr)
		}
	}

	if printDebug {
		fmt.Printf("[DEBUG] read %d bytes of result data\n", len(resultData))
		if len(resultData) < 256 {
			fmt.Printf("[DEBUG] result data (string) = %q\n", string(resultData))
			fmt.Printf("[DEBUG] result data (hex)    = % x\n", resultData)
		}
	}

	// Try to parse as JSON to validate
	var jsonTest interface{}
	if err := json.Unmarshal(resultData, &jsonTest); err != nil {
		fmt.Printf("[DEBUG] Warning: result data is not valid JSON: %v\n", err)
		// Continue anyway, maybe it's binary data
	}

	// Construct gas report
	gr := types.GasReport{
		Limit:          gasLimit,
		Remaining:      gasLimit - runtimeEnv.gasUsed,
		UsedExternally: 0,
		UsedInternally: runtimeEnv.gasUsed,
	}

	if printDebug {
		fmt.Printf("[DEBUG] GasReport: Used=%d (internally), Remaining=%d, Limit=%d\n",
			gr.UsedInternally, gr.Remaining, gr.Limit)
		fmt.Println("==============================================================")
	}

	return resultData, gr, nil
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
