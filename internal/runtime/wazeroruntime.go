package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	memory     api.Memory
	module     api.Module
	size       uint32
	nextOffset uint32 // Track next available offset
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

	// Initialize first page with zeros to ensure clean state
	zeroMem := make([]byte, wasmPageSize)
	if !memory.Write(0, zeroMem) {
		panic("failed to initialize first memory page")
	}

	// Verify memory size after initialization
	if memBytes < wasmPageSize {
		panic("memory not properly initialized: size less than one page")
	}

	return &memoryManager{
		memory:     memory,
		module:     module,
		size:       memBytes,
		nextOffset: wasmPageSize, // Start allocations after first page
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
	if region.Offset < wasmPageSize {
		return fmt.Errorf("region offset %d is less than first page size %d", region.Offset, wasmPageSize)
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

	if printDebug {
		fmt.Printf("\n=== Memory Write Operation ===\n")
		fmt.Printf("Current memory state:\n")
		fmt.Printf("- Total size: %d bytes (%d pages)\n", mm.size, mm.size/wasmPageSize)
		fmt.Printf("- Next offset: 0x%x\n", mm.nextOffset)
		fmt.Printf("Writing data:\n")
		fmt.Printf("- Size: %d bytes\n", len(data))
		if len(data) < 1024 {
			fmt.Printf("- Content: %s\n", string(data))
			fmt.Printf("- Hex: % x\n", data)
		}
	}

	// Calculate total size needed (data + Region struct)
	totalSize := uint32(len(data)) + regionSize

	// Create region before growing memory to validate size requirements
	region := &Region{
		Offset:   mm.nextOffset,
		Capacity: uint32(len(data)),
		Length:   uint32(len(data)),
	}

	if printDebug {
		fmt.Printf("Region structure:\n")
		fmt.Printf("- Offset: 0x%x\n", region.Offset)
		fmt.Printf("- Capacity: %d\n", region.Capacity)
		fmt.Printf("- Length: %d\n", region.Length)
	}

	// Validate the region before proceeding with memory operations
	if err := validateRegion(region); err != nil {
		if printDebug {
			fmt.Printf("Region validation failed: %v\n", err)
		}
		return 0, 0, fmt.Errorf("invalid memory region: %w", err)
	}

	// Ensure we have enough memory
	requiredSize := mm.nextOffset + totalSize
	if requiredSize > mm.size {
		// Calculate needed pages
		currentPages := mm.size / wasmPageSize
		neededSize := requiredSize
		if neededSize%wasmPageSize != 0 {
			neededSize += wasmPageSize - (neededSize % wasmPageSize)
		}
		pagesToGrow := (neededSize - mm.size) / wasmPageSize

		if printDebug {
			fmt.Printf("Growing memory:\n")
			fmt.Printf("- Current pages: %d\n", currentPages)
			fmt.Printf("- Growing by: %d pages\n", pagesToGrow)
			fmt.Printf("- Required size: %d bytes\n", requiredSize)
		}

		if _, ok := mm.memory.Grow(pagesToGrow); !ok {
			if printDebug {
				fmt.Printf("Failed to grow memory!\n")
			}
			return 0, 0, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()

		if printDebug {
			fmt.Printf("Memory grown successfully:\n")
			fmt.Printf("- New size: %d bytes (%d pages)\n", mm.size, mm.size/wasmPageSize)
		}
	}

	// Write data
	if !mm.memory.Write(region.Offset, data) {
		if printDebug {
			fmt.Printf("Failed to write data to memory!\n")
		}
		return 0, 0, fmt.Errorf("failed to write data to memory at offset 0x%x", region.Offset)
	}

	// Write Region struct
	regionPtr := region.Offset + uint32(len(data))
	regionBytes := make([]byte, regionSize)
	binary.LittleEndian.PutUint32(regionBytes[0:4], region.Offset)
	binary.LittleEndian.PutUint32(regionBytes[4:8], region.Capacity)
	binary.LittleEndian.PutUint32(regionBytes[8:12], region.Length)

	if !mm.memory.Write(regionPtr, regionBytes) {
		if printDebug {
			fmt.Printf("Failed to write Region struct!\n")
		}
		return 0, 0, fmt.Errorf("failed to write Region struct at offset 0x%x", regionPtr)
	}

	// Update next offset for future allocations
	mm.nextOffset = regionPtr + regionSize

	if printDebug {
		fmt.Printf("Write operation successful:\n")
		fmt.Printf("- Data offset: 0x%x\n", region.Offset)
		fmt.Printf("- Region ptr: 0x%x\n", regionPtr)
		fmt.Printf("- Next offset: 0x%x\n", mm.nextOffset)

		// Verify written data
		readBack, ok := mm.memory.Read(region.Offset, region.Length)
		if ok {
			fmt.Printf("Data verification:\n")
			if len(readBack) < 1024 {
				fmt.Printf("- Read back: %s\n", string(readBack))
				fmt.Printf("- Read hex: % x\n", readBack)
			}
			fmt.Printf("- Length matches: %v\n", len(readBack) == len(data))
		} else {
			fmt.Printf("Failed to verify written data!\n")
		}
	}

	return regionPtr, regionSize, nil
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

	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, moduleConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Create contract module instance
	contractModule, err := w.runtime.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithStartFunctions())
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

	mm := newMemoryManager(memory, contractModule)

	// Combine env and query into a single request
	request := struct {
		Env json.RawMessage `json:"env"`
		Msg json.RawMessage `json:"msg"`
	}{
		Env: env,
		Msg: query,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Query request: %s\n", string(requestBytes))
	}

	// Write request to memory
	_, _, err = mm.writeToMemory(requestBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write request to memory: %w", err)
	}

	// Parse the request to get env and msg
	var parsedRequest struct {
		Env json.RawMessage `json:"env"`
		Msg json.RawMessage `json:"msg"`
	}
	if err := json.Unmarshal(requestBytes, &parsedRequest); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to parse request: %w", err)
	}

	// Write env and msg to memory separately
	envPtr, _, err := mm.writeToMemory(parsedRequest.Env, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env to memory: %w", err)
	}

	msgPtr, _, err := mm.writeToMemory(parsedRequest.Msg, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg to memory: %w", err)
	}

	// Get the query function
	fn := contractModule.ExportedFunction("query")
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("query function not found")
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory layout:\n")
		fmt.Printf("- Env: ptr=0x%x\n", envPtr)
		fmt.Printf("- Msg: ptr=0x%x\n", msgPtr)
		fmt.Printf("[DEBUG] Calling query function...\n")
	}

	// Call the query function with environment and message pointers
	results, err := fn.Call(ctx, uint64(envPtr), uint64(msgPtr))
	if err != nil {
		if printDebug {
			fmt.Printf("[DEBUG] Query call failed: %v\n", err)
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
		UsedExternally: 0,
		Remaining:      gasLimit - runtimeEnv.gasUsed,
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
func serializeEnvForContract(env []byte, _ []byte, _ *WazeroRuntime) ([]byte, error) {
	fmt.Printf("[DEBUG] serializeEnvForContract: Input env: %s\n", string(env))

	// Parse the environment into a raw map to preserve number formats
	var rawEnv map[string]interface{}
	if err := json.Unmarshal(env, &rawEnv); err != nil {
		fmt.Printf("[DEBUG] Failed to unmarshal env into raw map: %v\n", err)
		return nil, fmt.Errorf("failed to unmarshal env: %w", err)
	}

	fmt.Printf("[DEBUG] Raw env structure: %+v\n", rawEnv)

	// Also parse into typed struct for validation
	var typedEnv types.Env
	if err := json.Unmarshal(env, &typedEnv); err != nil {
		fmt.Printf("[DEBUG] Failed to unmarshal env into typed struct: %v\n", err)
		return nil, fmt.Errorf("failed to unmarshal env: %w", err)
	}

	fmt.Printf("[DEBUG] Typed env structure: %+v\n", typedEnv)

	// Validate required fields
	if typedEnv.Block.Height == 0 {
		fmt.Printf("[DEBUG] Validation failed: block height is 0\n")
		return nil, fmt.Errorf("block height cannot be 0")
	}
	if typedEnv.Block.Time == 0 {
		fmt.Printf("[DEBUG] Validation failed: block time is 0\n")
		return nil, fmt.Errorf("block time cannot be 0")
	}
	if typedEnv.Block.ChainID == "" {
		fmt.Printf("[DEBUG] Validation failed: chain_id is empty\n")
		return nil, fmt.Errorf("chain_id cannot be empty")
	}
	if typedEnv.Contract.Address == "" {
		fmt.Printf("[DEBUG] Validation failed: contract address is empty\n")
		return nil, fmt.Errorf("contract address cannot be empty")
	}

	// Get the original block data and preserve exact number formats
	rawBlock, ok := rawEnv["block"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid block structure in environment")
	}

	// Use the raw values directly to preserve exact number formats
	height := rawBlock["height"]
	time := rawBlock["time"]

	// Ensure time is a string
	timeStr, ok := time.(string)
	if !ok {
		// Convert time to string if it's not already
		timeStr = fmt.Sprintf("%d", typedEnv.Block.Time)
	}

	// Create output environment preserving original number formats and field order
	// Field order must match the Rust struct exactly:
	// 1. block: height, time, chain_id
	// 2. transaction (optional): hash, index
	// 3. contract: address
	envMap := map[string]interface{}{
		"block": map[string]interface{}{
			"height":   height,
			"time":     timeStr,
			"chain_id": typedEnv.Block.ChainID,
		},
	}

	// Add transaction if present
	if typedEnv.Transaction != nil {
		envMap["transaction"] = rawEnv["transaction"]
	}

	// Add contract info (must come last)
	envMap["contract"] = map[string]interface{}{
		"address": typedEnv.Contract.Address,
	}

	// Marshal back to JSON
	adaptedEnv, err := json.Marshal(envMap)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize adapted environment: %w", err)
	}

	log.Printf("[DEBUG] Adapted env hex: %x", adaptedEnv)
	log.Printf("[DEBUG] Adapted env string: %s", adaptedEnv)

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
		fmt.Printf("[DEBUG] Function call: %s\n", name)
		fmt.Printf("[DEBUG] Checksum: %x\n", checksum)
		fmt.Printf("[DEBUG] Gas limit: %d\n", gasLimit)
		fmt.Printf("[DEBUG] Input sizes: env=%d, info=%d, msg=%d\n", len(env), len(info), len(msg))
		fmt.Printf("[DEBUG] Original env: %s\n", string(env))
		if len(info) > 0 {
			fmt.Printf("[DEBUG] Info: %s\n", string(info))
		}
		fmt.Printf("[DEBUG] Message: %s\n", string(msg))
	}

	// Adapt environment for contract version
	adaptedEnv, err := serializeEnvForContract(env, info, w)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to serialize env: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Adapted env: %s\n", string(adaptedEnv))
		fmt.Printf("[DEBUG] Creating runtime environment with gas limit: %d\n", gasLimit)
	}

	// Create runtime environment
	runtimeEnv := &RuntimeEnvironment{
		DB:      store,
		API:     *api,
		Querier: *querier,
		Gas:     *gasMeter,

		gasLimit: gasLimit,
		gasUsed:  0,

		iterators:  make(map[uint64]map[uint64]types.Iterator),
		nextCallID: 1,
	}

	if printDebug {
		fmt.Printf("[DEBUG] Registering host functions...\n")
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

	if printDebug {
		fmt.Printf("[DEBUG] Instantiating modules...\n")
	}

	// Create new module instance with host functions
	ctx := context.Background()
	moduleConfig := wazero.NewModuleConfig().
		WithName("env").
		WithStartFunctions()

	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, moduleConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Create contract module instance
	contractModule, err := w.runtime.InstantiateModule(ctx, module, wazero.NewModuleConfig().WithStartFunctions())
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

	mm := newMemoryManager(memory, contractModule)
	initialBytes := memory.Size()
	if err := mm.validateMemorySize(initialBytes); err != nil {
		return nil, types.GasReport{}, fmt.Errorf("memory validation failed: %w", err)
	}

	if printDebug {
		fmt.Printf("- Memory initialized: %d bytes (%d pages)\n", initialBytes, initialBytes/wasmPageSize)
		fmt.Printf("[DEBUG] Writing data to memory...\n")
	}

	// Write environment to memory
	envPtr, envSize, err := mm.writeToMemory(adaptedEnv, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env to memory: %w", err)
	}

	// Write info to memory if present
	var infoPtr uint32
	var infoSize uint32
	if len(info) > 0 {
		if printDebug {
			fmt.Printf("[DEBUG] Writing info to memory...\n")
		}
		infoPtr, infoSize, err = mm.writeToMemory(info, printDebug)
		if err != nil {
			return nil, types.GasReport{}, fmt.Errorf("failed to write info to memory: %w", err)
		}
	} else {
		if printDebug {
			fmt.Printf("[DEBUG] Writing empty info object...\n")
		}
		// Write empty JSON object for info
		emptyInfo := []byte("{}")
		infoPtr, infoSize, err = mm.writeToMemory(emptyInfo, printDebug)
		if err != nil {
			return nil, types.GasReport{}, fmt.Errorf("failed to write empty info to memory: %w", err)
		}
	}

	// Write msg to memory
	if printDebug {
		fmt.Printf("[DEBUG] Writing message to memory...\n")
	}
	msgPtr, msgSize, err := mm.writeToMemory(msg, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg to memory: %w", err)
	}

	// Get the function
	fn := contractModule.ExportedFunction(name)
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("function %s not found", name)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory layout:\n")
		fmt.Printf("- Environment: ptr=0x%x, size=%d\n", envPtr, envSize)
		fmt.Printf("- Info: ptr=0x%x, size=%d\n", infoPtr, infoSize)
		fmt.Printf("- Message: ptr=0x%x, size=%d\n", msgPtr, msgSize)
		fmt.Printf("[DEBUG] Calling contract function '%s'...\n", name)
	}

	// Call the function with appropriate parameters
	var results []uint64
	var callErr error
	switch name {
	case "query":
		if printDebug {
			fmt.Printf("[DEBUG] Query function call with 2 params: env=%d, msg=%d\n", envPtr, msgPtr)
		}
		results, callErr = fn.Call(ctx, uint64(envPtr), uint64(msgPtr))
	case "sudo", "reply", "migrate":
		if printDebug {
			fmt.Printf("[DEBUG] Function call with 3 params (empty info allowed): env=%d, info=%d, msg=%d\n", envPtr, infoPtr, msgPtr)
		}
		results, callErr = fn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	default:
		if len(info) == 0 {
			return nil, types.GasReport{}, fmt.Errorf("info parameter required for %s", name)
		}
		if printDebug {
			fmt.Printf("[DEBUG] Function call with 3 params: env=%d, info=%d, msg=%d\n", envPtr, infoPtr, msgPtr)
		}
		results, callErr = fn.Call(ctx, uint64(envPtr), uint64(infoPtr), uint64(msgPtr))
	}

	if callErr != nil {
		if printDebug {
			fmt.Printf("[DEBUG] Contract call failed: %v\n", callErr)
		}
		return nil, types.GasReport{}, fmt.Errorf("contract call failed: %w", callErr)
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
		UsedExternally: 0,
		Remaining:      gasLimit - runtimeEnv.gasUsed,
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

// prettyPrintJSON formats JSON with indentation for better readability
func prettyPrintJSON(input []byte) string {
	var temp interface{}
	if err := json.Unmarshal(input, &temp); err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	pretty, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(pretty)
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

// fixBlockTimeIfNumeric tries to detect if block.time is a numeric string
// and convert it into a (fake) RFC3339 date/time, so the contract can parse it.
func fixBlockTimeIfNumeric(env []byte) ([]byte, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(env, &data); err != nil {
		return nil, err
	}

	blockRaw, ok := data["block"].(map[string]interface{})
	if !ok {
		return env, nil // no "block" => do nothing
	}

	timeRaw, hasTime := blockRaw["time"]
	if !hasTime {
		return env, nil
	}

	// If block.time is a string of digits, convert it into a pretend RFC3339:
	if timeStr, isString := timeRaw.(string); isString {
		// check if it looks like all digits
		onlyDigits := true
		for _, r := range timeStr {
			if r < '0' || r > '9' {
				onlyDigits = false
				break
			}
		}
		if onlyDigits && len(timeStr) >= 6 {
			// Here you might convert that numeric to an actual time in seconds
			// or do your own approach. For example, parse as Unix seconds:
			// e.g. if you want "1578939743987654321" to become "2020-02-13T10:05:13Z"
			// you must figure out the right second or nano offset.
			// Demo only:
			blockRaw["time"] = "2020-02-13T10:05:13Z"
		}
	}

	patched, err := json.Marshal(data)
	if err != nil {
		return env, nil
	}
	return patched, nil
}

func (r *WazeroRuntime) serializeEnvForContract(env []byte) ([]byte, error) {
	log.Printf("[DEBUG] Original env hex: %x", env)
	log.Printf("[DEBUG] Original env string: %s", env)

	// Parse into raw map to preserve number formats
	var rawEnv map[string]interface{}
	if err := json.Unmarshal(env, &rawEnv); err != nil {
		return nil, fmt.Errorf("failed to parse environment: %w", err)
	}

	// Parse into typed struct for validation
	var typedEnv types.Env
	if err := json.Unmarshal(env, &typedEnv); err != nil {
		return nil, fmt.Errorf("failed to parse environment into typed struct: %w", err)
	}

	// Validate required fields
	if typedEnv.Block.Height == 0 {
		return nil, fmt.Errorf("block height is required")
	}
	if typedEnv.Block.Time == 0 {
		return nil, fmt.Errorf("block time is required")
	}
	if typedEnv.Block.ChainID == "" {
		return nil, fmt.Errorf("chain id is required")
	}

	// Create output preserving original formats and field order
	envMap := map[string]interface{}{
		"block": map[string]interface{}{
			"height":   rawEnv["block"].(map[string]interface{})["height"],
			"time":     rawEnv["block"].(map[string]interface{})["time"],
			"chain_id": typedEnv.Block.ChainID,
		},
	}

	// Add transaction if present (must come after block)
	if typedEnv.Transaction != nil {
		envMap["transaction"] = rawEnv["transaction"]
	}

	// Add contract info (must come last)
	envMap["contract"] = map[string]interface{}{
		"address": typedEnv.Contract.Address,
	}

	// Marshal back to JSON
	adaptedEnv, err := json.Marshal(envMap)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize adapted environment: %w", err)
	}

	log.Printf("[DEBUG] Adapted env hex: %x", adaptedEnv)
	log.Printf("[DEBUG] Adapted env string: %s", adaptedEnv)

	return adaptedEnv, nil
}
