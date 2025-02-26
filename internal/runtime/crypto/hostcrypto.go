package crypto

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/host"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	wazerotypes "github.com/tetratelabs/wazero/api"
)

type contextKey string

const envKey contextKey = "env"

// Add global handler variable
var cryptoHandler cryptoapi.CryptoOperations

// Add function to set the handler
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
// It reads the message and domain separation tag from contract memory using MemoryManager,
// charges gas, calls BLS12381HashToG1, allocates space for the result, writes it, and returns the pointer.
func hostBls12381HashToG1(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Retrieve the runtime environment from context.
	env := ctx.Value(envKey).(*host.RuntimeEnvironment)

	// Create a MemoryManager for the contract module.
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read the input message.
	message, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	// Read the domain separation tag.
	dst, err := mm.Read(dstPtr, dstLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation.
	env.Gas.(types.GasMeter).ConsumeGas(uint64(hashLen+dstLen)*constants.GasPerByte, "BLS12381 hash operation")

	// Hash to curve.
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		return 0
	}

	// Allocate memory for the result.
	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0
	}

	// Write the result into memory.
	if err := mm.Write(resultPtr, result); err != nil {
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
// It follows the same pattern as hostBls12381HashToG1.
func hostBls12381HashToG2(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	env := ctx.Value(envKey).(*host.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	message, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	dst, err := mm.Read(dstPtr, dstLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation.
	env.Gas.(types.GasMeter).ConsumeGas(uint64(hashLen+dstLen)*constants.GasPerByte, "BLS12381 hash operation")

	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		return 0
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0
	}

	if err := mm.Write(resultPtr, result); err != nil {
		return 0
	}

	return resultPtr
}

// hostBls12381PairingEquality implements bls12_381_pairing_equality.
// It reads the four compressed points from memory and calls BLS12381PairingEquality.
func hostBls12381PairingEquality(_ context.Context, mod wazerotypes.Module, a1Ptr, a1Len, a2Ptr, a2Len, b1Ptr, b1Len, b2Ptr, b2Len uint32) uint32 {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	a1, err := mm.Read(a1Ptr, a1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a1: %v", err))
	}
	a2, err := mm.Read(a2Ptr, a2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a2: %v", err))
	}
	b1, err := mm.Read(b1Ptr, b1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b1: %v", err))
	}
	b2, err := mm.Read(b2Ptr, b2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b2: %v", err))
	}

	result, err := cryptoHandler.BLS12381VerifyG1G2(
		[][]byte{a1, b1}, // g1 points
		[][]byte{a2, b2}, // g2 points
	)
	if err != nil {
		panic(fmt.Sprintf("failed to check pairing equality: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1Verify implements secp256r1_verify.
// It reads the hash, signature, and public key from memory via MemoryManager,
// calls Secp256r1Verify, and returns 1 if valid.
func hostSecp256r1Verify(_ context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	sig, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read public key: %v", err))
	}

	result, err := cryptoHandler.Secp256r1Verify(hash, sig, pubkey)
	if err != nil {
		panic(fmt.Sprintf("failed to verify secp256r1 signature: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1RecoverPubkey implements secp256r1_recover_pubkey.
// It reads the hash and signature from memory, recovers the public key,
// allocates memory for it, writes it, and returns the pointer and length.
func hostSecp256r1RecoverPubkey(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, recovery uint32) (uint32, uint32) {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	result, err := cryptoHandler.Secp256r1RecoverPubkey(hash, signature, byte(recovery))
	if err != nil {
		panic(fmt.Sprintf("failed to recover public key: %v", err))
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
	}

	if err := mm.Write(resultPtr, result); err != nil {
		panic(fmt.Sprintf("failed to write result: %v", err))
	}

	return resultPtr, uint32(len(result))
}
