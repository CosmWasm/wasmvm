package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateAddressFormats tests the MockValidateAddress implementation with different address formats.
func TestValidateAddressFormats(t *testing.T) {
	// Test valid Bech32 addresses
	validBech32Addresses := []string{
		"cosmos1q9f0qwgmwvyg0pyp38g4lw2cznugwz8pc9qd3l", // Uses proper Bech32 charset
		"osmo1m8pqpkly9nz3r30f0wp09h57mqkhsr9373pj9m",   // Uses proper Bech32 charset
		"juno1pxc48gd3cydz847wgvt0p23zlc5wf88pjdptnt",   // Uses proper Bech32 charset
	}

	for _, addr := range validBech32Addresses {
		_, err := MockValidateAddress(addr)
		require.NoError(t, err, "Valid Bech32 address should pass validation: %s", addr)
	}

	// Test valid Ethereum addresses
	validEthAddresses := []string{
		"0x1234567890123456789012345678901234567890",
		"0xabcdef1234567890abcdef1234567890abcdef12",
		"0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
	}

	for _, addr := range validEthAddresses {
		_, err := MockValidateAddress(addr)
		require.NoError(t, err, "Valid Ethereum address should pass validation: %s", addr)
	}

	// Test legacy addresses with hyphens or underscores
	legacyAddresses := []string{
		"contract-address",
		"reflect_acct_1",
	}

	for _, addr := range legacyAddresses {
		_, err := MockValidateAddress(addr)
		require.NoError(t, err, "Legacy test address should pass validation: %s", addr)
	}

	// Test invalid addresses
	invalidAddresses := []string{
		"",               // Empty string
		"cosmos",         // No data part
		"0x1234",         // Too short for Ethereum
		"cosmos@invalid", // Invalid character
		"0xXYZinvalidhex1234567890123456789012345678", // Invalid hex in Ethereum address
	}

	for _, addr := range invalidAddresses {
		_, err := MockValidateAddress(addr)
		require.Error(t, err, "Invalid address should fail validation: %s", addr)
	}
}
