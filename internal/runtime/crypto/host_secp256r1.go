package crypto

import (
	"fmt"
)

// Secp256r1VerifyHost implements the secp256r1 signature verification host function
func (h *HostFunctions) Secp256r1VerifyHost(hashPtr, sigPtr, pubkeyPtr uint32) (uint32, error) {
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
	ok, err := h.verifier.Secp256r1Verify([]byte(hash), []byte(sig), []byte(pubkey))
	if err != nil {
		return 0, fmt.Errorf("failed to verify secp256r1 signature: %w", err)
	}

	if ok {
		return 1, nil
	}
	return 0, nil
}

// Secp256r1RecoverPubkeyHost implements the secp256r1 public key recovery host function
func (h *HostFunctions) Secp256r1RecoverPubkeyHost(hashPtr uint32, sigPtr uint32, recoveryParam uint32) (uint32, error) {
	// Read regions
	hashRegion, err := h.mem.ReadRegion(hashPtr)
	if err != nil {
		return 0, err
	}

	sigRegion, err := h.mem.ReadRegion(sigPtr)
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

	// Try to recover public key
	pubkey, err := h.verifier.Secp256r1RecoverPubkey([]byte(hash), []byte(sig), byte(recoveryParam))
	if err != nil {
		return 0, fmt.Errorf("failed to recover secp256r1 public key: %w", err)
	}

	// Write result to memory
	ptr, _, err := h.allocator.Allocate(pubkey)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate memory for result: %w", err)
	}

	return ptr, nil
}
