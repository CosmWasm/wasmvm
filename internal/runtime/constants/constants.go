package constants

const (

	// Point lengths for BLS12-381
	BLS12_381_G1_POINT_LEN = 48
	BLS12_381_G2_POINT_LEN = 96
	wasmPageSize           = 65536
)

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
