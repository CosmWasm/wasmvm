package host

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero/api"
)

// abort handles contract aborts
func abort(ctx context.Context, mod api.Module, msgPtr, filePtr, line, col uint32) {
	// Read message
	msg, err := readNullTerminatedString(&memory.MemoryManager{Memory: mod.Memory()}, msgPtr)
	if err != nil {
		panic(fmt.Sprintf("abort: unable to read message: %v", err))
	}

	// Read file name
	file, err := readNullTerminatedString(&memory.MemoryManager{Memory: mod.Memory()}, filePtr)
	if err != nil {
		panic(fmt.Sprintf("abort: unable to read file name: %v", err))
	}

	// Format and panic with abort message
	panicMsg := fmt.Sprintf("contract aborted at %s:%d:%d: %s", file, line, col, msg)
	panic(panicMsg)
}

// Debug logs a debug message from the contract
func Debug(ctx context.Context, mod api.Module, msgPtr uint32) {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		fmt.Printf("[ERROR] Debug: failed to create MemoryManager: %v\n", err)
		return
	}

	// Read message
	msg, err := readNullTerminatedString(mm, msgPtr)
	if err != nil {
		fmt.Printf("[ERROR] Debug: unable to read message: %v\n", err)
		return
	}

	fmt.Printf("[DEBUG CONTRACT] %s\n", string(msg))
}

// DbRead reads a value from the key-value store
func DbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbRead: failed to create MemoryManager: %v", err))
	}

	// Read the key
	key, err := mm.ReadRegion(keyPtr)
	if err != nil {
		panic(fmt.Sprintf("DbRead: failed to read key: %v", err))
	}

	// Charge gas for the read operation
	env.Gas.ConsumeGas(constants.GasCostRead, "db_read")

	// Get the value from storage
	value, err := env.DB.Get(key)
	if err != nil {
		panic(fmt.Sprintf("DbRead: storage error: %v", err))
	}

	// If key not found, return 0
	if value == nil {
		return 0
	}

	// Allocate memory for the value and create a region
	valueOffset, err := mm.Allocate(uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("DbRead: failed to allocate memory: %v", err))
	}

	if err := mm.Write(valueOffset, value); err != nil {
		panic(fmt.Sprintf("DbRead: failed to write value to memory: %v", err))
	}

	// Create a region pointing to the value
	regionPtr, err := mm.CreateRegion(valueOffset, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("DbRead: failed to create region: %v", err))
	}

	return regionPtr
}

// DbWrite writes a key-value pair to storage
func DbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbWrite: failed to create MemoryManager: %v", err))
	}

	// Read key and value
	key, err := mm.ReadRegion(keyPtr)
	if err != nil {
		panic(fmt.Sprintf("DbWrite: failed to read key: %v", err))
	}

	value, err := mm.ReadRegion(valuePtr)
	if err != nil {
		panic(fmt.Sprintf("DbWrite: failed to read value: %v", err))
	}

	// Charge gas for the write operation
	env.Gas.ConsumeGas(constants.GasCostWrite, "db_write")

	// Write to storage
	if err := env.DB.Set(key, value); err != nil {
		panic(fmt.Sprintf("DbWrite: storage error: %v", err))
	}
}

// DbRemove deletes a key from storage
func DbRemove(ctx context.Context, mod api.Module, keyPtr uint32) {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbRemove: failed to create MemoryManager: %v", err))
	}

	// Read key
	key, err := mm.ReadRegion(keyPtr)
	if err != nil {
		panic(fmt.Sprintf("DbRemove: failed to read key: %v", err))
	}

	// Charge gas
	env.Gas.ConsumeGas(constants.GasCostWrite, "db_remove") // Deletion costs same as write

	// Delete from storage
	if err := env.DB.Delete(key); err != nil {
		panic(fmt.Sprintf("DbRemove: storage error: %v", err))
	}
}

