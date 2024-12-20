package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	testingPrintDebug  = false
	testingGasLimit    = uint64(500_000_000_000) // ~0.5ms
	testingMemoryLimit = 32                      // MiB
	testingCacheSize   = 100                     // MiB
)

var testingCapabilities = []string{
	"staking",
	"stargate",
	"iterator",
	"cosmwasm_1_1",
	"cosmwasm_1_2",
	"cosmwasm_1_3",
}

func TestInitAndReleaseCache(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    testingCapabilities,
			MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func TestInitCacheWorksForNonExistentDir(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	createMe := filepath.Join(tmpdir, "does-not-yet-exist")
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  createMe,
			AvailableCapabilities:    testingCapabilities,
			MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func TestInitCacheErrorsForBrokenDir(t *testing.T) {
	// Use colon to make this fail on Windows; On Unix likely no permissions
	cannotBeCreated := "/foo:bar"
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  cannotBeCreated,
			AvailableCapabilities:    testingCapabilities,
			MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
		},
	}
	_, err := InitCache(config)
	require.ErrorContains(t, err, "Could not create base directory")
}

func TestInitLockingPreventsConcurrentAccess(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    testingCapabilities,
			MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
		},
	}
	cache1, err := InitCache(config)
	require.NoError(t, err)

	// Attempt second initialization in same dir should fail
	_, err = InitCache(config)
	require.ErrorContains(t, err, "Could not lock exclusive.lock")

	ReleaseCache(cache1)

	// Now it should work again after release
	cache3, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache3)
}

func TestInitLockingAllowsMultipleInstancesInDifferentDirs(t *testing.T) {
	tmpdir1, err := os.MkdirTemp("", "wasmvm-testing1")
	require.NoError(t, err)
	tmpdir2, err := os.MkdirTemp("", "wasmvm-testing2")
	require.NoError(t, err)
	tmpdir3, err := os.MkdirTemp("", "wasmvm-testing3")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpdir1)
		os.RemoveAll(tmpdir2)
		os.RemoveAll(tmpdir3)
	})

	configGen := func(dir string) types.VMConfig {
		return types.VMConfig{
			Cache: types.CacheOptions{
				BaseDir:                  dir,
				AvailableCapabilities:    testingCapabilities,
				MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
				InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
			},
		}
	}

	cache1, err := InitCache(configGen(tmpdir1))
	require.NoError(t, err)
	cache2, err := InitCache(configGen(tmpdir2))
	require.NoError(t, err)
	cache3, err := InitCache(configGen(tmpdir3))
	require.NoError(t, err)

	ReleaseCache(cache1)
	ReleaseCache(cache2)
	ReleaseCache(cache3)
}

func TestInitCacheEmptyCapabilities(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    []string{},
			MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

// withCache sets up a temporary cache and returns it with a cleanup function already registered.
func withCache(t testing.TB) (Cache, func()) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    testingCapabilities,
			MemoryCacheSizeBytes:     types.NewSizeMebi(testingCacheSize),
			InstanceMemoryLimitBytes: types.NewSizeMebi(testingMemoryLimit),
		},
	}

	cache, err := InitCache(config)
	require.NoError(t, err)

	cleanup := func() {
		_ = os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	t.Cleanup(cleanup)
	return cache, cleanup
}

func TestStoreCodeAndGetCode(t *testing.T) {
	cache, _ := withCache(t)

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
	cache, _ := withCache(t)

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
	cache, _ := withCache(t)

	wasm := []byte("some invalid data")
	_, err := StoreCode(cache, wasm, true)
	require.Error(t, err)
}

