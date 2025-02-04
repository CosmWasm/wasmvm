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

	// Contract operations
	gasCostInstantiate = 40000 // Base cost for contract instantiation
	gasCostExecute     = 20000 // Base cost for contract execution
)

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

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() GasConfig {
	return GasConfig{
		PerByte:        gasPerByte,
		DatabaseRead:   gasCostRead,
		DatabaseWrite:  gasCostWrite,
		ExternalQuery:  gasCostQuery,
		IteratorCreate: gasCostIteratorCreate,
		IteratorNext:   gasCostIteratorNext,
		Instantiate:    gasCostInstantiate,
		Execute:        gasCostExecute,
	}
}

// GasState tracks gas usage during execution
type GasState struct {
	config GasConfig
	limit  uint64
	used   uint64
}

func (g *GasState) GasConsumed() uint64 {
	return g.GetGasUsed()
}

// NewGasState creates a new GasState with the given limit
func NewGasState(limit uint64) *GasState {
	return &GasState{
		config: DefaultGasConfig(),
		limit:  limit,
		used:   0,
	}
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) error {
	g.used += amount
	if g.used > g.limit {
		return fmt.Errorf("out of gas: used %d, limit %d - %s", g.used, g.limit, description)
	}
	return nil
}

// ConsumeMemory charges gas for memory operations
func (g *GasState) ConsumeMemory(size uint32) error {
	cost := uint64(size) * g.config.PerByte
	return g.ConsumeGas(cost, fmt.Sprintf("memory allocation: %d bytes", size))
}

// ConsumeRead charges gas for database read operations
func (g *GasState) ConsumeRead(size uint32) error {
	// Base cost plus per-byte cost
	cost := g.config.DatabaseRead + (uint64(size) * g.config.PerByte)
	return g.ConsumeGas(cost, "db read")
}

// ConsumeWrite charges gas for database write operations
func (g *GasState) ConsumeWrite(size uint32) error {
	// Base cost plus per-byte cost
	cost := g.config.DatabaseWrite + (uint64(size) * g.config.PerByte)
	return g.ConsumeGas(cost, "db write")
}

// ConsumeQuery charges gas for external query operations
func (g *GasState) ConsumeQuery() error {
	return g.ConsumeGas(g.config.ExternalQuery, "external query")
}

// ConsumeIterator charges gas for iterator operations
func (g *GasState) ConsumeIterator(create bool) error {
	var cost uint64
	var desc string
	if create {
		cost = g.config.IteratorCreate
		desc = "create iterator"
	} else {
		cost = g.config.IteratorNext
		desc = "iterator next"
	}
	return g.ConsumeGas(cost, desc)
}

// GetGasUsed returns the amount of gas used
func (g *GasState) GetGasUsed() uint64 {
	return g.used
}

// GetGasLimit returns the gas limit
func (g *GasState) GetGasLimit() uint64 {
	return g.limit
}

// GetGasRemaining returns the remaining gas
func (g *GasState) GetGasRemaining() uint64 {
	if g.used > g.limit {
		return 0
	}
	return g.limit - g.used
}

// HasGas checks if there is enough gas remaining
func (g *GasState) HasGas(required uint64) bool {
	return g.GetGasRemaining() >= required
}
