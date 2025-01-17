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
	"unicode/utf8"

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

// writeToMemory writes data to memory and returns the offset where it was written
func (mm *memoryManager) writeToMemory(data []byte, printDebug bool) (uint32, uint32, error) {
	dataSize := uint32(len(data))
	if dataSize == 0 {
		return 0, 0, nil
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
	//	runtimeConfig := wazero.NewRuntimeConfig().
	//		WithMemoryLimitPages(1024).      // Set max memory to 64 MiB (1024 * 64KB)
	//		WithMemoryCapacityFromMax(true). // Eagerly allocate memory to ensure it's initialized
	//		WithDebugInfoEnabled(true)       // Enable debug info

	r := wazero.NewRuntimeWithConfig(context.Background(), wazero.NewRuntimeConfigInterpreter())

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

	// Calculate total memory needed
	envDataSize := uint32(len(env))
	infoDataSize := uint32(len(info))
	msgDataSize := uint32(len(msg))

	// Write env data to memory
	envPtr, envAllocSize, err := mm.writeToMemory(env, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write env to memory: %w", err)
	}

	// Write info data to memory
	infoPtr, infoAllocSize, err := mm.writeToMemory(info, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write info to memory: %w", err)
	}

	// Write msg data to memory
	msgPtr, msgAllocSize, err := mm.writeToMemory(msg, printDebug)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to write msg to memory: %w", err)
	}

	// Create Region structs
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
		fmt.Printf("  env region ptr: %d\n", envRegionPtr)
		fmt.Printf("  info region ptr: %d\n", infoRegionPtr)
		fmt.Printf("  msg region ptr: %d\n", msgRegionPtr)
		fmt.Printf("  env data ptr: %d, size: %d\n", envPtr, envDataSize)
		fmt.Printf("  info data ptr: %d, size: %d\n", infoPtr, infoDataSize)
		fmt.Printf("  msg data ptr: %d, size: %d\n", msgPtr, msgDataSize)
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

	// Calculate total memory needed for data and Region structs
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

	// Create Region structs
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

func (w *WazeroRuntime) getContractModule(checksum []byte) (wazero.CompiledModule, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	module, ok := w.compiledModules[hex.EncodeToString(checksum)]
	if !ok {
		return nil, fmt.Errorf("module not found for checksum %x", checksum)
	}
	return module, nil
}

// tryDeserializeMemory attempts to extract readable text from a memory dump
func tryDeserializeMemory(data []byte) string {
	var results []string

	// First try to interpret as UTF-8 text
	if str := tryUTF8(data); str != "" {
		results = append(results, "As text: "+str)
	}

	// Try to find null-terminated C strings
	if strs := tryCStrings(data); len(strs) > 0 {
		results = append(results, "As C strings: "+strings.Join(strs, ", "))
	}

	// Try to find JSON fragments
	if json := tryJSON(data); json != "" {
		results = append(results, "As JSON: "+json)
	}

	// Always include hex representation in a readable format
	hexStr := formatHexDump(data)
	results = append(results, "As hex dump:\n"+hexStr)

	return strings.Join(results, "\n")
}

// formatHexDump creates a formatted hex dump with both hex and ASCII representation
func formatHexDump(data []byte) string {
	var hexDump strings.Builder
	var asciiDump strings.Builder
	var result strings.Builder

	for i := 0; i < len(data); i += 16 {
		// Write offset
		fmt.Fprintf(&result, "%04x:  ", i)

		hexDump.Reset()
		asciiDump.Reset()

		// Process 16 bytes per line
		for j := 0; j < 16; j++ {
			if i+j < len(data) {
				// Write hex representation
				fmt.Fprintf(&hexDump, "%02x ", data[i+j])

				// Write ASCII representation
				if data[i+j] >= 32 && data[i+j] <= 126 {
					asciiDump.WriteByte(data[i+j])
				} else {
					asciiDump.WriteByte('.')
				}
			} else {
				// Pad with spaces if we're at the end
				hexDump.WriteString("   ")
				asciiDump.WriteByte(' ')
			}

			// Add extra space between groups of 8 bytes
			if j == 7 {
				hexDump.WriteByte(' ')
			}
		}

		// Combine hex and ASCII representations
		fmt.Fprintf(&result, "%-49s |%s|\n", hexDump.String(), asciiDump.String())
	}

	return result.String()
}

// tryUTF8 attempts to interpret the data as UTF-8 text
func tryUTF8(data []byte) string {
	// Remove null bytes and control characters except newline and tab
	cleaned := make([]byte, 0, len(data))
	hasNonControl := false
	for _, b := range data {
		if b == 0 {
			continue
		}
		if b >= 32 || b == '\n' || b == '\t' {
			if b >= 32 && b <= 126 { // ASCII printable characters
				hasNonControl = true
			}
			cleaned = append(cleaned, b)
		}
	}

	// Only process if we found some printable characters
	if !hasNonControl {
		return ""
	}

	// Try to decode as UTF-8
	if str := string(cleaned); utf8.ValidString(str) {
		// Only return if we have some meaningful content
		if trimmed := strings.TrimSpace(str); len(trimmed) > 3 { // Require at least a few characters
			return trimmed
		}
	}
	return ""
}

// tryCStrings attempts to find null-terminated C strings in the data
func tryCStrings(data []byte) []string {
	var strings []string
	start := 0
	inString := false

	for i := 0; i < len(data); i++ {
		// Look for string start - first printable character
		if !inString {
			if data[i] >= 32 && data[i] <= 126 {
				start = i
				inString = true
			}
			continue
		}

		// Look for string end - null byte or non-printable character
		if data[i] == 0 || (data[i] < 32 && data[i] != '\n' && data[i] != '\t') {
			if i > start {
				str := tryUTF8(data[start:i])
				if str != "" && len(str) > 3 { // Require at least a few characters
					strings = append(strings, str)
				}
			}
			inString = false
		}
	}

	// Check final segment if we're still in a string
	if inString && start < len(data) {
		str := tryUTF8(data[start:])
		if str != "" && len(str) > 3 {
			strings = append(strings, str)
		}
	}

	return strings
}

// tryJSON attempts to find valid JSON fragments in the data
func tryJSON(data []byte) string {
	// Look for common JSON markers
	start := -1
	for i := 0; i < len(data); i++ {
		if data[i] == '{' || data[i] == '[' {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}

	// Try to find the end of the JSON
	var stack []byte
	stack = append(stack, data[start])
	for i := start + 1; i < len(data); i++ {
		if len(stack) == 0 {
			// Try to parse what we found
			if jsonStr := tryUTF8(data[start:i]); jsonStr != "" {
				var js interface{}
				if err := json.Unmarshal([]byte(jsonStr), &js); err == nil {
					pretty, _ := json.MarshalIndent(js, "", "  ")
					return string(pretty)
				}
			}
			break
		}

		switch data[i] {
		case '{', '[':
			stack = append(stack, data[i])
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return ""
}

// readMemoryAndDeserialize reads memory at the given offset and size, and attempts to deserialize it
func readMemoryAndDeserialize(memory api.Memory, offset, size uint32) (string, error) {
	data, ok := memory.Read(offset, size)
	if !ok {
		return "", fmt.Errorf("failed to read memory at offset=%d size=%d", offset, size)
	}

	if readable := tryDeserializeMemory(data); readable != "" {
		return readable, nil
	}

	// If no readable text found, return the traditional hex dump
	return fmt.Sprintf("As hex: %s", hex.EncodeToString(data)), nil
}
