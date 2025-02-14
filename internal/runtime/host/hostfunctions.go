package host

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/tetratelabs/wazero/api"

	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	// Return codes for cryptographic operations
	SECP256K1_VERIFY_CODE_VALID   uint32 = 0
	SECP256K1_VERIFY_CODE_INVALID uint32 = 1

	// BLS12-381 return codes
	BLS12_381_VALID_PAIRING   uint32 = 0
	BLS12_381_INVALID_PAIRING uint32 = 1

	BLS12_381_AGGREGATE_SUCCESS     uint32 = 0
	BLS12_381_HASH_TO_CURVE_SUCCESS uint32 = 0

	// Size limits for BLS12-381 operations (MI = 1024*1024, KI = 1024)
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	envKey contextKey = "env"
)

// GasState tracks gas consumption
type GasState struct {
	limit uint64
	used  uint64
}

func NewGasState(limit uint64) GasState {
	return GasState{
		limit: limit,
		used:  0,
	}
}

// GasConsumed implements types.GasMeter
func (g GasState) GasConsumed() uint64 {
	return g.used
}

// allocateInContract calls the contract's allocate function.
// It handles memory allocation within the WebAssembly module's memory space.
func allocateInContract(ctx context.Context, mod api.Module, size uint32) (uint32, error) {
	allocateFn := mod.ExportedFunction("allocate")
	if allocateFn == nil {
		return 0, fmt.Errorf("contract does not export 'allocate' function")
	}
	results, err := allocateFn.Call(ctx, uint64(size))
	if err != nil {
		return 0, fmt.Errorf("failed to call 'allocate': %w", err)
	}
	if len(results) != 1 {
		return 0, fmt.Errorf("expected 1 result from 'allocate', got %d", len(results))
	}
	return uint32(results[0]), nil
}

// readNullTerminatedString reads bytes from memory starting at addrPtr until a null byte is found.
func readNullTerminatedString(memManager *memory.MemoryManager, addrPtr uint32) ([]byte, error) {
	var buf []byte
	for i := addrPtr; ; i++ {
		b, err := memManager.Read(i, 1)
		if err != nil {
			return nil, fmt.Errorf("memory access error at offset %d: %w", i, err)
		}
		if b[0] == 0 {
			break
		}
		buf = append(buf, b[0])
	}
	return buf, nil
}

// hostHumanizeAddress implements addr_humanize.
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostHumanizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*runtime.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to convert to a human-readable address.
	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: API.HumanizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: humanized address: '%s'\n", human)

	// Write the result back into memory.
	if err := env.MemManager.Write(addrPtr, []byte(human)); err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to write humanized address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: successfully wrote humanized address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostCanonicalizeAddress reads a null-terminated address from memory,
// calls the API to canonicalize it, logs intermediate results, and writes
// the canonical address back into memory.
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostCanonicalizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.memManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to canonicalize the address.
	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: API.CanonicalizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: canonical address (hex): %x\n", canonical)

	// Write the canonical address back to memory.
	if err := env.memManager.Write(addrPtr, canonical); err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to write canonical address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: successfully wrote canonical address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostValidateAddress reads a null-terminated address from memory,
