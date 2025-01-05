package crypto

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
)

// HostFunctions provides cryptographic host functions
type HostFunctions struct {
	verifier  *Verifier
	mem       *memory.HostFunctions
	allocator *memory.Allocator
}

// NewHostFunctions creates a new set of crypto host functions
func NewHostFunctions(mem *memory.HostFunctions, allocator *memory.Allocator) *HostFunctions {
	return &HostFunctions{
		verifier:  New(),
		mem:       mem,
		allocator: allocator,
	}
}

// VerifySecp256k1 verifies a secp256k1 signature
func (h *HostFunctions) VerifySecp256k1(hashPtr, sigPtr, pubkeyPtr uint32) (uint32, error) {
	// Read regions
	hashRegion, err := h.mem.ReadRegion(hashPtr)
	if err != nil {
		return 0, err
	}

	sigRegion, err := h.mem.ReadRegion(sigPtr)
	if err != nil {
		return 0, err
	}

	pubkeyRegion, err := h.mem.ReadRegion(pubkeyPtr)
	if err != nil {
		return 0, err
	}

	// Read data from memory
	hash, err := h.mem.ReadString(hashRegion)
	if err != nil {
		return 0, err
	}

	sig, err := h.mem.ReadString(sigRegion)
	if err != nil {
		return 0, err
	}

	pubkey, err := h.mem.ReadString(pubkeyRegion)
	if err != nil {
		return 0, err
	}

	// Verify signature
	ok, err := h.verifier.VerifySecp256k1Signature([]byte(hash), []byte(sig), []byte(pubkey))
	if err != nil {
		return 0, err
	}

	if ok {
		return 1, nil
	}
	return 0, nil
}

// VerifyEd25519 verifies an ed25519 signature
func (h *HostFunctions) VerifyEd25519(msgPtr, sigPtr, pubkeyPtr uint32) (uint32, error) {
	// Read regions
	msgRegion, err := h.mem.ReadRegion(msgPtr)
	if err != nil {
		return 0, err
	}

	sigRegion, err := h.mem.ReadRegion(sigPtr)
	if err != nil {
		return 0, err
	}

	pubkeyRegion, err := h.mem.ReadRegion(pubkeyPtr)
	if err != nil {
		return 0, err
	}

	// Read data from memory
	msg, err := h.mem.ReadString(msgRegion)
	if err != nil {
		return 0, err
	}

	sig, err := h.mem.ReadString(sigRegion)
	if err != nil {
		return 0, err
	}

	pubkey, err := h.mem.ReadString(pubkeyRegion)
	if err != nil {
		return 0, err
	}

	// Verify signature
	ok, err := h.verifier.VerifyEd25519Signature([]byte(msg), []byte(sig), []byte(pubkey))
	if err != nil {
		return 0, err
	}

	if ok {
		return 1, nil
	}
	return 0, nil
}

// Hash computes various cryptographic hashes
func (h *HostFunctions) Hash(hashType uint32, msgPtr uint32) (uint32, error) {
	// Read message region
	msgRegion, err := h.mem.ReadRegion(msgPtr)
	if err != nil {
		return 0, err
	}

	// Read message from memory
	msg, err := h.mem.ReadString(msgRegion)
	if err != nil {
		return 0, err
	}

	// Compute hash based on type
	var hash []byte
	switch hashType {
	case 1: // SHA256
		hash = h.verifier.SHA256([]byte(msg))
	case 2: // SHA512
		hash = h.verifier.SHA512([]byte(msg))
	default:
		return 0, fmt.Errorf("unsupported hash type: %d", hashType)
	}

	// Write hash to memory and return pointer
	ptr, _, err := h.allocator.Allocate(hash)
	if err != nil {
		return 0, err
	}

	return ptr, nil
}
