package gas2

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Gas constants (CosmWasm 2.x)
const (
	// GasMultiplier is how many CosmWasm gas points equal 1 Cosmos SDK gas point (reduced 1000x in 2.x).
	GasMultiplier uint64 = 140_000
	// InstanceCost for loading a WASM instance (unchanged from 1.x).
	InstanceCost uint64 = 60_000
	// InstanceCostDiscount for cached instances (about 30x cheaper than full load).
	InstanceCostDiscount uint64 = 2_000
	// CompileCost per byte for compiling WASM code.
	CompileCost uint64 = 3
	// EventPerAttributeCost per event attribute (count).
	EventPerAttributeCost uint64 = 10
	// EventAttributeDataCost per byte of event attribute data.
	EventAttributeDataCost uint64 = 1
	// EventAttributeDataFreeTier bytes of attribute data with no charge.
	EventAttributeDataFreeTier uint64 = 100
	// CustomEventCost per custom event emitted.
	CustomEventCost uint64 = 20
	// ContractMessageDataCost per byte of message passed to contract (still 0 by default).
	ContractMessageDataCost uint64 = 0
	// GasCostHumanAddress to convert a canonical address to human-readable.
	GasCostHumanAddress uint64 = 5
	// GasCostCanonicalAddress to convert a human address to canonical form.
	GasCostCanonicalAddress uint64 = 4
	// GasCostValidateAddress (humanize + canonicalize).
	GasCostValidateAddress uint64 = GasCostHumanAddress + GasCostCanonicalAddress
)

var defaultPerByteUncompressCost = wasmvmtypes.UFraction{
	Numerator:   15,
	Denominator: 100,
}

// DefaultPerByteUncompressCost returns the default uncompress cost fraction.
func DefaultPerByteUncompressCost() wasmvmtypes.UFraction {
	return defaultPerByteUncompressCost
}

// GasRegister defines the gas registration interface.
type GasRegister interface {
	UncompressCosts(byteLength int) types.Gas
	SetupContractCost(discount bool, msgLen int) types.Gas
	ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas
	EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas
	ToWasmVMGas(source types.Gas) uint64
	FromWasmVMGas(source uint64) types.Gas
}

// WasmGasRegisterConfig holds configuration parameters for gas costs.
type WasmGasRegisterConfig struct {
	InstanceCost               types.Gas
	InstanceCostDiscount       types.Gas
	CompileCost                types.Gas
	UncompressCost             wasmvmtypes.UFraction
	GasMultiplier              types.Gas
	EventPerAttributeCost      types.Gas
	EventAttributeDataCost     types.Gas
	EventAttributeDataFreeTier uint64
	ContractMessageDataCost    types.Gas
	CustomEventCost            types.Gas
}

// DefaultGasRegisterConfig returns the default configuration for CosmWasm 2.x.
func DefaultGasRegisterConfig() WasmGasRegisterConfig {
	return WasmGasRegisterConfig{
		InstanceCost:               InstanceCost,
		InstanceCostDiscount:       InstanceCostDiscount,
		CompileCost:                CompileCost,
		UncompressCost:             DefaultPerByteUncompressCost(),
		GasMultiplier:              GasMultiplier,
		EventPerAttributeCost:      EventPerAttributeCost,
		EventAttributeDataCost:     EventAttributeDataCost,
		EventAttributeDataFreeTier: EventAttributeDataFreeTier,
		ContractMessageDataCost:    ContractMessageDataCost,
		CustomEventCost:            CustomEventCost,
	}
}

// WasmGasRegister implements GasRegister.
type WasmGasRegister struct {
	c WasmGasRegisterConfig
}

// NewDefaultWasmGasRegister creates a new gas register with default config.
func NewDefaultWasmGasRegister() WasmGasRegister {
	return NewWasmGasRegister(DefaultGasRegisterConfig())
}

// NewWasmGasRegister creates a new gas register with the given configuration.
func NewWasmGasRegister(c WasmGasRegisterConfig) WasmGasRegister {
	if c.GasMultiplier == 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "GasMultiplier cannot be 0"))
	}
	return WasmGasRegister{c: c}
}

// UncompressCosts returns the gas cost to uncompress a WASM bytecode of the given length.
func (g WasmGasRegister) UncompressCosts(byteLength int) types.Gas {
	if byteLength < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "byteLength cannot be negative"))
	}
	numerator := g.c.UncompressCost.Numerator
	denom := g.c.UncompressCost.Denominator
	gasCost := uint64(byteLength) * numerator / denom
	return types.Gas(gasCost)
}

// SetupContractCost returns the gas cost to set up contract execution/instantiation.
func (g WasmGasRegister) SetupContractCost(discount bool, msgLen int) types.Gas {
	if msgLen < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "msgLen cannot be negative"))
	}
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	msgDataCost := types.Gas(msgLen) * g.c.ContractMessageDataCost
	return baseCost + msgDataCost
}

// ReplyCosts returns the gas cost for handling a submessage reply.
// CosmWasm 2.x no longer includes event attributes or error messages in reply,
// so we only charge the base cost.
func (g WasmGasRegister) ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas {
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	// In v2.x, additional reply data is not charged.
	return baseCost
}

// EventCosts returns the gas cost for contract-emitted events.
// It computes the cost for a list of event attributes and events.
func (g WasmGasRegister) EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas {
	gasUsed, remainingFree := g.eventAttributeCosts(attrs, g.c.EventAttributeDataFreeTier)
	for _, evt := range events {
		// Charge for any event attributes that exist.
		gasEvt, newFree := g.eventAttributeCosts(evt.Attributes, remainingFree)
		gasUsed += gasEvt
		remainingFree = newFree
	}
	gasUsed += types.Gas(len(events)) * g.c.CustomEventCost
	return gasUsed
}

// eventAttributeCosts computes the gas cost for a set of event attributes given a free byte allowance.
func (g WasmGasRegister) eventAttributeCosts(attrs []wasmvmtypes.EventAttribute, freeTier uint64) (types.Gas, uint64) {
	if len(attrs) == 0 {
		return 0, freeTier
	}
	var totalBytes uint64 = 0
	for _, attr := range attrs {
		totalBytes += uint64(len(attr.Key)) + uint64(len(attr.Value))
	}
	if totalBytes <= freeTier {
		remainingFree := freeTier - totalBytes
		return 0, remainingFree
	}
	chargeBytes := totalBytes - freeTier
	gasCost := types.Gas(chargeBytes) * g.c.EventAttributeDataCost
	return gasCost, 0
}

// ToWasmVMGas converts SDK gas to CosmWasm VM gas.
func (g WasmGasRegister) ToWasmVMGas(source types.Gas) uint64 {
	x := uint64(source) * uint64(g.c.GasMultiplier)
	if x < uint64(source) {
		panic(wasmvmtypes.ErrorOutOfGas{Descriptor: "CosmWasm gas overflow"})
	}
	return x
}

// FromWasmVMGas converts CosmWasm VM gas to SDK gas.
func (g WasmGasRegister) FromWasmVMGas(source uint64) types.Gas {
	return types.Gas(source / uint64(g.c.GasMultiplier))
}
