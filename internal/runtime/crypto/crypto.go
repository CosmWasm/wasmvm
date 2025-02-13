package crypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
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
func BLS12381HashToG1(message, dst []byte) ([]byte, error) {
	g1 := bls12381.NewG1()
	point, err := g1.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G1: %w", err)
	}
	return g1.ToCompressed(point), nil
}

// BLS12381HashToG2 hashes arbitrary bytes to a compressed G2 point.
func BLS12381HashToG2(message, dst []byte) ([]byte, error) {
	g2 := bls12381.NewG2()
	point, err := g2.HashToCurve(message, dst)
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

// Secp256r1Verify verifies a signature using NIST P-256 (secp256r1)
func Secp256r1Verify(hash, signature, pubkey []byte) (bool, error) {
	if len(hash) != 32 {
		return false, fmt.Errorf("hash must be 32 bytes")
	}
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes")
	}

	// Parse the public key using crypto/ecdh
	curve := ecdh.P256()
	pk, err := curve.NewPublicKey(pubkey)
	if err != nil {
		return false, fmt.Errorf("invalid public key: %w", err)
	}

	// Convert to *ecdsa.PublicKey for verification
	ecdsaPub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(pk.Bytes()[1:33]), // Skip the first byte (format) and take 32 bytes for X
		Y:     new(big.Int).SetBytes(pk.Bytes()[33:]),  // Take the remaining 32 bytes for Y
	}

	// Split signature into r and s
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Verify the signature
	return ecdsa.Verify(ecdsaPub, hash, r, s), nil
}

// Secp256r1RecoverPubkey recovers a P-256 public key from a signature.
// hash is the message digest (NOT the preimage),
// signature should be 64 bytes (r and s concatenated),
// recovery is the recovery byte (0 or 1).
func Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes")
	}
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes")
	}
	if recovery > 1 {
		return nil, fmt.Errorf("recovery id must be 0 or 1")
	}

	// Parse r and s values from signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Get curve parameters
	curve := elliptic.P256()
	params := curve.Params()

	// Calculate x coordinate
	rx := r
	if recovery == 1 {
		rx = new(big.Int).Add(r, params.N)
	}

	// Calculate y coordinate
	y2 := new(big.Int)
	y2.Mul(rx, rx)
	y2.Mul(y2, rx)
	threeX := new(big.Int).Mul(rx, big.NewInt(3))
	y2.Sub(y2, threeX)
	y2.Add(y2, params.B)
	y2.Mod(y2, params.P)

	y := new(big.Int).ModSqrt(y2, params.P)
	if y == nil {
		return nil, fmt.Errorf("invalid signature: square root does not exist")
	}

	// Choose the correct y value based on parity
	if y.Bit(0) != uint(recovery) {
		y.Sub(params.P, y)
	}

	// Create public key point
	pub := &ecdsa.PublicKey{
		Curve: curve,
		X:     rx,
		Y:     y,
	}

	// Verify that this public key produces a valid signature
	if !ecdsa.Verify(pub, hash, r, s) {
		return nil, fmt.Errorf("invalid signature: verification failed")
	}

	// Convert to compressed format (33 bytes: 0x02 or 0x03 prefix + 32 bytes X coordinate)
	compressed := make([]byte, 33)
	compressed[0] = byte(0x02 + (y.Bit(0)))
	xBytes := pub.X.Bytes()
	// Pad X coordinate to 32 bytes if necessary
	copy(compressed[33-len(xBytes):], xBytes)

	return compressed, nil
}
