package host

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/tetratelabs/wazero/api"
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
	env := envVal.(*types.RuntimeEnvironment)

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
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
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
	if err := env.MemManager.Write(addrPtr, canonical); err != nil {
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
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Read the address as a null-terminated string.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
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
	env := envVal.(*types.RuntimeEnvironment)
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
	env := envVal.(*types.RuntimeEnvironment)

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
	env.GasUsed += uint64(len(key)+len(value)) * constants.GasPerByte

	totalLen := 4 + len(key) + 4 + len(value)
	offset, err := env.MemManager.Allocate(uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	keyLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLenData, uint32(len(key)))
	if err := env.MemManager.Write(offset, keyLenData); err != nil {
		panic(fmt.Sprintf("failed to write key length: %v", err))
	}

	if err := env.MemManager.Write(offset+4, key); err != nil {
		panic(fmt.Sprintf("failed to write key: %v", err))
	}

	valLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(valLenData, uint32(len(value)))
	if err := env.MemManager.Write(offset+4+uint32(len(key)), valLenData); err != nil {
		panic(fmt.Sprintf("failed to write value length: %v", err))
	}

	if err := env.MemManager.Write(offset+8+uint32(len(key)), value); err != nil {
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
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2
	}

	if !iter.Valid() {
		return 0, 0, 0
	}

	value := iter.Value()
	env.GasUsed += uint64(len(value)) * constants.GasPerByte

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
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Charge base gas cost for DB read
	if err := env.Gas.ConsumeGas(constants.GasCostRead, "db_read base cost"); err != nil {
		panic(err) // Or handle more gracefully
	}

	// Read key length and charge per byte
	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		return 0
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	// Charge per-byte gas for key
	if err := env.Gas.ConsumeGas(uint64(keyLen)*constants.GasPerByte, "db_read key bytes"); err != nil {
		panic(err)
	}

	// Rest of existing code...

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key data: %v\n", err)
		return 0
	}
	fmt.Printf("Key data: %x\n", key)

	value := env.DB.Get(key)
	fmt.Printf("Value found: %x\n", value)

	valuePtr, err := env.MemManager.Allocate(uint32(len(value)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory: %v\n", err)
		return 0
	}

	if err := env.MemManager.Write(valuePtr, value); err != nil {
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
	env := envVal.(*types.RuntimeEnvironment)

	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	valLenBytes, err := env.MemManager.Read(valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := env.MemManager.Read(valuePtr+4, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, value)
}

// hostSecp256k1Verify implements secp256k1_verify.
func hostSecp256k1Verify(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Get the environment and memory
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1Verify: runtime environment not found in context")
		return SECP256K1_VERIFY_CODE_INVALID
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Charge gas for this operation
	gasToCharge := env.GasConfig.Secp256k1VerifyCost + uint64(len(hash)+len(sig)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Verify signature using the crypto handler
	valid, err := cryptoHandler.Secp256k1Verify(hash, sig, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Secp256k1Verify failed: %v\n", err)
		return SECP256K1_VERIFY_CODE_INVALID
	}

	if valid {
		return SECP256K1_VERIFY_CODE_VALID
	}
	return SECP256K1_VERIFY_CODE_INVALID
}

// hostSecp256k1RecoverPubkey implements secp256k1_recover_pubkey.
func hostSecp256k1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, recoveryParam uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1RecoverPubkey: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return 0
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Secp256k1RecoverPubkeyCost + uint64(len(hash)+len(sig))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1RecoverPubkey: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Recover pubkey using cryptoHandler
	pubkey, err := cryptoHandler.Secp256k1RecoverPubkey(hash, sig, byte(recoveryParam))
	if err != nil {
		fmt.Printf("ERROR: Secp256k1RecoverPubkey failed: %v\n", err)
		return 0
	}

	// Allocate region for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(pubkey)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for recovered pubkey: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, pubkey) {
		fmt.Printf("ERROR: Failed to write recovered pubkey to memory\n")
		return 0
	}

	return resultPtr
}

// hostEd25519Verify implements ed25519_verify.
func hostEd25519Verify(ctx context.Context, mod api.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519Verify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	message, ok := mem.Read(msgPtr, msgLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read message from memory\n")
		return 0
	}

	signature, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519VerifyCost + uint64(len(message)+len(signature)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Verify signature
	valid, err := cryptoHandler.Ed25519Verify(message, signature, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Ed25519Verify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify.
func hostEd25519BatchVerify(ctx context.Context, mod api.Module, msgsPtr, msgsLen, sigsPtr, sigsLen, pubkeysPtr, pubkeysLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519BatchVerify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read array counts and pointers
	msgsData, ok := mem.Read(msgsPtr, msgsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read messages array from memory\n")
		return 0
	}

	sigsData, ok := mem.Read(sigsPtr, sigsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signatures array from memory\n")
		return 0
	}

	pubkeysData, ok := mem.Read(pubkeysPtr, pubkeysLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public keys array from memory\n")
		return 0
	}

	// Parse arrays (implementation depends on how arrays are serialized)
	// This is a simplified example - actual parsing logic may differ
	messages, signatures, pubkeys := parseArraysForBatchVerify(msgsData, sigsData, pubkeysData)

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519BatchVerifyCost * uint64(len(messages))
	dataSize := 0
	for i := 0; i < len(messages); i++ {
		dataSize += len(messages[i]) + len(signatures[i]) + len(pubkeys[i])
	}
	gasToCharge += uint64(dataSize) * constants.GasPerByte

	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519BatchVerify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Batch verify signatures
	valid, err := cryptoHandler.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		fmt.Printf("ERROR: Ed25519BatchVerify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// parseArraysForBatchVerify parses the array data for Ed25519BatchVerify
// Implementation depends on how arrays are serialized in the contract
func parseArraysForBatchVerify(msgsData, sigsData, pubkeysData []byte) ([][]byte, [][]byte, [][]byte) {
	// Example implementation - actual parsing may differ based on serialization format
	// This is a placeholder implementation

	// In a real implementation, you would parse the arrays from their serialized format
	// Here we're creating dummy data just to satisfy the function signature
	count := 1 // In reality, extract this from the data

	messages := make([][]byte, count)
	signatures := make([][]byte, count)
	pubkeys := make([][]byte, count)

	// Fill with dummy data for demonstration
	for i := 0; i < count; i++ {
		messages[i] = []byte("message")
		signatures[i] = make([]byte, 64)
		pubkeys[i] = make([]byte, 32)
	}

	return messages, signatures, pubkeys
}

// hostBls12381AggregateG1 implements bls12_381_aggregate_g1.
func hostBls12381AggregateG1(ctx context.Context, mod api.Module, g1sPtr, g1sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG1: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 points
	g1s, ok := mem.Read(g1sPtr, g1sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return 0
	}

	pointCount := len(g1s) / constants.BLS12_381_G1_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G1 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG1Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during G1 aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g1s, constants.BLS12_381_G1_POINT_LEN)

	// Use cryptoHandler to aggregate points
	result, err := cryptoHandler.BLS12381AggregateG1(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G1 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G1 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// splitIntoPoints splits a byte array into equal-sized points
func splitIntoPoints(data []byte, pointLen int) [][]byte {
	pointCount := len(data) / pointLen
	points := make([][]byte, pointCount)

	for i := 0; i < pointCount; i++ {
		points[i] = data[i*pointLen : (i+1)*pointLen]
	}

	return points
}

// hostBls12381AggregateG2 implements bls12_381_aggregate_g2.
func hostBls12381AggregateG2(ctx context.Context, mod api.Module, g2sPtr, g2sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data
	mem := mod.Memory()
	g2s, ok := mem.Read(g2sPtr, g2sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return 0
	}

	pointCount := len(g2s) / constants.BLS12_381_G2_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G2 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG2Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g2s, constants.BLS12_381_G2_POINT_LEN)

	// Use cryptoHandler interface instead of direct function call
	result, err := cryptoHandler.BLS12381AggregateG2(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G2 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G2 point to memory\n")
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
	env := envVal.(*types.RuntimeEnvironment)

	// Read the 4-byte length prefix from the key pointer.
	lenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key.
	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	env.DB.Delete(key)
}

// Add missing gasPerByte constant
const gasPerByte = constants.GasPerByte

// cryptoHandler holds crypto operations - initialized at runtime
var cryptoHandler cryptoapi.CryptoOperations

// SetCryptoHandler sets the crypto handler for host functions
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// readMessage reads a message of specified length from memory
func readMessage(mod api.Module, ptr, len uint32) ([]byte, error) {
	if len > constants.BLS12_381_MAX_MESSAGE_SIZE {
		return nil, fmt.Errorf("message too large: %d > %d", len, constants.BLS12_381_MAX_MESSAGE_SIZE)
	}

	mem := mod.Memory()
	data, ok := mem.Read(ptr, len)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", ptr, len)
	}
	return data, nil
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
func hostBls12381HashToG1(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Get environment context
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG1: runtime environment not found in context")
		return 0
	}

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G1 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
func hostBls12381HashToG2(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381HashToG2Cost.TotalCost(uint64(len(message)))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Hash-to-G2: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G2 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381VerifyG1G2 implements bls12_381_verify.
func hostBls12381VerifyG1G2(ctx context.Context, mod api.Module, g1PointsPtr, g1PointsLen, g2PointsPtr, g2PointsLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381VerifyG1G2: runtime environment not found in context")
		return BLS12_381_INVALID_PAIRING
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 and G2 points
	g1Data, ok := mem.Read(g1PointsPtr, g1PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g2Data, ok := mem.Read(g2PointsPtr, g2PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g1Count := len(g1Data) / constants.BLS12_381_G1_POINT_LEN
	g2Count := len(g2Data) / constants.BLS12_381_G2_POINT_LEN

	if g1Count != g2Count {
		fmt.Printf("ERROR: Number of G1 points (%d) must match number of G2 points (%d)\n", g1Count, g2Count)
		return BLS12_381_INVALID_PAIRING
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381VerifyCost.TotalCost(uint64(g1Count))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during BLS verification: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return BLS12_381_INVALID_PAIRING
	}

	// Split into individual points
	g1Points := splitIntoPoints(g1Data, constants.BLS12_381_G1_POINT_LEN)
	g2Points := splitIntoPoints(g2Data, constants.BLS12_381_G2_POINT_LEN)

	// Verify pairing
	valid, err := cryptoHandler.BLS12381VerifyG1G2(g1Points, g2Points)
	if err != nil {
		fmt.Printf("ERROR: BLS12-381 verification failed: %v\n", err)
		return BLS12_381_INVALID_PAIRING
	}

	if valid {
		return BLS12_381_VALID_PAIRING
	}
	return BLS12_381_INVALID_PAIRING
}