// calls the API to validate it, and logs the process.
// Returns 1 if the address is valid and 0 otherwise.
func hostValidateAddress(ctx context.Context, mod api.Module, addrPtr uint32) uint32 {
	env := ctx.Value(envKey).(*RuntimeEnvironment)
	mem := mod.Memory()

	// Read the address as a null-terminated string.
	addr, err := readNullTerminatedString(env.memManager, addrPtr)
	if err != nil {
		panic(fmt.Sprintf("[ERROR] hostValidateAddress: failed to read address from memory: %v", err))
	}
	fmt.Printf("[DEBUG] hostValidateAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Validate the address.
	_, err = env.API.ValidateAddress(string(addr))
	if err != nil {
		fmt.Printf("[DEBUG] hostValidateAddress: API.ValidateAddress failed: %v\n", err)
		return 0 // reject invalid address
	}
	fmt.Printf("[DEBUG] hostValidateAddress: address validated successfully\n")
	return 1 // valid
}

// hostScan implements db_scan.
func hostScan(ctx context.Context, mod api.Module, startPtr, startLen, order uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostScan: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	mem := mod.Memory()

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

	// Store the iterator and pack the call and iterator IDs.
	callID := env.StartCall()
	iterID := env.StoreIterator(callID, iter)
	return uint32(callID<<16 | iterID&0xFFFF)
}

// hostDbNext implements db_next.
func hostDbNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbNext: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}
	if !iter.Valid() {
		return 0
	}

	key := iter.Key()
	value := iter.Value()

	// Charge gas for the returned data.
	env.gasUsed += uint64(len(key)+len(value)) * constants.GasPerByte

	totalLen := 4 + len(key) + 4 + len(value)
	offset, err := env.memManager.Allocate(uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	keyLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLenData, uint32(len(key)))
	if err := env.memManager.Write(offset, keyLenData); err != nil {
		panic(fmt.Sprintf("failed to write key length: %v", err))
	}

	if err := env.memManager.Write(offset+4, key); err != nil {
		panic(fmt.Sprintf("failed to write key: %v", err))
	}

	valLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(valLenData, uint32(len(value)))
	if err := env.memManager.Write(offset+4+uint32(len(key)), valLenData); err != nil {
		panic(fmt.Sprintf("failed to write value length: %v", err))
	}

	if err := env.memManager.Write(offset+8+uint32(len(key)), value); err != nil {
		panic(fmt.Sprintf("failed to write value: %v", err))
	}

	iter.Next()
	return offset
}

// hostNextValue implements db_next_value.
func hostNextValue(ctx context.Context, mod api.Module, callID, iterID uint64) (valPtr, valLen, errCode uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostNextValue: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	mem := mod.Memory()

	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2
	}

	if !iter.Valid() {
		return 0, 0, 0
	}

	value := iter.Value()
	env.gasUsed += uint64(len(value)) * constants.GasPerByte

	valOffset, err := allocateInContract(ctx, mod, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, valOffset, value, false); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	iter.Next()
	return valOffset, uint32(len(value)), 0
}

// hostDbRead implements db_read.
func hostDbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbRead: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	fmt.Printf("=== Host Function: db_read ===\n")
	fmt.Printf("Input keyPtr: 0x%x\n", keyPtr)

	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key length: %v\n", err)
		return 0
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)
	fmt.Printf("Key length: %d bytes\n", keyLen)

	key, err := env.memManager.Read(keyPtr+4, keyLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key data: %v\n", err)
		return 0
	}
	fmt.Printf("Key data: %x\n", key)

	value := env.DB.Get(key)
	fmt.Printf("Value found: %x\n", value)

	valuePtr, err := env.memManager.Allocate(uint32(len(value)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory: %v\n", err)
		return 0
	}

	if err := env.memManager.Write(valuePtr, value); err != nil {
		fmt.Printf("ERROR: Failed to write value to memory: %v\n", err)
		return 0
	}

	return valuePtr
}

// hostDbWrite implements db_write.
func hostDbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbWrite: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	keyLenBytes, err := env.memManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	valLenBytes, err := env.memManager.Read(valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	key, err := env.memManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := env.memManager.Read(valuePtr+4, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, value)
}

// hostSecp256k1Verify implements secp256k1_verify.
func hostSecp256k1Verify(ctx context.Context, mod api.Module, hash_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostSecp256k1Verify: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	message, err := env.memManager.Read(hash_ptr, 32)
	if err != nil {
		return 0
	}

	signature, err := env.memManager.Read(sig_ptr, 64)
	if err != nil {
		return 0
	}

	pubKey, err := env.memManager.Read(pubkey_ptr, 33)
	if err != nil {
		return 0
	}

	verified, _, err := env.API.Secp256k1Verify(message, signature, pubKey)
	if err != nil {
		return 0
	}
	if verified {
		return 1
	}
	return 0
}

