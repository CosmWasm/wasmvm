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

func createTestContract(t *testing.T, vm *VM, path string) CodeID {
	wasm, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	id, err := vm.Create(wasm)
	require.NoError(t, err)
	return id
}

func TestCreateAndGet(t *testing.T) {
	vm := withVM(t)

	wasm, err := ioutil.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	id, err := vm.Create(wasm)
	require.NoError(t, err)

	code, err := vm.GetCode(id)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

func TestHappyPath(t *testing.T) {
	vm := withVM(t)
	id := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// init
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	ires, flags, _, err := vm.Instantiate(id, env, info, msg, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))
	assert.False(t, flags.IBCEnabled)
	assert.False(t, flags.Stargate)

	// handle
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	hres, _, err := vm.Execute(id, env, info, []byte(`{"release":{}}`), store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
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
