// Package types provides core types used throughout the wasmvm package.
package types

// Gas represents the amount of computational resources consumed during execution.
type Gas = uint64

// GasMeter is a read-only version of the sdk gas meter
// It is a copy of an interface declaration from cosmos-sdk
// https://github.com/cosmos/cosmos-sdk/blob/18890a225b46260a9adc587be6fa1cc2aff101cd/store/types/gas.go#L34
type GasMeter interface {
	GasConsumed() Gas
}
