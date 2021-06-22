package cosmwasm

import (
	"github.com/CosmWasm/wasmvm/api"
	"github.com/CosmWasm/wasmvm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

const TESTING_FEATURES = "staking,stargate"
const TESTING_PRINT_DEBUG = false
const TESTING_GAS_LIMIT = 100_000_000
const TESTING_MEMORY_LIMIT = 32 // MiB
const TESTING_CACHE_SIZE = 100  // MiB

const HACKATOM_TEST_CONTRACT = "./api/testdata/hackatom.wasm"

func withVM(t *testing.T) *VM {
	tmpdir, err := ioutil.TempDir("", "wasmvm-testing")
	require.NoError(t, err)
	vm, err := NewVM(tmpdir, TESTING_FEATURES, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG, TESTING_CACHE_SIZE)
	require.NoError(t, err)

	t.Cleanup(func() {
		vm.Cleanup()
		os.RemoveAll(tmpdir)
	})
	return vm
}

func createTestContract(t *testing.T, vm *VM, path string) Checksum {
	wasm, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	checksum, err := vm.Create(wasm)
	require.NoError(t, err)
	return checksum
}

func TestCreateAndGet(t *testing.T) {
	vm := withVM(t)

	wasm, err := ioutil.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	checksum, err := vm.Create(wasm)
	require.NoError(t, err)

	code, err := vm.GetCode(checksum)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

func TestHappyPath(t *testing.T) {
	vm := withVM(t)
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	ires, _, err := vm.Instantiate(checksum, env, info, msg, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// execute
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	hres, _, err := vm.Execute(checksum, env, info, []byte(`{"release":{}}`), store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 1, len(hres.Messages))

	// make sure it read the balance properly and we got 250 atoms
	dispatch := hres.Messages[0]
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "bob", send.ToAddress)
	assert.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	assert.Equal(t, expectedData, hres.Data)
}

func TestGetMetrics(t *testing.T) {
	vm := withVM(t)

	// GetMetrics 1
	metrics, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Create contract
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	// GetMetrics 2
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	ires, _, err := vm.Instantiate(checksum, env, info, msg1, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 3
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{
		HitsFsCache:         1,
		ElementsMemoryCache: 1,
		SizeMemoryCache:     3254696,
	}, metrics)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	ires, _, err = vm.Instantiate(checksum, env, info, msg2, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 4
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{
		HitsMemoryCache:     1,
		HitsFsCache:         1,
		ElementsMemoryCache: 1,
		SizeMemoryCache:     3254696,
	}, metrics)

	// Pin
	err = vm.Pin(checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{
		HitsMemoryCache:           2,
		HitsFsCache:               1,
		ElementsPinnedMemoryCache: 1,
		ElementsMemoryCache:       1,
		SizePinnedMemoryCache:     3254696,
		SizeMemoryCache:           3254696,
	}, metrics)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	ires, _, err = vm.Instantiate(checksum, env, info, msg3, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 6
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{
		HitsPinnedMemoryCache:     1,
		HitsMemoryCache:           2,
		HitsFsCache:               1,
		ElementsPinnedMemoryCache: 1,
		ElementsMemoryCache:       1,
		SizePinnedMemoryCache:     3254696,
		SizeMemoryCache:           3254696,
	}, metrics)

	// Unpin
	err = vm.Unpin(checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{
		HitsPinnedMemoryCache:     1,
		HitsMemoryCache:           2,
		HitsFsCache:               1,
		ElementsPinnedMemoryCache: 0,
		ElementsMemoryCache:       1,
		SizePinnedMemoryCache:     0,
		SizeMemoryCache:           3254696,
	}, metrics)

	// Instantiate 4
	msg4 := []byte(`{"verifier": "fred", "beneficiary": "jeff"}`)
	ires, _, err = vm.Instantiate(checksum, env, info, msg4, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 8
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{
		HitsPinnedMemoryCache:     1,
		HitsMemoryCache:           3,
		HitsFsCache:               1,
		ElementsPinnedMemoryCache: 0,
		ElementsMemoryCache:       1,
		SizePinnedMemoryCache:     0,
		SizeMemoryCache:           3254696,
	}, metrics)
}
