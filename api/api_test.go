package api

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestDemoDBAccess(t *testing.T) {
	l := NewLookup()
	foo := []byte("foo")
	bar := []byte("bar")
	missing := []byte("missing")
	l.Set(foo, []byte("long text that fills the buffer"))
	l.Set(bar, []byte("short"))

	// long
	err := UpdateDB(l, foo)
	require.NoError(t, err)
	require.Equal(t, "long text that fills the buffer.", string(l.Get(foo)))

	// short
	err = UpdateDB(l, bar)
	require.NoError(t, err)
	err = UpdateDB(l, bar)
	require.NoError(t, err)
	err = UpdateDB(l, bar)
	require.NoError(t, err)
	require.Equal(t, "short...", string(l.Get(bar)))

	// missing
	err = UpdateDB(l, missing)
	require.NoError(t, err)
	require.Equal(t, ".", string(l.Get(missing)))

	err = UpdateDB(l, nil)
	require.Error(t, err)
}

func TestInitAndReleaseCache(t *testing.T) {
	dataDir := "/foo"
	_, err := InitCache(dataDir)
	require.Error(t, err)

	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	t.Log(tmpdir)
// 	defer os.RemoveAll(tmpdir)

	_, err = InitCache(tmpdir)
	require.NoError(t, err)
// 	ReleaseCache(cache)
}

func withCache(t *testing.T) (Cache, func()) {
	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	cache, err := InitCache(tmpdir)
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


func TestInstantiateFails(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	id := []byte("foo")
	params := []byte("{}")
	msg := []byte("{}")
	db := NewLookup()

	_, err := Instantiate(cache, id, params, msg, db, 100000000)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
}

func TestHandleFails(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	id := []byte("foo")
	params := []byte("{}")
	msg := []byte("{}")
	db := NewLookup()

	_, err := Handle(cache, id, params, msg, db, 100000000)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
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
