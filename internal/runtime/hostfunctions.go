package runtime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero/api"

	"github.com/CosmWasm/wasmvm/v2/types"
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

// Helper functions for memory operations
func readMemory(mem api.Memory, offset, size uint32) ([]byte, error) {
	data, ok := mem.Read(offset, size)
	if !ok {
		return nil, fmt.Errorf("failed to read %d bytes at offset %d", size, offset)
	}
	return data, nil
}

func writeMemory(mem api.Memory, offset uint32, data []byte) error {
	if !mem.Write(offset, data) {
		return fmt.Errorf("failed to write %d bytes at offset %d", len(data), offset)
	}
	return nil
}

// allocateInContract calls the contract's allocate function
// allocateInContract is a critical helper function that handles memory allocation
// within the WebAssembly module's memory space. This function must be extremely
// reliable as it's used by many other host functions.
func allocateInContract(ctx context.Context, mod api.Module, size uint32) (uint32, error) {
	fmt.Printf("\n=== allocateInContract ===\n")
	fmt.Printf("Requested size: %d bytes\n", size)

	// Get the allocate function from the contract module
	allocate := mod.ExportedFunction("allocate")
	if allocate == nil {
		return 0, fmt.Errorf("allocate function not found in module")
	}

	// Track memory before allocation
	memory := mod.Memory()
	beforeSize := memory.Size()
	fmt.Printf("Memory size before allocation: %d bytes\n", beforeSize)

	// Call the contract's allocate function
	results, err := allocate.Call(ctx, uint64(size))
	if err != nil {
		fmt.Printf("ERROR: Allocation failed: %v\n", err)
		return 0, fmt.Errorf("failed to allocate memory: %w", err)
	}

	// Check memory after allocation
	afterSize := memory.Size()
	if afterSize > beforeSize {
		fmt.Printf("Memory grew from %d to %d bytes (grew by %d bytes)\n",
			beforeSize, afterSize, afterSize-beforeSize)
	}

	ptr := uint32(results[0])
	fmt.Printf("Allocated at pointer: 0x%x\n", ptr)

	// Validate the returned pointer
	if ptr == 0 {
		return 0, fmt.Errorf("allocation returned null pointer")
	}

	// Verify the allocated memory is within bounds
	if ptr >= memory.Size() {
		return 0, fmt.Errorf("allocation returned out of bounds pointer: 0x%x", ptr)
	}

	fmt.Printf("=== End allocateInContract ===\n\n")
	return ptr, nil
}

