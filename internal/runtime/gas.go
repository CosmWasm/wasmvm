package runtime

import "fmt"

// Gas costs for various operations
const (
	// Memory operations
	gasPerByte = 1

	// Database operations
	gasCostRead  = 100
	gasCostWrite = 200
	gasCostQuery = 500

	// Iterator operations
	gasCostIteratorCreate = 10000 // Base cost for creating an iterator
	gasCostIteratorNext   = 1000  // Base cost for iterator next operations
)

// GasConfig holds gas costs for different operations
type GasConfig struct {
	PerByte        uint64
	DatabaseRead   uint64
	DatabaseWrite  uint64
	ExternalQuery  uint64
	IteratorCreate uint64
	IteratorNext   uint64
}

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() GasConfig {
	return GasConfig{
		PerByte:        gasPerByte,
		DatabaseRead:   gasCostRead,
		DatabaseWrite:  gasCostWrite,
		ExternalQuery:  gasCostQuery,
		IteratorCreate: gasCostIteratorCreate,
		IteratorNext:   gasCostIteratorNext,
	}
}

// GasState tracks gas usage during execution
type GasState struct {
	limit uint64
	used  uint64
}

// NewGasState creates a new GasState with the given limit
func NewGasState(limit uint64) *GasState {
	return &GasState{
		limit: limit,
		used:  0,
	}
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) {
	g.used += amount
	if g.used > g.limit {
		panic(fmt.Sprintf("out of gas: used %d, limit %d - %s", g.used, g.limit, description))
	}
}

// GetGasUsed returns the amount of gas used
func (g *GasState) GetGasUsed() uint64 {
	return g.used
}

// GetGasLimit returns the gas limit
func (g *GasState) GetGasLimit() uint64 {
	return g.limit
}
