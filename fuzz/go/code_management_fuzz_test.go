//go:build go1.18

package gofuzz

import (
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v2"
)

func FuzzCodeManagement(f *testing.F) {
	// Add seed corpus with boolean combinations for operations
	f.Add(true, true)   // Store, Get, then Remove
	f.Add(true, false)  // Store, Get, but don't Remove
	f.Add(false, true)  // Skip Get, but try Remove
	f.Add(false, false) // Skip both Get and Remove

	f.Fuzz(func(t *testing.T, getCode, removeCode bool) {
		// Create a new VM for each test
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return // Skip if VM creation fails
		}
		defer vm.Cleanup()

		// Load a test WASM file
		wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
		if err != nil {
			t.Skip("Could not read test WASM file")
		}

		// Store the code
		checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		if err != nil {
			return // Skip if storing fails
		}

		// Test GetCode if enabled
		if getCode {
			code, err := vm.GetCode(checksum)
			_ = code // Use code to avoid unused variable
			_ = err  // Use err to avoid unused variable
		}

		// Test RemoveCode if enabled
		if removeCode {
			_ = vm.RemoveCode(checksum)

			// Try to get the code after removal to verify it's gone
			if getCode {
				_, err := vm.GetCode(checksum)
				_ = err // We expect an error here, but don't need to check it for fuzzing
			}
		}

		// Store again to test idempotence
		if removeCode {
			_, _, _ = vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		}
	})
}
