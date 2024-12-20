package api

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

func TestValidateAddressFailure(t *testing.T) {
	// Set up cache and ensure cleanup after test
	cache, cleanup := withCache(t)
	t.Cleanup(cleanup)

	// Create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(testingGasLimit)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	// If the human address is larger than 32 bytes, it triggers an error in address validation
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier":"` + longName + `","beneficiary":"bob"}`)

	igasMeter := types.GasMeter(gasMeter)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)

	// The contract call succeeds at the VM level but returns a JSON error inside ContractResult
	require.Nil(t, result.Ok)
	require.Equal(t, "Generic error: addr_validate errored: human encoding too long", result.Err)
}