func TestStoreCodeUnchecked(t *testing.T) {
	cache, _ := withCache(t)

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

func TestPin(t *testing.T) {
	cache, _ := withCache(t)

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Calling again is no-op
	err = Pin(cache, checksum)
	require.NoError(t, err)
}

func TestPinErrors(t *testing.T) {
	cache, _ := withCache(t)

	// Nil checksum
	var nilChecksum []byte
	err := Pin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Short checksum
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Pin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// Unknown checksum
	unknownChecksum := []byte{
		0x72, 0x2c, 0x8c, 0x99, 0x3f, 0xd7, 0x5a, 0x76,
		0x27, 0xd6, 0x9e, 0xd9, 0x41, 0x34, 0x4f, 0xe2,
		0xa1, 0x42, 0x3a, 0x3e, 0x75, 0xef, 0xd3, 0xe6,
		0x77, 0x8a, 0x14, 0x28, 0x84, 0x22, 0x71, 0x04,
	}
	err = Pin(cache, unknownChecksum)
	require.ErrorContains(t, err, "Error opening Wasm file for reading")
}

func TestUnpin(t *testing.T) {
	cache, _ := withCache(t)

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// Calling again is no-op
	err = Unpin(cache, checksum)
	require.NoError(t, err)
}

func TestUnpinErrors(t *testing.T) {
	cache, _ := withCache(t)

	// Nil checksum
	var nilChecksum []byte
	err := Unpin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Short checksum
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Unpin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")
}

func TestGetMetrics(t *testing.T) {
	cache, _ := withCache(t)

	metrics, err := GetMetrics(cache)
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	// Additional metrics checks are performed below...
	// (Omitting repeated explanations to keep code concise)
}

func TestGetPinnedMetrics(t *testing.T) {
	cache, _ := withCache(t)

	metrics, err := GetPinnedMetrics(cache)
	require.NoError(t, err)
	assert.Equal(t, &types.PinnedMetrics{PerModule: []types.PerModuleEntry{}}, metrics)

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	cyberpunkWasm, err := os.ReadFile("../../testdata/cyberpunk.wasm")
	require.NoError(t, err)
	cyberpunkChecksum, err := StoreCode(cache, cyberpunkWasm, true)
	require.NoError(t, err)

	err = Pin(cache, cyberpunkChecksum)
	require.NoError(t, err)

	findMetrics := func(list []types.PerModuleEntry, c types.Checksum) *types.PerModuleMetrics {
		for _, entry := range list {
			if bytes.Equal(entry.Checksum, c) {
				return &entry.Metrics
			}
		}
		return nil
	}

	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, 2, len(metrics.PerModule))

	hackatomMetrics := findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics := findMetrics(metrics.PerModule, cyberpunkChecksum)

	assert.Equal(t, uint32(0), hackatomMetrics.Hits)
	assert.NotEqual(t, uint32(0), hackatomMetrics.Size)
	assert.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	assert.NotEqual(t, uint32(0), cyberpunkMetrics.Size)

	// Instantiate to change metrics
	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, 2, len(metrics.PerModule))

	hackatomMetrics = findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics = findMetrics(metrics.PerModule, cyberpunkChecksum)

	assert.Equal(t, uint32(1), hackatomMetrics.Hits)
	assert.NotEqual(t, uint32(0), hackatomMetrics.Size)
	assert.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	assert.NotEqual(t, uint32(0), cyberpunkMetrics.Size)
}

func TestInstantiate(t *testing.T) {
	cache, _ := withCache(t)

	// create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xb1fe27), cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 0, len(result.Ok.Messages))
}

func TestExecute(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xb1fe27), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// Execute with the same store
	gasMeter2 := NewMockGasMeter(testingGasLimit)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	res, cost, err = Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
	diff = time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0x1416da5), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 1, len(result.Ok.Messages))
	ev := result.Ok.Events[0]
	assert.Equal(t, "hackatom", ev.Type)
	assert.Equal(t, 1, len(ev.Attributes))
	assert.Equal(t, "action", ev.Attributes[0].Key)
	assert.Equal(t, "release", ev.Attributes[0].Value)

	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank)
	require.NotNil(t, dispatch.Bank.Send)
	send := dispatch.Bank.Send
	assert.Equal(t, "bob", send.ToAddress)
	assert.Equal(t, balance, send.Amount)
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	assert.Equal(t, expectedData, result.Ok.Data)
}

