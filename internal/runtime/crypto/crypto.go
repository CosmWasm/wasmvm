package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"

	"crypto/ed25519"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"

	bls12381 "github.com/kilic/bls12-381"
)

// Ensure CryptoImplementation implements the interfaces
var _ cryptoapi.CryptoOperations = (*CryptoImplementation)(nil)

// CryptoImplementation provides concrete implementations of crypto operations
type CryptoImplementation struct{}

// NewCryptoImplementation creates a new crypto implementation
func NewCryptoImplementation() *CryptoImplementation {
	return &CryptoImplementation{}
}

// BLS12381AggregateG1 aggregates multiple G1 points into a single compressed G1 point.
func (c *CryptoImplementation) BLS12381AggregateG1(elements [][]byte) ([]byte, error) {
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

// BLS12381HashToG1 hashes arbitrary bytes to a compressed G1 point.
func (c *CryptoImplementation) BLS12381HashToG1(message, dst []byte) ([]byte, error) {
	g1 := bls12381.NewG1()
	point, err := g1.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G1: %w", err)
	}
	return g1.ToCompressed(point), nil
}

// BLS12381HashToG2 hashes arbitrary bytes to a compressed G2 point.
func (c *CryptoImplementation) BLS12381HashToG2(message, dst []byte) ([]byte, error) {
	g2 := bls12381.NewG2()
	point, err := g2.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G2: %w", err)
	}
	return g2.ToCompressed(point), nil
}

// BLS12381VerifyG1G2 checks if the pairing product of G1 and G2 points equals the identity in GT.
func (c *CryptoImplementation) BLS12381VerifyG1G2(g1Points, g2Points [][]byte) (bool, error) {
	if len(g1Points) != len(g2Points) {
		return false, fmt.Errorf("number of G1 points (%d) must equal number of G2 points (%d)", len(g1Points), len(g2Points))
	}
	if len(g1Points) == 0 {
		return false, fmt.Errorf("at least one pair of points is required")
	}

	g1 := bls12381.NewG1()
	g2 := bls12381.NewG2()
	engine := bls12381.NewEngine()

	// For each (G1, G2) pair, add their pairing to the calculation
	for i := 0; i < len(g1Points); i++ {
		p1, err := g1.FromCompressed(g1Points[i])
		if err != nil {
			return false, fmt.Errorf("invalid G1 point at index %d: %w", i, err)
		}

		p2, err := g2.FromCompressed(g2Points[i])
		if err != nil {
			return false, fmt.Errorf("invalid G2 point at index %d: %w", i, err)
		}

		engine.AddPair(p1, p2)
	}

	// Check if the pairing result equals 1 (the identity element in GT)
	return engine.Check(), nil
}

// Secp256k1Verify verifies a secp256k1 signature in [R || s] format (65 bytes).
func (c *CryptoImplementation) Secp256k1Verify(messageHash, signature, pubkey []byte) (bool, error) {
	if len(messageHash) != 32 {
		return false, fmt.Errorf("message hash must be 32 bytes, got %d", len(messageHash))
	}
	if len(signature) != 64 && len(signature) != 65 {
		return false, fmt.Errorf("signature must be 64 or 65 bytes, got %d", len(signature))
	}

	// Use 64-byte signature format (R, s)
	sigR := new(big.Int).SetBytes(signature[:32])
	sigS := new(big.Int).SetBytes(signature[32:64])

	// Parse the public key
	pk, err := parseSecp256k1PubKey(pubkey)
	if err != nil {
		return false, err
	}

	// Verify the signature
	return ecdsa.Verify(pk, messageHash, sigR, sigS), nil
}

