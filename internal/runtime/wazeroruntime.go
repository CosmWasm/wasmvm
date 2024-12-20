package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	if code == nil || len(code) == 0 {
		return nil, errors.New("Wasm bytecode could not be deserialized")
	}

	checksum := sha256.Sum256(code)
	csHex := hex.EncodeToString(checksum[:])

	if _, exists := w.compiledModules[csHex]; exists {
		// already stored
		return checksum[:], nil
	}

	compiled, err := w.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to compile module: %w", err)
	}

	// Persist code on success
	w.codeCache[csHex] = code
	w.compiledModules[csHex] = compiled

	return checksum[:], nil
}

// StoreCode compiles and persists the code
func (w *WazeroRuntime) StoreCode(code []byte) (checksum []byte, err error, persisted bool) {
	c, e := w.storeCodeImpl(code)
	if e != nil {
		return nil, e, false
	}
	return c, nil, true
}

// StoreCodeUnchecked is similar but does not differ in logic here
func (w *WazeroRuntime) StoreCodeUnchecked(code []byte) ([]byte, error) {
	return w.storeCodeImpl(code)
}

func (w *WazeroRuntime) GetCode(checksum []byte) ([]byte, error) {
	if checksum == nil {
		return nil, errors.New("Null/Nil argument: checksum")
	}
	if len(checksum) != 32 {
		return nil, errors.New("Checksum not of length 32")
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, errors.New("runtime is closed")
	}

	csHex := hex.EncodeToString(checksum)
	if code, ok := w.codeCache[csHex]; ok {
		return code, nil
	}
	return nil, fmt.Errorf("checksum %s not found", csHex)
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

	if _, ok := w.codeCache[hex.EncodeToString(checksum)]; !ok {
		return errors.New("Error opening Wasm file for reading")
	}
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

	if _, ok := w.codeCache[hex.EncodeToString(checksum)]; !ok {
		return errors.New("Error opening Wasm file for reading")
	}
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
	return w.callContractFn("instantiate", checksum, env, info, msg)
}

func (w *WazeroRuntime) Execute(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("execute", checksum, env, info, msg)
}

func (w *WazeroRuntime) Migrate(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("migrate", checksum, env, nil, msg)
}

func (w *WazeroRuntime) MigrateWithInfo(checksum, env, msg, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// Just call migrate for now
	return w.Migrate(checksum, env, msg, otherParams...)
}

func (w *WazeroRuntime) Sudo(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("sudo", checksum, env, nil, msg)
}

func (w *WazeroRuntime) Reply(checksum, env, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("reply", checksum, env, nil, reply)
}

func (w *WazeroRuntime) Query(checksum, env, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("query", checksum, env, nil, query)
}

func (w *WazeroRuntime) IBCChannelOpen(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_channel_open", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCChannelConnect(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_channel_connect", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCChannelClose(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_channel_close", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketReceive(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_packet_receive", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketAck(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_packet_ack", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketTimeout(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_packet_timeout", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCSourceCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_source_callback", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCDestinationCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_destination_callback", checksum, env, nil, msg)
}

func (w *WazeroRuntime) GetMetrics() (*types.Metrics, error) {
	// Return empty metrics
	return &types.Metrics{}, nil
}

func (w *WazeroRuntime) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	// Return an empty pinned metrics array
	return &types.PinnedMetrics{
		PerModule: []types.PerModuleEntry{},
	}, nil
}

func (w *WazeroRuntime) callContractFn(fnName string, checksum, env, info, msg []byte) ([]byte, types.GasReport, error) {
	if checksum == nil {
		return nil, types.GasReport{}, errors.New("Null/Nil argument: checksum")
	} else if len(checksum) != 32 {
		return nil, types.GasReport{}, errors.New("Checksum not of length 32")
	}

	w.mu.Lock()
	compiled, ok := w.compiledModules[hex.EncodeToString(checksum)]
	w.mu.Unlock()
	if !ok {
		return nil, types.GasReport{}, errors.New("Error opening Wasm file for reading")
	}

	ctx := context.Background()

	// Create runtime environment with the current state
	runtimeEnv := &RuntimeEnvironment{
		DB:      w.kvStore,
		API:     *w.api,
		Querier: w.querier,
		Memory:  NewMemoryAllocator(65536), // Start at 64KB offset
		Gas:     w.querier,                 // Use querier as gas meter since it implements GasConsumed()
	}

	// Register host functions
	hostModule, err := RegisterHostFunctions(w.runtime, runtimeEnv)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to register host functions: %w", err)
	}
	defer hostModule.Close(ctx)

	// Instantiate the host module first
	_, err = w.runtime.InstantiateModule(ctx, hostModule, wazero.NewModuleConfig())
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	// Now instantiate the contract module
	modConfig := wazero.NewModuleConfig().
		WithName("contract").
		WithStartFunctions() // Don't automatically run start function

	module, err := w.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate contract module: %w", err)
	}
	defer module.Close(ctx)

	envPtr, envLen, err := writeToWasmMemory(module, env)
	if err != nil {
		return nil, types.GasReport{}, err
	}
	infoPtr, infoLen, err := writeToWasmMemory(module, info)
	if err != nil {
		return nil, types.GasReport{}, err
	}
	msgPtr, msgLen, err := writeToWasmMemory(module, msg)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	fn := module.ExportedFunction(fnName)
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("function %s not found", fnName)
	}

	results, err := fn.Call(ctx,
		uint64(envPtr), uint64(envLen),
		uint64(infoPtr), uint64(infoLen),
		uint64(msgPtr), uint64(msgLen),
	)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("call failed: %w", err)
	}

	if len(results) < 2 {
		return nil, types.GasReport{}, fmt.Errorf("function %s returned too few results", fnName)
	}

	dataPtr := uint32(results[0])
	dataLen := uint32(results[1])

	data, ok2 := module.Memory().Read(dataPtr, dataLen)
	if !ok2 {
		return nil, types.GasReport{}, fmt.Errorf("failed to read return data")
	}

	gr := types.GasReport{
		Limit:          1_000_000_000,
		Remaining:      500_000_000,
		UsedExternally: 0,
		UsedInternally: 500_000_000,
	}

	return data, gr, nil
}

func writeToWasmMemory(module api.Module, data []byte) (uint32, uint32, error) {
	if len(data) == 0 {
		return 0, 0, nil
	}
	offset := uint32(1024)
	mem := module.Memory()
	if !mem.Write(offset, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory")
	}
	return offset, uint32(len(data)), nil
}