// DbScan creates an iterator over a key range
func DbScan(ctx context.Context, mod api.Module, startPtr, endPtr uint32, order int32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbScan: failed to create MemoryManager: %v", err))
	}

	// Read start and end keys
	start, err := mm.ReadRegion(startPtr)
	if err != nil {
		panic(fmt.Sprintf("DbScan: failed to read start key: %v", err))
	}

	var end []byte
	if endPtr != 0 {
		end, err = mm.ReadRegion(endPtr)
		if err != nil {
			panic(fmt.Sprintf("DbScan: failed to read end key: %v", err))
		}
	}

	// Charge gas
	env.Gas.ConsumeGas(constants.GasCostIteratorCreate, "db_scan")

	// Create iterator
	var iter types.Iterator
	if order == 1 { // descending
		iter = env.DB.ReverseIterator(start, end)
	} else { // ascending
		iter = env.DB.Iterator(start, end)
	}

	// Store the iterator
	callID := env.StartCall()
	iterID := env.StoreIterator(callID, iter)

	// Return combined ID (upper 16 bits: callID, lower 16 bits: iterID)
	return uint32(callID<<16 | iterID&0xFFFF)
}

// DbNext gets the next key-value pair from an iterator
func DbNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbNext: failed to create MemoryManager: %v", err))
	}

	// Extract callID and iterID
	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	// Get the iterator
	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}

	// Charge gas
	env.Gas.ConsumeGas(constants.GasCostIteratorNext, "db_next")

	// Get next item
	if !iter.Valid() {
		return 0
	}

	key := iter.Key()
	value := iter.Value()

	// Move to next position
	iter.Next()

	// Allocate memory for result
	keyOffset, err := mm.Allocate(uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("DbNext: failed to allocate memory for key: %v", err))
	}

	valueOffset, err := mm.Allocate(uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("DbNext: failed to allocate memory for value: %v", err))
	}

	// Write key and value
	if err := mm.Write(keyOffset, key); err != nil {
		panic(fmt.Sprintf("DbNext: failed to write key: %v", err))
	}

	if err := mm.Write(valueOffset, value); err != nil {
		panic(fmt.Sprintf("DbNext: failed to write value: %v", err))
	}

	// Create a KV pair region
	pairRegion, err := createKVRegion(mm, keyOffset, uint32(len(key)), valueOffset, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("DbNext: failed to create KV region: %v", err))
	}

	return pairRegion
}

// Helper to create a KV pair region
func createKVRegion(mm *memory.MemoryManager, keyOffset, keyLen, valueOffset, valueLen uint32) (uint32, error) {
	// Allocate memory for KV struct (offset1, len1, offset2, len2)
	kvPtr, err := mm.Allocate(16)
	if err != nil {
		return 0, err
	}

	// Create KV struct
	kv := make([]byte, 16)
	binary.LittleEndian.PutUint32(kv[0:4], keyOffset)
	binary.LittleEndian.PutUint32(kv[4:8], keyLen)
	binary.LittleEndian.PutUint32(kv[8:12], valueOffset)
	binary.LittleEndian.PutUint32(kv[12:16], valueLen)

	// Write KV struct
	if err := mm.Write(kvPtr, kv); err != nil {
		return 0, err
	}

	return kvPtr, nil
}

// DbNextKey gets just the next key from an iterator
func DbNextKey(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbNextKey: failed to create MemoryManager: %v", err))
	}

	// Extract callID and iterID
	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	// Get the iterator
	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}

	// Charge gas
	env.Gas.ConsumeGas(constants.GasCostIteratorNext, "db_next_key")

	// Get next key
	if !iter.Valid() {
		return 0
	}

	key := iter.Key()

	// Move to next position
	iter.Next()

	// Create region for key
	keyOffset, err := mm.Allocate(uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("DbNextKey: failed to allocate memory: %v", err))
	}

	if err := mm.Write(keyOffset, key); err != nil {
		panic(fmt.Sprintf("DbNextKey: failed to write key: %v", err))
	}

	regionPtr, err := mm.CreateRegion(keyOffset, uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("DbNextKey: failed to create region: %v", err))
	}

	return regionPtr
}

// DbNextValue gets just the next value from an iterator
func DbNextValue(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("DbNextValue: failed to create MemoryManager: %v", err))
	}

	// Extract callID and iterID
	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	// Get the iterator
	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}

	// Charge gas
	env.Gas.ConsumeGas(constants.GasCostIteratorNext, "db_next_value")

	// Get next value
	if !iter.Valid() {
		return 0
	}

	value := iter.Value()

	// Move to next position
	iter.Next()

	// Create region for value
	valueOffset, err := mm.Allocate(uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("DbNextValue: failed to allocate memory: %v", err))
	}

	if err := mm.Write(valueOffset, value); err != nil {
		panic(fmt.Sprintf("DbNextValue: failed to write value: %v", err))
	}

	regionPtr, err := mm.CreateRegion(valueOffset, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("DbNextValue: failed to create region: %v", err))
	}

	return regionPtr
}

