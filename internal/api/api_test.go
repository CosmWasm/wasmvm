package api

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// prettyPrint returns a properly formatted string representation of a struct
func prettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%#v", v)
	}
	return string(b)
}

func TestValidateAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	// if the human address is larger than 32 bytes, this will lead to an error in the go side
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier": "` + longName + `", "beneficiary": "bob"}`)

	// make sure the call doesn't error, but we get a JSON-encoded error result from ContractResult
	igasMeter := types.GasMeter(gasMeter)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)

	// DEBUG: print all calls with proper formatting and deserialization
	fmt.Printf("\n=== Debug Information ===\n")
	fmt.Printf("Cache:     %#v\n", cache)
	fmt.Printf("Checksum:  %x\n", checksum)

	// Deserialize env
	var envObj types.Env
	_ = json.Unmarshal(env, &envObj)
	fmt.Printf("Env:       %s\n", prettyPrint(envObj))

	// Deserialize info
	var infoObj types.MessageInfo
	_ = json.Unmarshal(info, &infoObj)
	fmt.Printf("Info:      %s\n", prettyPrint(infoObj))

	// Deserialize msg
	var msgObj map[string]interface{}
	_ = json.Unmarshal(msg, &msgObj)
	fmt.Printf("Msg:       %s\n", prettyPrint(msgObj))

	fmt.Printf("Gas Meter: %#v\n", igasMeter)
	fmt.Printf("Store:     %#v\n", store)
	fmt.Printf("API:       %#v\n", api)
	fmt.Printf("Querier:   %s\n", prettyPrint(querier))
	fmt.Printf("======================\n\n")

	require.NoError(t, err)
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)

	// ensure the error message is what we expect
	require.Nil(t, result.Ok)
	// with this error
	require.Equal(t, "Generic error: addr_validate errored: human encoding too long", result.Err)
}
