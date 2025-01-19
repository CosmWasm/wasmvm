package runtime

var requiredHostFunctions = map[string]bool{
	// Memory management
	"allocate":   true,
	"deallocate": true,

	// Storage functions
	"db_read":   true,
	"db_write":  true,
	"db_remove": true,
	"db_scan":   true,
	"db_next":   true,

	// Address functions
	"addr_validate":     true,
	"addr_canonicalize": true,
	"addr_humanize":     true,

	// Crypto functions
	"secp256k1_verify":         true,
	"secp256k1_recover_pubkey": true,
	"ed25519_verify":           true,
	"ed25519_batch_verify":     true,

	// Debug/Logging
	"debug": true,

	// Iterator functions
	"db_next_key":   true,
	"db_next_value": true,
}
