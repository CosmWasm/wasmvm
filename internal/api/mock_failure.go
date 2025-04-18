package api

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

/***** Mock types.GoAPI ****/

func MockFailureCanonicalizeAddress(human string) (canonical []byte, gasCost uint64, err error) {
	return nil, 0, fmt.Errorf("mock failure - canonical_address")
}

func MockFailureHumanizeAddress(canon []byte) (human string, gasCost uint64, err error) {
	return "", 0, fmt.Errorf("mock failure - human_address")
}

func MockFailureValidateAddress(human string) (uint64, error) {
	return 0, fmt.Errorf("mock failure - validate_address")
}

func NewMockFailureAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockFailureHumanizeAddress,
		CanonicalizeAddress: MockFailureCanonicalizeAddress,
		ValidateAddress:     MockFailureValidateAddress,
	}
}
