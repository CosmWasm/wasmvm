package api

import (
	"errors"

	"github.com/CosmWasm/wasmvm/v2/types"
)

/* **** Mock types.GoAPI *****/

// MockFailureCanonicalizeAddress returns a generic error
func MockFailureCanonicalizeAddress(_ string) (canonical []byte, gasCost uint64, err error) {
	return nil, 0, errors.New("mock failure - canonicalize address")
}

// MockFailureHumanizeAddress returns a generic error
func MockFailureHumanizeAddress(_ []byte) (human string, gasCost uint64, err error) {
	return "", 0, errors.New("mock failure - humanize address")
}

// MockFailureValidateAddress returns a generic error
func MockFailureValidateAddress(_ string) (uint64, error) {
	return 0, errors.New("mock failure - validate address")
}

// NewMockFailureAPI creates a new mock API that fails
func NewMockFailureAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockFailureHumanizeAddress,
		CanonicalizeAddress: MockFailureCanonicalizeAddress,
		ValidateAddress:     MockFailureValidateAddress,
	}
}
