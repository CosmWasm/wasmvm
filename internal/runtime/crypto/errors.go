package crypto

import "errors"

var (
	// ErrInvalidHashFormat is returned when a hash has an invalid format
	ErrInvalidHashFormat = errors.New("invalid hash format")
	// ErrInvalidSignatureFormat is returned when a signature has an invalid format
	ErrInvalidSignatureFormat = errors.New("invalid signature format")
	// ErrInvalidPubkeyFormat is returned when a public key has an invalid format
	ErrInvalidPubkeyFormat = errors.New("invalid public key format")
	// ErrInvalidBatchFormat is returned when batch verification inputs have mismatched lengths
	ErrInvalidBatchFormat = errors.New("invalid batch format: mismatched input lengths")
)
