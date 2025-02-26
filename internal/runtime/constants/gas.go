package constants

const (
	// Gas multiplier for wazero operations
	GasMultiplier uint64 = 100

	// Maximum sizes for BLS operations
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)
