package types

type Gas = uint64

type GasMeter interface {
	GasConsumed() Gas
}

// GasConfig holds gas costs for different operations
type GasConfig struct {
	// Memory operations
	PerByte uint64

	// Database operations
	DatabaseRead  uint64
	DatabaseWrite uint64
	ExternalQuery uint64

	// Iterator operations
	IteratorCreate uint64
	IteratorNext   uint64

	// Contract operations
	Instantiate uint64
	Execute     uint64

	Bls12381AggregateG1Cost GasCost
	Bls12381AggregateG2Cost GasCost
}

type GasCost struct {
	BaseCost uint64
	PerPoint uint64
}

func (c GasCost) TotalCost(pointCount uint64) uint64 {
	return c.BaseCost + c.PerPoint*pointCount
}
