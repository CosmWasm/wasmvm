package gas

import (
	rterrors "github.com/CosmWasm/wasmvm/v2/internal/runtime/error"
)

// Meter tracks gas consumption during contract execution
type Meter interface {
	// Consume charges the specified amount of gas
	Consume(amount uint64) error
	// Remaining returns the amount of gas left
	Remaining() uint64
	// HasGas checks if there is any gas left
	HasGas() bool
}

// DefaultMeter is the default implementation of Meter
type DefaultMeter struct {
	limit    uint64
	consumed uint64
}

// NewDefaultMeter creates a new gas meter with the specified limit
func NewDefaultMeter(limit uint64) *DefaultMeter {
	return &DefaultMeter{
		limit:    limit,
		consumed: 0,
	}
}

func (m *DefaultMeter) Consume(amount uint64) error {
	if m.consumed+amount > m.limit {
		return &rterrors.GasError{
			Wanted:    amount,
			Available: m.Remaining(),
		}
	}
	m.consumed += amount
	return nil
}

func (m *DefaultMeter) Remaining() uint64 {
	if m.consumed >= m.limit {
		return 0
	}
	return m.limit - m.consumed
}

func (m *DefaultMeter) HasGas() bool {
	return m.Remaining() > 0
}

// Report contains information about gas usage
type Report struct {
	Limit     uint64
	Remaining uint64
	Used      uint64
}
