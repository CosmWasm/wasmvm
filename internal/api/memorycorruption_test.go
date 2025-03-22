package api_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func TestMemoryCorruptionProtection(t *testing.T) {
	// Setup temporary directory for cache
	tmpdir, err := os.MkdirTemp("", "wasmvm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Initialize cache with a restrictive memory limit to test boundary enforcement
	cache, err := api.InitCache(types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    []string{"staking", "stargate", "iterator"},
			MemoryCacheSizeBytes:     types.NewSizeMebi(100),
			InstanceMemoryLimitBytes: types.NewSizeMebi(1), // 1 MiB limit for testing
		},
	})
	require.NoError(t, err)
	defer api.ReleaseCache(cache)

	// Common setup for instantiation and execution
	env, err := json.Marshal(api.MockEnv())
	require.NoError(t, err)
	info, err := json.Marshal(api.MockInfo("test-sender", nil))
	require.NoError(t, err)
	gasLimit := uint64(1000000)

	// Create mock objects for each test run
	setupTest := func(t *testing.T) (*types.GasMeter, types.KVStore, *types.GoAPI, *api.Querier) {
		t.Helper()
		gasMeter := api.NewMockGasMeter(gasLimit)
		// Need to convert MockGasMeter to *types.GasMeter for the API
		var typedGasMeter types.GasMeter = gasMeter
		store := api.NewLookup(gasMeter)
		goapi := api.NewMockAPI()

		// Create querier and convert to expected type
		mockQuerier := api.DefaultQuerier("test-contract", types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
		var typedQuerier api.Querier = mockQuerier.(*api.MockQuerier)

		return &typedGasMeter, store, goapi, &typedQuerier
	}

	// Test 1: Invalid WASM Structure
	t.Run("Invalid WASM Structure", func(t *testing.T) {
		// Test with an invalid magic number
		invalidMagic := []byte{0x01, 0x02, 0x03, 0x04} // Should be 0x00, 0x61, 0x73, 0x6D
		_, err := api.StoreCode(cache, invalidMagic, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Invalid Wasm: Invalid magic number")

		// Test with truncated valid WASM (original test)
		validWasm, err := os.ReadFile("../../testdata/hackatom.wasm")
		require.NoError(t, err)
		malformedWasm := validWasm[:len(validWasm)/2]
		_, err = api.StoreCode(cache, malformedWasm, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Error during static Wasm validation:")
	})

	// Test 2: Out-of-Bounds Memory Access
	t.Run("Out-of-Bounds Memory Access", func(t *testing.T) {
		gasMeter, store, goapi, querier := setupTest(t)

		// Load a custom WASM file designed to test out-of-bounds access
		outOfBoundsWasm, err := os.ReadFile("../../testdata/oob.wasm")
		require.NoError(t, err)

		// Store the code
		checksum, err := api.StoreCode(cache, outOfBoundsWasm, true)
		require.NoError(t, err)

		// Instantiate the contract
		initMsg := []byte(`{}`)
		_, _, err = api.Instantiate(cache, checksum, env, info, initMsg, gasMeter, store, goapi, querier, gasLimit, false)
		require.NoError(t, err)

		// Execute, expecting a trap
		execMsg := []byte(`{"test":{}}`) // Message triggers execute function
		_, _, err = api.Execute(cache, checksum, env, info, execMsg, gasMeter, store, goapi, querier, gasLimit, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "trap", "Expected execution to trap due to out-of-bounds access")
	})

	// Test 3: Invalid Memory Growth
	t.Run("Invalid Memory Growth", func(t *testing.T) {
		gasMeter, store, goapi, querier := setupTest(t)

		// Load a custom WASM file that attempts excessive memory growth
		invalidGrowthWasm, err := os.ReadFile("../../testdata/invalidgrowth.wasm")
		require.NoError(t, err)

		// Store the code
		checksum, err := api.StoreCode(cache, invalidGrowthWasm, true)
		require.NoError(t, err)

		// Instantiate the contract
		initMsg := []byte(`{}`)
		_, _, err = api.Instantiate(cache, checksum, env, info, initMsg, gasMeter, store, goapi, querier, gasLimit, false)
		require.NoError(t, err)

		// Execute, expecting a trap
		execMsg := []byte(`{"grow":{}}`)
		_, _, err = api.Execute(cache, checksum, env, info, execMsg, gasMeter, store, goapi, querier, gasLimit, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "trap", "Expected trap due to memory growth exceeding maximum")
	})

	// Test 4: Bulk Memory Operations
	t.Run("Bulk Memory Operations", func(t *testing.T) {
		gasMeter, store, goapi, querier := setupTest(t)

		// Load a custom WASM file with an invalid bulk operation
		bulkMemoryWasm, err := os.ReadFile("../../testdata/bulkmemory.wasm")
		require.NoError(t, err)

		// Store the code
		checksum, err := api.StoreCode(cache, bulkMemoryWasm, true)
		require.NoError(t, err)

		// Instantiate the contract
		initMsg := []byte(`{}`)
		_, _, err = api.Instantiate(cache, checksum, env, info, initMsg, gasMeter, store, goapi, querier, gasLimit, false)
		require.NoError(t, err)

		// Execute, expecting a trap
		execMsg := []byte(`{"copy":{}}`)
		_, _, err = api.Execute(cache, checksum, env, info, execMsg, gasMeter, store, goapi, querier, gasLimit, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "trap", "Expected trap due to out-of-bounds memory copy")
	})
}
