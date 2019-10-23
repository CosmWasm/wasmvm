package api

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/confio/go-cosmwasm/types"
)

type Lookup struct {
	data map[string]string
}

func NewLookup() *Lookup {
	return &Lookup{data: make(map[string]string)}
}

func (l *Lookup) Get(key []byte) []byte {
	val := l.data[string(key)]
	return []byte(val)
}

func (l *Lookup) Set(key, value []byte) {
	l.data[string(key)] = string(value)
}

var _ KVStore = (*Lookup)(nil)

func TestInitAndReleaseCache(t *testing.T) {
	dataDir := "/foo"
	_, err := InitCache(dataDir, 3)
	require.Error(t, err)

	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	t.Log(tmpdir)
	defer os.RemoveAll(tmpdir)

	cache, err := InitCache(tmpdir, 3)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func withCache(t *testing.T) (Cache, func()) {
	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	cache, err := InitCache(tmpdir, 3)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	return cache, cleanup
}

func TestCreateAndGet(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	id, err := Create(cache, wasm)
	require.NoError(t, err)

	code, err := GetCode(cache, id)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestCreateFailsWithBadData(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm := []byte("some invalid data")
	_, err := Create(cache, wasm)
	require.Error(t, err)
}

func mockParams(signer string) types.Params {
	return types.Params{
		Block: types.BlockInfo{},
		Message: types.MessageInfo{
			Signer: signer,
			SentFunds: []types.Coin{{
				Denom: "ATOM",
				Amount: "100",
			}},
		},
		Contract: types.ContractInfo{
			Address: "contract",
			Balance: []types.Coin{{
				Denom: "ATOM",
				Amount: "100",
			}},
		},
	}
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	// instantiate it with this store
	store := NewLookup()
	params, err := json.Marshal(mockParams("creator"))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, err := Instantiate(cache, id, params, msg, store, 100000000)
	require.NoError(t, err)
	require.Equal(t, `{"ok":{"messages":[],"log":null,"data":null}}`, string(res))

	var resp types.CosmosResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 0, len(resp.Ok.Messages))
}

func TestHandle(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	// instantiate it with this store
	store := NewLookup()
	params, err := json.Marshal(mockParams("creator"))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	_, err = Instantiate(cache, id, params, msg, store, 100000000)
	require.NoError(t, err)

	// execute with the same store
	params, err = json.Marshal(mockParams("fred"))
	require.NoError(t, err)
	res, err := Handle(cache, id, params, []byte(`{}`), store, 100000000)
	require.NoError(t, err)

	var resp types.CosmosResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
}

func TestQueryFails(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	id := []byte("foo")
	path := []byte("/some/stuff")
	data := []byte("{}")
	db := NewLookup()

	_, err := Query(cache, id, path, data, db, 100000000)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
}
