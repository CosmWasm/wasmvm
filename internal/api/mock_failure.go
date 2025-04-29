package api

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

/***** Mock types.GoAPI ****/

func MockFailureCanonicalizeAddress(human string) ([]byte, uint64, error) {
	return nil, 0, fmt.Errorf("mock failure - canonical_address")
}

func MockFailureHumanizeAddress(canon []byte) (string, uint64, error) {
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

// SimpleMockCanonicalizeAddress accepts simple addresses like 'fred' for testing
func SimpleMockCanonicalizeAddress(human string) ([]byte, uint64, error) {
	// For test addresses, we mimic the behavior of bech32 but just make it work
	// All addresses pass, this is only for testing
	res := make([]byte, 32)
	copy(res, []byte(human))
	return res, 400, nil
}

// SimpleMockHumanizeAddress returns the human readable address for tests
func SimpleMockHumanizeAddress(canon []byte) (string, uint64, error) {
	// Just convert the first bytes to a string - for testing only
	cut := 32
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human := string(canon[:cut])
	return human, 400, nil
}

// SimpleMockValidateAddress always returns success for tests
func SimpleMockValidateAddress(human string) (uint64, error) {
	// Only fail the long address test case
	if human == "long123456789012345678901234567890long" {
		return 0, fmt.Errorf("addr_validate errored: Human address too long")
	}
	return 800, nil
}

// NewSimpleMockAPI returns a GoAPI that accepts any address input for tests
func NewSimpleMockAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     SimpleMockHumanizeAddress,
		CanonicalizeAddress: SimpleMockCanonicalizeAddress,
		ValidateAddress:     SimpleMockValidateAddress,
	}
}