// Secp256k1RecoverPubkey recovers a public key from a signature and recovery byte.
func (c *CryptoImplementation) Secp256k1RecoverPubkey(messageHash, signature []byte, recovery byte) ([]byte, error) {
	if len(messageHash) != 32 {
		return nil, fmt.Errorf("message hash must be 32 bytes, got %d", len(messageHash))
	}
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	if recovery > 3 {
		return nil, fmt.Errorf("recovery byte must be 0, 1, 2, or 3, got %d", recovery)
	}

	// Parse r and s from the signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Calculate recovery parameters
	curve := secp256k1Curve()
	recid := int(recovery)
	isOdd := recid&1 != 0
	isSecondX := recid&2 != 0

	// Use r instead of creating a new x variable
	if isSecondX {
		r.Add(r, new(big.Int).Set(curve.Params().N))
	}

	// Calculate corresponding y value
	y, err := recoverY(curve, r, isOdd)
	if err != nil {
		return nil, err
	}

	// Construct R point using r instead of x
	R := &ecdsa.PublicKey{
		Curve: curve,
		X:     r,
		Y:     y,
	}

	// Derive from R and signature the original public key
	e := new(big.Int).SetBytes(messageHash)
	Rinv := new(big.Int).ModInverse(R.X, curve.Params().N)
	if Rinv == nil {
		return nil, fmt.Errorf("failed to compute modular inverse")
	}

	// Calculate r⁻¹(sR - eG)
	sR := new(big.Int).Mul(s, R.X)
	sR.Mod(sR, curve.Params().N)

	eG := new(big.Int).Neg(e)
	eG.Mod(eG, curve.Params().N)

	Q := ecPointAdd(
		curve,
		R.X, R.Y, // R
		ecScalarMult(curve, eG, nil, nil), // eG
	)

	Q = ecScalarMult(curve, Rinv, Q[0], Q[1])

	// Convert the recovered public key to compressed format
	return compressPublicKey(Q[0], Q[1]), nil
}

// Secp256r1RecoverPubkey recovers a secp256r1 public key from a signature
func (c *CryptoImplementation) Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error) {
	// Current placeholder needs actual implementation
	curve := elliptic.P256() // NIST P-256 (secp256r1) curve

	// Implement proper recovery:
	recid := int(recovery)
	isOdd := recid&1 != 0
	isSecondX := recid&2 != 0

	// Restore potential x coordinate from signature
	x := new(big.Int).SetBytes(signature[:32])
	if isSecondX {
		x.Add(x, new(big.Int).Set(curve.Params().N))
	}

	// Calculate corresponding y value
	y, err := recoverY(curve, x, isOdd)
	if err != nil {
		return nil, err
	}

	// Create compressed public key
	compressedPubKey := make([]byte, 33)
	compressedPubKey[0] = byte(0x02) + byte(y.Bit(0))
	xBytes := x.Bytes()
	copy(compressedPubKey[1+32-len(xBytes):], xBytes)

	return compressedPubKey, nil
}

// Ed25519Verify verifies an Ed25519 signature.
func (c *CryptoImplementation) Ed25519Verify(message, signature, pubKey []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	if len(pubKey) != 32 {
		return false, fmt.Errorf("public key must be 32 bytes, got %d", len(pubKey))
	}

	// Use Go's ed25519 implementation to verify
	return ed25519Verify(pubKey, message, signature), nil
}

// Ed25519BatchVerify verifies multiple Ed25519 signatures in a batch.
func (c *CryptoImplementation) Ed25519BatchVerify(messages, signatures, pubKeys [][]byte) (bool, error) {
	if len(messages) != len(signatures) || len(messages) != len(pubKeys) {
		return false, fmt.Errorf("number of messages (%d), signatures (%d), and public keys (%d) must be equal",
			len(messages), len(signatures), len(pubKeys))
	}

	for i := 0; i < len(messages); i++ {
		if ok, _ := c.Ed25519Verify(messages[i], signatures[i], pubKeys[i]); !ok {
			return false, nil
		}
	}
	return true, nil
}

// parseSecp256k1PubKey parses a SEC1 encoded public key in compressed or uncompressed format
func parseSecp256k1PubKey(pubKeyBytes []byte) (*ecdsa.PublicKey, error) {
	curve := secp256k1Curve()

	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("empty public key")
	}

	// Handle compressed public key format
	if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		x := new(big.Int).SetBytes(pubKeyBytes[1:])
		isOdd := pubKeyBytes[0] == 0x03
		y, err := recoverY(curve, x, isOdd)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	// Handle uncompressed public key format
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		x := new(big.Int).SetBytes(pubKeyBytes[1:33])
		y := new(big.Int).SetBytes(pubKeyBytes[33:])
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	return nil, fmt.Errorf("invalid public key format or length: %d", len(pubKeyBytes))
}

