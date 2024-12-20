package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	// Maximum number of iterators per contract call
	maxIteratorsPerCall = 100
	// Gas costs for iterator operations
	gasCostIteratorCreate = 2000
	gasCostIteratorNext   = 100
)

// RuntimeEnvironment holds the environment for contract execution
type RuntimeEnvironment struct {
	DB      types.KVStore
	API     *types.GoAPI
	Querier types.Querier
	Memory  *MemoryAllocator
	Gas     types.GasMeter
	GasUsed types.Gas // Track gas usage internally

	// Iterator management
	iteratorsMutex sync.RWMutex
	iterators      map[uint64]map[uint64]types.Iterator
	nextIterID     uint64
	nextCallID     uint64
}

// NewRuntimeEnvironment creates a new runtime environment
func NewRuntimeEnvironment(db types.KVStore, api *types.GoAPI, querier types.Querier) *RuntimeEnvironment {
	return &RuntimeEnvironment{
		DB:        db,
		API:       api,
		Querier:   querier,
		iterators: make(map[uint64]map[uint64]types.Iterator),
	}
}

// StartCall starts a new contract call and returns a call ID
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()

	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]types.Iterator)
	return e.nextCallID
}

// StoreIterator stores an iterator and returns its ID
func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter types.Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()

	e.nextIterID++
	if e.iterators[callID] == nil {
		e.iterators[callID] = make(map[uint64]types.Iterator)
	}
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

// GetIterator retrieves an iterator by its IDs
func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) types.Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()

	if callMap, exists := e.iterators[callID]; exists {
		return callMap[iterID]
	}
	return nil
}

// EndCall cleans up all iterators for a call
func (e *RuntimeEnvironment) EndCall(callID uint64) {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()

	delete(e.iterators, callID)
}

// IteratorID represents a unique identifier for an iterator
type IteratorID struct {
	CallID     uint64
	IteratorID uint64
}

// hostGet implements db_get
func hostGet(ctx context.Context, mod api.Module, keyPtr uint32, keyLen uint32) (dataPtr uint32, dataLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	key, err := ReadMemory(mem, keyPtr, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value := env.DB.Get(key)
	if len(value) == 0 {
		return 0, 0
	}

	// Allocate memory for the result
	offset, err := env.Memory.Allocate(mem, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	if err := WriteMemory(mem, offset, value); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	return offset, uint32(len(value))
}

// hostSet implements db_set
func hostSet(ctx context.Context, mod api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	key, err := ReadMemory(mem, keyPtr, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	val, err := ReadMemory(mem, valPtr, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, val)
}

// hostHumanizeAddress implements api_humanize_address
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) (resPtr, resLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read address from memory: %v", err))
	}

	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		return 0, 0
	}

	// Allocate memory for the result
	offset, err := env.Memory.Allocate(mem, uint32(len(human)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	if err := WriteMemory(mem, offset, []byte(human)); err != nil {
		panic(fmt.Sprintf("failed to write humanized address: %v", err))
	}

	return offset, uint32(len(human))
}

// hostQueryExternal implements querier_query
func hostQueryExternal(ctx context.Context, mod api.Module, reqPtr, reqLen, gasLimit uint32) (resPtr, resLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	req, err := ReadMemory(mem, reqPtr, reqLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read query request: %v", err))
	}

	res := types.RustQuery(env.Querier, req, uint64(gasLimit))
	serialized, err := json.Marshal(res)
	if err != nil {
		return 0, 0
	}

	// Allocate memory for the result
	offset, err := env.Memory.Allocate(mem, uint32(len(serialized)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	if err := WriteMemory(mem, offset, serialized); err != nil {
		panic(fmt.Sprintf("failed to write query response: %v", err))
	}

	return offset, uint32(len(serialized))
}

// hostCanonicalizeAddress implements api_canonicalize_address
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) (resPtr, resLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read address from memory: %v", err))
	}

	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		return 0, 0
	}

	// Allocate memory for the result
	offset, err := env.Memory.Allocate(mem, uint32(len(canonical)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	if err := WriteMemory(mem, offset, []byte(canonical)); err != nil {
		panic(fmt.Sprintf("failed to write canonicalized address: %v", err))
	}

	return offset, uint32(len(canonical))
}

// hostValidateAddress implements api_validate_address
func hostValidateAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read address from memory: %v", err))
	}

	_, err = env.API.ValidateAddress(string(addr))
	if err != nil {
		return 0 // Return 0 for invalid address
	}

	return 1 // Return 1 for valid address
}

// hostScan implements db_scan
func hostScan(ctx context.Context, mod api.Module, startPtr, startLen, endPtr, endLen uint32, order uint32) (uint64, uint64, uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator creation
	if env.GasUsed+gasCostIteratorCreate > env.Gas.GasConsumed() {
		return 0, 0, 1 // Return error code 1 for out of gas
	}
	env.GasUsed += gasCostIteratorCreate

	start, err := ReadMemory(mem, startPtr, startLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read start key from memory: %v", err))
	}

	end, err := ReadMemory(mem, endPtr, endLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read end key from memory: %v", err))
	}

	// Check iterator limits
	callID := env.StartCall()
	if len(env.iterators[callID]) >= maxIteratorsPerCall {
		return 0, 0, 2 // Return error code 2 for too many iterators
	}

	// Get iterator from DB with order
	var iter types.Iterator
	if order == 1 {
		iter = env.DB.ReverseIterator(start, end)
	} else {
		iter = env.DB.Iterator(start, end)
	}
	if iter == nil {
		return 0, 0, 3 // Return error code 3 for iterator creation failure
	}

	// Store iterator in the environment
	iterID := env.StoreIterator(callID, iter)

	return callID, iterID, 0 // Return 0 for success
}

// hostNext implements db_next
func hostNext(ctx context.Context, mod api.Module, callID, iterID uint64) (keyPtr, keyLen, valPtr, valLen, errCode uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator next operation
	if env.GasUsed+gasCostIteratorNext > env.Gas.GasConsumed() {
		return 0, 0, 0, 0, 1 // Return error code 1 for out of gas
	}
	env.GasUsed += gasCostIteratorNext

	// Get iterator from environment
	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 0, 0, 2 // Return error code 2 for invalid iterator
	}

	// Check if there are more items
	if !iter.Valid() {
		return 0, 0, 0, 0, 0 // Return 0 for end of iteration
	}

	// Get key and value
	key := iter.Key()
	value := iter.Value()

	// Allocate memory for key
	keyOffset, err := env.Memory.Allocate(mem, uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for key: %v", err))
	}
	if err := WriteMemory(mem, keyOffset, key); err != nil {
		panic(fmt.Sprintf("failed to write key to memory: %v", err))
	}

	// Allocate memory for value
	valOffset, err := env.Memory.Allocate(mem, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value: %v", err))
	}
	if err := WriteMemory(mem, valOffset, value); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	// Move to next item
	iter.Next()

	return keyOffset, uint32(len(key)), valOffset, uint32(len(value)), 0
}