// hostSecp256k1RecoverPubkey implements secp256k1_recover_pubkey.
func hostSecp256k1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, sigPtr, recID uint32) uint64 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostSecp256k1RecoverPubkey: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	hash, err := env.memManager.Read(hashPtr, 32)
	if err != nil {
		return 0
	}

	sig, err := env.memManager.Read(sigPtr, 64)
	if err != nil {
		return 0
	}

	pubkey, _, err := env.API.Secp256k1RecoverPubkey(hash, sig, uint8(recID))
	if err != nil {
		return 0
	}

	offset, err := env.memManager.Allocate(uint32(len(pubkey)))
	if err != nil {
		return 0
	}

	if err := env.memManager.Write(offset, pubkey); err != nil {
		return 0
	}

	return uint64(offset)
}

// hostEd25519Verify implements ed25519_verify.
func hostEd25519Verify(ctx context.Context, mod api.Module, msg_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostEd25519Verify: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	message, err := env.memManager.Read(msg_ptr, 32)
	if err != nil {
		return 0
	}

	signature, err := env.memManager.Read(sig_ptr, 64)
	if err != nil {
		return 0
	}

	pubKey, err := env.memManager.Read(pubkey_ptr, 32)
	if err != nil {
		return 0
	}

	verified, _, err := env.API.Ed25519Verify(message, signature, pubKey)
	if err != nil {
		return 0
	}
	if verified {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify.
func hostEd25519BatchVerify(ctx context.Context, mod api.Module, msgs_ptr, sigs_ptr, pubkeys_ptr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostEd25519BatchVerify: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	countBytes, err := env.memManager.Read(msgs_ptr, 4)
	if err != nil {
		return 0
	}
	count := binary.LittleEndian.Uint32(countBytes)

	messages := make([][]byte, count)
	msgPtr := msgs_ptr + 4
	for i := uint32(0); i < count; i++ {
		lenBytes, err := env.memManager.Read(msgPtr, 4)
		if err != nil {
			return 0
		}
		msgLen := binary.LittleEndian.Uint32(lenBytes)
		msgPtr += 4
		msg, err := env.memManager.Read(msgPtr, msgLen)
		if err != nil {
			return 0
		}
		messages[i] = msg
		msgPtr += msgLen
	}

	signatures := make([][]byte, count)
	sigPtr := sigs_ptr
	for i := uint32(0); i < count; i++ {
		sig, err := env.memManager.Read(sigPtr, 64)
		if err != nil {
			return 0
		}
		signatures[i] = sig
		sigPtr += 64
	}

	pubkeys := make([][]byte, count)
	pubkeyPtr := pubkeys_ptr
	for i := uint32(0); i < count; i++ {
		pubkey, err := env.memManager.Read(pubkeyPtr, 32)
		if err != nil {
			return 0
		}
		pubkeys[i] = pubkey
		pubkeyPtr += 32
	}

	verified, _, err := env.API.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		return 0
	}
	if verified {
		return 1
	}
	return 0
}

// hostDebug implements debug.
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

// hostQueryChain implements query_chain.
// Input layout: at reqPtr, 4 bytes little-endian length followed by that many bytes of request.
// Output: at the returned offset, 4 bytes length prefix followed by the JSON of ChainResponse.
func hostQueryChain(ctx context.Context, mod api.Module, reqPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostQueryChain: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	mem := mod.Memory()

	lenBytes, err := readMemory(mem, reqPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read query request length: %v", err))
	}
	reqLen := binary.LittleEndian.Uint32(lenBytes)

	req, err := readMemory(mem, reqPtr+4, reqLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read query request: %v", err))
	}

	res := types.RustQuery(env.Querier, req, env.Gas.GasConsumed())

	serialized, err := json.Marshal(res)
	if err != nil {
		return 0
	}

	totalLen := 4 + len(serialized)
	offset, err := allocateInContract(ctx, mod, uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for chain response: %v", err))
	}

	lenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenData, uint32(len(serialized)))
	if err := writeMemory(mem, offset, lenData, false); err != nil {
		panic(fmt.Sprintf("failed to write response length: %v", err))
	}

	if err := writeMemory(mem, offset+4, serialized, false); err != nil {
		panic(fmt.Sprintf("failed to write response data: %v", err))
	}

	return offset
}

