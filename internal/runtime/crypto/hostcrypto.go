package crypto

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/hostapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/tetratelabs/wazero"
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
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)

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
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
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

// SetupCryptoHandlers initializes the crypto system by creating and setting the global crypto handler
func SetupCryptoHandlers() error {
	// Create a new implementation of the CryptoOperations interface
	impl := NewCryptoImplementation()

	// Set it as the global handler
	SetCryptoHandler(impl)

	return nil
}

// RegisterHostFunctions registers all crypto host functions with the provided module
func RegisterHostFunctions(mod wazero.HostModuleBuilder) {
	// Register BLS functions
	mod.NewFunctionBuilder().WithFunc(hostBls12381HashToG1).Export("bls12_381_hash_to_g1")
	mod.NewFunctionBuilder().WithFunc(hostBls12381HashToG2).Export("bls12_381_hash_to_g2")
	mod.NewFunctionBuilder().WithFunc(hostBls12381PairingEquality).Export("bls12_381_pairing_equality")

	// Register secp256r1 functions
	mod.NewFunctionBuilder().WithFunc(hostSecp256r1Verify).Export("secp256r1_verify")
	mod.NewFunctionBuilder().WithFunc(hostSecp256r1RecoverPubkey).Export("secp256r1_recover_pubkey")

	// Register secp256k1 functions
	mod.NewFunctionBuilder().WithFunc(secp256k1Verify).Export("secp256k1_verify")
	mod.NewFunctionBuilder().WithFunc(secp256k1RecoverPubkey).Export("secp256k1_recover_pubkey")

	// Register ed25519 functions
	mod.NewFunctionBuilder().WithFunc(hostEd25519Verify).Export("ed25519_verify")
	mod.NewFunctionBuilder().WithFunc(hostEd25519BatchVerify).Export("ed25519_batch_verify")
}

// secp256k1Verify implements secp256k1_verify
// It reads message hash, signature, and public key from memory, calls Secp256k1Verify,
// and returns 1 if valid or 0 otherwise
func secp256k1Verify(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Retrieve the runtime environment from context
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)

	// Create memory manager to access WebAssembly memory
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read hash from memory
	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	// Read signature from memory
	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		return 0
	}

	// Read public key from memory
	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation
	env.Gas.(types.GasMeter).ConsumeGas(
		uint64(hashLen+sigLen+pubkeyLen)*constants.GasPerByte,
		"secp256k1 verification",
	)

	// Call the implementation function
	result, err := cryptoHandler.Secp256k1Verify(hash, signature, pubkey)
	if err != nil {
		return 0
	}

	if result {
		return 1
	}
	return 0
}

// secp256k1RecoverPubkey implements secp256k1_recover_pubkey
// It reads hash and signature from memory, recovers the public key,
// allocates space for the result, and returns the pointer and length
func secp256k1RecoverPubkey(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, recovery uint32) (uint32, uint32) {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0, 0
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		return 0, 0
	}

	// Charge gas for operation
	env.Gas.(types.GasMeter).ConsumeGas(
		uint64(hashLen+sigLen)*constants.GasPerByte,
		"secp256k1 key recovery",
	)

	result, err := cryptoHandler.Secp256k1RecoverPubkey(hash, signature, byte(recovery))
	if err != nil {
		return 0, 0
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0, 0
	}

	if err := mm.Write(resultPtr, result); err != nil {
		return 0, 0
	}

	return resultPtr, uint32(len(result))
}

// hostEd25519Verify implements ed25519_verify
// It reads message, signature, and public key from memory and calls Ed25519Verify
func hostEd25519Verify(ctx context.Context, mod wazerotypes.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	message, err := mm.Read(msgPtr, msgLen)
	if err != nil {
		return 0
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		return 0
	}

	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		return 0
	}

	// Charge gas
	env.Gas.(types.GasMeter).ConsumeGas(
		uint64(msgLen+sigLen+pubkeyLen)*constants.GasPerByte,
		"ed25519 verification",
	)

	result, err := cryptoHandler.Ed25519Verify(message, signature, pubkey)
	if err != nil {
		return 0
	}

	if result {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify
// It reads multiple messages, signatures, and public keys from memory and calls Ed25519BatchVerify
func hostEd25519BatchVerify(ctx context.Context, mod wazerotypes.Module, msgsPtr, sigsPtr, pubkeysPtr uint32) uint32 {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read batch data from arrays of pointers
	// This is a simplified version - actual implementation needs to read array of arrays

	// Example (would need to be adapted to actual memory layout):
	// Read number of items in batch
	countPtr, err := mm.Read(msgsPtr, 4)
	if err != nil {
		return 0
	}
	count := uint32(countPtr[0]) | uint32(countPtr[1])<<8 | uint32(countPtr[2])<<16 | uint32(countPtr[3])<<24

	messages := make([][]byte, count)
	signatures := make([][]byte, count)
	pubkeys := make([][]byte, count)

	totalBytes := uint64(0)

	// Read each message, signature, and pubkey
	for i := uint32(0); i < count; i++ {
		// Read message pointer and length
		msgPtrAddr := msgsPtr + 8*i + 4
		msgLenAddr := msgsPtr + 8*i + 8
		msgPtr, ok := mm.ReadUint32(msgPtrAddr)
		if !ok {
			return 0
		}
		msgLen, ok := mm.ReadUint32(msgLenAddr)
		if !ok {
			return 0
		}

		// Read signature pointer and length
		sigPtrAddr := sigsPtr + 8*i + 4
		sigLenAddr := sigsPtr + 8*i + 8
		sigPtr, ok := mm.ReadUint32(sigPtrAddr)
		if !ok {
			return 0
		}
		sigLen, ok := mm.ReadUint32(sigLenAddr)
		if !ok {
			return 0
		}

		// Read pubkey pointer and length
		pubkeyPtrAddr := pubkeysPtr + 8*i + 4
		pubkeyLenAddr := pubkeysPtr + 8*i + 8
		pubkeyPtr, ok := mm.ReadUint32(pubkeyPtrAddr)
		if !ok {
			return 0
		}
		pubkeyLen, ok := mm.ReadUint32(pubkeyLenAddr)
		if !ok {
			return 0
		}

		// Read actual data
		msg, err := mm.Read(msgPtr, msgLen)
		if err != nil {
			return 0
		}
		sig, err := mm.Read(sigPtr, sigLen)
		if err != nil {
			return 0
		}
		pk, err := mm.Read(pubkeyPtr, pubkeyLen)
		if err != nil {
			return 0
		}

		messages[i] = msg
		signatures[i] = sig
		pubkeys[i] = pk

		totalBytes += uint64(msgLen + sigLen + pubkeyLen)
	}

	// Charge gas
	env.Gas.(types.GasMeter).ConsumeGas(
		totalBytes*constants.GasPerByte,
		"ed25519 batch verification",
	)

	result, err := cryptoHandler.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		return 0
	}

	if result {
		return 1
	}
	return 0
}