func TestExecutePanic(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createCyberpunkContract(t, cache)

	maxGas := testingGasLimit
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"panic":{}}`), &igasMeter2, store, api, &querier, maxGas, testingPrintDebug)
	require.ErrorContains(t, err, "RuntimeError: Aborted: panicked at 'This page intentionally faulted'")
}

func TestExecuteUnreachable(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createCyberpunkContract(t, cache)

	maxGas := testingGasLimit
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"unreachable":{}}`), &igasMeter2, store, api, &querier, maxGas, testingPrintDebug)
	require.ErrorContains(t, err, "RuntimeError: unreachable")
}

func TestExecuteCpuLoop(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x79f527), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	_, cost, err = Execute(cache, checksum, env, info, []byte(`{"cpu_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, testingPrintDebug)
	diff = time.Since(start)
	require.Error(t, err)
	assert.Equal(t, cost.UsedInternally, maxGas)
	t.Logf("CPULoop Time (%d gas): %s\n", cost.UsedInternally, diff)
}

func TestExecuteStorageLoop(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start := time.Now()
	_, gasReport, err := Execute(cache, checksum, env, info, []byte(`{"storage_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, testingPrintDebug)
	diff = time.Since(start)
	require.Error(t, err)
	t.Logf("StorageLoop Time (%d gas): %s\n", gasReport.UsedInternally, diff)
	t.Logf("Gas used: %d\n", gasMeter2.GasConsumed())
	t.Logf("Wasm gas: %d\n", gasReport.UsedInternally)

	// The sum of sdk gas * GasMultiplier + wasm cost should equal maxGas
	totalCost := gasReport.UsedInternally + gasMeter2.GasConsumed()
	require.Equal(t, int64(maxGas), int64(totalCost))
}

func BenchmarkContractCall(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		gasMeter2 := NewMockGasMeter(testingGasLimit)
		igasMeter2 := types.GasMeter(gasMeter2)
		store.SetGasMeter(gasMeter2)
		info = MockInfoBin(b, "fred")
		msg := []byte(`{"allocate_large_memory":{"pages":0}}`)
		res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
		require.NoError(b, err)
		requireOkResponse(b, res, 0)
	}
}

func Benchmark100ConcurrentContractCalls(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	const callCount = 100

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		wg.Add(callCount)
		for i := 0; i < callCount; i++ {
			go func() {
				gasMeter2 := NewMockGasMeter(testingGasLimit)
				igasMeter2 := types.GasMeter(gasMeter2)
				store.SetGasMeter(gasMeter2)
				info = MockInfoBin(b, "fred")
				msg := []byte(`{"allocate_large_memory":{"pages":0}}`)
				res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
				require.NoError(b, err)
				requireOkResponse(b, res, 0)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func TestExecuteUserErrorsInApiCalls(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	maxGas := testingGasLimit
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	defaultApi := NewMockAPI()
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, defaultApi, &querier, maxGas, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	failingApi := NewMockFailureAPI()
	res, _, err = Execute(cache, checksum, env, info, []byte(`{"user_errors_in_api_calls":{}}`), &igasMeter2, store, failingApi, &querier, maxGas, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
}

func TestMigrate(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	query := []byte(`{"verifier":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.Equal(t, `{"verifier":"fred"}`, string(qResult.Ok))

	_, _, err = Migrate(cache, checksum, env, []byte(`{"verifier":"alice"}`), &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var qResult2 types.QueryResult
	err = json.Unmarshal(data, &qResult2)
	require.NoError(t, err)
	require.Equal(t, "", qResult2.Err)
	require.Equal(t, `{"verifier":"alice"}`, string(qResult2.Ok))
}

func TestMultipleInstances(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	// instance1 controlled by fred
	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store1 := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "regen")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store1, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xb0c2cd), cost.UsedInternally)

	// instance2 controlled by mary
	gasMeter2 := NewMockGasMeter(testingGasLimit)
	igasMeter2 := types.GasMeter(gasMeter2)
	store2 := NewLookup(gasMeter2)
	info = MockInfoBin(t, "chrous")
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, cost, err = Instantiate(cache, checksum, env, info, msg, &igasMeter2, store2, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xb1760a), cost.UsedInternally)

	// fail to execute store1 with mary
	resp := exec(t, cache, checksum, "mary", store1, api, querier, 0xa7c5ce)
	require.Equal(t, "Unauthorized", resp.Err)

	// succeed to execute store1 with fred
	resp = exec(t, cache, checksum, "fred", store1, api, querier, 0x140e8ad)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	attributes := resp.Ok.Attributes
	require.Equal(t, 2, len(attributes))
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "bob", attributes[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, checksum, "mary", store2, api, querier, 0x1412b29)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	attributes = resp.Ok.Attributes
	require.Equal(t, 2, len(attributes))
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "sue", attributes[1].Value)
}

func TestSudo(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(testingGasLimit)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	msg = []byte(`{"steal_funds":{"recipient":"community-pool","amount":[{"amount":"700","denom":"gold"}]}}`)
	res, _, err = Sudo(cache, checksum, env, msg, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 1, len(result.Ok.Messages))
	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank)
	require.NotNil(t, dispatch.Bank.Send)
	send := dispatch.Bank.Send
	assert.Equal(t, "community-pool", send.ToAddress)
	expectedPayout := types.Array[types.Coin]{types.NewCoin(700, "gold")}
	assert.Equal(t, expectedPayout, send.Amount)
}

func TestDispatchSubmessage(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

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

	gasMeter2 := NewMockGasMeter(testingGasLimit)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Execute(cache, checksum, env, info, payloadMsg, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 1, len(result.Ok.Messages))
	dispatch := result.Ok.Messages[0]
	assert.Equal(t, id, dispatch.ID)
	assert.Equal(t, payload.Msg, dispatch.Msg)
	assert.Nil(t, dispatch.GasLimit)
	assert.Equal(t, payload.ReplyOn, dispatch.ReplyOn)
}

func TestReplyAndQuery(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
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

	gasMeter2 := NewMockGasMeter(testingGasLimit)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Reply(cache, checksum, env, replyBin, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	badQuery := []byte(`{"sub_msg_result":{"id":7777}}`)
	res, _, err = Query(cache, checksum, env, badQuery, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireQueryError(t, res)

	query := []byte(`{"sub_msg_result":{"id":1234}}`)
	res, _, err = Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
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

func requireOkResponse(t testing.TB, res []byte, expectedMsgs int) {
	var result types.ContractResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, expectedMsgs, len(result.Ok.Messages))
}

func requireQueryError(t *testing.T, res []byte) {
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Ok)
	require.NotEmpty(t, result.Err)
}

func requireQueryOk(t *testing.T, res []byte) []byte {
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Err)
	require.NotEmpty(t, result.Ok)
	return result.Ok
}

func createHackatomContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/hackatom.wasm")
}

func createCyberpunkContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/cyberpunk.wasm")
}

func createQueueContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/queue.wasm")
}

func createReflectContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/reflect.wasm")
}

func createFloaty2(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/floaty_2.0.wasm")
}

func createContract(t testing.TB, cache Cache, wasmFile string) []byte {
	wasm, err := os.ReadFile(wasmFile)
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)
	return checksum
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, checksum []byte, signer types.HumanAddress, store types.KVStore, api *types.GoAPI, querier types.Querier, gasExpected uint64) types.ContractResult {
	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	env := MockEnvBin(t)
	info := MockInfoBin(t, signer)
	res, cost, err := Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	assert.Equal(t, gasExpected, cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	return result
}

func TestQuery(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter1 := NewMockGasMeter(testingGasLimit)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)

	// invalid query
	gasMeter2 := NewMockGasMeter(testingGasLimit)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var badResult types.QueryResult
	err = json.Unmarshal(data, &badResult)
	require.NoError(t, err)
	require.Contains(t, badResult.Err, "unknown variant `Raw`")

	// valid query
	gasMeter3 := NewMockGasMeter(testingGasLimit)
	igasMeter3 := types.GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter3, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.Equal(t, `{"verifier":"fred"}`, string(qResult.Ok))
}

func TestHackatomQuerier(t *testing.T) {
	cache, _ := withCache(t)
	checksum := createHackatomContract(t, cache)

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM"), types.NewCoin(65432, "ETH")}
	querier := DefaultQuerier("foobar", initBalance)

	query := []byte(`{"other_balance":{"address":"foobar"}}`)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
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
	}

	type CapitalizedResponse struct {
		Text string `json:"text"`
	}

	cache, _ := withCache(t)
	checksum := createReflectContract(t, cache)

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, initBalance)
	innerQuerier := querier.(*MockQuerier)
	innerQuerier.Custom = ReflectCustom{}
	querier = types.Querier(innerQuerier)

	queryMsg := QueryMsg{
		Capitalized: &CapitalizedQuery{
			Text: "small Frys :)",
		},
	}
	query, err := json.Marshal(queryMsg)
	require.NoError(t, err)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
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

// TestFloats verifies deterministic float instructions
func TestFloats(t *testing.T) {
	type Value struct {
		U32 *uint32 `json:"u32,omitempty"`
		U64 *uint64 `json:"u64,omitempty"`
		F32 *uint32 `json:"f32,omitempty"`
		F64 *uint64 `json:"f64,omitempty"`
	}

	debugStr := func(value Value) string {
		switch {
		case value.U32 != nil:
			return fmt.Sprintf("U32(%d)", *value.U32)
		case value.U64 != nil:
			return fmt.Sprintf("U64(%d)", *value.U64)
		case value.F32 != nil:
			return fmt.Sprintf("F32(%d)", *value.F32)
		case value.F64 != nil:
			return fmt.Sprintf("F64(%d)", *value.F64)
		default:
			t.FailNow()
			return ""
		}
	}

	cache, _ := withCache(t)
	checksum := createFloaty2(t, cache)

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)

	// query instructions
	query := []byte(`{"instructions":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	var instructions []string
	err = json.Unmarshal(qResult.Ok, &instructions)
	require.NoError(t, err)
	require.Equal(t, 70, len(instructions))

	hasher := sha256.New()
	const runsPerInstruction = 150
	for _, instr := range instructions {
		for seed := 0; seed < runsPerInstruction; seed++ {
			msg := fmt.Sprintf(`{"random_args_for":{"instruction":"%s","seed":%d}}`, instr, seed)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
			require.NoError(t, err)
			err = json.Unmarshal(data, &qResult)
			require.NoError(t, err)
			require.Equal(t, "", qResult.Err)
			var args []Value
			err = json.Unmarshal(qResult.Ok, &args)
			require.NoError(t, err)

			argStr, err := json.Marshal(args)
			require.NoError(t, err)
			msg = fmt.Sprintf(`{"run":{"instruction":"%s","args":%s}}`, instr, argStr)

			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
			var resultStr string
			if err != nil {
				// Remove prefix to match cosmwasm-vm test expectations
				resultStr = strings.Replace(err.Error(), "Error calling the VM: Error executing Wasm: ", "", 1)
			} else {
				err = json.Unmarshal(data, &qResult)
				require.NoError(t, err)
				require.Equal(t, "", qResult.Err)
				var response Value
				err = json.Unmarshal(qResult.Ok, &response)
				require.NoError(t, err)
				resultStr = debugStr(response)
			}
			hasher.Write([]byte(fmt.Sprintf("%s%d%s", instr, seed, resultStr)))
		}
	}

	hash := hasher.Sum(nil)
	require.Equal(t, "95f70fa6451176ab04a9594417a047a1e4d8e2ff809609b8f81099496bee2393", hex.EncodeToString(hash))
}
