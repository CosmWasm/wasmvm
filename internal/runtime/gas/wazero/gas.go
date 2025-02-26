package wazerogasometer

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
)

// WazeroGasMeter implements both wazero's api.GasMeter and CosmWasm's types.GasMeter
type WazeroGasMeter struct {
	limit     uint64
	consumed  uint64
	gasConfig types.GasConfig
}

// NewWazeroGasMeter creates a new gas meter compatible with both systems
func NewWazeroGasMeter(limit uint64, config types.GasConfig) *WazeroGasMeter {
	return &WazeroGasMeter{
		limit:     limit,
		consumed:  0,
		gasConfig: config,
	}
}

// WithGasMeter attaches the gas meter to the context
func (g *WazeroGasMeter) WithGasMeter(ctx context.Context) context.Context {
	return context.WithValue(ctx, gasMeterKey{}, g)
}

// gasMeterKey is a private type for the context key to avoid collisions
type gasMeterKey struct{}

// Implements wazero api.GasMeter
func (g *WazeroGasMeter) Gas(uint64) error {
	return nil // Always allow gas during compilation
}

// ConsumeFuel implements wazero's gas consumption during execution
func (g *WazeroGasMeter) ConsumeFuel(fuel uint64) error {
	// Convert wazero fuel units to CosmWasm gas units
	gasToCharge := fuel * g.gasConfig.GasMultiplier
	return g.ConsumeGas(gasToCharge, "wazero operation")
}

// Implements types.GasMeter
func (g *WazeroGasMeter) GasConsumed() uint64 {
	return g.consumed
}

// ConsumeGas implements types.GasMeter
func (g *WazeroGasMeter) ConsumeGas(amount uint64, descriptor string) error {
	if g.consumed+amount > g.limit {
		return types.OutOfGasError{Descriptor: descriptor}
	}
	g.consumed += amount
	return nil
}

// GasRemaining returns remaining gas
func (g *WazeroGasMeter) GasRemaining() uint64 {
	if g.consumed >= g.limit {
		return 0
	}
	return g.limit - g.consumed
}

// HasGas checks if there is enough gas remaining
func (g *WazeroGasMeter) HasGas(required uint64) bool {
	return g.GasRemaining() >= required
}

// GasForOperation calculates gas needed for a specific operation
func (g *WazeroGasMeter) GasForOperation(op types.GasOperation) uint64 {
	switch op {
	case types.GasOperationMemoryRead:
		return g.gasConfig.PerByte
	case types.GasOperationMemoryWrite:
		return g.gasConfig.PerByte
	case types.GasOperationDBRead:
		return g.gasConfig.DatabaseRead
	case types.GasOperationDBWrite:
		return g.gasConfig.DatabaseWrite
	case types.GasOperationDBDelete:
		return g.gasConfig.DatabaseWrite
	case types.GasOperationCompile:
		return g.gasConfig.CompileCost
	// Add other operations as needed
	default:
		return 0
	}
}

// GetGasMeterFromContext retrieves the gas meter from context
func GetGasMeterFromContext(ctx context.Context) (*WazeroGasMeter, bool) {
	meter, ok := ctx.Value(gasMeterKey{}).(*WazeroGasMeter)
	return meter, ok
}

// ConsumeGasFromContext consumes gas from the meter in context
func ConsumeGasFromContext(ctx context.Context, amount uint64, description string) error {
	meter, ok := GetGasMeterFromContext(ctx)
	if !ok {
		return fmt.Errorf("gas meter not found in context")
	}
	return meter.ConsumeGas(amount, description)
}
