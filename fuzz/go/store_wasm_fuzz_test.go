//go:build go1.18

package gofuzz

import (
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v2"
)

const (
	TESTING_GAS_LIMIT    = uint64(500_000_000_000) // ~0.5ms
	TESTING_MEMORY_LIMIT = 32                      // MiB
	TESTING_CACHE_SIZE   = 100                     // MiB
)

var TESTING_CAPABILITIES = []string{"staking", "stargate", "iterator"}

func FuzzStoreCode(f *testing.F) {
	// Add seed corpus with valid WASM files
	seedFiles := []string{
		"../../testdata/hackatom.wasm",
		"../../testdata/cyberpunk.wasm",
	}

	for _, file := range seedFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}

	// Add some manual edge cases
	f.Add([]byte{})                                   // empty
	f.Add([]byte{0x00, 0x61, 0x73, 0x6d})             // valid header only
	f.Add([]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00}) // valid header + version prefix

	f.Fuzz(func(t *testing.T, wasm []byte) {
		// Create a new VM for each test to avoid state contamination
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return // Skip if VM creation fails
		}
		defer vm.Cleanup()

		// Test StoreCode
		_, _, _ = vm.StoreCode(wasm, TESTING_GAS_LIMIT)

		// Test SimulateStoreCode (this doesn't actually store anything)
		_, _, _ = vm.SimulateStoreCode(wasm, TESTING_GAS_LIMIT)
	})
}
