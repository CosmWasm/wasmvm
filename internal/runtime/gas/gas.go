package gas

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// GasState tracks gas consumption
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

// GasConsumed implements types.GasMeter
func (g *GasState) GasConsumed() uint64 {
	return g.used
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) error {
	g.used += amount
	if g.used > g.limit {
		return fmt.Errorf("out of gas: used %d, limit %d - %s", g.used, g.limit, description)
	}
	return nil
}

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() types.GasConfig {
	return types.GasConfig{
		PerByte:                 constants.GasPerByte,
		DatabaseRead:            constants.GasCostRead,
		DatabaseWrite:           constants.GasCostWrite,
		ExternalQuery:           constants.GasCostQuery,
		IteratorCreate:          constants.GasCostIteratorCreate,
		IteratorNext:            constants.GasCostIteratorNext,
		Instantiate:             constants.GasCostInstantiate,
		Execute:                 constants.GasCostExecute,
		Bls12381AggregateG1Cost: types.GasCost{BaseCost: 1000, PerPoint: 100},
		Bls12381AggregateG2Cost: types.GasCost{BaseCost: 1000, PerPoint: 100},
	}
}
