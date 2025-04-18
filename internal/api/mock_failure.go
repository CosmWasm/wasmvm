package api

import (
	"errors"

	"github.com/CosmWasm/wasmvm/v2/types"
)

/***** Mock types.GoAPI ****/

// MockFailureCanonicalizeAddress mocks address canonicalization with failure
func MockFailureCanonicalizeAddress(addr string) (canonical []byte, gasCost uint64, err error) {
	return nil, 0, errors.New("mock failure - canonical_address")
}

// MockFailureHumanizeAddress mocks address humanization with failure
func MockFailureHumanizeAddress(addr []byte) (human string, gasCost uint64, err error) {
	return "", 0, errors.New("mock failure - human_address")
}

// MockFailureValidateAddress mocks address validation with failure
func MockFailureValidateAddress(addr string) (uint64, error) {
	return 0, errors.New("mock failure - validate_address")
}

// NewMockFailureAPI creates a new mock API that fails
func NewMockFailureAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockFailureHumanizeAddress,
		CanonicalizeAddress: MockFailureCanonicalizeAddress,
		ValidateAddress:     MockFailureValidateAddress,
	}
}
