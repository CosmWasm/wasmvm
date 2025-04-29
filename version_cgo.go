//go:build cgo && !nolink_libwasmvm

package cosmwasm

import (
	"github.com/CosmWasm/wasmvm/v3/internal/api"
)

func libwasmvmVersionImpl() (string, error) {
	return api.LibwasmvmVersion()
}
