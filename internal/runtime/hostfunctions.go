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
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read the input address from guest memory.
	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		// If we fail to read memory, return a non-zero error code.
		return 1
	}

	// Call the API to humanize the address.
	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		// On failure, return a non-zero error code.
		return 1
	}

	// We must write the result back into the same memory location, if it fits.
	if uint32(len(human)) > addrLen {
		// If the humanized address is larger than the provided buffer,
		// return an error code.
		return 1
	}

	// Write the humanized address back to memory
	if err := WriteMemory(mem, addrPtr, []byte(human)); err != nil {
		return 1
	}

	// Return 0 on success
	return 0
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
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, addrLen uint32) uint32 {
	// Retrieve your runtime environment.
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read the input address from guest memory.
	addr, err := ReadMemory(mem, addrPtr, addrLen)
	if err != nil {
		// If we fail to read memory, return a non-zero error code.
		return 1
	}

	// Call the API to canonicalize the address.
	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		// On failure, just return a non-zero error code.
		return 1
	}

	// Here we must decide where to write the canonical address.
	// Without details, let's assume we write it back to the same location.
	if uint32(len(canonical)) > addrLen {
		// If the canonical address is larger than the provided buffer,
		// we have no way to signal that other than returning an error.
		return 1
	}

	// Write the canonical address back to the memory at addrPtr.
	if err := WriteMemory(mem, addrPtr, canonical); err != nil {
		return 1
	}

	// Return 0 on success.
	return 0
}

