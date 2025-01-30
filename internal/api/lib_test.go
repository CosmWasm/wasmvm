package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	TESTING_PRINT_DEBUG  = true
	TESTING_GAS_LIMIT    = uint64(1_000_000_000_000) // ~1ms
	TESTING_MEMORY_LIMIT = 64                        // MiB
	TESTING_CACHE_SIZE   = 2048                      // MiB (2GB)
)

var TESTING_CAPABILITIES = []string{"staking", "stargate", "iterator", "cosmwasm_1_1", "cosmwasm_1_2", "cosmwasm_1_3", "cosmwasm_1_4", "cosmwasm_2_0", "cosmwasm_2_1", "cosmwasm_2_2"}

type CapitalizedResponse struct {
	Text string `json:"text"`
}

func TestInitAndReleaseCache(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

// wasmd expects us to create the base directory
// https://github.com/CosmWasm/wasmd/blob/v0.30.0/x/wasm/keeper/keeper.go#L128
func TestInitCacheWorksForNonExistentDir(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	createMe := filepath.Join(tmpdir, "does-not-yet-exist")
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  createMe,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func TestInitCacheErrorsForBrokenDir(t *testing.T) {
	// Use colon to make this fail on Windows
	// https://gist.github.com/doctaphred/d01d05291546186941e1b7ddc02034d3
	// On Unix we should not have permission to create this.
	cannotBeCreated := "/foo:bar"
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  cannotBeCreated,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	_, err := InitCache(config)
	require.ErrorContains(t, err, "Could not create base directory")
}

func TestInitLockingPreventsConcurrentAccess(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	config1 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache1, err1 := InitCache(config1)
	require.NoError(t, err1)

	config2 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	_, err2 := InitCache(config2)
	require.ErrorContains(t, err2, "Could not lock exclusive.lock. Is a different VM running in the same directory already?")

	ReleaseCache(cache1)

	// Now we can try again
	config3 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache3, err3 := InitCache(config3)
	require.NoError(t, err3)
	ReleaseCache(cache3)
}

func TestInitLockingAllowsMultipleInstancesInDifferentDirs(t *testing.T) {
	tmpdir1, err := os.MkdirTemp("", "wasmvm-testing1")
	require.NoError(t, err)
	tmpdir2, err := os.MkdirTemp("", "wasmvm-testing2")
	require.NoError(t, err)
	tmpdir3, err := os.MkdirTemp("", "wasmvm-testing3")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir1)
	defer os.RemoveAll(tmpdir2)
	defer os.RemoveAll(tmpdir3)

	config1 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir1,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache1, err1 := InitCache(config1)
	require.NoError(t, err1)
	config2 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir2,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache2, err2 := InitCache(config2)
	require.NoError(t, err2)
	config3 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir3,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache3, err3 := InitCache(config3)
	require.NoError(t, err3)

	ReleaseCache(cache1)
	ReleaseCache(cache2)
	ReleaseCache(cache3)
}

func TestInitCacheEmptyCapabilities(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    []string{},
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func withCache(tb testing.TB) (Cache, func()) {
	tb.Helper()
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(tb, err)
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(tb, err)

	cleanup := func() {
		os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	return cache, cleanup
}

func TestStoreCodeAndGetCode(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	require.Equal(t, expectedChecksum[:], checksum)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestRemoveCode(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	// First removal works
	err = RemoveCode(cache, checksum)
	require.NoError(t, err)

	// Second removal fails
	err = RemoveCode(cache, checksum)
	require.ErrorContains(t, err, "Wasm file does not exist")
}

func TestStoreCodeFailsWithBadData(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm := []byte("some invalid data")
	_, err := StoreCode(cache, wasm, true)
	require.Error(t, err)
}

func TestStoreCodeUnchecked(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCodeUnchecked(cache, wasm)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	require.Equal(t, expectedChecksum[:], checksum)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestStoreCodeUncheckedWorksWithInvalidWasm(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	// Look for "interface_version_8" in the wasm file and replace it with "interface_version_9".
	// This makes the wasm file invalid.
	wasm = bytes.Replace(wasm, []byte("interface_version_8"), []byte("interface_version_9"), 1)

	// StoreCode should fail
	_, err = StoreCode(cache, wasm, true)
	require.ErrorContains(t, err, "contract has unknown")

	// StoreCodeUnchecked should not fail
	checksum, err := StoreCodeUnchecked(cache, wasm)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	assert.Equal(t, expectedChecksum[:], checksum)
}

func TestPin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Pin(cache, checksum)
	require.NoError(t, err)
}

func TestPinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Pin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Pin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// Unknown checksum (errors in cosmwasm-vm)
	unknownChecksum := []byte{
		0x72, 0x2c, 0x8c, 0x99, 0x3f, 0xd7, 0x5a, 0x76, 0x27, 0xd6, 0x9e, 0xd9, 0x41, 0x34,
		0x4f, 0xe2, 0xa1, 0x42, 0x3a, 0x3e, 0x75, 0xef, 0xd3, 0xe6, 0x77, 0x8a, 0x14, 0x28,
		0x84, 0x22, 0x71, 0x04,
	}
	err = Pin(cache, unknownChecksum)
	require.ErrorContains(t, err, "Error opening Wasm file for reading")
}

func TestUnpin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Unpin(cache, checksum)
	require.NoError(t, err)
}

func TestUnpinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Unpin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Unpin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// No error case triggered in cosmwasm-vm is known right now
}

func TestGetMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.Metrics{}, metrics)

	// Store contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	// GetMetrics 2
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg2, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 4
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Pin
	err = Pin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg3, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 6
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Unpin
	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 4
	msg4 := []byte(`{"verifier": "fred", "beneficiary": "jeff"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg4, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 8
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)
}

func TestGetPinnedMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.PinnedMetrics{PerModule: make([]types.PerModuleEntry, 0)}, metrics)

	// Store contract 1
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Store contract 2
	cyberpunkWasm, err := os.ReadFile("../../testdata/cyberpunk.wasm")
	require.NoError(t, err)
	cyberpunkChecksum, err := StoreCode(cache, cyberpunkWasm, true)
	require.NoError(t, err)

	err = Pin(cache, cyberpunkChecksum)
	require.NoError(t, err)

	findMetrics := func(list []types.PerModuleEntry, checksum types.Checksum) *types.PerModuleMetrics {
		found := (*types.PerModuleMetrics)(nil)

		for _, structure := range list {
			if bytes.Equal(structure.Checksum, checksum) {
				found = &structure.Metrics
				break
			}
		}

		return found
	}

	// GetMetrics 2
	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Len(t, metrics.PerModule, 2)

	hackatomMetrics := findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics := findMetrics(metrics.PerModule, cyberpunkChecksum)

	require.Equal(t, uint32(0), hackatomMetrics.Hits)
	require.NotEqual(t, uint32(0), hackatomMetrics.Size)
	require.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	require.NotEqual(t, uint32(0), cyberpunkMetrics.Size)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Len(t, metrics.PerModule, 2)

	hackatomMetrics = findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics = findMetrics(metrics.PerModule, cyberpunkChecksum)

	require.Equal(t, uint32(1), hackatomMetrics.Hits)
	require.NotEqual(t, uint32(0), hackatomMetrics.Size)
	require.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	require.NotEqual(t, uint32(0), cyberpunkMetrics.Size)
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa3e4ae), cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Empty(t, result.Ok.Messages)
}

func TestExecute(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa3e4ae), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// execute with the same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	res, cost, err = Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0x12899a6), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// make sure it read the balance properly and we got 250 atoms
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	// Ensure we got our custom event
	require.Len(t, result.Ok.Events, 1)
	ev := result.Ok.Events[0]
	require.Equal(t, "hackatom", ev.Type)
	require.Len(t, ev.Attributes, 1)
	require.Equal(t, "action", ev.Attributes[0].Key)
	require.Equal(t, "release", ev.Attributes[0].Value)

	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	require.Equal(t, "bob", send.ToAddress)
	require.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	require.Equal(t, expectedData, result.Ok.Data)
}

func TestExecutePanic(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a panic
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"panic":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.Error(t, err)
	require.Contains(t, err.Error(), "RuntimeError: Aborted: panicked at src/contract.rs:127:5:\nThis page intentionally faulted")
}

func TestExecuteUnreachable(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a panic
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"unreachable":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.ErrorContains(t, err, "RuntimeError: unreachable")
}

func TestExecuteCpuLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x72c3ce), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// execute a cpu loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	_, cost, err = Execute(cache, checksum, env, info, []byte(`{"cpu_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, false)
	diff = time.Since(start)
	require.Error(t, err)
	require.Equal(t, cost.UsedInternally, maxGas)
	t.Logf("CPULoop Time (%d gas): %s\n", cost.UsedInternally, diff)
}

func TestExecuteStorageLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a storage loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start := time.Now()
	_, gasReport, err := Execute(cache, checksum, env, info, []byte(`{"storage_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, false)
	diff := time.Since(start)
	require.Error(t, err)
	t.Logf("StorageLoop Time (%d gas): %s\n", gasReport.UsedInternally, diff)
	t.Logf("Gas used: %d\n", gasMeter2.GasConsumed())
	t.Logf("Wasm gas: %d\n", gasReport.UsedInternally)

	// the "sdk gas" * GasMultiplier + the wasm cost should equal the maxGas (or be very close)
	totalCost := gasReport.UsedInternally + gasMeter2.GasConsumed()
	require.Equal(t, int64(maxGas), int64(totalCost))
}

func BenchmarkContractCall(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
		igasMeter2 := types.GasMeter(gasMeter2)
		store.SetGasMeter(gasMeter2)
		info = MockInfoBin(b, "fred")
		msg := []byte(`{"allocate_large_memory":{"pages":0}}`) // replace with noop once we have it
		res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(b, err)
		requireOkResponse(b, res, 0)
	}
}

func Benchmark100ConcurrentContractCalls(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	info = MockInfoBin(b, "fred")

	const callCount = 100 // Calls per benchmark iteration

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		errChan := make(chan error, callCount)
		resChan := make(chan []byte, callCount)
		wg.Add(callCount)
		info = mockInfoBinNoAssert("fred")
		for i := 0; i < callCount; i++ {
			go func() {
				defer wg.Done()
				gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
				igasMeter2 := types.GasMeter(gasMeter2)
				store.SetGasMeter(gasMeter2)
				msg := []byte(`{"allocate_large_memory":{"pages":0}}`) // replace with noop once we have it
				res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
				errChan <- err
				resChan <- res
			}()
		}
		wg.Wait()
		close(errChan)
		close(resChan)

		// Now check results in the main test goroutine
		for i := 0; i < callCount; i++ {
			require.NoError(b, <-errChan)
			requireOkResponse(b, <-resChan, 0)
		}
	}
}

func TestExecuteUserErrorsInApiCalls(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	defaultApi := NewMockAPI()
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, defaultApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	failingApi := NewMockFailureAPI()
	res, _, err = Execute(cache, checksum, env, info, []byte(`{"user_errors_in_api_calls":{}}`), &igasMeter2, store, failingApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
}

func TestMigrate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// verifier is fred
	query := []byte(`{"verifier":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.JSONEq(t, `{"verifier":"fred"}`, string(qResult.Ok))

	// migrate to a new verifier - alice
	// we use the same code blob as we are testing hackatom self-migration
	info = MockInfoBin(t, "admin")
	_, _, err = MigrateWithInfo(cache, checksum, env, []byte(`{"verifier":"alice"}`), info, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// should update verifier to alice
	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult2 types.QueryResult
	err = json.Unmarshal(data, &qResult2)
	require.NoError(t, err)
	require.Equal(t, "", qResult2.Err)
	require.JSONEq(t, `{"verifier":"alice"}`, string(qResult2.Ok))
}

func TestMultipleInstances(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// instance1 controlled by fred
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store1 := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "regen")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store1, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa2aeb8), cost.UsedInternally)

	// instance2 controlled by mary
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store2 := NewLookup(gasMeter2)
	info = MockInfoBin(t, "chrous")
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, cost, err = Instantiate(cache, checksum, env, info, msg, &igasMeter2, store2, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa35f43), cost.UsedInternally)

	// fail to execute store1 with mary
	resp := exec(t, cache, checksum, "mary", store1, api, querier, 0x9a2b03)
	require.Equal(t, "Unauthorized", resp.Err)

	// succeed to execute store1 with fred
	resp = exec(t, cache, checksum, "fred", store1, api, querier, 0x1281a12)
	require.Equal(t, "", resp.Err)
	require.Len(t, resp.Ok.Messages, 1)
	attributes := resp.Ok.Attributes
	require.Len(t, attributes, 2)
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "bob", attributes[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, checksum, "mary", store2, api, querier, 0x12859dc)
	require.Equal(t, "", resp.Err)
	require.Len(t, resp.Ok.Messages, 1)
	attributes = resp.Ok.Attributes
	require.Len(t, attributes, 2)
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "sue", attributes[1].Value)
}

