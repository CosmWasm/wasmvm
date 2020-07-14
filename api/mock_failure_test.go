package api

import "fmt"

/***** Mock GoAPI ****/

func MockFailureCanonicalAddress(human string) ([]byte, error) {
	return nil, fmt.Errorf("mock failure - canonical_address")
}

func MockFailureHumanAddress(canon []byte) (string, error) {
	return "", fmt.Errorf("mock failure - human_address")
}

func NewMockFailureAPI() *GoAPI {
	return &GoAPI{
		HumanAddress:     MockFailureHumanAddress,
		CanonicalAddress: MockFailureCanonicalAddress,
	}
}
