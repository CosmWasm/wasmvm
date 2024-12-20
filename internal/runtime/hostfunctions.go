package runtime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

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

// NewRuntimeEnvironment creates a new runtime environment
func NewRuntimeEnvironment(db types.KVStore, api *types.GoAPI, querier types.Querier) *RuntimeEnvironment {
	return &RuntimeEnvironment{
		DB:        db,
		API:       *api,
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
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) (uint32, uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read address from memory: %v", err))
	}

	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		return 0, 0 // Return 0, 0 for invalid address
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

// hostCanonicalizeAddress implements addr_canonicalize
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) (uint32, uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read address from memory: %v", err))
	}

	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		return 0, 0 // Return 0, 0 for invalid address
	}

	// Allocate memory for the result
	offset, err := env.Memory.Allocate(mem, uint32(len(canonical)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	if err := WriteMemory(mem, offset, canonical); err != nil {
		panic(fmt.Sprintf("failed to write canonical address: %v", err))
	}

	return offset, uint32(len(canonical))
}

// hostValidateAddress implements addr_validate
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
func hostScan(ctx context.Context, mod api.Module, startPtr, startLen, order uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator creation
	if env.GasUsed+gasCostIteratorCreate > env.Gas.GasConsumed() {
		return 1 // Return error code 1 for out of gas
	}
	env.GasUsed += gasCostIteratorCreate

	// Read start key
	var start []byte
	var err error

	if startPtr != 0 {
		start, err = ReadMemory(mem, startPtr, startLen)
		if err != nil {
			panic(fmt.Sprintf("failed to read start key from memory: %v", err))
		}
	}

	// Start a new call context for this iterator
	callID := env.StartCall()
	if len(env.iterators[callID]) >= maxIteratorsPerCall {
		return 2 // Return error code 2 for too many iterators
	}

	// Get iterator from DB with order
	var iter types.Iterator
	if order == 1 {
		iter = env.DB.ReverseIterator(start, nil)
	} else {
		iter = env.DB.Iterator(start, nil)
	}
	if iter == nil {
		return 3 // Return error code 3 for iterator creation failure
	}

	// Store iterator in the environment
	iterID := env.StoreIterator(callID, iter)

	// Pack the call_id and iter_id into a single u32
	return uint32(iterID)
}

// hostNext implements db_next
func hostNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator next operation
	if env.GasUsed+gasCostIteratorNext > env.Gas.GasConsumed() {
		return 1 // Return error code 1 for out of gas
	}
	env.GasUsed += gasCostIteratorNext

	// Extract call_id and iter_id from the packed uint32
	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	// Get iterator from environment
	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 2 // Return error code 2 for invalid iterator
	}

	// Check if there are more items
	if !iter.Valid() {
		return 0 // Return 0 for end of iteration
	}

	// Get key and value
	key := iter.Key()
	_ = iter.Value() // We read the value but don't use it in this implementation

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

	return keyOffset
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

// hostAbort implements the abort function required by Wasm modules
func hostAbort(ctx context.Context, mod api.Module, code uint32) {
	panic(fmt.Sprintf("Wasm contract aborted with code: %d", code))
}

// hostDbRead implements db_read
func hostDbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read length prefix (4 bytes) from the key pointer
	lenBytes, err := ReadMemory(mem, keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key
	key, err := ReadMemory(mem, keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value := env.DB.Get(key)
	if len(value) == 0 {
		return 0
	}

	// Allocate memory for the result: 4 bytes for length + actual value
	totalLen := 4 + len(value)
	offset, err := env.Memory.Allocate(mem, uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	// Write length prefix
	lenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenData, uint32(len(value)))
	if err := WriteMemory(mem, offset, lenData); err != nil {
		panic(fmt.Sprintf("failed to write value length to memory: %v", err))
	}

	// Write value
	if err := WriteMemory(mem, offset+4, value); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	return offset
}

// hostDbWrite implements db_write
func hostDbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read key length prefix (4 bytes)
	keyLenBytes, err := ReadMemory(mem, keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	// Read value length prefix (4 bytes)
	valLenBytes, err := ReadMemory(mem, valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	// Read the actual key and value
	key, err := ReadMemory(mem, keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := ReadMemory(mem, valuePtr+4, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, value)
}

// hostDbRemove implements db_remove
func hostDbRemove(ctx context.Context, mod api.Module, keyPtr uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read length prefix (4 bytes) from the key pointer
	lenBytes, err := ReadMemory(mem, keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key
	key, err := ReadMemory(mem, keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	env.DB.Delete(key)
}

// RegisterHostFunctions registers all host functions with the wazero runtime
func RegisterHostFunctions(runtime wazero.Runtime, env *RuntimeEnvironment) (wazero.CompiledModule, error) {
	builder := runtime.NewHostModuleBuilder("env")

	// Register abort function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, code uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostAbort(ctx, m, code)
		}).
		WithParameterNames("code").
		Export("abort")

	// Register DB functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen uint32) (uint32, uint32) {
			ctx = context.WithValue(ctx, "env", env)
			return hostGet(ctx, m, keyPtr, keyLen)
		}).
		WithParameterNames("key_ptr", "key_len").
		Export("db_get")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostSet(ctx, m, keyPtr, keyLen, valPtr, valLen)
		}).
		WithParameterNames("key_ptr", "key_len", "val_ptr", "val_len").
		Export("db_set")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, startPtr, startLen, order uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostScan(ctx, m, startPtr, startLen, order)
		}).
		WithParameterNames("start_ptr", "start_len", "order").
		WithResultNames("result").
		Export("db_scan")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostNext(ctx, m, iterID)
		}).
		WithParameterNames("iter_id").
		WithResultNames("result").
		Export("db_next")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, callID, iterID uint64) (uint32, uint32, uint32) {
			ctx = context.WithValue(ctx, "env", env)
			return hostNextKey(ctx, m, callID, iterID)
		}).
		WithParameterNames("call_id", "iter_id").
		Export("db_next_key")

	// Register API functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) (uint32, uint32) {
			ctx = context.WithValue(ctx, "env", env)
			return hostHumanizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("ptr", "len").
		Export("api_humanize_address")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) (uint32, uint32) {
			ctx = context.WithValue(ctx, "env", env)
			return hostCanonicalizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("ptr", "len").
		Export("addr_canonicalize")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostValidateAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("result").
		Export("addr_validate")

	// Register Query functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, reqPtr, reqLen, gasLimit uint32) (uint32, uint32) {
			ctx = context.WithValue(ctx, "env", env)
			return hostQueryExternal(ctx, m, reqPtr, reqLen, gasLimit)
		}).
		WithParameterNames("req_ptr", "req_len", "gas_limit").
		Export("querier_query")

	// Register DB read function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostDbRead(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_read")

	// Register DB write function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, valuePtr uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostDbWrite(ctx, m, keyPtr, valuePtr)
		}).
		WithParameterNames("key_ptr", "value_ptr").
		Export("db_write")

	// Register DB remove function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostDbRemove(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_remove")

	return builder.Compile(context.Background())
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
