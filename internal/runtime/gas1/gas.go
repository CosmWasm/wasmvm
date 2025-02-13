package types

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/types" // SDK store types (for Gas type)
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Gas constants (CosmWasm 1.x)
const (
	// GasMultiplier is how many CosmWasm gas points equal 1 Cosmos SDK gas point.
	GasMultiplier uint64 = 140_000_000
	// InstanceCost is how much SDK gas is charged each time we load a WASM instance.
	InstanceCost uint64 = 60_000
	// InstanceCostDiscount is the discounted instance cost (unused in 1.x, so 0).
	InstanceCostDiscount uint64 = 0
	// CompileCost is how much SDK gas is charged per byte for compiling WASM code.
	CompileCost uint64 = 3
	// EventPerAttributeCost is the SDK gas cost per event attribute (count of attributes).
	EventPerAttributeCost uint64 = 10
	// EventAttributeDataCost is the SDK gas cost per byte of event attribute data (key + value).
	EventAttributeDataCost uint64 = 1
	// EventAttributeDataFreeTier is the number of bytes of event attribute data with no charge.
	EventAttributeDataFreeTier uint64 = 100
	// CustomEventCost is the SDK gas cost for each custom event emitted by the contract.
	CustomEventCost uint64 = 20
	// ContractMessageDataCost is the SDK gas cost per byte of the message sent to the contract (0 in 1.x).
	ContractMessageDataCost uint64 = 0
	// GasCostHumanAddress is the SDK gas cost to convert a canonical address to a human-readable address.
	GasCostHumanAddress uint64 = 5
	// GasCostCanonicalAddress is the SDK gas cost to convert a human address to canonical form.
	GasCostCanonicalAddress uint64 = 4
	// GasCostValidateAddress is the SDK gas cost to validate an address (humanize + canonicalize).
	GasCostValidateAddress uint64 = GasCostHumanAddress + GasCostCanonicalAddress
)

// Fractional gas cost for uncompressing WASM bytecode (0.15 gas per byte).
var defaultPerByteUncompressCost = wasmvmtypes.UFraction{
	Numerator:   15,
	Denominator: 100,
}

// DefaultPerByteUncompressCost returns the fraction of gas per byte for decompression.
func DefaultPerByteUncompressCost() wasmvmtypes.UFraction {
	return defaultPerByteUncompressCost
}

// GasRegister defines an interface for querying gas costs.
type GasRegister interface {
	// UncompressCosts returns the SDK gas cost to uncompress a WASM bytecode of given length.
	UncompressCosts(byteLength int) types.Gas
	// SetupContractCost returns the SDK gas cost to setup (instantiate/execute) a contract.
	// If discount is true, use the discounted instance cost (e.g. contract is cached in memory).
	// msgLen is the length of the execute/instantiate message bytes.
	SetupContractCost(discount bool, msgLen int) types.Gas
	// ReplyCosts returns the SDK gas cost to handle a submessage reply.
	// If discount is true, use discounted cost for the contract instance (already loaded).
	// It accounts for any events in the reply and any response data.
	ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas
	// EventCosts returns the SDK gas cost to persist events (attributes and custom events).
	EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas
	// ToWasmVMGas converts from SDK gas units to CosmWasm VM gas units.
	ToWasmVMGas(source types.Gas) uint64
	// FromWasmVMGas converts from CosmWasm VM gas units to SDK gas units.
	FromWasmVMGas(source uint64) types.Gas
}

// WasmGasRegisterConfig holds configuration for gas costs.
type WasmGasRegisterConfig struct {
	InstanceCost               types.Gas             // Base cost for loading a Wasm instance.
	InstanceCostDiscount       types.Gas             // Discounted instance cost (if contract is cached).
	CompileCost                types.Gas             // Cost per byte for compiling WASM code.
	UncompressCost             wasmvmtypes.UFraction // Fractional cost per byte for uncompressing code.
	GasMultiplier              types.Gas             // CosmWasm gas to SDK gas conversion factor.
	EventPerAttributeCost      types.Gas             // Gas per event attribute (count).
	EventAttributeDataCost     types.Gas             // Gas per byte of event attribute data.
	EventAttributeDataFreeTier uint64                // Free tier (bytes) for event attribute data.
	ContractMessageDataCost    types.Gas             // Gas per byte of contract message data.
	CustomEventCost            types.Gas             // Gas per custom event.
}

// DefaultGasRegisterConfig returns the default gas costs configuration (CosmWasm 1.x defaults).
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

// WasmGasRegister implements the GasRegister interface using a configuration.
type WasmGasRegister struct {
	c WasmGasRegisterConfig
}

// NewDefaultWasmGasRegister creates a GasRegister with default config.
func NewDefaultWasmGasRegister() WasmGasRegister {
	return NewWasmGasRegister(DefaultGasRegisterConfig())
}

