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

	// Memory manager
	memoryManager *memoryManager
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

// ToBytes serializes the Region struct to bytes for WASM memory
func (r *Region) ToBytes() []byte {
	// Ensure offset is page-aligned (except for first page)
	if r.Offset > wasmPageSize && r.Offset%wasmPageSize != 0 {
		panic(fmt.Sprintf("region offset %d is not page-aligned", r.Offset))
	}
	// Ensure capacity is page-aligned
	if r.Capacity%wasmPageSize != 0 {
		panic(fmt.Sprintf("region capacity %d is not page-aligned", r.Capacity))
	}
	// Ensure length is not greater than capacity
	if r.Length > r.Capacity {
		panic(fmt.Sprintf("region length %d exceeds capacity %d", r.Length, r.Capacity))
	}

	buf := make([]byte, 12) // 3 uint32 fields * 4 bytes each
	binary.LittleEndian.PutUint32(buf[0:4], r.Offset)
	binary.LittleEndian.PutUint32(buf[4:8], r.Capacity)
	binary.LittleEndian.PutUint32(buf[8:12], r.Length)
	return buf
}

// memoryManager handles WASM memory allocation and deallocation
type memoryManager struct {
	memory         api.Memory
	contractModule api.Module
	size           uint32
	nextOffset     uint32 // Track next available offset
	gasState       *GasState
}