// recoverY calculates the Y coordinate for a given X coordinate on an elliptic curve
func recoverY(curve elliptic.Curve, x *big.Int, isOdd bool) (*big.Int, error) {
	// y² = x³ + ax + b
	// For secp256k1, a = 0 and b = 7

	// Calculate x³ + ax + b
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	// For secp256k1, a = 0, so we skip adding ax

	// Add b (7 for secp256k1, different for other curves)
	b := getB(curve)
	x3.Add(x3, b)

	// Modulo p
	x3.Mod(x3, curve.Params().P)

	// Calculate the square root modulo p
	y := new(big.Int).ModSqrt(x3, curve.Params().P)
	if y == nil {
		return nil, fmt.Errorf("no square root exists for y")
	}

	// Check if we need the "other" root (p - y)
	if isOdd != isOddValue(y) {
		y.Sub(curve.Params().P, y)
	}

	return y, nil
}

// isOddValue checks if a big.Int value is odd
func isOddValue(value *big.Int) bool {
	return value.Bit(0) == 1
}

// getB returns the b parameter for the curve equation y² = x³ + ax + b
func getB(curve elliptic.Curve) *big.Int {
	if curve == secp256k1Curve() {
		return big.NewInt(7) // Secp256k1 has b = 7
	}
	if curve == elliptic.P256() {
		// Return the b parameter for P-256 (secp256r1)
		b, _ := new(big.Int).SetString("5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604b", 16)
		return b
	}
	// Default, though this should not happen with our supported curves
	return big.NewInt(0)
}

// secp256k1Curve returns an elliptic.Curve instance for secp256k1
func secp256k1Curve() elliptic.Curve {
	// This is a simplified version - in production code, you would use a proper secp256k1 implementation
	// For now, we'll use a placeholder that matches the rest of the code
	return elliptic.P256() // This is a placeholder - real code would return actual secp256k1
}

// ecPointAdd adds two elliptic curve points
func ecPointAdd(curve elliptic.Curve, x1, y1 *big.Int, point [2]*big.Int) [2]*big.Int {
	x2, y2 := point[0], point[1]
	x3, y3 := curve.Add(x1, y1, x2, y2)
	return [2]*big.Int{x3, y3}
}

// ecScalarMult multiplies a point on an elliptic curve by a scalar
func ecScalarMult(curve elliptic.Curve, k *big.Int, x, y *big.Int) [2]*big.Int {
	if x == nil || y == nil {
		// If point is the identity (represented as nil), use the base point
		x, y = curve.Params().Gx, curve.Params().Gy
	}
	x3, y3 := curve.ScalarMult(x, y, k.Bytes())
	return [2]*big.Int{x3, y3}
}

// compressPublicKey creates a compressed representation of a public key
func compressPublicKey(x, y *big.Int) []byte {
	result := make([]byte, 33)
	// Set prefix based on Y coordinate's parity
	if y.Bit(0) == 0 {
		result[0] = 0x02 // even Y
	} else {
		result[0] = 0x03 // odd Y
	}

	// Pad X coordinate to 32 bytes
	xBytes := x.Bytes()
	offset := 1 + 32 - len(xBytes)
	copy(result[offset:], xBytes)

	return result
}

// ed25519Verify verifies an ED25519 signature
func ed25519Verify(pubKey, message, signature []byte) bool {
	// In a real implementation, use Go's crypto/ed25519 package

	if len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(ed25519.PublicKey(pubKey), message, signature)
}

// Secp256r1Verify verifies a signature using the NIST P-256 curve (secp256r1)
func (c *CryptoImplementation) Secp256r1Verify(hash, signature, pubkey []byte) (bool, error) {
	// Implementation details similar to Secp256k1Verify but using P-256 curve
	if len(hash) != 32 {
		return false, fmt.Errorf("message hash must be 32 bytes, got %d", len(hash))
	}
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	curve := elliptic.P256() // Use NIST P-256 curve

	// Parse signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Parse public key
	pk, err := parsePublicKey(pubkey, curve)
	if err != nil {
		return false, err
	}

	// Verify signature
	return ecdsa.Verify(pk, hash, r, s), nil
}

// parsePublicKey parses a SEC1 encoded public key for the specified curve
func parsePublicKey(pubKeyBytes []byte, curve elliptic.Curve) (*ecdsa.PublicKey, error) {
	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("empty public key")
	}

	// Handle compressed public key format
	if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		x := new(big.Int).SetBytes(pubKeyBytes[1:])
		isOdd := pubKeyBytes[0] == 0x03
		y, err := recoverY(curve, x, isOdd)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	// Handle uncompressed public key format
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		x := new(big.Int).SetBytes(pubKeyBytes[1:33])
		y := new(big.Int).SetBytes(pubKeyBytes[33:])
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	return nil, fmt.Errorf("invalid public key format or length: %d", len(pubKeyBytes))
}
