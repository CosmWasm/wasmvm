package runtime

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"

	bls12381 "github.com/kilic/bls12-381"
)

// BLS12381AggregateG1 aggregates multiple G1 points into a single compressed G1 point.
func BLS12381AggregateG1(elements [][]byte) ([]byte, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("no elements to aggregate")
	}

	g1 := bls12381.NewG1()
	result := g1.Zero()

	for _, element := range elements {
		point, err := g1.FromCompressed(element)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress G1 point: %w", err)
		}
		g1.Add(result, result, point)
	}

	return g1.ToCompressed(result), nil
}

// BLS12381AggregateG2 aggregates multiple G2 points into a single compressed G2 point.
func BLS12381AggregateG2(elements [][]byte) ([]byte, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("no elements to aggregate")
	}

	g2 := bls12381.NewG2()
	result := g2.Zero()

	for _, element := range elements {
		point, err := g2.FromCompressed(element)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress G2 point: %w", err)
		}
		g2.Add(result, result, point)
	}

	return g2.ToCompressed(result), nil
}

// BLS12381HashToG1 hashes arbitrary bytes to a compressed G1 point.
func BLS12381HashToG1(message []byte) ([]byte, error) {
	g1 := bls12381.NewG1()
	// You can choose a domain separation string of your liking.
	// Here, we use a placeholder domain: "BLS12381G1_XMD:SHA-256_SSWU_RO_"
	point, err := g1.HashToCurve(message, []byte("BLS12381G1_XMD:SHA-256_SSWU_RO_"))
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G1: %w", err)
	}
	return g1.ToCompressed(point), nil
}

// BLS12381HashToG2 hashes arbitrary bytes to a compressed G2 point.
func BLS12381HashToG2(message []byte) ([]byte, error) {
	g2 := bls12381.NewG2()
	// Similar domain separation string for G2.
	point, err := g2.HashToCurve(message, []byte("BLS12381G2_XMD:SHA-256_SSWU_RO_"))
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G2: %w", err)
	}
	return g2.ToCompressed(point), nil
}

// BLS12381PairingEquality checks if e(a1, a2) == e(b1, b2) in the BLS12-381 pairing.
func BLS12381PairingEquality(a1Compressed, a2Compressed, b1Compressed, b2Compressed []byte) (bool, error) {
	g1 := bls12381.NewG1()
	g2 := bls12381.NewG2()

	a1, err := g1.FromCompressed(a1Compressed)
	if err != nil {
		return false, fmt.Errorf("failed to decompress a1: %w", err)
	}
	a2, err := g2.FromCompressed(a2Compressed)
	if err != nil {
		return false, fmt.Errorf("failed to decompress a2: %w", err)
	}
	b1, err := g1.FromCompressed(b1Compressed)
	if err != nil {
		return false, fmt.Errorf("failed to decompress b1: %w", err)
	}
	b2, err := g2.FromCompressed(b2Compressed)
	if err != nil {
		return false, fmt.Errorf("failed to decompress b2: %w", err)
	}

	engine := bls12381.NewEngine()
	// AddPair computes pairing e(a1, a2).
	engine.AddPair(a1, a2)
	// AddPairInv computes pairing e(b1, b2)^(-1), so effectively we check e(a1,a2) * e(b1,b2)^(-1) == 1.
	engine.AddPairInv(b1, b2)

	ok := engine.Check()
	return ok, nil
}

// Secp256r1Verify verifies a P-256 ECDSA signature.
// hash is the message digest (NOT the preimage),
// signature should be 64 bytes (r and s concatenated),
// pubkey should be an uncompressed or compressed public key in standard format.
func Secp256r1Verify(hash, signature, pubkey []byte) (bool, error) {
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
// In general, ECDSA on P-256 is not commonly used with "public key recovery" like secp256k1.
// This is non-standard and provided here as a placeholder or with specialized tooling only.
func Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error) {
	// ECDSA on secp256r1 (P-256) does not support public key recovery in the standard library.
	// Typically one would need a specialized library. This stub is included for completeness.
	return nil, fmt.Errorf("public key recovery is not standard for secp256r1")
}
