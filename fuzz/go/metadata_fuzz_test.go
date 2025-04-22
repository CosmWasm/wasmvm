//go:build go1.18

package gofuzz

import (
	"bytes"
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v2"
)

func FuzzMetadata(f *testing.F) {
	// Add seed corpus with operations to perform
	f.Add(true, true, true)    // Analyze, GetMetrics, GetPinnedMetrics
	f.Add(true, false, false)  // Only Analyze
	f.Add(false, true, false)  // Only GetMetrics
	f.Add(false, false, true)  // Only GetPinnedMetrics
	f.Add(false, false, false) // Nothing

	f.Fuzz(func(t *testing.T, analyze, getMetrics, getPinnedMetrics bool) {
		// Create a new VM for each test
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return // Skip if VM creation fails
		}
		defer vm.Cleanup()

		// Load test WASM files
		testWasmFiles := []string{
			"../../testdata/hackatom.wasm",
			"../../testdata/cyberpunk.wasm",
		}

		var checksums []wasmvm.Checksum
		for _, path := range testWasmFiles {
			wasm, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			// Don't store invalid WASM
			if !isValidWasm(wasm) {
				continue
			}

			// Store the code
			checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
			if err != nil {
				continue
			}

			checksums = append(checksums, checksum)

			// Pin some codes to test more conditions
			if len(checksums)%2 == 0 {
				_ = vm.Pin(checksum)
			}
		}

		// Skip if no contracts were stored
		if len(checksums) == 0 {
			return
		}

		// Test AnalyzeCode if enabled
		if analyze && len(checksums) > 0 {
			_, _ = vm.AnalyzeCode(checksums[0])
		}

		// Test GetMetrics if enabled
		if getMetrics {
			_, _ = vm.GetMetrics()
		}

		// Test GetPinnedMetrics if enabled
		if getPinnedMetrics {
			_, _ = vm.GetPinnedMetrics()
		}
	})
}

// Helper function to check if a byte slice is valid WASM
func isValidWasm(wasm []byte) bool {
	// Check for WASM magic number
	if len(wasm) < 4 {
		return false
	}
	return bytes.Equal(wasm[:4], []byte{0x00, 0x61, 0x73, 0x6D})
}
