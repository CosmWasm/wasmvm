package api

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/go-cosmwasm/types"
)

func TestCanonicalAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv("creator"))
	require.NoError(t, err)

	// if the human address is larger than 32 bytes, this will lead to an error in the go side
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier": "` + longName + `", "beneficiary": "bob"}`)

	_, _, err = Instantiate(cache, id, params, msg, &gasMeter, store, api, &querier, 100000000)
	require.Error(t, err)

	// message from MockCanonicalAddress (go callback)
	expected := "human encoding too long"
	require.True(t, strings.Contains(err.Error(), expected), err.Error())
}

func TestHumanAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv("creator"))
	require.NoError(t, err)

	// instantiate it normally
	msg := []byte(`{"verifier": "short", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, id, params, msg, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)

	// call query which will call canonicalize address
	badApi := NewMockFailureAPI()
	gasMeter3 := NewMockGasMeter(100000000)
	query := []byte(`{"verifier":{}}`)
	_, _, err = Query(cache, id, query, &gasMeter3, store, badApi, &querier, 100000000)
	require.Error(t, err)

	// message from MockFailureHumanAddresss (go callback)
	expected := "mock failure - human_address"
	require.True(t, strings.Contains(err.Error(), expected), err.Error())
}
