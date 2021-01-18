package cosmwasm

import (
	"encoding/json"
	"github.com/CosmWasm/wasmvm/api"
	"github.com/CosmWasm/wasmvm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

const IBC_TEST_CONTRACT = "./api/testdata/ibc_reflect.wasm"

func TestIBC(t *testing.T) {
	vm := withVM(t)

	wasm, err := ioutil.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)

	id, err := vm.Create(wasm)
	require.NoError(t, err)

	code, err := vm.GetCode(id)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

type IBCInitMsg struct {
	ReflectCodeID uint64 `json:"reflect_code_id"`
}

func toBytes(t *testing.T, v interface{}) []byte {
	bz, err := json.Marshal(v)
	require.NoError(t, err)
	return bz
}

func TestIBCHandshake(t *testing.T) {
	// code id of the reflect contract
	const REFLECT_ID uint64 = 101
	// address of first reflect contract instance that we created
	const REFLECT_ADDR = "reflect-acct-1"
	// channel id for handshake
	const CHANNEL_ID = "channel-432"

	vm := withVM(t)
	id := createTestContract(t, vm, IBC_TEST_CONTRACT)
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// init
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := IBCInitMsg{
		ReflectCodeID: REFLECT_ID,
	}
	ires, _, err := vm.Instantiate(id, env, info, toBytes(t, msg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	// fails on bad version
	channel := api.MockIBCChannel(CHANNEL_ID, types.Ordered, "random-garbage")
	_, err = vm.IBCChannelOpen(id, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.Error(t, err)
	// passes on good version
	channel = api.MockIBCChannel(CHANNEL_ID, types.Ordered, "ibc-reflect")
	channel.CounterpartyVersion = ""
	_, err = vm.IBCChannelOpen(id, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	env = api.MockEnv()
	// completes and dispatches message to create reflect contract
	channel = api.MockIBCChannel(CHANNEL_ID, types.Ordered, "ibc-reflect")
	res, _, err := vm.IBCChannelConnect(id, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Messages))

	// make sure it read the balance properly and we got 250 atoms
	dispatch := res.Messages[0]
	require.NotNil(t, dispatch.Wasm, "%#v", dispatch)
	require.NotNil(t, dispatch.Wasm.Instantiate, "%#v", dispatch)
	init := dispatch.Wasm.Instantiate
	assert.Equal(t, REFLECT_ID, init.CodeID)
	assert.Empty(t, init.Send)
}
