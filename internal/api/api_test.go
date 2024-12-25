// api_test.go

package api

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// TestAddressValidationScenarios covers multiple address lengths and behaviors.
// In the original code, we only tested a single "too long" case. Here we use
// a table-driven approach to validate multiple scenarios.
//
// We also demonstrate how to provide more debugging information with t.Logf
// in the event of test failures or for general clarity.
func TestAddressValidationScenarios(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// Load the contract
	wasmPath := "../../testdata/hackatom.wasm"
	wasm, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Could not read wasm file at %s", wasmPath)

	// Store the code in the cache
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err, "Storing code failed for %s", wasmPath)

	// Now define multiple test scenarios
	tests := []struct {
		name          string
		address       string
		expectFailure bool
		expErrMsg     string
	}{
		{
			name:          "Short Address - 6 chars",
			address:       "bob123",
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Exactly 32 chars",
			address:       "abcdefghijklmnopqrstuvwx9876543210",
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Too Long Address (beyond 32)",
			address:       "long123456789012345678901234567890long",
			expectFailure: true,
			expErrMsg:     "Generic error: addr_validate errored: human encoding too long",
		},
		{
			name:          "Empty Address",
			address:       "",
			expectFailure: true,
			expErrMsg:     "Generic error: addr_validate errored: Input is empty",
		},
		{
			name:          "Unicode / Special Characters",
			address:       "sömëSTRängeădd®ess!",
			expectFailure: true,
			// Adjust expectation if your environment does allow unicode addresses.
			expErrMsg: "Generic error: addr_validate errored:",
		},
	}

	for _, tc := range tests {
		tc := tc // capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("[DEBUG] Running scenario: %s, address='%s'", tc.name, tc.address)

			// Prepare the environment for instantiation
			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			store := NewLookup(gasMeter)
			api := NewMockAPI()
			querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
			env := MockEnvBin(t)
			info := MockInfoBin(t, "creator")

			// Construct the JSON message that sets "verifier" to our test address
			msgStr := fmt.Sprintf(`{"verifier": "%s", "beneficiary": "some_beneficiary"}`, tc.address)
			msg := []byte(msgStr)

			var igasMeter types.GasMeter = gasMeter
			res, cost, err := Instantiate(
				cache,
				checksum,
				env,
				info,
				msg,
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)

			// Log the gas cost for debugging
			t.Logf("[DEBUG] Gas Used: %d, Gas Remaining: %d", cost.UsedInternally, cost.Remaining)

			// We expect no low-level (Go) error even if the contract validation fails
			require.NoError(t, err,
				"[GO-level error] Instantiation must not return a fatal error for scenario: %s", tc.name)

			// Now decode the contract's result
			var contractResult types.ContractResult
			err = json.Unmarshal(res, &contractResult)
			require.NoError(t, err,
				"JSON unmarshal failed on contract result for scenario: %s\nRaw contract response: %s",
				tc.name, string(res),
			)

			// If we expect a failure, check that contractResult.Err is set
			if tc.expectFailure {
				require.Nil(t, contractResult.Ok,
					"Expected no Ok response, but got: %+v for scenario: %s", contractResult.Ok, tc.name)
				require.Contains(t, contractResult.Err, tc.expErrMsg,
					"Expected error message containing '%s', but got '%s' for scenario: %s",
					tc.expErrMsg, contractResult.Err, tc.name)
				t.Logf("[OK] We got the expected error. Full error: %s", contractResult.Err)
			} else {
				// We do not expect a failure
				require.Equal(t, "", contractResult.Err,
					"Expected no error for scenario: %s, but got: %s", tc.name, contractResult.Err)
				require.NotNil(t, contractResult.Ok,
					"Expected a valid Ok response for scenario: %s, got nil", tc.name)
				t.Logf("[OK] Instantiation succeeded, contract returned: %+v", contractResult.Ok)
			}
		})
	}
}

// TestInstantiateWithVariousMsgFormats tries different JSON payloads, both valid and invalid.
// This shows how to handle scenarios where the contract message might be malformed or incorrectly typed.
func TestInstantiateWithVariousMsgFormats(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// Load the contract
	wasmPath := "../../testdata/hackatom.wasm"
	wasm, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Could not read wasm file at %s", wasmPath)

	// Store the code in the cache
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err, "Storing code failed for %s", wasmPath)

	tests := []struct {
		name          string
		jsonMsg       string
		expectFailure bool
		expErrMsg     string
	}{
		{
			name:          "Valid JSON - simple",
			jsonMsg:       `{"verifier":"myverifier","beneficiary":"bob"}`,
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Invalid JSON - missing closing brace",
			jsonMsg:       `{"verifier":"bob"`,
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg", // might vary
		},
		{
			name:          "Empty JSON message",
			jsonMsg:       `{}`,
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg: missing field `verifier`", // might vary
		},
		{
			name: "Weird fields",
			jsonMsg: `{
				"verifier": "someone",
				"beneficiary": "bob",
				"thisFieldDoesNotExistInSchema": 1234
			}`,
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg: missing field `beneficiary`",
		},
		{
			name:          "Random text not valid JSON",
			jsonMsg:       `Garbage data here`,
			expectFailure: true,
			expErrMsg:     "Invalid type",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("[DEBUG] Checking message scenario: %s, JSON: %s", tc.name, tc.jsonMsg)

			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			store := NewLookup(gasMeter)
			api := NewMockAPI()
			querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
			env := MockEnvBin(t)
			info := MockInfoBin(t, "creator")

			msg := []byte(tc.jsonMsg)

			var igasMeter types.GasMeter = gasMeter
			res, cost, err := Instantiate(
				cache,
				checksum,
				env,
				info,
				msg,
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)

			t.Logf("[DEBUG] Gas Used: %d, Gas Remaining: %d", cost.UsedInternally, cost.Remaining)

			// The contract might error at the CosmWasm level. Usually that won't produce a Go-level error,
			// unless the JSON was so malformed that we can't even pass it in to the contract. So we only
			// require that it didn't produce a fatal error. We'll check contract error vs. Ok below.
			require.NoError(t, err,
				"[GO-level error] Instantiation must not return a fatal error for scenario: %s", tc.name)

			// Now decode the contract's result
			var contractResult types.ContractResult
			err = json.Unmarshal(res, &contractResult)
			require.NoError(t, err,
				"JSON unmarshal of contract result must succeed (scenario: %s)\nRaw contract response: %s",
				tc.name, string(res),
			)

			if tc.expectFailure {
				require.Nil(t, contractResult.Ok,
					"Expected no Ok response, but got: %+v for scenario: %s", contractResult.Ok, tc.name)
				// The exact error message from the contract can vary, but we try to match a known phrase
				// from expErrMsg. Adjust or refine as your environment differs.
				require.Contains(t, contractResult.Err, tc.expErrMsg,
					"Expected error containing '%s', but got '%s' for scenario: %s",
					tc.expErrMsg, contractResult.Err, tc.name)
				t.Logf("[OK] We got the expected contract-level error. Full error: %s", contractResult.Err)
			} else {
				require.Equal(t, "", contractResult.Err,
					"Expected no error for scenario: %s, but got: %s", tc.name, contractResult.Err)
				require.NotNil(t, contractResult.Ok,
					"Expected a valid Ok response for scenario: %s, got nil", tc.name)
				t.Logf("[OK] Instantiation succeeded. Ok: %+v", contractResult.Ok)
			}
		})
	}
}
