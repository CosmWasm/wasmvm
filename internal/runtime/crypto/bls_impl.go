package crypto

import (
	"fmt"

	bls12381 "github.com/kilic/bls12-381"
)

// Implementation of BLS12381AggregateG1 and BLS12381AggregateG2

// BLS12381AggregateG2 aggregates multiple G2 points into a single compressed G2 point.
func (c *CryptoImplementation) BLS12381AggregateG2(elements [][]byte) ([]byte, error) {
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
