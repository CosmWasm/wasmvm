//go:build cgo

package cosmwasm

import (
	"github.com/CosmWasm/wasmvm/v2/internal/api"
)

func libwasmvmVersionImpl() (string, error) {
	return api.LibwasmvmVersion()
}
