package gas1

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// --- ErrorOutOfGas ---
//
// ErrorOutOfGas is returned when the gas consumption exceeds the allowed limit.
type ErrorOutOfGas struct {
	Descriptor string
}

func (e ErrorOutOfGas) Error() string {
	return fmt.Sprintf("out of gas: %s", e.Descriptor)
}

// --- Constants ---
const (
	// Cost of one Wasm VM instruction (CosmWasm 1.x uses 150 gas per op).
	wasmInstructionCost uint64 = 150

	// Conversion multiplier: CosmWasm gas units are 100x the Cosmos SDK gas units.
	gasMultiplier uint64 = 100

	// Cost per byte for memory copy operations (host ↔ wasm).
	memoryCopyCost uint64 = 1
)

// --- GasState ---
// GasState tracks gas usage during a contract execution (CosmWasm 1.x compatible).
type GasState struct {
	gasLimit      uint64         // Total gas limit (in CosmWasm gas units)
	usedInternal  uint64         // Gas used for internal Wasm operations (in CosmWasm gas units)
	externalUsed  uint64         // Gas used externally (from the Cosmos SDK GasMeter, in SDK gas units)
	initialExtern uint64         // Initial external gas consumed at start (SDK units)
	gasMeter      types.GasMeter // Reference to an external (SDK) GasMeter
}

// NewGasState creates a new GasState.
// The given gas limit is in Cosmos SDK gas units; it is converted to CosmWasm gas units.
// The provided gasMeter is used to track external gas usage.
func NewGasState(limitSDK uint64, meter types.GasMeter) *GasState {
	gs := &GasState{
		gasLimit:     limitSDK * gasMultiplier,
		usedInternal: 0,
		externalUsed: 0,
		gasMeter:     meter,
	}
	if meter != nil {
		gs.initialExtern = meter.GasConsumed()
	}
	return gs
}

// ConsumeWasmGas consumes gas for executing the given number of Wasm instructions.
func (gs *GasState) ConsumeWasmGas(numInstr uint64) error {
	if numInstr == 0 {
		return nil
	}
	cost := numInstr * wasmInstructionCost
	return gs.consumeInternalGas(cost, "Wasm execution")
}

// ConsumeMemoryGas charges gas for copying numBytes of data.
func (gs *GasState) ConsumeMemoryGas(numBytes uint64) error {
	if numBytes == 0 {
		return nil
	}
	cost := numBytes * memoryCopyCost
	return gs.consumeInternalGas(cost, "Memory operation")
}

// ConsumeDBReadGas charges gas for a database read, based on key and value sizes.
func (gs *GasState) ConsumeDBReadGas(keyLen, valueLen int) error {
	totalBytes := uint64(0)
	if keyLen > 0 {
		totalBytes += uint64(keyLen)
	}
	if valueLen > 0 {
		totalBytes += uint64(valueLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "DB read")
}

// ConsumeDBWriteGas charges gas for a database write, based on key and value sizes.
func (gs *GasState) ConsumeDBWriteGas(keyLen, valueLen int) error {
	totalBytes := uint64(0)
	if keyLen > 0 {
		totalBytes += uint64(keyLen)
	}
	if valueLen > 0 {
		totalBytes += uint64(valueLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "DB write")
}

// ConsumeQueryGas charges gas for an external query operation.
func (gs *GasState) ConsumeQueryGas(reqLen, respLen int) error {
	totalBytes := uint64(0)
	if reqLen > 0 {
		totalBytes += uint64(reqLen)
	}
	if respLen > 0 {
		totalBytes += uint64(respLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "External query")
}

// consumeInternalGas deducts the given cost from internal gas usage and checks combined gas.
func (gs *GasState) consumeInternalGas(cost uint64, descriptor string) error {
	if cost == 0 {
		return nil
	}
	gs.usedInternal += cost

	// Update external usage from the Cosmos SDK GasMeter.
	if gs.gasMeter != nil {
		currentExtern := gs.gasMeter.GasConsumed()
		if currentExtern < gs.initialExtern {
			gs.initialExtern = currentExtern
		}
		gs.externalUsed = currentExtern - gs.initialExtern
	}

	combinedUsed := gs.usedInternal + (gs.externalUsed * gasMultiplier)
	if combinedUsed > gs.gasLimit {
		return ErrorOutOfGas{Descriptor: descriptor}
	}
	return nil
}

// GasUsed returns the internal gas used in Cosmos SDK gas units.
func (gs *GasState) GasUsed() uint64 {
	used := gs.usedInternal / gasMultiplier
	if gs.usedInternal%gasMultiplier != 0 {
		used++
	}
	return used
}

// Report returns a GasReport summarizing gas usage.
func (gs *GasState) Report() types.GasReport {
	if gs.gasMeter != nil {
		currentExtern := gs.gasMeter.GasConsumed()
		if currentExtern < gs.initialExtern {
			gs.initialExtern = currentExtern
		}
		gs.externalUsed = currentExtern - gs.initialExtern
	}
	usedExternWasm := gs.externalUsed * gasMultiplier
	usedInternWasm := gs.usedInternal
	var remaining uint64
	if gs.gasLimit >= (usedInternWasm + usedExternWasm) {
		remaining = gs.gasLimit - (usedInternWasm + usedExternWasm)
	}
	return types.GasReport{
		Limit:          gs.gasLimit,
		Remaining:      remaining,
		UsedExternally: usedExternWasm,
		UsedInternally: usedInternWasm,
	}
}

// DebugString returns a human-readable summary of the current gas state.
func (gs *GasState) DebugString() string {
	report := gs.Report()
	usedExternSDK := gs.externalUsed
	usedInternSDK := gs.GasUsed()
	totalSDK := usedExternSDK + usedInternSDK
	return fmt.Sprintf(
		"GasState{limit=%d, usedIntern=%d, usedExtern=%d, combined=%d | SDK gas: internal=%d, external=%d, total=%d}",
		report.Limit, report.UsedInternally, report.UsedExternally, report.UsedInternally+report.UsedExternally,
		usedInternSDK, usedExternSDK, totalSDK,
	)
}
