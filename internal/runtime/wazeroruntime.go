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

	// Maximum memory pages (1024 pages = 64 MiB)
	maxMemoryPages = 32768
)

// Region describes data allocated in Wasm's linear memory
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// toBytes converts a Region to its byte representation (little endian)
func (r *Region) toBytes() []byte {
	bytes := make([]byte, 12)
	binary.LittleEndian.PutUint32(bytes[0:4], r.Offset)
	binary.LittleEndian.PutUint32(bytes[4:8], r.Capacity)
	binary.LittleEndian.PutUint32(bytes[8:12], r.Length)
	return bytes
}

// fromBytes reads a Region from its byte representation (little endian)
func regionFromBytes(data []byte) (*Region, error) {
	if len(data) != 12 {
		return nil, fmt.Errorf("invalid region size: expected 12 bytes, got %d", len(data))
	}
	return &Region{
		Offset:   binary.LittleEndian.Uint32(data[0:4]),
		Capacity: binary.LittleEndian.Uint32(data[4:8]),
		Length:   binary.LittleEndian.Uint32(data[8:12]),
	}, nil
}

// validateRegion performs plausibility checks on a Region
func validateRegion(region *Region) error {
	if region.Offset == 0 {
		return fmt.Errorf("region has zero offset")
	}
	if region.Length > region.Capacity {
		return fmt.Errorf("region length %d exceeds capacity %d", region.Length, region.Capacity)
	}
	if uint64(region.Offset)+uint64(region.Capacity) > math.MaxUint32 {
		return fmt.Errorf("region out of range: offset %d, capacity %d", region.Offset, region.Capacity)
	}
	return nil
}

// readRegion reads a Region struct from memory at the given pointer
func (m *memoryManager) readRegion(ptr uint32) (*Region, error) {
	if ptr == 0 {
		return nil, fmt.Errorf("null pointer")
	}

	// Read 12 bytes for the Region struct
	data, ok := m.memory.Read(ptr, 12)
	if !ok {
		return nil, fmt.Errorf("failed to read Region at ptr=%d", ptr)
	}

	region, err := regionFromBytes(data)
	if err != nil {
		return nil, err
	}

	if err := validateRegion(region); err != nil {
		return nil, err
	}

	return region, nil
}

// writeRegion writes a Region struct to memory at the given pointer
func (m *memoryManager) writeRegion(ptr uint32, region *Region) error {
	if ptr == 0 {
		return fmt.Errorf("null pointer")
	}

	if err := validateRegion(region); err != nil {
		return err
	}

	if !m.memory.Write(ptr, region.toBytes()) {
		return fmt.Errorf("failed to write Region at ptr=%d", ptr)
	}

	return nil
}

// allocateRegion allocates memory for a Region struct and the data it will contain
func (m *memoryManager) allocateRegion(size uint32) (*Region, uint32, error) {
	// Get the allocate function
	allocate := m.module.ExportedFunction("allocate")
	if allocate == nil {
		return nil, 0, fmt.Errorf("allocate function not found in WASM module")
	}

	// Check if requested size is within bounds
	memSize := uint64(m.memory.Size() * wasmPageSize)
	if uint64(size) > memSize {
		return nil, 0, fmt.Errorf("requested allocation size %d exceeds memory size %d", size, memSize)
	}

	// Allocate memory for the data
	dataResult, err := allocate.Call(context.Background(), uint64(size))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to allocate data memory: %w", err)
	}
	dataPtr := uint32(dataResult[0])

	// Verify data pointer is within bounds
	if uint64(dataPtr)+uint64(size) > memSize {
		return nil, 0, fmt.Errorf("allocated memory region [%d, %d] exceeds memory size %d", dataPtr, dataPtr+size, memSize)
	}

	// Create the Region struct
	region := &Region{
		Offset:   dataPtr,
		Capacity: size,
		Length:   0,
	}

	// Allocate memory for the Region struct
	regionResult, err := allocate.Call(context.Background(), uint64(regionSize))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to allocate region memory: %w", err)
	}
	regionPtr := uint32(regionResult[0])

	// Verify region pointer is within bounds
	if uint64(regionPtr)+uint64(regionSize) > memSize {
		return nil, 0, fmt.Errorf("allocated region struct [%d, %d] exceeds memory size %d", regionPtr, regionPtr+regionSize, memSize)
	}

	// Write the Region struct to memory
	if !m.memory.Write(regionPtr, region.toBytes()) {
		return nil, 0, fmt.Errorf("failed to write region to memory at ptr=%d", regionPtr)
	}

	return region, regionPtr, nil
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

