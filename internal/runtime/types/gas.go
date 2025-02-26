package types

// GasConfig defines costs for various operations in the VM
type GasConfig struct {
	// Basic costs
	PerByte       uint64
	DatabaseRead  uint64
	DatabaseWrite uint64
	CompileCost   uint64
	GasMultiplier uint64

	// Crypto operation costs
	Secp256k1VerifyCost        uint64
	Secp256k1RecoverPubkeyCost uint64
	Ed25519VerifyCost          uint64
	Ed25519BatchVerifyCost     uint64

	// BLS12-381 operation costs
	Bls12381AggregateG1Cost OperationCost
	Bls12381AggregateG2Cost OperationCost
	Bls12381HashToG1Cost    OperationCost
	Bls12381HashToG2Cost    OperationCost
	Bls12381VerifyCost      OperationCost
}

// OperationCost defines a cost function with base and variable components
type OperationCost struct {
	Base     uint64
	Variable uint64
}

// TotalCost calculates the total cost for n operations
func (c OperationCost) TotalCost(n uint64) uint64 {
	return c.Base + c.Variable*n
}
