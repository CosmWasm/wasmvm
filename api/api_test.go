package api

import (
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

func TestCreateAndGetFails(t *testing.T) {
	dataDir := "/foo"
	wasm := []byte("code goes here")

	_, err := Create(dataDir, wasm)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())

	id := []byte("should be return from above")

	_, err = GetCode(dataDir, id)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
}

func TestInstantiateFails(t *testing.T) {
	dataDir := "/foo"
	id := []byte("foo")
	params := []byte("{}")
	msg := []byte("{}")
	db := NewLookup()

	_, err := Instantiate(dataDir, id, params, msg, db, 100000000)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
}

func TestHandleFails(t *testing.T) {
	dataDir := "/foo"
	id := []byte("foo")
	params := []byte("{}")
	msg := []byte("{}")
	db := NewLookup()

	_, err := Handle(dataDir, id, params, msg, db, 100000000)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
}

func TestQueryFails(t *testing.T) {
	dataDir := "/foo"
	id := []byte("foo")
	path := []byte("/some/stuff")
	data := []byte("{}")
	db := NewLookup()

	_, err := Query(dataDir, id, path, data, db, 100000000)
	require.Error(t, err)
	require.Equal(t, "not implemented", err.Error())
}