func TestSudo(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// call sudo with same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	msg = []byte(`{"steal_funds":{"recipient":"community-pool","amount":[{"amount":"700","denom":"gold"}]}}`)
	res, _, err = Sudo(cache, checksum, env, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "community-pool", send.ToAddress)
	expectedPayout := types.Array[types.Coin]{types.NewCoin(700, "gold")}
	assert.Equal(t, expectedPayout, send.Amount)
}

func TestDispatchSubmessage(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// dispatch a submessage
	var id uint64 = 1234
	payload := types.SubMsg{
		ID: id,
		Msg: types.CosmosMsg{Bank: &types.BankMsg{Send: &types.SendMsg{
			ToAddress: "friend",
			Amount:    types.Array[types.Coin]{types.NewCoin(1, "token")},
		}}},
		ReplyOn: types.ReplyAlways,
	}
	payloadBin, err := json.Marshal(payload)
	require.NoError(t, err)
	payloadMsg := []byte(fmt.Sprintf(`{"reflect_sub_msg":{"msgs":[%s]}}`, string(payloadBin)))

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Execute(cache, checksum, env, info, payloadMsg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	dispatch := result.Ok.Messages[0]
	assert.Equal(t, id, dispatch.ID)
	assert.Equal(t, payload.Msg, dispatch.Msg)
	assert.Nil(t, dispatch.GasLimit)
	assert.Equal(t, payload.ReplyOn, dispatch.ReplyOn)
}

func TestReplyAndQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	var id uint64 = 1234
	data := []byte("foobar")
	events := types.Array[types.Event]{{
		Type: "message",
		Attributes: types.Array[types.EventAttribute]{{
			Key:   "signer",
			Value: "caller-addr",
		}},
	}}
	reply := types.Reply{
		ID: id,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: events,
				Data:   data,
			},
		},
	}
	replyBin, err := json.Marshal(reply)
	require.NoError(t, err)

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Reply(cache, checksum, env, replyBin, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// now query the state to see if it stored the data properly
	badQuery := []byte(`{"sub_msg_result":{"id":7777}}`)
	res, _, err = Query(cache, checksum, env, badQuery, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireQueryError(t, res)

	query := []byte(`{"sub_msg_result":{"id":1234}}`)
	res, _, err = Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	qResult := requireQueryOk(t, res)

	var stored types.Reply
	err = json.Unmarshal(qResult, &stored)
	require.NoError(t, err)
	assert.Equal(t, id, stored.ID)
	require.NotNil(t, stored.Result.Ok)
	val := stored.Result.Ok
	require.Equal(t, data, val.Data)
	require.Equal(t, events, val.Events)
}

func requireOkResponse(tb testing.TB, res []byte, expectedMsgs int) {
	tb.Helper()
	var result types.ContractResult
	err := json.Unmarshal(res, &result)
	require.NoError(tb, err)
	require.Equal(tb, "", result.Err)
	require.Len(tb, result.Ok.Messages, expectedMsgs)
}

func requireQueryError(t *testing.T, res []byte) {
	t.Helper()
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Ok)
	require.NotEmpty(t, result.Err)
}

