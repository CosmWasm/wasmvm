package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

/*** Mock KVStore ****/
// Much of this code is borrowed from Cosmos-SDK store/transient.go

type Lookup struct {
	db *dbm.MemDB
}
func NewLookup() Lookup {
	return Lookup {
		db: dbm.NewMemDB(),
	}
}

// Get wraps the underlying DB's Get method panicing on error.
func (l Lookup) Get(key []byte) []byte {
	v, err := l.db.Get(key)
	if err != nil {
		panic(err)
	}

	return v
}

// Set wraps the underlying DB's Set method panicing on error.
func (l Lookup) Set(key, value []byte) {
	if err := l.db.Set(key, value); err != nil {
		panic(err)
	}
}

// Delete wraps the underlying DB's Delete method panicing on error.
func (l Lookup) Delete(key []byte) {
	if err := l.db.Delete(key); err != nil {
		panic(err)
	}
}

// Iterator wraps the underlying DB's Iterator method panicing on error.
func (l Lookup) Iterator(start, end []byte) dbm.Iterator {
	iter, err := l.db.Iterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

// ReverseIterator wraps the underlying DB's ReverseIterator method panicing on error.
func (l Lookup) ReverseIterator(start, end []byte) dbm.Iterator {
	iter, err := l.db.ReverseIterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}



// type Lookup struct {
// 	data map[string]string
// }
//
//
// func (l *Lookup) Get(key []byte) []byte {
// 	val := l.data[string(key)]
// 	return []byte(val)
// }
//
// func (l *Lookup) Set(key, value []byte) {
// 	l.data[string(key)] = string(value)
// }
//
// func (l *Lookup) Delete(key []byte) {
// 	delete(l.data, string(key))
// }

var _ KVStore = (*Lookup)(nil)

/***** Mock GoAPI ****/

const CanonicalLength = 32

func MockCanonicalAddress(human string) ([]byte, error) {
	if len(human) > CanonicalLength {
		return nil, fmt.Errorf("human encoding too long")
	}
	res := make([]byte, CanonicalLength)
	copy(res, []byte(human))
	return res, nil
}

func MockHumanAddress(canon []byte) (string, error) {
	if len(canon) != CanonicalLength {
		return "", fmt.Errorf("wrong canonical length")
	}
	cut := CanonicalLength
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human := string(canon[:cut])
	return human, nil
}

func NewMockAPI() *GoAPI {
	return &GoAPI{
		HumanAddress:     MockHumanAddress,
		CanonicalAddress: MockCanonicalAddress,
	}
}

func TestMockApi(t *testing.T) {
	human := "foobar"
	canon, err := MockCanonicalAddress(human)
	require.NoError(t, err)
	assert.Equal(t, CanonicalLength, len(canon))

	recover, err := MockHumanAddress(canon)
	require.NoError(t, err)
	assert.Equal(t, recover, human)
}
