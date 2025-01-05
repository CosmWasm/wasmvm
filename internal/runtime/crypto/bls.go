package crypto

import (
	"fmt"

	bls12381 "github.com/kilic/bls12-381"
)

// BLS12381AggregateG1 aggregates multiple G1 points into a single compressed G1 point.
func (v *Verifier) BLS12381AggregateG1(elements [][]byte) ([]byte, error) {
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
func (v *Verifier) BLS12381AggregateG2(elements [][]byte) ([]byte, error) {
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

// BLS12381HashToG1 hashes a message to a G1 point.
func (v *Verifier) BLS12381HashToG1(message []byte) ([]byte, error) {
	g1 := bls12381.NewG1()
	domain := []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_")
	point, err := g1.HashToCurve(message, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G1: %w", err)
	}
	return g1.ToCompressed(point), nil
}

// BLS12381HashToG2 hashes a message to a G2 point.
func (v *Verifier) BLS12381HashToG2(message []byte) ([]byte, error) {
	g2 := bls12381.NewG2()
	domain := []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_")
	point, err := g2.HashToCurve(message, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G2: %w", err)
	}
	return g2.ToCompressed(point), nil
}

// BLS12381PairingEquality checks if e(a1, a2) = e(b1, b2).
func (v *Verifier) BLS12381PairingEquality(a1Compressed, a2Compressed, b1Compressed, b2Compressed []byte) (bool, error) {
	engine := bls12381.NewEngine()
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

	engine.AddPair(a1, a2)
	engine.AddPairInv(b1, b2)
	return engine.Check(), nil
}
