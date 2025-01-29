package types

type Gas = uint64

type GasMeter interface {
	GasConsumed() Gas
}
