package constants

const (

	// Point lengths for BLS12-381
	BLS12_381_G1_POINT_LEN = 48
	BLS12_381_G2_POINT_LEN = 96
	WasmPageSize           = 65536
)

// Gas costs for various operations
const (
	// Memory operations
	GasPerByte = 3

	// Database operations
	GasCostRead  = 100
	GasCostWrite = 200
	GasCostQuery = 500

	// Iterator operations
	GasCostIteratorCreate = 10000 // Base cost for creating an iterator
	GasCostIteratorNext   = 1000  // Base cost for iterator next operations

	// Contract operations
	GasCostInstantiate = 40000 // Base cost for contract instantiation
	GasCostExecute     = 20000 // Base cost for contract execution
)
