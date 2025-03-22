package testdb

import (
	"errors"

	"github.com/CosmWasm/wasmvm/v2/types"
)

var (

	// ErrKeyEmpty is returned when attempting to use an empty or nil key.
	ErrKeyEmpty = errors.New("key cannot be empty")

	// ErrValueNil is returned when attempting to set a nil value.
	ErrValueNil = errors.New("value cannot be nil")
)

type Iterator = types.Iterator