// allocateAndWrite allocates memory and writes data directly, returning the pointer
func (m *memoryManager) allocateAndWrite(data []byte) (uint32, error) {
	if data == nil {
		return 0, nil
	}

	// Get the allocate function
	allocate := m.module.ExportedFunction("allocate")
	if allocate == nil {
		return 0, fmt.Errorf("allocate function not found in WASM module")
	}

	// Calculate memory size in bytes
	memSize := m.memory.Size()
	memSizeBytes := memSize * uint32(wasmPageSize)

	// Check if we have enough memory
	if uint32(len(data)) > memSizeBytes {
		return 0, fmt.Errorf("requested allocation size %d exceeds memory size %d", len(data), memSizeBytes)
	}

	// Allocate memory for the data
	dataResult, err := allocate.Call(context.Background(), uint64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("failed to allocate data memory: %w", err)
	}
	dataPtr := uint32(dataResult[0])

	// Verify data pointer is within bounds
	if dataPtr == 0 || dataPtr+uint32(len(data)) > memSizeBytes {
		return 0, fmt.Errorf("allocated memory region [%d, %d] exceeds memory size %d", dataPtr, dataPtr+uint32(len(data)), memSizeBytes)
	}

	// Write the actual data
	if !m.memory.Write(dataPtr, data) {
		deallocate := m.module.ExportedFunction("deallocate")
		if deallocate != nil {
			if _, err := deallocate.Call(context.Background(), uint64(dataPtr)); err != nil {
				fmt.Printf("[DEBUG][Memory] Deallocation failed for ptr=%d: %v\n", dataPtr, err)
			}
		}
		return 0, fmt.Errorf("failed to write data to memory at ptr=%d size=%d", dataPtr, len(data))
	}

	return dataPtr, nil
}