// NewWasmGasRegister creates a GasRegister with a given config.
func NewWasmGasRegister(c WasmGasRegisterConfig) WasmGasRegister {
	if c.GasMultiplier == 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "GasMultiplier cannot be 0"))
	}
	return WasmGasRegister{c: c}
}

// UncompressCosts computes SDK gas cost for decompressing a WASM bytecode of length `byteLength`.
func (g WasmGasRegister) UncompressCosts(byteLength int) types.Gas {
	if byteLength < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "byteLength cannot be negative"))
	}
	// Gas = floor(byteLength * (Numerator/Denominator))
	numerator := g.c.UncompressCost.Numerator
	denom := g.c.UncompressCost.Denominator
	gasCost := uint64(byteLength) * numerator / denom
	return types.Gas(gasCost)
}

// SetupContractCost computes the SDK gas cost to set up a contract execution/instantiation.
func (g WasmGasRegister) SetupContractCost(discount bool, msgLen int) types.Gas {
	if msgLen < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "msgLen cannot be negative"))
	}
	// Base instance load cost, discounted if applicable
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	// Cost for contract message bytes
	msgDataCost := types.Gas(msgLen) * g.c.ContractMessageDataCost
	return baseCost + msgDataCost
}

// ReplyCosts computes the SDK gas cost for handling a contract submessage reply.
func (g WasmGasRegister) ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas {
	// Base cost for invoking the contract's reply handler (treat similar to an execution)
	var base types.Gas = g.c.InstanceCost
	if discount {
		base = g.c.InstanceCostDiscount
	}
	// Gas for any events emitted in the reply
	var eventGas types.Gas = 0
	if reply.Result != nil {
		// The submessage succeeded and returned events
		eventGas = g.EventCosts(reply.Result.Attributes, reply.Result.Events)
	}
	// Gas for any reply data or error message (counts as ContractMessageDataCost)
	var dataLen int
	if reply.Result != nil && reply.Result.Data != nil {
		dataLen = len(reply.Result.Data)
	} else if reply.Error != "" {
		dataLen = len(reply.Error)
	}
	msgDataGas := types.Gas(dataLen) * g.c.ContractMessageDataCost
	return base + eventGas + msgDataGas
}

// EventCosts computes the SDK gas cost for persisting event attributes and custom events from a contract.
func (g WasmGasRegister) EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas {
	// Calculate gas for attributes of the base event (with free tier applied)
	gasUsed, remainingFree := g.eventAttributeCosts(attrs, g.c.EventAttributeDataFreeTier)
	// Calculate gas for attributes of each custom event
	for _, evt := range events {
		gasForEvt, newFree := g.eventAttributeCosts(evt.Attributes, remainingFree)
		gasUsed += gasForEvt
		remainingFree = newFree
	}
	// Add cost for each custom event itself
	gasUsed += types.Gas(len(events)) * g.c.CustomEventCost
	return gasUsed
}

// eventAttributeCosts is a helper to compute gas for a slice of attributes, given a remaining free-byte tier.
func (g WasmGasRegister) eventAttributeCosts(attrs []wasmvmtypes.EventAttribute, freeTier uint64) (types.Gas, uint64) {
	if len(attrs) == 0 {
		// No attributes, no cost, freeTier remains unchanged
		return 0, freeTier
	}
	// Total length of all (key + value) for attributes
	var totalAttrBytes uint64 = 0
	for _, attr := range attrs {
		totalAttrBytes += uint64(len(attr.Key)) + uint64(len(attr.Value))
	}
	// Apply free tier: consume free bytes first
	if totalAttrBytes <= freeTier {
		// All attribute data is within free tier
		remainingFree := freeTier - totalAttrBytes
		return 0, remainingFree
	}
	// If attribute data exceeds free tier, charge gas for the portion above free tier
	chargeBytes := totalAttrBytes
	if freeTier > 0 {
		chargeBytes = totalAttrBytes - freeTier
		freeTier = 0
	}
	// Gas cost = chargeBytes * EventAttributeDataCost
	gasCost := types.Gas(chargeBytes) * g.c.EventAttributeDataCost
	return gasCost, 0
}

// ToWasmVMGas converts a Cosmos SDK gas value to the equivalent CosmWasm VM gas.
func (g WasmGasRegister) ToWasmVMGas(source types.Gas) uint64 {
	// Multiply by GasMultiplier (with overflow check)
	x := uint64(source) * uint64(g.c.GasMultiplier)
	if x < uint64(source) {
		// Overflow occurred if multiplying exceeded 64-bit range
		panic(types.ErrorOutOfGas{Descriptor: "CosmWasm gas overflow"})
	}
	return x
}

// FromWasmVMGas converts a CosmWasm VM gas amount to Cosmos SDK gas units.
func (g WasmGasRegister) FromWasmVMGas(source uint64) types.Gas {
	return types.Gas(source / uint64(g.c.GasMultiplier))
}