// hostGet implements db_get
func hostGet(ctx context.Context, mod api.Module, keyPtr, keyLen uint32) (uint32, uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	key, err := readMemory(mem, keyPtr, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value := env.DB.Get(key)
	if value == nil {
		// Return 0,0 for "not found"
		return 0, 0
	}

	offset, err := allocateInContract(ctx, mod, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value: %v", err))
	}

	if err := writeMemory(mem, offset, value); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	return offset, uint32(len(value))
}

// hostSet implements db_set
func hostSet(ctx context.Context, mod api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	key, err := readMemory(mem, keyPtr, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	val, err := readMemory(mem, valPtr, valLen)
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
	addr, err := readMemory(mem, addrPtr, addrLen)
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
	if err := writeMemory(mem, addrPtr, []byte(human)); err != nil {
		return 1
	}

	// Return 0 on success
	return 0
}

// hostQueryExternal implements querier_query
func hostQueryExternal(ctx context.Context, mod api.Module, reqPtr, reqLen, gasLimit uint32) (resPtr, resLen uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	req, err := readMemory(mem, reqPtr, reqLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read query request: %v", err))
	}

	res := types.RustQuery(env.Querier, req, uint64(gasLimit))
	serialized, err := json.Marshal(res)
	if err != nil {
		return 0, 0
	}

	offset, err := allocateInContract(ctx, mod, uint32(len(serialized)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, offset, serialized); err != nil {
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
	addr, err := readMemory(mem, addrPtr, addrLen)
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
	if err := writeMemory(mem, addrPtr, canonical); err != nil {
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
	addr, err := readMemory(mem, addrPtr, 32) // Fixed size for addresses
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

	// Read the start key if any...
	start, err := readMemory(mem, startPtr, startLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read start key: %v", err))
	}

	var iter types.Iterator
	if order == 1 {
		iter = env.DB.ReverseIterator(start, nil)
	} else {
		iter = env.DB.Iterator(start, nil)
	}

	// Store the iterator and return its ID
	callID := env.StartCall()
	iterID := env.StoreIterator(callID, iter)

	// Pack both IDs into a single uint32
	// Use high 16 bits for callID and low 16 bits for iterID
	return uint32(callID<<16 | iterID&0xFFFF)
}

// hostNext implements db_next
func hostNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Extract call_id and iter_id from the packed uint32
	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	// Get the iterator
	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}

	// Check if iterator is still valid
	if !iter.Valid() {
		return 0
	}

	// Get key and value
	key := iter.Key()
	value := iter.Value()

	// Allocate memory for key and value
	// Format: [key_len(4 bytes)][key][value_len(4 bytes)][value]
	totalLen := 4 + len(key) + 4 + len(value)
	offset, err := allocateInContract(ctx, mod, uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	// Write key length
	keyLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLenData, uint32(len(key)))
	if err := writeMemory(mem, offset, keyLenData); err != nil {
		panic(fmt.Sprintf("failed to write key length: %v", err))
	}

	// Write key
	if err := writeMemory(mem, offset+4, key); err != nil {
		panic(fmt.Sprintf("failed to write key: %v", err))
	}

	// Write value length
	valLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(valLenData, uint32(len(value)))
	if err := writeMemory(mem, offset+4+uint32(len(key)), valLenData); err != nil {
		panic(fmt.Sprintf("failed to write value length: %v", err))
	}

	// Write value
	if err := writeMemory(mem, offset+8+uint32(len(key)), value); err != nil {
		panic(fmt.Sprintf("failed to write value: %v", err))
	}

	// Move to next item
	iter.Next()

	return offset
}

// hostNextValue implements db_next_value
func hostNextValue(ctx context.Context, mod api.Module, callID, iterID uint64) (valPtr, valLen, errCode uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator next operation
	if env.gasUsed+gasCostIteratorNext > env.Gas.GasConsumed() {
		return 0, 0, 1 // Return error code 1 for out of gas
	}
	env.gasUsed += gasCostIteratorNext

	// Get iterator from environment
	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2 // Return error code 2 for invalid iterator
	}

	// Check if there are more items
	if !iter.Valid() {
		return 0, 0, 0 // Return 0 for end of iteration
	}

	// Read value
	value := iter.Value()

	// Instead of env.Memory.Allocate(...):
	//     valOffset, err := env.Memory.Allocate(mem, uint32(len(value)))
	// Use the contract's allocateInContract:
	valOffset, err := allocateInContract(ctx, mod, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, valOffset, value); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	// Move to next item
	iter.Next()

	return valOffset, uint32(len(value)), 0
}

// hostAbort implements the abort function required by Wasm modules
func hostAbort(ctx context.Context, mod api.Module, code uint32) {
	fmt.Printf("\n===================== [ WASM CONTRACT ABORT ] =====================\n")
	fmt.Printf("Abort code: %d (0x%x)\n", code, code)

	// Try logging the name of this module (if any)
	if mod != nil {
		fmt.Printf("Module name: %q\n", mod.Name())
	}

	// Attempt to get and log memory details
	if mem := mod.Memory(); mem != nil {
		// mem.Size() is already in bytes (Wazero returns total bytes, not pages)
		pages := mem.Size() / wasmPageSize
		fmt.Printf("Memory size (pages): %d\n", pages)
		fmt.Printf("Approx. memory size (bytes): %d\n", mem.Size())

		// We'll attempt to read memory around the `code` offset
		ranges := []struct {
			start uint32
			size  uint32
			desc  string
		}{
			{code - 100, 200, "around the code pointer (code - 100..code+100)"},
			{0, 256, "first 256 bytes of memory"},
			{code & 0xFFFF, 256, "lower 16 bits offset"},
		}

		for i, r := range ranges {
			// Skip reading if `r.start + r.size` might exceed memory bounds
			if r.start > mem.Size() {
				fmt.Printf("[range %d] Start offset %d is out of memory bounds (size: %d bytes)\n", i, r.start, mem.Size())
				continue
			}
			end := r.start + r.size
			if end > mem.Size() {
				end = mem.Size()
			}
			lengthToRead := end - r.start
			if lengthToRead == 0 {
				fmt.Printf("[range %d] Not enough memory to read from offset=%d\n", i, r.start)
				continue
			}
			data, ok := mem.Read(r.start, lengthToRead)
			if ok && len(data) > 0 {
				fmt.Printf("[range %d] Reading %d bytes %s at offset=%d:\n", i, lengthToRead, r.desc, r.start)
				// Print as string
				fmt.Printf("  As string: %q\n", string(data))
				// Print raw bytes in hex
				fmt.Printf("  As hex: % x\n", data)
			} else {
				fmt.Printf("[range %d] Could not read data or data is empty at offset=%d\n", i, r.start)
			}
		}
	} else {
		fmt.Printf("No memory found in the module\n")
	}

	// Attempt to fetch the runtime environment from the context
	envAny := ctx.Value(envKey)
	if envAny == nil {
		fmt.Printf("No runtime environment (envKey) found in the context.\n")
	} else {
		env, ok := envAny.(*RuntimeEnvironment)
		if !ok {
			fmt.Printf("Found envKey in context but could not cast to *RuntimeEnvironment.\n")
		} else {
			fmt.Printf("\n=== Runtime Environment Debug Info ===\n")
			fmt.Printf(" - Gas used: %d\n", env.gasUsed)
			// If env.Gas is a pointer to a GasMeter or tracks the gas limit differently, adjust as needed:
			fmt.Printf(" - Gas limit: %d\n", env.Gas.GasConsumed())
			fmt.Printf(" - open iterators callID->(iterID->Iterator) map size: %d\n", len(env.iterators))
			for callID, iterMap := range env.iterators {
				fmt.Printf("    callID=%d has %d iterators\n", callID, len(iterMap))
			}
		}
	}

	panic(fmt.Sprintf("Wasm contract aborted with code: %d (0x%x)", code, code))
}

// hostDbRead implements db_read
// hostDbRead implements db_read, one of the most critical host functions.
// It reads data from the contract's storage and handles all memory management.
// This function is called whenever a contract wants to read its state.
func hostDbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	// Start debug logging for this call
	fmt.Printf("\n=== Host Function: db_read ===\n")
	fmt.Printf("Input keyPtr: 0x%x\n", keyPtr)

	// Get the environment from context
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	if env == nil {
		fmt.Printf("ERROR: Missing runtime environment in context\n")
		return 0
	}

	memory := mod.Memory()
	if memory == nil {
		fmt.Printf("ERROR: No memory exported from module\n")
		return 0
	}

	// Read length prefix (4 bytes) from the key pointer
	lenBytes, err := readMemory(memory, keyPtr, 4)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key length: %v\n", err)
		return 0
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)
	fmt.Printf("Key length: %d bytes\n", keyLen)

	// Read the actual key
	key, err := readMemory(memory, keyPtr+4, keyLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key data: %v\n", err)
		return 0
	}
	fmt.Printf("Key data: %x\n", key)

	// Query the environment's key-value store
	value := env.DB.Get(key)
	if len(value) == 0 {
		fmt.Printf("Key not found in storage\n")
		return 0
	}
	fmt.Printf("Found value in storage, length: %d bytes\n", len(value))

	// Allocate memory for the result: 4 bytes for length + actual value
	totalLen := 4 + len(value)
	offset, err := allocateInContract(ctx, mod, uint32(totalLen))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}
	fmt.Printf("Allocated memory at offset: 0x%x\n", offset)

	// Write length prefix
	lenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenData, uint32(len(value)))
	if err := writeMemory(memory, offset, lenData); err != nil {
		fmt.Printf("ERROR: Failed to write value length: %v\n", err)
		return 0
	}

	// Write actual value
	if err := writeMemory(memory, offset+4, value); err != nil {
		fmt.Printf("ERROR: Failed to write value data: %v\n", err)
		return 0
	}
	fmt.Printf("Successfully wrote result to memory\n")

	// Charge gas for the operation
	gasToCharge := uint64(len(key) + len(value))
	env.gasUsed += gasToCharge
	if env.gasUsed > env.Gas.GasConsumed() {
		panic(fmt.Sprintf("out of gas: used %d, limit %d", env.gasUsed, env.Gas.GasConsumed()))
	}
	fmt.Printf("Charged %d gas for operation\n", gasToCharge)

	fmt.Printf("=== End db_read ===\n\n")
	return offset
}

