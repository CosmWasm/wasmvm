package crypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"
)

// Secp256r1Verify verifies a P-256 ECDSA signature.
// hash is the message digest (NOT the preimage),
// signature should be 64 bytes (r and s concatenated),
// pubkey should be an uncompressed or compressed public key in standard format.
func (v *Verifier) Secp256r1Verify(hash, signature, pubkey []byte) (bool, error) {
	// Parse public key using crypto/ecdh
	curve := ecdh.P256()
	key, err := curve.NewPublicKey(pubkey)
	if err != nil {
		return false, fmt.Errorf("invalid public key: %w", err)
	}

	// Get the raw coordinates for ECDSA verification
	rawKey := key.Bytes()
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), rawKey)
	if x == nil {
		return false, errors.New("failed to parse public key coordinates")
	}

	// Parse signature: must be exactly 64 bytes => r (first 32 bytes), s (second 32 bytes).
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])

	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	verified := ecdsa.Verify(pub, hash, r, s)
	return verified, nil
}

// Secp256r1RecoverPubkey tries to recover a P-256 public key from a signature.
// This is a placeholder as P-256 doesn't support standard pubkey recovery.
func (v *Verifier) Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error) {
	return nil, errors.New("secp256r1 public key recovery not supported")
}