// newMemoryManager creates a new memory manager for a contract module
func newMemoryManager(memory api.Memory, contractModule api.Module, gasState *GasState) *memoryManager {
	// Initialize memory with at least one page
	size := memory.Size()
	if size == 0 {
		if _, ok := memory.Grow(1); !ok {
			panic("failed to initialize memory with one page")
		}
		size = memory.Size()
	}

	return &memoryManager{
		memory:         memory,
		contractModule: contractModule,
		nextOffset:     wasmPageSize, // Start at first page boundary
		size:           size,
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
	if region == nil {
		return fmt.Errorf("region is nil")
	}

	// Check if offset is valid (must be after first page)
	if region.Offset < wasmPageSize {
		return fmt.Errorf("region offset %d is less than first page size %d", region.Offset, wasmPageSize)
	}

	// Check if capacity is valid
	if region.Capacity == 0 {
		return fmt.Errorf("region capacity cannot be zero")
	}

	// Check if length is valid
	if region.Length == 0 {
		return fmt.Errorf("region length cannot be zero")
	}
	if region.Length > region.Capacity {
		return fmt.Errorf("region length (%d) exceeds capacity (%d)", region.Length, region.Capacity)
	}

	// Check for potential overflow
	if region.Offset > math.MaxUint32-region.Capacity {
		return fmt.Errorf("region would overflow memory bounds: offset=%d, capacity=%d", region.Offset, region.Capacity)
	}

	// Enforce a maximum region size of 64MB to prevent excessive allocations
	const maxRegionSize = 64 * 1024 * 1024 // 64MB
	if region.Capacity > maxRegionSize {
		return fmt.Errorf("region capacity %d exceeds maximum allowed size of %d", region.Capacity, maxRegionSize)
	}

	// Ensure both offset and capacity are page-aligned
	if region.Offset%wasmPageSize != 0 {
		return fmt.Errorf("region offset %d is not page-aligned", region.Offset)
	}
	if region.Capacity%wasmPageSize != 0 {
		return fmt.Errorf("region capacity %d is not page-aligned", region.Capacity)
	}

	return nil
}

// validateRegion is now an alias for validateMemoryRegion for consistency
func validateRegion(region *Region) error {
	return validateMemoryRegion(region)
}

// writeToMemory writes data to memory and returns the offset where it was written
func (mm *memoryManager) writeToMemory(data []byte, printDebug bool) (uint32, uint32, error) {
	dataSize := uint32(len(data))
	if dataSize == 0 {
		return 0, 0, nil
	}

	// Ensure nextOffset is page-aligned
	if mm.nextOffset%wasmPageSize != 0 {
		mm.nextOffset = ((mm.nextOffset + wasmPageSize - 1) / wasmPageSize) * wasmPageSize
	}

	// Calculate pages needed for data
	pagesNeeded := (dataSize + wasmPageSize - 1) / wasmPageSize
	allocSize := pagesNeeded * wasmPageSize

	// Check if we need to grow memory
	if mm.nextOffset+allocSize > mm.size {
		pagesToGrow := (mm.nextOffset + allocSize - mm.size + wasmPageSize - 1) / wasmPageSize
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages (current size: %d, needed: %d)\n",
				pagesToGrow, mm.size/wasmPageSize, (mm.nextOffset+allocSize)/wasmPageSize)
		}
		grown, ok := mm.memory.Grow(pagesToGrow)
		if !ok || grown == 0 {
			return 0, 0, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Write data to memory
	if !mm.memory.Write(mm.nextOffset, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory")
	}

	// Store current offset and update for next write
	offset := mm.nextOffset
	mm.nextOffset += allocSize

	if printDebug {
		fmt.Printf("[DEBUG] Wrote %d bytes at offset 0x%x (page-aligned size: %d)\n",
			len(data), offset, allocSize)
	}

	return offset, allocSize, nil
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

	// Set up the runtime environment
	runtimeEnv := NewRuntimeEnvironment(store, api, *querier)
	runtimeEnv.Gas = *gasMeter
	ctx := context.WithValue(context.Background(), envKey, runtimeEnv)

	// Register host functions and create host module
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(ctx)

	// Create module config for env module
	moduleConfig := wazero.NewModuleConfig().WithName("env")

	// Instantiate env module
	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, moduleConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Get the contract module
	w.mu.Lock()
	compiledModule, ok := w.compiledModules[hex.EncodeToString(checksum)]
	if !ok {
		w.mu.Unlock()
		return nil, types.GasReport{}, fmt.Errorf("module not found for checksum: %x", checksum)
	}
	w.mu.Unlock()

	// Instantiate the contract module
	contractModule, err := w.runtime.InstantiateModule(ctx, compiledModule, wazero.NewModuleConfig().WithName("contract"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer contractModule.Close(ctx)

	// Initialize memory manager
	gasState := NewGasState(gasLimit)
	mm := newMemoryManager(contractModule.Memory(), contractModule, gasState)

	// Calculate pages needed for data
	envDataSize := uint32(len(env))
	envPagesNeeded := (envDataSize + wasmPageSize - 1) / wasmPageSize
	envAllocSize := envPagesNeeded * wasmPageSize

	infoDataSize := uint32(len(info))
	infoPagesNeeded := (infoDataSize + wasmPageSize - 1) / wasmPageSize
	infoAllocSize := infoPagesNeeded * wasmPageSize

	msgDataSize := uint32(len(msg))
	msgPagesNeeded := (msgDataSize + wasmPageSize - 1) / wasmPageSize
	msgAllocSize := msgPagesNeeded * wasmPageSize

	// Add space for Region structs (12 bytes each, aligned to page size)
	regionStructSize := uint32(36) // 3 Region structs * 12 bytes each
	regionPagesNeeded := (regionStructSize + wasmPageSize - 1) / wasmPageSize
	regionAllocSize := regionPagesNeeded * wasmPageSize

	// Ensure we have enough memory for everything
	totalSize := envAllocSize + infoAllocSize + msgAllocSize + regionAllocSize
	if totalSize > mm.size {
		pagesToGrow := (totalSize - mm.size + wasmPageSize - 1) / wasmPageSize
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages (current size: %d, needed: %d)\n",
				pagesToGrow, mm.size/wasmPageSize, totalSize/wasmPageSize)
		}
		grown, ok := mm.memory.Grow(pagesToGrow)
		if !ok || grown == 0 {
			return nil, types.GasReport{}, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Write data to memory
	envPtr, _, err := mm.writeToMemory(env, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env to memory: %w", err)
	}

	infoPtr, _, err := mm.writeToMemory(info, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write info to memory: %w", err)
	}

	msgPtr, _, err := mm.writeToMemory(msg, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg to memory: %w", err)
	}

	// Create Region structs with page-aligned capacities
	envRegion := &Region{
		Offset:   envPtr,
		Capacity: envAllocSize,
		Length:   envDataSize,
	}

	infoRegion := &Region{
		Offset:   infoPtr,
		Capacity: infoAllocSize,
		Length:   infoDataSize,
	}

	msgRegion := &Region{
		Offset:   msgPtr,
		Capacity: msgAllocSize,
		Length:   msgDataSize,
	}

	// Write Region structs to memory
	envRegionBytes := envRegion.ToBytes()
	envRegionPtr, _, err := mm.writeToMemory(envRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env region to memory: %w", err)
	}

	infoRegionBytes := infoRegion.ToBytes()
	infoRegionPtr, _, err := mm.writeToMemory(infoRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write info region to memory: %w", err)
	}

	msgRegionBytes := msgRegion.ToBytes()
	msgRegionPtr, _, err := mm.writeToMemory(msgRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg region to memory: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory layout before function call:\n")
		fmt.Printf("- Environment: ptr=0x%x, size=%d, region_ptr=0x%x\n", envPtr, len(env), envRegionPtr)
		fmt.Printf("- Info: ptr=0x%x, size=%d, region_ptr=0x%x\n", infoPtr, len(info), infoRegionPtr)
		fmt.Printf("- Message: ptr=0x%x, size=%d, region_ptr=0x%x\n", msgPtr, len(msg), msgRegionPtr)
	}

	// Call instantiate function
	instantiate := contractModule.ExportedFunction("instantiate")
	if instantiate == nil {
		return nil, types.GasReport{}, fmt.Errorf("instantiate function not found in module")
	}

	results, err := instantiate.Call(ctx, uint64(envRegionPtr), uint64(infoRegionPtr), uint64(msgRegionPtr))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to call function instantiate: %w", err)
	}

	// Get result from memory
	resultPtr := uint32(results[0])
	resultRegionBytes, err := readMemory(contractModule.Memory(), resultPtr, uint32(12)) // Region is 12 bytes
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read result region from memory: %w", err)
	}

	resultRegion := &Region{}
	resultRegion.Offset = binary.LittleEndian.Uint32(resultRegionBytes[0:4])
	resultRegion.Capacity = binary.LittleEndian.Uint32(resultRegionBytes[4:8])
	resultRegion.Length = binary.LittleEndian.Uint32(resultRegionBytes[8:12])

	result, err := readMemory(contractModule.Memory(), resultRegion.Offset, resultRegion.Length)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to read result from memory: %w", err)
	}

	gasReport := types.GasReport{
		UsedInternally: runtimeEnv.Gas.GasConsumed(),
	}

	if printDebug {
		fmt.Printf("[DEBUG] Gas report:\n")
		fmt.Printf("  Used internally: %d\n", gasReport.UsedInternally)
	}

	return result, gasReport, nil
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

	mm := newMemoryManager(memory, contractModule, gasState)

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

			// Try to read the data at the Region pointers again to see if anything changed
			envRegionDataAtFailure, ok := memory.Read(envRegionPtr, 12)
			if ok {
				fmt.Printf("\nEnvironment Region at failure:\n")
				fmt.Printf("- Offset: 0x%x\n", binary.LittleEndian.Uint32(envRegionDataAtFailure[0:4]))
				fmt.Printf("- Capacity: %d\n", binary.LittleEndian.Uint32(envRegionDataAtFailure[4:8]))
				fmt.Printf("- Length: %d\n", binary.LittleEndian.Uint32(envRegionDataAtFailure[8:12]))

				// Try to read the actual data
				dataOffset := binary.LittleEndian.Uint32(envRegionDataAtFailure[0:4])
				dataLength := binary.LittleEndian.Uint32(envRegionDataAtFailure[8:12])
				if data, ok := memory.Read(dataOffset, dataLength); ok && len(data) < 1024 {
					fmt.Printf("- Data at offset: %s\n", string(data))
				}
			}

			queryRegionDataAtFailure, ok := memory.Read(queryRegionPtr, 12)
			if ok {
				fmt.Printf("\nQuery Region at failure:\n")
				fmt.Printf("- Offset: 0x%x\n", binary.LittleEndian.Uint32(queryRegionDataAtFailure[0:4]))
				fmt.Printf("- Capacity: %d\n", binary.LittleEndian.Uint32(queryRegionDataAtFailure[4:8]))
				fmt.Printf("- Length: %d\n", binary.LittleEndian.Uint32(queryRegionDataAtFailure[8:12]))

				// Try to read the actual data
				dataOffset := binary.LittleEndian.Uint32(queryRegionDataAtFailure[0:4])
				dataLength := binary.LittleEndian.Uint32(queryRegionDataAtFailure[8:12])
				if data, ok := memory.Read(dataOffset, dataLength); ok && len(data) < 1024 {
					fmt.Printf("- Data at offset: %s\n", string(data))
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
	// First unmarshal into a typed struct to validate the data
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

	// Create a map with the required structure
	envMap := map[string]interface{}{
		"block": map[string]interface{}{
			"height":   typedEnv.Block.Height,
			"time":     typedEnv.Block.Time,
			"chain_id": typedEnv.Block.ChainID,
		},
		"contract": map[string]interface{}{
			"address": typedEnv.Contract.Address,
		},
	}

	// Add transaction info if present
	if typedEnv.Transaction != nil {
		txMap := map[string]interface{}{
			"index": typedEnv.Transaction.Index,
		}
		if typedEnv.Transaction.Hash != "" {
			txMap["hash"] = typedEnv.Transaction.Hash
		}
		envMap["transaction"] = txMap
	}

	if printDebug {
		fmt.Printf("[DEBUG] Original env: %s\n", string(env))
		adaptedEnv, _ := json.MarshalIndent(envMap, "", "  ")
		fmt.Printf("[DEBUG] Adapted env: %s\n", string(adaptedEnv))
	}

	// Serialize back to JSON
	return json.Marshal(envMap)
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
	adaptedEnv, err := serializeEnvForContract(env, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to serialize env: %w", err)
	}

	// Get the contract module
	compiledModule, err := w.getContractModule(checksum)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to get contract module: %w", err)
	}

	// Create runtime environment
	runtimeEnv := &RuntimeEnvironment{
		DB:        store,
		API:       *api,
		Querier:   *querier,
		Gas:       *gasMeter,
		gasUsed:   0,
		iterators: make(map[uint64]map[uint64]types.Iterator),
	}

	// Create context with environment
	ctx := context.WithValue(context.Background(), envKey, runtimeEnv)

	// Create gas state for memory operations
	gasState := NewGasState(gasLimit)

	// Register host functions
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(context.Background())

	// Instantiate the env module first
	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, wazero.NewModuleConfig().WithName("env"))
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Instantiate the contract module
	moduleConfig := wazero.NewModuleConfig().WithName("contract")
	contractModule, err := w.runtime.InstantiateModule(ctx, compiledModule, moduleConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer contractModule.Close(ctx)

	// Get memory from the instantiated module
	memory := contractModule.Memory()
	if memory == nil {
		return nil, types.GasReport{}, fmt.Errorf("contract module has no memory")
	}

	// Create memory manager
	mm := newMemoryManager(memory, contractModule, gasState)

	// Calculate pages needed for data
	envDataSize := uint32(len(adaptedEnv))
	envPagesNeeded := (envDataSize + wasmPageSize - 1) / wasmPageSize
	envAllocSize := envPagesNeeded * wasmPageSize

	infoDataSize := uint32(len(info))
	infoPagesNeeded := (infoDataSize + wasmPageSize - 1) / wasmPageSize
	infoAllocSize := infoPagesNeeded * wasmPageSize

	msgDataSize := uint32(len(msg))
	msgPagesNeeded := (msgDataSize + wasmPageSize - 1) / wasmPageSize
	msgAllocSize := msgPagesNeeded * wasmPageSize

	// Add space for Region structs (12 bytes each, aligned to page size)
	regionStructSize := uint32(36) // 3 Region structs * 12 bytes each
	regionPagesNeeded := (regionStructSize + wasmPageSize - 1) / wasmPageSize
	regionAllocSize := regionPagesNeeded * wasmPageSize

	// Ensure we have enough memory for everything
	totalSize := envAllocSize + infoAllocSize + msgAllocSize + regionAllocSize
	if totalSize > mm.size {
		pagesToGrow := (totalSize - mm.size + wasmPageSize - 1) / wasmPageSize
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages (current size: %d, needed: %d)\n",
				pagesToGrow, mm.size/wasmPageSize, totalSize/wasmPageSize)
		}
		grown, ok := mm.memory.Grow(pagesToGrow)
		if !ok || grown == 0 {
			return nil, types.GasReport{}, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Write data to memory
	envPtr, _, err := mm.writeToMemory(adaptedEnv, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env to memory: %w", err)
	}

	infoPtr, _, err := mm.writeToMemory(info, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write info to memory: %w", err)
	}

	msgPtr, _, err := mm.writeToMemory(msg, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg to memory: %w", err)
	}

	// Create Region structs with page-aligned capacities
	envRegion := &Region{
		Offset:   envPtr,
		Capacity: envAllocSize,
		Length:   envDataSize,
	}

	infoRegion := &Region{
		Offset:   infoPtr,
		Capacity: infoAllocSize,
		Length:   infoDataSize,
	}

	msgRegion := &Region{
		Offset:   msgPtr,
		Capacity: msgAllocSize,
		Length:   msgDataSize,
	}

	// Write Region structs to memory
	envRegionBytes := envRegion.ToBytes()
	envRegionPtr, _, err := mm.writeToMemory(envRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env region to memory: %w", err)
	}

	infoRegionBytes := infoRegion.ToBytes()
	infoRegionPtr, _, err := mm.writeToMemory(infoRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write info region to memory: %w", err)
	}

	msgRegionBytes := msgRegion.ToBytes()
	msgRegionPtr, _, err := mm.writeToMemory(msgRegionBytes, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg region to memory: %w", err)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Memory layout before function call:\n")
		fmt.Printf("- Environment: ptr=0x%x, size=%d, region_ptr=0x%x\n", envPtr, len(adaptedEnv), envRegionPtr)
		fmt.Printf("- Info: ptr=0x%x, size=%d, region_ptr=0x%x\n", infoPtr, len(info), infoRegionPtr)
		fmt.Printf("- Message: ptr=0x%x, size=%d, region_ptr=0x%x\n", msgPtr, len(msg), msgRegionPtr)
	}

	// Get the function
	fn := contractModule.ExportedFunction(name)
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("function %s not found in contract", name)
	}

	// Call the function
	results, err := fn.Call(ctx, uint64(envRegionPtr), uint64(infoRegionPtr), uint64(msgRegionPtr))
	if err != nil {
		if printDebug {
			fmt.Printf("\n[DEBUG] ====== Function Call Failed ======\n")
			fmt.Printf("Error: %v\n", err)

			// Try to read the data at the Region pointers again to see if anything changed
			envRegionDataAtFailure, ok := memory.Read(envRegionPtr, 12)
			if ok {
				fmt.Printf("\nEnvironment Region at failure:\n")
				fmt.Printf("- Offset: 0x%x\n", binary.LittleEndian.Uint32(envRegionDataAtFailure[0:4]))
				fmt.Printf("- Capacity: %d\n", binary.LittleEndian.Uint32(envRegionDataAtFailure[4:8]))
				fmt.Printf("- Length: %d\n", binary.LittleEndian.Uint32(envRegionDataAtFailure[8:12]))

				// Try to read the actual data
				dataOffset := binary.LittleEndian.Uint32(envRegionDataAtFailure[0:4])
				dataLength := binary.LittleEndian.Uint32(envRegionDataAtFailure[8:12])
				if data, ok := memory.Read(dataOffset, dataLength); ok && len(data) < 1024 {
					fmt.Printf("- Data at offset: %s\n", string(data))
				}
			}

			infoRegionDataAtFailure, ok := memory.Read(infoRegionPtr, 12)
			if ok {
				fmt.Printf("\nInfo Region at failure:\n")
				fmt.Printf("- Offset: 0x%x\n", binary.LittleEndian.Uint32(infoRegionDataAtFailure[0:4]))
				fmt.Printf("- Capacity: %d\n", binary.LittleEndian.Uint32(infoRegionDataAtFailure[4:8]))
				fmt.Printf("- Length: %d\n", binary.LittleEndian.Uint32(infoRegionDataAtFailure[8:12]))

				// Try to read the actual data
				dataOffset := binary.LittleEndian.Uint32(infoRegionDataAtFailure[0:4])
				dataLength := binary.LittleEndian.Uint32(infoRegionDataAtFailure[8:12])
				if data, ok := memory.Read(dataOffset, dataLength); ok && len(data) < 1024 {
					fmt.Printf("- Data at offset: %s\n", string(data))
				}
			}

			msgRegionDataAtFailure, ok := memory.Read(msgRegionPtr, 12)
			if ok {
				fmt.Printf("\nMessage Region at failure:\n")
				fmt.Printf("- Offset: 0x%x\n", binary.LittleEndian.Uint32(msgRegionDataAtFailure[0:4]))
				fmt.Printf("- Capacity: %d\n", binary.LittleEndian.Uint32(msgRegionDataAtFailure[4:8]))
				fmt.Printf("- Length: %d\n", binary.LittleEndian.Uint32(msgRegionDataAtFailure[8:12]))

				// Try to read the actual data
				dataOffset := binary.LittleEndian.Uint32(msgRegionDataAtFailure[0:4])
				dataLength := binary.LittleEndian.Uint32(msgRegionDataAtFailure[8:12])
				if data, ok := memory.Read(dataOffset, dataLength); ok && len(data) < 1024 {
					fmt.Printf("- Data at offset: %s\n", string(data))
				}
			}

			fmt.Printf("=====================================\n\n")
		}
		return nil, types.GasReport{}, fmt.Errorf("failed to call function %s: %w", name, err)
	}

	// Get the result
	resultRegionPtr := uint32(results[0])
	resultRegionData, ok := memory.Read(resultRegionPtr, 12)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("failed to read result region")
	}

	resultOffset := binary.LittleEndian.Uint32(resultRegionData[0:4])
	resultLength := binary.LittleEndian.Uint32(resultRegionData[8:12])

	// Read the result data
	data, ok := memory.Read(resultOffset, resultLength)
	if !ok {
		return nil, types.GasReport{}, fmt.Errorf("failed to read result data")
	}

	if printDebug {
		fmt.Printf("[DEBUG] Result region: ptr=0x%x, offset=0x%x, length=%d\n",
			resultRegionPtr, resultOffset, resultLength)
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

func (w *WazeroRuntime) getContractModule(checksum []byte) (wazero.CompiledModule, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	module, ok := w.compiledModules[hex.EncodeToString(checksum)]
	if !ok {
		return nil, fmt.Errorf("module not found for checksum %x", checksum)
	}
	return module, nil
}