// hostNextKey implements db_next_key.
func hostNextKey(ctx context.Context, mod api.Module, callID, iterID uint64) (keyPtr, keyLen, errCode uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostNextKey: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	mem := mod.Memory()

	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2
	}

	if !iter.Valid() {
		return 0, 0, 0
	}

	key := iter.Key()
	env.gasUsed += uint64(len(key)) * gasPerByte

	keyOffset, err := allocateInContract(ctx, mod, uint32(len(key)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for key (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, keyOffset, key, false); err != nil {
		panic(fmt.Sprintf("failed to write key to memory: %v", err))
	}

	iter.Next()
	return keyOffset, uint32(len(key)), 0
}

// hostBls12381AggregateG1 implements bls12_381_aggregate_g1.
func hostBls12381AggregateG1(ctx context.Context, mod api.Module, g1sPtr, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostBls12381AggregateG1: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	mem := mod.Memory()

	g1s, err := readMemory(mem, g1sPtr, BLS12_381_MAX_AGGREGATE_SIZE)
	if err != nil {
		fmt.Printf("ERROR: Failed to read G1 points from memory: %v\n", err)
		return 0
	}

	pointCount := len(g1s) / BLS12_381_G1_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G1 points to aggregate\n")
		return 0
	}

	gasCost := env.GasConfig.Bls12381AggregateG1Cost.TotalCost(uint64(pointCount))
	env.gasUsed += gasCost
	if env.gasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during aggregation: used %d, limit %d\n", env.gasUsed, env.Gas.GasConsumed())
		return 0
	}

	result, err := BLS12381AggregateG1(splitIntoPoints(g1s, BLS12_381_G1_POINT_LEN))
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G1 points: %v\n", err)
		return 0
	}

	if err := writeMemory(mem, outPtr, result, false); err != nil {
		fmt.Printf("ERROR: Failed to write aggregated G1 point to memory: %v\n", err)
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// splitIntoPoints splits a byte slice into a slice of points of fixed length.
func splitIntoPoints(data []byte, pointLen int) [][]byte {
	var points [][]byte
	for i := 0; i < len(data); i += pointLen {
		points = append(points, data[i:i+pointLen])
	}
	return points
}

// hostBls12381AggregateG2 implements bls12_381_aggregate_g2.
func hostBls12381AggregateG2(ctx context.Context, mod api.Module, g2sPtr, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostBls12381AggregateG2: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)
	mem := mod.Memory()

	g2s, err := readMemory(mem, g2sPtr, BLS12_381_MAX_AGGREGATE_SIZE)
	if err != nil {
		fmt.Printf("ERROR: Failed to read G2 points from memory: %v\n", err)
		return 0
	}

	pointCount := len(g2s) / BLS12_381_G2_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G2 points to aggregate\n")
		return 0
	}

	gasCost := env.GasConfig.Bls12381AggregateG2Cost.TotalCost(uint64(pointCount))
	env.gasUsed += gasCost
	if env.gasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during aggregation: used %d, limit %d\n", env.gasUsed, env.Gas.GasConsumed())
		return 0
	}

	result, err := BLS12381AggregateG2(splitIntoPoints(g2s, BLS12_381_G2_POINT_LEN))
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G2 points: %v\n", err)
		return 0
	}

	if err := writeMemory(mem, outPtr, result, false); err != nil {
		fmt.Printf("ERROR: Failed to write aggregated G2 point to memory: %v\n", err)
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// hostDbRemove implements db_remove.
func hostDbRemove(ctx context.Context, mod api.Module, keyPtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbRemove: runtime environment not found in context")
	}
	env := envVal.(*RuntimeEnvironment)

	// Read the 4-byte length prefix from the key pointer.
	lenBytes, err := env.memManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key.
	key, err := env.memManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	env.DB.Delete(key)
}