// hostValidateAddress implements addr_validate
func hostValidateAddress(ctx context.Context, mod api.Module, addrPtr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read the address bytes directly (no length prefix in Rust)
	addr, err := ReadMemory(mem, addrPtr, 32) // Fixed size for addresses
	if err != nil {
		panic(fmt.Sprintf("failed to read address from memory: %v", err))
	}

	// Convert to string and validate
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

// hostSecp256k1Verify implements secp256k1_verify
func hostSecp256k1Verify(ctx context.Context, mod api.Module, hash_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read message from memory (32 bytes for hash)
	message, err := ReadMemory(mem, hash_ptr, 32)
	if err != nil {
		return 0
	}

	// Read signature from memory (64 bytes for signature)
	signature, err := ReadMemory(mem, sig_ptr, 64)
	if err != nil {
		return 0
	}

	// Read public key from memory (33 bytes for compressed pubkey)
	pubKey, err := ReadMemory(mem, pubkey_ptr, 33)
	if err != nil {
		return 0
	}

	// Call the API to verify the signature
	verified, _, err := env.API.Secp256k1Verify(message, signature, pubKey)
	if err != nil {
		return 0
	}

	if verified {
		return 1
	}
	return 0
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

// hostSecp256k1RecoverPubkey implements secp256k1_recover_pubkey
func hostSecp256k1RecoverPubkey(ctx context.Context, mod api.Module, hash_ptr, sig_ptr, rec_id uint32) uint64 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read message hash from memory (32 bytes)
	hash, err := ReadMemory(mem, hash_ptr, 32)
	if err != nil {
		return 0
	}

	// Read signature from memory (64 bytes)
	sig, err := ReadMemory(mem, sig_ptr, 64)
	if err != nil {
		return 0
	}

	// Call the API to recover the public key
	pubkey, _, err := env.API.Secp256k1RecoverPubkey(hash, sig, uint8(rec_id))
	if err != nil {
		return 0
	}

	// Allocate memory for the result
	offset, err := env.Memory.Allocate(mem, uint32(len(pubkey)))
	if err != nil {
		return 0
	}

	// Write the recovered public key to memory
	if err := WriteMemory(mem, offset, pubkey); err != nil {
		return 0
	}

	return uint64(offset)
}

// hostEd25519Verify implements ed25519_verify
func hostEd25519Verify(ctx context.Context, mod api.Module, msg_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read message from memory (32 bytes for message hash)
	message, err := ReadMemory(mem, msg_ptr, 32)
	if err != nil {
		return 0
	}

	// Read signature from memory (64 bytes for ed25519 signature)
	signature, err := ReadMemory(mem, sig_ptr, 64)
	if err != nil {
		return 0
	}

	// Read public key from memory (32 bytes for ed25519 pubkey)
	pubKey, err := ReadMemory(mem, pubkey_ptr, 32)
	if err != nil {
		return 0
	}

	// Call the API to verify the signature
	verified, _, err := env.API.Ed25519Verify(message, signature, pubKey)
	if err != nil {
		return 0
	}

	if verified {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify
func hostEd25519BatchVerify(ctx context.Context, mod api.Module, msgs_ptr, sigs_ptr, pubkeys_ptr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read the number of messages (first 4 bytes)
	countBytes, err := ReadMemory(mem, msgs_ptr, 4)
	if err != nil {
		return 0
	}
	count := binary.LittleEndian.Uint32(countBytes)

	// Read messages
	messages := make([][]byte, count)
	msgPtr := msgs_ptr + 4
	for i := uint32(0); i < count; i++ {
		// Read message length
		lenBytes, err := ReadMemory(mem, msgPtr, 4)
		if err != nil {
			return 0
		}
		msgLen := binary.LittleEndian.Uint32(lenBytes)
		msgPtr += 4

		// Read message
		msg, err := ReadMemory(mem, msgPtr, msgLen)
		if err != nil {
			return 0
		}
		messages[i] = msg
		msgPtr += msgLen
	}

	// Read signatures
	signatures := make([][]byte, count)
	sigPtr := sigs_ptr
	for i := uint32(0); i < count; i++ {
		// Each signature is 64 bytes
		sig, err := ReadMemory(mem, sigPtr, 64)
		if err != nil {
			return 0
		}
		signatures[i] = sig
		sigPtr += 64
	}

	// Read public keys
	pubkeys := make([][]byte, count)
	pubkeyPtr := pubkeys_ptr
	for i := uint32(0); i < count; i++ {
		// Each public key is 32 bytes
		pubkey, err := ReadMemory(mem, pubkeyPtr, 32)
		if err != nil {
			return 0
		}
		pubkeys[i] = pubkey
		pubkeyPtr += 32
	}

	// Call the API to verify the signatures
	verified, _, err := env.API.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		return 0
	}

	if verified {
		return 1
	}
	return 0
}

// hostDebug implements debug
func hostDebug(ctx context.Context, mod api.Module, msgPtr uint32) {
	mem := mod.Memory()

	// Read message from memory (null-terminated string)
	var msg []byte
	offset := msgPtr
	for {
		// Read one byte at a time
		b, err := ReadMemory(mem, offset, 1)
		if err != nil || len(b) == 0 || b[0] == 0 {
			break
		}
		msg = append(msg, b[0])
		offset++
	}

	// Print debug message
	fmt.Printf("Debug: %s\n", string(msg))
}

// RegisterHostFunctions registers all host functions with the wazero runtime
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

	// Register DB functions (unchanged)
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

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostHumanizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("result").
		Export("addr_humanize")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostValidateAddress(ctx, m, addrPtr)
		}).
		WithParameterNames("addr_ptr").
		WithResultNames("result").
		Export("addr_validate")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostCanonicalizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("result").
		Export("addr_canonicalize")

	// Register Query functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, reqPtr, reqLen, gasLimit uint32) (uint32, uint32) {
			ctx = context.WithValue(ctx, "env", env)
			return hostQueryExternal(ctx, m, reqPtr, reqLen, gasLimit)
		}).
		WithParameterNames("req_ptr", "req_len", "gas_limit").
		Export("querier_query")

	// Register secp256k1_verify function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hash_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostSecp256k1Verify(ctx, m, hash_ptr, sig_ptr, pubkey_ptr)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "pubkey_ptr").
		WithResultNames("result").
		Export("secp256k1_verify")

	// Register DB read/write/remove functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostDbRead(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_read")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, valuePtr uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostDbWrite(ctx, m, keyPtr, valuePtr)
		}).
		WithParameterNames("key_ptr", "value_ptr").
		Export("db_write")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostDbRemove(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_remove")

	// Register secp256k1_recover_pubkey function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hash_ptr, sig_ptr, rec_id uint32) uint64 {
			ctx = context.WithValue(ctx, "env", env)
			return hostSecp256k1RecoverPubkey(ctx, m, hash_ptr, sig_ptr, rec_id)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "rec_id").
		WithResultNames("result").
		Export("secp256k1_recover_pubkey")

	// Register ed25519_verify function with i32i32i32_i32 signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msg_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostEd25519Verify(ctx, m, msg_ptr, sig_ptr, pubkey_ptr)
		}).
		WithParameterNames("msg_ptr", "sig_ptr", "pubkey_ptr").
		WithResultNames("result").
		Export("ed25519_verify")

	// Register ed25519_batch_verify function with i32i32i32_i32 signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgs_ptr, sigs_ptr, pubkeys_ptr uint32) uint32 {
			ctx = context.WithValue(ctx, "env", env)
			return hostEd25519BatchVerify(ctx, m, msgs_ptr, sigs_ptr, pubkeys_ptr)
		}).
		WithParameterNames("msgs_ptr", "sigs_ptr", "pubkeys_ptr").
		WithResultNames("result").
		Export("ed25519_batch_verify")

	// Register debug function with i32_v signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr uint32) {
			ctx = context.WithValue(ctx, "env", env)
			hostDebug(ctx, m, msgPtr)
		}).
		WithParameterNames("msg_ptr").
		Export("debug")

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