// AddrValidate validates a human address
func AddrValidate(ctx context.Context, mod api.Module, addrPtr uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("AddrValidate: failed to create MemoryManager: %v", err))
	}

	// Read address
	addrBytes, err := mm.ReadRegion(addrPtr)
	if err != nil {
		panic(fmt.Sprintf("AddrValidate: failed to read address: %v", err))
	}

	addr := string(addrBytes)

	// Validate address
	_, err = env.API.ValidateAddress(addr)
	if err != nil {
		return 1 // invalid
	}

	return 0 // valid
}

// AddrCanonicalize canonicalizes a human address
func AddrCanonicalize(ctx context.Context, mod api.Module, humanPtr, canonPtr uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("AddrCanonicalize: failed to create MemoryManager: %v", err))
	}

	// Read human address
	humanBytes, err := mm.ReadRegion(humanPtr)
	if err != nil {
		panic(fmt.Sprintf("AddrCanonicalize: failed to read address: %v", err))
	}

	human := string(humanBytes)

	// Canonicalize
	canonical, _, err := env.API.CanonicalizeAddress(human)
	if err != nil {
		return 1 // error
	}

	// Write to output region
	if err := writeToRegion(mm, canonPtr, canonical); err != nil {
		panic(fmt.Sprintf("AddrCanonicalize: failed to write canonical address: %v", err))
	}

	return 0 // success
}

// QueryChain forwards queries to the chain
func QueryChain(ctx context.Context, mod api.Module, requestPtr uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("QueryChain: failed to create MemoryManager: %v", err))
	}

	// Read request
	request, err := mm.ReadRegion(requestPtr)
	if err != nil {
		panic(fmt.Sprintf("QueryChain: failed to read request: %v", err))
	}

	// Charge gas
	env.Gas.ConsumeGas(constants.GasCostQuery, "query_chain")

	// Forward query
	result, err := env.Querier.Query(request)
	if err != nil {
		// Return error as JSON
		errorResult := fmt.Sprintf(`{"error":%q}`, err.Error())
		resultPtr, err := createStringRegion(mm, errorResult)
		if err != nil {
			panic(fmt.Sprintf("QueryChain: failed to create error result: %v", err))
		}
		return resultPtr
	}

	// Create region for result
	resultPtr, err := createStringRegion(mm, string(result))
	if err != nil {
		panic(fmt.Sprintf("QueryChain: failed to create result region: %v", err))
	}

	return resultPtr
}

// Helper to create a string region
func createStringRegion(mm *memory.MemoryManager, s string) (uint32, error) {
	data := []byte(s)
	dataOffset, err := mm.Allocate(uint32(len(data)))
	if err != nil {
		return 0, err
	}

	if err := mm.Write(dataOffset, data); err != nil {
		return 0, err
	}

	regionPtr, err := mm.CreateRegion(dataOffset, uint32(len(data)))
	if err != nil {
		return 0, err
	}

	return regionPtr, nil
}

// Define the crypto functions that are registered
func Secp256k1Verify(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Secp256k1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, recoveryParam uint32) (uint32, uint32) {
	// Implementation will be delegated to the crypto package
	return 0, 0 // Placeholder
}

func Ed25519Verify(ctx context.Context, mod api.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Ed25519BatchVerify(ctx context.Context, mod api.Module, msgsPtr, sigsPtr, pubkeysPtr uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Bls12381AggregateG1(ctx context.Context, mod api.Module, pointsPtr, pointsLen, outPtr uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Bls12381AggregateG2(ctx context.Context, mod api.Module, pointsPtr, pointsLen, outPtr uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Bls12381PairingCheck(ctx context.Context, mod api.Module, g1PointsPtr, g1PointsLen, g2PointsPtr, g2PointsLen uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Bls12381HashToG1(ctx context.Context, mod api.Module, msgPtr, msgLen, dstPtr, dstLen uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}

func Bls12381HashToG2(ctx context.Context, mod api.Module, msgPtr, msgLen, dstPtr, dstLen uint32) uint32 {
	// Implementation will be delegated to the crypto package
	return 0 // Placeholder
}