// readData reads data from memory at the given pointer, handling both raw data pointers and Region struct pointers
func (m *memoryManager) readData(ptr uint32) ([]byte, error) {
	if ptr == 0 {
		return nil, nil
	}

	// Calculate memory size in bytes
	memSize := m.memory.Size()
	memSizeBytes := memSize * uint32(wasmPageSize)

	// Check if pointer is within bounds
	if ptr >= memSizeBytes {
		return nil, fmt.Errorf("pointer %d is out of memory bounds (size: %d)", ptr, memSizeBytes)
	}

	// First try to read as a Region struct
	regionData, ok := m.memory.Read(ptr, regionSize)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at ptr=%d", ptr)
	}

	// Try to parse as a Region struct
	region, err := regionFromBytes(regionData)
	if err == nil && region.Offset != 0 && region.Length <= region.Capacity {
		// Verify region bounds
		if region.Offset+region.Length > memSizeBytes {
			return nil, fmt.Errorf("region [%d, %d] exceeds memory bounds (size: %d)", region.Offset, region.Offset+region.Length, memSizeBytes)
		}

		// Looks like a valid Region struct, read the actual data
		data, ok := m.memory.Read(region.Offset, region.Length)
		if !ok {
			return nil, fmt.Errorf("failed to read data at ptr=%d length=%d", region.Offset, region.Length)
		}
		// Make a copy to ensure we own the data
		result := make([]byte, len(data))
		copy(result, data)
		return result, nil
	}

	// Not a valid Region struct, try to read as raw data
	// First read 4 bytes for the length prefix
	lengthData, ok := m.memory.Read(ptr, 4)
	if !ok {
		return nil, fmt.Errorf("failed to read length prefix at ptr=%d", ptr)
	}
	length := binary.LittleEndian.Uint32(lengthData)

	// Verify length is within bounds
	if ptr+4+length > memSizeBytes {
		return nil, fmt.Errorf("raw data region [%d, %d] exceeds memory bounds (size: %d)", ptr+4, ptr+4+length, memSizeBytes)
	}

	// Then read the actual data
	data, ok := m.memory.Read(ptr+4, length)
	if !ok {
		return nil, fmt.Errorf("failed to read raw data at ptr=%d length=%d", ptr+4, length)
	}

	// Make a copy to ensure we own the data
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func NewWazeroRuntime() (*WazeroRuntime, error) {
	// Create a new wazero runtime with memory configuration
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(maxMemoryPages). // Set max memory to 64 MiB (1024 * 64KB)
		WithMemoryCapacityFromMax(true).      // Eagerly allocate memory to ensure availability
		WithCloseOnContextDone(true)          // Ensure resources are cleaned up

	r := wazero.NewRuntimeWithConfig(context.Background(), runtimeConfig)

	return &WazeroRuntime{
		runtime:         r,
		codeCache:       make(map[string][]byte),
		compiledModules: make(map[string]wazero.CompiledModule),
		closed:          false,
		pinnedModules:   make(map[string]struct{}),
		moduleHits:      make(map[string]uint32),
		moduleSizes:     make(map[string]uint64),
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
// checksum is kept as a parameter for future contract version validation
func serializeEnvForContract(env []byte, _ []byte, _ *WazeroRuntime) ([]byte, error) {
	// Parse and validate the environment
	var typedEnv types.Env
	if err := json.Unmarshal(env, &typedEnv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal env: %w", err)
	}

	// Validate required fields
	if typedEnv.Block.ChainID == "" {
		return nil, fmt.Errorf("missing required field: block.chain_id")
	}
	if typedEnv.Contract.Address == "" {
		return nil, fmt.Errorf("missing required field: contract.address")
	}
	if typedEnv.Transaction == nil {
		return nil, fmt.Errorf("missing required field: transaction")
	}

	// Return the validated environment bytes directly
	return env, nil
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

	ctx := context.Background()

	// 4) Create runtime environment
	runtimeEnv := &RuntimeEnvironment{
		DB:        store,
		API:       *api,
		Querier:   *querier,
		Gas:       *gasMeter,
		gasLimit:  gasLimit,
		gasUsed:   0,
		iterators: make(map[uint64]map[uint64]types.Iterator),
	}

	// 5) Register and instantiate the host module "env"
	if printDebug {
		fmt.Println("[DEBUG] Registering host functions ...")
	}
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error registering host functions: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer func() {
		if printDebug {
			fmt.Println("[DEBUG] Closing host module ...")
		}
		hostModule.Close(ctx)
	}()

	// 6) Instantiate the host module first
	envModule, err := w.runtime.InstantiateModule(ctx, hostModule,
		wazero.NewModuleConfig().WithName("env"))
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error instantiating env module: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer func() {
		if printDebug {
			fmt.Println("[DEBUG] Closing env module ...")
		}
		envModule.Close(ctx)
	}()

	// 7) Instantiate the contract module with proper memory configuration
	if printDebug {
		fmt.Println("[DEBUG] Instantiating contract module ...")
	}
	contractModule, err := w.runtime.InstantiateModule(ctx, compiled,
		wazero.NewModuleConfig().
			WithName("contract").
			WithStartFunctions())
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error instantiating module: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer func() {
		if printDebug {
			fmt.Println("[DEBUG] Closing contract module ...")
		}
		contractModule.Close(ctx)
	}()

	// 8) Create memory manager and validate memory
	memory := contractModule.Memory()
	if memory == nil {
		const errStr = "[callContractFn] Error: no memory section in module"
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// Validate memory size
	memSize := memory.Size()
	if memSize == 0 {
		const errStr = "[callContractFn] Error: memory size is 0"
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// Calculate memory size in bytes
	memSizeBytes := memSize * uint32(wasmPageSize)

	if printDebug {
		fmt.Printf("[DEBUG][Memory] Module memory size: %d pages (%d bytes)\n", memSize, memSizeBytes)
	}

	// Initialize memory with zero values
	zeroMem := make([]byte, memSizeBytes)
	if !memory.Write(0, zeroMem) {
		const errStr = "[callContractFn] Error: failed to initialize memory"
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	mm := newMemoryManager(memory, contractModule)

	// Write data directly to memory using Region structs
	if printDebug {
		fmt.Printf("[DEBUG] Writing environment to memory (size=%d) ...\n", len(adaptedEnv))
	}
	envPtr, err := mm.allocateAndWrite(adaptedEnv)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to write env: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Writing msg to memory (size=%d) ...\n", len(msg))
	}
	msgPtr, err := mm.allocateAndWrite(msg)
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
		infoPtr, err := mm.allocateAndWrite(info)
		if err != nil {
			errStr := fmt.Sprintf("[callContractFn] Error: failed to write info: %v", err)
			fmt.Println(errStr)
			return nil, types.GasReport{}, errors.New(errStr)
		}
		callParams = []uint64{uint64(envPtr), uint64(infoPtr), uint64(msgPtr)}
		if printDebug {
			fmt.Printf("[DEBUG][Memory] Call parameters: env_ptr=%d, info_ptr=%d, msg_ptr=%d\n", envPtr, infoPtr, msgPtr)
		}

	case "query", "sudo", "reply":
		callParams = []uint64{uint64(envPtr), uint64(msgPtr)}
		if printDebug {
			fmt.Printf("[DEBUG][Memory] Call parameters: env_ptr=%d, msg_ptr=%d\n", envPtr, msgPtr)
		}

	default:
		errStr := fmt.Sprintf("[callContractFn] Error: unknown function name: %s", name)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	// Call the contract function
	fn := contractModule.ExportedFunction(name)
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

	// Read result directly from memory
	resultPtr := uint32(results[0])
	resultData, err := mm.readData(resultPtr)
	if err != nil {
		errStr := fmt.Sprintf("[callContractFn] Error: failed to read result data: %v", err)
		fmt.Println(errStr)
		return nil, types.GasReport{}, errors.New(errStr)
	}

	if printDebug {
		fmt.Printf("[DEBUG] resultData length=%d\n", len(resultData))
		if len(resultData) < 256 {
			fmt.Printf("[DEBUG] resultData (string) = %q\n", string(resultData))
			fmt.Printf("[DEBUG] resultData (hex)    = % x\n", resultData)
		} else {
			fmt.Println("[DEBUG] resultData is larger than 256 bytes, skipping direct print.")
		}
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
