package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/go-cosmwasm/types"
)

func TestApiFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)

	// if the human address is larger than 32 bytes, this will lead to an error in the go side
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier": "` + longName + `", "beneficiary": "bob"}`)

	_, _, err = Instantiate(cache, id, params, msg, &gasMeter, store, api, &querier, 100000000)
	require.Error(t, err)
	fmt.Println(err.Error())

	// message from MockCanonicalAddress (go callback)
	expected := "human encoding too long"
	require.True(t, strings.HasSuffix(err.Error(), expected), err.Error())
}
