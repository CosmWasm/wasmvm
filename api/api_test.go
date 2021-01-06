package api

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/types"
)

func TestCanonicalAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	// if the human address is larger than 32 bytes, this will lead to an error in the go side
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier": "` + longName + `", "beneficiary": "bob"}`)

	// make sure the call doesn't error, but we get a JSON-encoded error result from InitResult
	igasMeter := GasMeter(gasMeter)
	res, _, err := Instantiate(cache, id, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var resp types.InitResult
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)

	// ensure the error message is what we expect
	require.Nil(t, resp.Ok)
	// with this error
	require.Equal(t, "Generic error: canonicalize_address errored: human encoding too long", resp.Err)
}

func TestHumanAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	// instantiate it normally
	msg := []byte(`{"verifier": "short", "beneficiary": "bob"}`)
	igasMeter := GasMeter(gasMeter)
	_, _, err = Instantiate(cache, id, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// call query which will call canonicalize address
	badApi := NewMockFailureAPI()
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	query := []byte(`{"verifier":{}}`)
	igasMeter3 := GasMeter(gasMeter3)
	res, _, err := Query(cache, id, env, query, &igasMeter3, store, badApi, &querier, TESTING_GAS_LIMIT, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var resp types.QueryResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)

	// ensure the error message is what we expect (system -ok, stderr -generic)
	require.Nil(t, resp.Ok)
	// with this error
	require.Equal(t, "Generic error: humanize_address errored: mock failure - human_address", resp.Err)
}
