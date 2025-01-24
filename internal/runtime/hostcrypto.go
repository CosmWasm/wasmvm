package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// hostBls12381HashToG1 implements bls12_381_hash_to_g1
func hostBls12381HashToG1(ctx context.Context, mod api.Module, hashPtr, hashLen uint32) (uint32, uint32) {
	mem := mod.Memory()

	// Read hash
	hash, err := readMemory(mem, hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	// Perform hash-to-curve
	result, err := BLS12381HashToG1(hash)
	if err != nil {
		panic(fmt.Sprintf("failed to hash to G1: %v", err))
	}

	// Allocate memory for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
	}

	// Write result
	if err := writeMemory(mem, resultPtr, result, false); err != nil {
		panic(fmt.Sprintf("failed to write result: %v", err))
	}

	return resultPtr, uint32(len(result))
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2
func hostBls12381HashToG2(ctx context.Context, mod api.Module, hashPtr, hashLen uint32) (uint32, uint32) {
	mem := mod.Memory()

	// Read hash
	hash, err := readMemory(mem, hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	// Perform hash-to-curve
	result, err := BLS12381HashToG2(hash)
	if err != nil {
		panic(fmt.Sprintf("failed to hash to G2: %v", err))
	}

	// Allocate memory for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
	}

	// Write result
	if err := writeMemory(mem, resultPtr, result, false); err != nil {
		panic(fmt.Sprintf("failed to write result: %v", err))
	}

	return resultPtr, uint32(len(result))
}

// hostBls12381PairingEquality implements bls12_381_pairing_equality
func hostBls12381PairingEquality(_ context.Context, mod api.Module, a1Ptr, a1Len, a2Ptr, a2Len, b1Ptr, b1Len, b2Ptr, b2Len uint32) uint32 {
	mem := mod.Memory()

	// Read points
	a1, err := readMemory(mem, a1Ptr, a1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a1: %v", err))
	}
	a2, err := readMemory(mem, a2Ptr, a2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a2: %v", err))
	}
	b1, err := readMemory(mem, b1Ptr, b1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b1: %v", err))
	}
	b2, err := readMemory(mem, b2Ptr, b2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b2: %v", err))
	}

	// Check pairing equality
	result, err := BLS12381PairingEquality(a1, a2, b1, b2)
	if err != nil {
		panic(fmt.Sprintf("failed to check pairing equality: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1Verify implements secp256r1_verify
func hostSecp256r1Verify(_ context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	mem := mod.Memory()

	// Read hash
	hash, err := readMemory(mem, hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	// Read signature
	sig, err := readMemory(mem, sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	// Read public key
	pubkey, err := readMemory(mem, pubkeyPtr, pubkeyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read public key: %v", err))
	}

	// Verify signature
	result, err := Secp256r1Verify(hash, sig, pubkey)
	if err != nil {
		panic(fmt.Sprintf("failed to verify secp256r1 signature: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1RecoverPubkey implements secp256r1_recover_pubkey
func hostSecp256r1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, recovery uint32) (uint32, uint32) {
	mem := mod.Memory()

	// Read inputs
	hash, err := readMemory(mem, hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}
	signature, err := readMemory(mem, sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	// Recover public key
	pubkey, err := Secp256r1RecoverPubkey(hash, signature, byte(recovery))
	if err != nil {
		panic(fmt.Sprintf("failed to recover public key: %v", err))
	}

	// Allocate memory for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(pubkey)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
	}

	// Write result
	if err := writeMemory(mem, resultPtr, pubkey, false); err != nil {
		panic(fmt.Sprintf("failed to write result: %v", err))
	}

	return resultPtr, uint32(len(pubkey))
}