// hostNextKey implements db_next_key
func hostNextKey(ctx context.Context, mod api.Module, callID, iterID uint64) (keyPtr, keyLen, errCode uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator next operation
	if env.GasUsed+gasCostIteratorNext > env.Gas.GasConsumed() {
		return 0, 0, 1 // Return error code 1 for out of gas
	}
	env.GasUsed += gasCostIteratorNext

	// Get iterator from environment
	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2 // Return error code 2 for invalid iterator
	}

	// Check if there are more items
	if !iter.Valid() {
		return 0, 0, 0 // Return 0 for end of iteration
	}

	// Get key
	key := iter.Key()

	// Allocate memory for key
	keyOffset, err := env.Memory.Allocate(mem, uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for key: %v", err))
	}
	if err := WriteMemory(mem, keyOffset, key); err != nil {
		panic(fmt.Sprintf("failed to write key to memory: %v", err))
	}

	// Move to next item
	iter.Next()

	return keyOffset, uint32(len(key)), 0
}

// hostNextValue implements db_next_value
func hostNextValue(ctx context.Context, mod api.Module, callID, iterID uint64) (valPtr, valLen, errCode uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator next operation
	if env.GasUsed+gasCostIteratorNext > env.Gas.GasConsumed() {
		return 0, 0, 1 // Return error code 1 for out of gas
	}
	env.GasUsed += gasCostIteratorNext

	// Get iterator from environment
	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2 // Return error code 2 for invalid iterator
	}

	// Check if there are more items
	if !iter.Valid() {
		return 0, 0, 0 // Return 0 for end of iteration
	}

	// Get value
	value := iter.Value()

	// Allocate memory for value
	valOffset, err := env.Memory.Allocate(mem, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value: %v", err))
	}
	if err := WriteMemory(mem, valOffset, value); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	// Move to next item
	iter.Next()

	return valOffset, uint32(len(value)), 0
}

// hostCloseIterator implements db_close_iterator
func hostCloseIterator(ctx context.Context, mod api.Module, callID, iterID uint64) {
	env := ctx.Value("env").(*RuntimeEnvironment)

	// Get iterator from environment
	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return
	}

	// Close the iterator
	iter.Close()

	// Remove from environment
	env.iteratorsMutex.Lock()
	defer env.iteratorsMutex.Unlock()

	if callMap, exists := env.iterators[callID]; exists {
		delete(callMap, iterID)
	}
}

// RegisterHostFunctions registers all host functions into a module named "env"
func RegisterHostFunctions(r wazero.Runtime, env *RuntimeEnvironment) (wazero.CompiledModule, error) {
	// Initialize memory allocator if not already done
	if env.Memory == nil {
		env.Memory = NewMemoryAllocator(65536) // Start at 64KB offset
	}

	// Build a module that exports these functions
	hostModBuilder := r.NewHostModuleBuilder("env")

	// Register memory management functions
	RegisterMemoryManagement(hostModBuilder, env.Memory)

	// Register DB functions
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostGet).
		Export("db_get")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostSet).
		Export("db_set")

	// Register API functions
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostHumanizeAddress).
		Export("api_humanize_address")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostCanonicalizeAddress).
		Export("api_canonicalize_address")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostValidateAddress).
		Export("api_validate_address")

	// Register Query functions
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostQueryExternal).
		Export("querier_query")

	// Register Iterator functions
	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostScan).
		Export("db_scan")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostNext).
		Export("db_next")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostNextKey).
		Export("db_next_key")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostNextValue).
		Export("db_next_value")

	hostModBuilder.NewFunctionBuilder().
		WithFunc(hostCloseIterator).
		Export("db_close_iterator")

	// Compile the host module
	compiled, err := hostModBuilder.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to compile host module: %w", err)
	}

	return compiled, nil
}

// When you instantiate a contract, you can do something like:
//
// compiledHost, err := RegisterHostFunctions(runtime, env)
// if err != nil {
//   ...
// }
// _, err = runtime.InstantiateModule(ctx, compiledHost, wazero.NewModuleConfig())
// if err != nil {
//   ...
// }
//
// Then, instantiate your contract module which imports "env" module's functions.
