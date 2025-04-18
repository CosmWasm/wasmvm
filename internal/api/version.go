package api

import "github.com/CosmWasm/wasmvm/v2/internal/ffi"

func LibwasmvmVersion() (string, error) {
	return ffi.Version(), nil
}
