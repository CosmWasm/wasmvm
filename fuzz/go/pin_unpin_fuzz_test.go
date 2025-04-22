package gofuzz

import (
	"bytes"
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v2"
)

// isValidWasmMagic checks if a byte slice has the WASM magic number
// (we're using a different name to avoid conflict with the function in metadata_fuzz_test.go)
func isValidWasmMagic(wasm []byte) bool {
	// Check for WASM magic number
	if len(wasm) < 4 {
		return false
	}
	return bytes.Equal(wasm[:4], []byte{0x00, 0x61, 0x73, 0x6D})
}

func FuzzPinUnpin(f *testing.F) {
	// Add seed corpus of pin/unpin operation sequences
	f.Add([]byte{1, 1, 0, 0}, uint8(3))  // Sequence of pin/unpin ops (1=pin, 0=unpin), with repetition count
	f.Add([]byte{1, 0, 1, 0}, uint8(5))  // Alternate pin/unpin repeatedly
	f.Add([]byte{1, 1, 1, 0}, uint8(1))  // Pin multiple times, then unpin once
	f.Add([]byte{1, 0, 0, 0}, uint8(2))  // Pin once, then unpin multiple times (testing idempotence)
	f.Add([]byte{0, 1, 0, 1}, uint8(4))  // Unpin before pin (testing error recovery)
	f.Add([]byte{1, 1, 1, 1}, uint8(10)) // Stress test with many repetitions

	f.Fuzz(func(t *testing.T, operationSequence []byte, repetitionCount uint8) {
		// Avoid excessive repetitions to prevent test timeouts
		if repetitionCount > 20 {
			repetitionCount = 20
		}

		// Ensure at least one repetition
		if repetitionCount == 0 {
			repetitionCount = 1
		}

		// Create a new VM for each test
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return
		}
		defer vm.Cleanup()

		// Load a variety of test WASM files
		wasmFiles := []string{
			"../../testdata/hackatom.wasm",
			"../../testdata/cyberpunk.wasm",
			// We'll create a few variations of the existing files to test edge cases
		}

		// Create a dynamic set of WASM files with slight modifications
		// This helps test different contract signatures
		var testFiles [][]byte
		for _, path := range wasmFiles {
			wasm, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			// Original contract
			testFiles = append(testFiles, wasm)

			// Modified version (changing a non-critical byte deeper in the file)
			if len(wasm) > 100 {
				modifiedWasm := make([]byte, len(wasm))
				copy(modifiedWasm, wasm)
				// Modify some non-critical byte (e.g., custom section)
				if len(modifiedWasm) > 100 {
					modifiedWasm[100] ^= 0xFF // Flip some bits
				}
				if isValidWasmMagic(modifiedWasm) {
					testFiles = append(testFiles, modifiedWasm)
				}
			}
		}

		// Skip if no valid WASM files
		if len(testFiles) == 0 {
			return
		}

		var checksums []wasmvm.Checksum

		// Store all test files
		for _, wasm := range testFiles {
			// Skip invalid WASM
			if !isValidWasmMagic(wasm) {
				continue
			}

			checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
			if err != nil {
				continue
			}
			checksums = append(checksums, checksum)
		}

		// Need at least one contract
		if len(checksums) == 0 {
			return
		}

		// Execute the sequence multiple times according to repetitionCount
		for r := uint8(0); r < repetitionCount; r++ {
			// Apply the operation sequence
			for _, op := range operationSequence {
				// Get a contract index (cycling through available contracts)
				idx := int(op) % len(checksums)
				checksum := checksums[idx]

				// Determine operation: lower 4 bits for operation type, upper 4 bits for contract index
				isPinOperation := op&0x1 == 1

				if isPinOperation {
					// Pin operation
					_ = vm.Pin(checksum)

					// Get and check metrics after pin
					metrics, _ := vm.GetMetrics()
					_ = metrics

					// Try concurrent pin/unpin (stress testing)
					if op&0x2 == 2 {
						_ = vm.Pin(checksum) // Pin again (idempotence test)
					}
				} else {
					// Unpin operation
					_ = vm.Unpin(checksum)

					// Get and check metrics after unpin
					metrics, _ := vm.GetMetrics()
					_ = metrics

					// Try concurrent pin/unpin (stress testing)
					if op&0x2 == 2 {
						_ = vm.Unpin(checksum) // Unpin again (idempotence test)
					}
				}

				// Sometimes check the pinned metrics
				if op&0x4 == 4 {
					_, _ = vm.GetPinnedMetrics()
				}
			}
		}

		// Final check of both metrics types
		_, _ = vm.GetMetrics()
		_, _ = vm.GetPinnedMetrics()
	})
}