func requireQueryOk(t *testing.T, res []byte) []byte {
	t.Helper()
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Err)
	require.NotEmpty(t, result.Ok)
	return result.Ok
}

func createHackatomContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/hackatom.wasm")
}

func createCyberpunkContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/cyberpunk.wasm")
}

func createQueueContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/queue.wasm")
}

func createReflectContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/reflect.wasm")
}

//func createFloaty2(tb testing.TB, cache Cache) []byte {
//	tb.Helper()
//	return createContract(tb, cache, "../../testdata/floaty_2.0.wasm")
//}

func createContract(tb testing.TB, cache Cache, wasmFile string) []byte {
	tb.Helper()
	wasm, err := os.ReadFile(wasmFile)
	require.NoError(tb, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(tb, err)
	return checksum
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, checksum []byte, signer types.HumanAddress, store types.KVStore, api *types.GoAPI, querier types.Querier, gasExpected uint64) types.ContractResult {
	t.Helper()
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	env := MockEnvBin(t)
	info := MockInfoBin(t, signer)
	res, cost, err := Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	assert.Equal(t, gasExpected, cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	return result
}

func TestQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// invalid query
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var badResult types.QueryResult
	err = json.Unmarshal(data, &badResult)
	require.NoError(t, err)
	require.Contains(t, badResult.Err, "Error parsing into type hackatom::msg::QueryMsg: unknown variant `Raw`, expected one of")

	// make a valid query
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := types.GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter3, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.JSONEq(t, `{"verifier":"fred"}`, string(qResult.Ok))
}

func TestHackatomQuerier(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM"), types.NewCoin(65432, "ETH")}
	querier := DefaultQuerier("foobar", initBalance)

	// make a valid query to the other address
	query := []byte(`{"other_balance":{"address":"foobar"}}`)
	// TODO The query happens before the contract is initialized. How is this legal?
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	var balances types.AllBalancesResponse
	err = json.Unmarshal(qResult.Ok, &balances)
	require.NoError(t, err)
	require.Equal(t, balances.Amount, initBalance)
}

func TestCustomReflectQuerier(t *testing.T) {
	type CapitalizedQuery struct {
		Text string `json:"text"`
	}

	type QueryMsg struct {
		Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
		// There are more queries but we don't use them yet
		// https://github.com/CosmWasm/cosmwasm/blob/v0.11.0-alpha3/contracts/reflect/src/msg.rs#L18-L28
	}

	type CapitalizedResponse struct {
		Text string `json:"text"`
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, initBalance)
	// we need this to handle the custom requests from the reflect contract
	innerQuerier := querier.(*MockQuerier)
	innerQuerier.Custom = ReflectCustom{}
	querier = types.Querier(innerQuerier)

	// make a valid query to the other address
	queryMsg := QueryMsg{
		Capitalized: &CapitalizedQuery{
			Text: "small Frys :)",
		},
	}
	query, err := json.Marshal(queryMsg)
	require.NoError(t, err)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)

	var response CapitalizedResponse
	err = json.Unmarshal(qResult.Ok, &response)
	require.NoError(t, err)
	require.Equal(t, "SMALL FRYS :)", response.Text)
}

