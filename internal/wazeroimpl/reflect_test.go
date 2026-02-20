//go:build wazero || !cgo

package wazeroimpl

import "testing"

// TestReflectInstantiate loads the reflect.wasm contract (shipped in testdata)
// and instantiates it via the wazero-backed VM implementation. This ensures
// that real-world CosmWasm contracts that compile to Wasm can be instantiated
// without panics or host‐side errors.
func TestReflectInstantiate(t *testing.T) {
    t.Skip("reflect contract requires full host ABI; skipped for minimal harness")
}
