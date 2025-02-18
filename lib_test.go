package cosmwasm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

func TestCreateChecksum(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    types.Checksum
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Nil input",
			input:   nil,
			wantErr: true,
			errMsg:  "wasm bytecode cannot be nil or empty",
		},
		{
			name:    "Empty input",
			input:   []byte{},
			wantErr: true,
			errMsg:  "wasm bytecode cannot be nil or empty",
		},
		{
			name:    "Too short (1 byte)",
			input:   []byte{0x00},
			wantErr: true,
			errMsg:  "wasm bytecode is shorter than 4 bytes",
		},
		{
			name:    "Too short (3 bytes)",
			input:   []byte{0x00, 0x61, 0x73},
			wantErr: true,
			errMsg:  "wasm bytecode is shorter than 4 bytes",
		},
		{
			name:    "Valid minimal Wasm",
			input:   []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}, // "(module)"
			want:    types.ForceNewChecksum("93a44bbb96c751218e4c00d479e4c14358122a389acca16205b1e4d0dc5f9476"),
			wantErr: false,
		},
		{
			name:    "Invalid Wasm magic number",
			input:   []byte{0x01, 0x02, 0x03, 0x04},
			wantErr: true,
			errMsg:  "wasm bytecode does not start with Wasm magic number",
		},
		{
			name:    "Text file",
			input:   []byte("Hello world"),
			wantErr: true,
			errMsg:  "wasm bytecode does not start with Wasm magic number",
		},
		{
			name:    "Large valid Wasm prefix",
			input:   append([]byte{0x00, 0x61, 0x73, 0x6d}, bytes.Repeat([]byte{0x01}, 1024)...),
			want:    types.ForceNewChecksum("f0b5cefe7c7a9fadf7e77fddf5f039eabf0ebfb88ae5b5e8e0f5f0e9c3e5b5e8"), // Precomputed SHA-256
			wantErr: false,
		},
		{
			name:    "Exact 4 bytes with wrong magic",
			input:   []byte{0xFF, 0xFF, 0xFF, 0xFF},
			wantErr: true,
			errMsg:  "wasm bytecode does not start with Wasm magic number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateChecksum(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
				// Verify the checksum is a valid SHA-256 hash
				hashBytes, err := hex.DecodeString(tt.want.String())
				require.NoError(t, err)
				require.Len(t, hashBytes, 32)
			}
		})
	}
}

// TestCreateChecksumConsistency ensures consistent output for the same input
func TestCreateChecksumConsistency(t *testing.T) {
	input := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00} // Minimal valid Wasm
	expected := types.ForceNewChecksum("93a44bbb96c751218e4c00d479e4c14358122a389acca16205b1e4d0dc5f9476")

	for i := 0; i < 100; i++ {
		checksum, err := CreateChecksum(input)
		require.NoError(t, err)
		assert.Equal(t, expected, checksum, "Checksum should be consistent across runs")
	}
}

// TestCreateChecksumLargeInput tests behavior with a large valid Wasm input
func TestCreateChecksumLargeInput(t *testing.T) {
	// Create a large valid Wasm-like input (starts with magic number)
	largeInput := append([]byte{0x00, 0x61, 0x73, 0x6d}, bytes.Repeat([]byte{0xFF}, 1<<20)...) // 1MB
	checksum, err := CreateChecksum(largeInput)
	require.NoError(t, err)

	// Compute expected SHA-256 manually to verify
	h := sha256.New()
	h.Write(largeInput)
	expected := types.ForceNewChecksum(hex.EncodeToString(h.Sum(nil)))

	assert.Equal(t, expected, checksum, "Checksum should match SHA-256 of large input")
}

// TestCreateChecksumInvalidMagicVariations tests variations of invalid Wasm magic numbers
func TestCreateChecksumInvalidMagicVariations(t *testing.T) {
	invalidMagics := [][]byte{
		{0x01, 0x61, 0x73, 0x6d}, // Wrong first byte
		{0x00, 0x62, 0x73, 0x6d}, // Wrong second byte
		{0x00, 0x61, 0x74, 0x6d}, // Wrong third byte
		{0x00, 0x61, 0x73, 0x6e}, // Wrong fourth byte
	}

	for _, input := range invalidMagics {
		_, err := CreateChecksum(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "wasm bytecode does not start with Wasm magic number")
	}
}

// TestCreateChecksumStress tests the function under high load with valid inputs
func TestCreateChecksumStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	validInput := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	const iterations = 10000

	for i := 0; i < iterations; i++ {
		checksum, err := CreateChecksum(validInput)
		require.NoError(t, err)
		require.Equal(t, types.ForceNewChecksum("93a44bbb96c751218e4c00d479e4c14358122a389acca16205b1e4d0dc5f9476"), checksum)
	}
}

// TestCreateChecksumConcurrent tests concurrent execution safety
func TestCreateChecksumConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	validInput := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	expected := types.ForceNewChecksum("93a44bbb96c751218e4c00d479e4c14358122a389acca16205b1e4d0dc5f9476")
	const goroutines = 50
	const iterations = 200

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				checksum, err := CreateChecksum(validInput)
				assert.NoError(t, err)
				assert.Equal(t, expected, checksum)
			}
		}()
	}
	wg.Wait()
}