// testfloats is disabled temporarily because of its high output

/*

// TestFloats is a port of the float_instrs_are_deterministic test in cosmwasm-vm
func TestFloats(t *testing.T) {
	type Value struct {
		U32 *uint32 `json:"u32,omitempty"`
		U64 *uint64 `json:"u64,omitempty"`
		F32 *uint32 `json:"f32,omitempty"`
		F64 *uint64 `json:"f64,omitempty"`
	}

	// helper to print the value in the same format as Rust's Debug trait
	debugStr := func(value Value) string {
		if value.U32 != nil {
			return fmt.Sprintf("U32(%d)", *value.U32)
		} else if value.U64 != nil {
			return fmt.Sprintf("U64(%d)", *value.U64)
		} else if value.F32 != nil {
			return fmt.Sprintf("F32(%d)", *value.F32)
		} else if value.F64 != nil {
			return fmt.Sprintf("F64(%d)", *value.F64)
		} else {
			t.FailNow()
			return ""
		}
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createFloaty2(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)

	// query instructions
	query := []byte(`{"instructions":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, false)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Empty(t, qResult.Err)
	var instructions []string
	err = json.Unmarshal(qResult.Ok, &instructions)
	require.NoError(t, err)
	// little sanity check
	require.Len(t, instructions, 70)

	hasher := sha256.New()
	const RUNS_PER_INSTRUCTION = 150
	for _, instr := range instructions {
		for seed := 0; seed < RUNS_PER_INSTRUCTION; seed++ {
			// query some input values for the instruction
			msg := fmt.Sprintf(`{"random_args_for":{"instruction":"%s","seed":%d}}`, instr, seed)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, false)
			require.NoError(t, err)
			err = json.Unmarshal(data, &qResult)
			require.NoError(t, err)
			require.Empty(t, qResult.Err)
			var args []Value
			err = json.Unmarshal(qResult.Ok, &args)
			require.NoError(t, err)

			// build the run message
			argStr, err := json.Marshal(args)
			require.NoError(t, err)
			msg = fmt.Sprintf(`{"run":{"instruction":"%s","args":%s}}`, instr, argStr)

			// run the instruction
			// this might throw a runtime error (e.g. if the instruction traps)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, false)
			var result string
			if err != nil {
				require.Error(t, err)
				// remove the prefix to make the error message the same as in the cosmwasm-vm test
				result = strings.Replace(err.Error(), "Error calling the VM: Error executing Wasm: ", "", 1)
			} else {
				err = json.Unmarshal(data, &qResult)
				require.NoError(t, err)
				require.Empty(t, qResult.Err)
				var response Value
				err = json.Unmarshal(qResult.Ok, &response)
				require.NoError(t, err)
				result = debugStr(response)
			}
			// add the result to the hash
			hasher.Write([]byte(fmt.Sprintf("%s%d%s", instr, seed, result)))
		}
	}

	hash := hasher.Sum(nil)
	require.Equal(t, "6e9ffbe929a2c1bcbffca0d4e9d0935371045bba50158a01ec082459a4cbbd2a", hex.EncodeToString(hash))
}
*/

// mockInfoBinNoAssert creates the message binary without using testify assertions
func mockInfoBinNoAssert(sender types.HumanAddress) []byte {
	info := types.MessageInfo{
		Sender: sender,
		Funds:  types.Array[types.Coin]{},
	}
	res, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}
	return res
}