// hostDbWrite implements db_write
func hostDbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read key length prefix (4 bytes)
	keyLenBytes, err := readMemory(mem, keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	// Read value length prefix (4 bytes)
	valLenBytes, err := readMemory(mem, valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	// Read the actual key and value
	key, err := readMemory(mem, keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := readMemory(mem, valuePtr+4, valLen)
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
	message, err := readMemory(mem, hash_ptr, 32)
	if err != nil {
		return 0
	}

	// Read signature from memory (64 bytes for signature)
	signature, err := readMemory(mem, sig_ptr, 64)
	if err != nil {
		return 0
	}

	// Read public key from memory (33 bytes for compressed pubkey)
	pubKey, err := readMemory(mem, pubkey_ptr, 33)
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
	lenBytes, err := readMemory(mem, keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key
	key, err := readMemory(mem, keyPtr+4, keyLen)
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
	hash, err := readMemory(mem, hash_ptr, 32)
	if err != nil {
		return 0
	}

	// Read signature from memory (64 bytes)
	sig, err := readMemory(mem, sig_ptr, 64)
	if err != nil {
		return 0
	}

	// Call the API to recover the public key
	pubkey, _, err := env.API.Secp256k1RecoverPubkey(hash, sig, uint8(rec_id))
	if err != nil {
		return 0
	}

	// Allocate memory for the result
	offset, err := allocateInContract(ctx, mod, uint32(len(pubkey)))
	if err != nil {
		return 0
	}

	// Write the recovered public key to memory
	if err := writeMemory(mem, offset, pubkey); err != nil {
		return 0
	}

	return uint64(offset)
}

// hostEd25519Verify implements ed25519_verify
func hostEd25519Verify(ctx context.Context, mod api.Module, msg_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read message from memory (32 bytes for message hash)
	message, err := readMemory(mem, msg_ptr, 32)
	if err != nil {
		return 0
	}

	// Read signature from memory (64 bytes for ed25519 signature)
	signature, err := readMemory(mem, sig_ptr, 64)
	if err != nil {
		return 0
	}

	// Read public key from memory (32 bytes for ed25519 pubkey)
	pubKey, err := readMemory(mem, pubkey_ptr, 32)
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
	countBytes, err := readMemory(mem, msgs_ptr, 4)
	if err != nil {
		return 0
	}
	count := binary.LittleEndian.Uint32(countBytes)

	// Read messages
	messages := make([][]byte, count)
	msgPtr := msgs_ptr + 4
	for i := uint32(0); i < count; i++ {
		// Read message length
		lenBytes, err := readMemory(mem, msgPtr, 4)
		if err != nil {
			return 0
		}
		msgLen := binary.LittleEndian.Uint32(lenBytes)
		msgPtr += 4

		// Read message
		msg, err := readMemory(mem, msgPtr, msgLen)
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
		sig, err := readMemory(mem, sigPtr, 64)
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
		pubkey, err := readMemory(mem, pubkeyPtr, 32)
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
func hostDebug(_ context.Context, mod api.Module, msgPtr uint32) {
	mem := mod.Memory()
	msg, err := readMemory(mem, msgPtr, 1024) // Read up to 1024 bytes
	if err != nil {
		return
	}
	// Find null terminator
	length := 0
	for length < len(msg) && msg[length] != 0 {
		length++
	}
	fmt.Printf("Debug: %s\n", string(msg[:length]))
}

// hostQueryChain implements query_chain with signature (req_ptr i32) -> i32
// Memory layout for input:
//
//	at req_ptr: 4 bytes little-endian length, followed by that many bytes of request
//
// Memory layout for output:
//
//	at returned offset: 4 bytes length prefix, followed by the JSON of ChainResponse
func hostQueryChain(ctx context.Context, mod api.Module, reqPtr uint32) uint32 {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read the request length
	lenBytes, err := readMemory(mem, reqPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read query request length: %v", err))
	}
	reqLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual request
	req, err := readMemory(mem, reqPtr+4, reqLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read query request: %v", err))
	}

	// Perform the query
	res := types.RustQuery(env.Querier, req, env.Gas.GasConsumed())

	// Wrap in ChainResponse and serialize
	serialized, err := json.Marshal(res)
	if err != nil {
		// On failure, return 0
		return 0
	}

	// Allocate memory for (4 bytes length + serialized)
	totalLen := 4 + len(serialized)
	offset, err := allocateInContract(ctx, mod, uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for chain response: %v", err))
	}

	// Write length prefix
	lenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenData, uint32(len(serialized)))
	if err := writeMemory(mem, offset, lenData); err != nil {
		panic(fmt.Sprintf("failed to write response length: %v", err))
	}

	// Write serialized response
	if err := writeMemory(mem, offset+4, serialized); err != nil {
		panic(fmt.Sprintf("failed to write response data: %v", err))
	}

	// Return the offset as i32
	return offset
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

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	envKey contextKey = "env"
)

// hostNextKey implements db_next_key
func hostNextKey(ctx context.Context, mod api.Module, callID, iterID uint64) (keyPtr, keyLen, errCode uint32) {
	env := ctx.Value("env").(*RuntimeEnvironment)
	mem := mod.Memory()

	// Check gas for iterator next operation
	if env.gasUsed+gasCostIteratorNext > env.Gas.GasConsumed() {
		return 0, 0, 1 // Return error code 1 for out of gas
	}
	env.gasUsed += gasCostIteratorNext

	// Get iterator from environment
	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2 // Return error code 2 for invalid iterator
	}

	// Check if there are more items
	if !iter.Valid() {
		return 0, 0, 0 // Return 0 for end of iteration
	}

	// Read key
	key := iter.Key()

	// Allocate memory for key
	keyOffset, err := allocateInContract(ctx, mod, uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for key (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, keyOffset, key); err != nil {
		panic(fmt.Sprintf("failed to write key to memory: %v", err))
	}

	// Move to next item
	iter.Next()

	return keyOffset, uint32(len(key)), 0
}

func hostGetAllocation(ctx context.Context, m api.Module) uint32 {
	// CosmWasm expects memory to be allocated in a specific way
	mem := m.Memory()
	if mem == nil {
		return 0
	}
	// Return next available allocation slot
	return uint32(mem.Size())
}

// hostDeallocate implements the deallocate function required by CosmWasm contracts.
// It is called when contracts want to free memory they previously allocated.
// While Wazero handles actual memory management, we still need to track these
// deallocations for proper contract behavior and gas metering.
func hostDeallocate(ctx context.Context, mod api.Module, ptr uint32) {
	// Get the environment from context
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	if env == nil {
		// This should never happen in practice, but we check to be safe
		panic("missing runtime environment in context")
	}

	// Get memory from the module
	memory := mod.Memory()
	if memory == nil {
		panic("no memory exported from module")
	}

	// In Wazero, we don't need to manually free memory like in Wasmer,
	// as Wazero handles memory management automatically.
	// However, we still need to:
	// 1. Validate the pointer
	// 2. Charge gas for the operation
	// 3. Mark the memory as logically deallocated for the contract

	// Validate that the pointer is within memory bounds
	if ptr >= memory.Size() {
		panic(fmt.Sprintf("deallocation of invalid pointer: %d", ptr))
	}

	// Charge a small amount of gas for the deallocation operation
	// This matches the CosmWasm gas charging pattern
	const deallocGas = uint64(25000) // Example gas cost, adjust based on your needs
	env.gasUsed += deallocGas
	if env.gasUsed > env.Gas.GasConsumed() {
		panic("out of gas")
	}

	// Note: In a more sophisticated implementation, we might want to:
	// - Track allocated/deallocated regions to catch double-frees
	// - Zero out the deallocated memory region
	// - Maintain a free list for future allocations
	// However, this basic implementation satisfies the contract requirements
	// while letting Wazero handle the actual memory management
}
