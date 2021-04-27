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

	checksum, err := vm.Create(wasm)
	require.NoError(t, err)

	code, err := vm.GetCode(checksum)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

// IBCInstantiateMsg is the Go version of
// https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/contracts/ibc-reflect/src/msg.rs#L9-L11
type IBCInstantiateMsg struct {
	ReflectCodeID uint64 `json:"reflect_code_id"`
}

// IBCExecuteMsg is the Go version of
// https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/contracts/ibc-reflect/src/msg.rs#L15
type IBCExecuteMsg struct {
	InitCallback InitCallback `json:"init_callback"`
}

// InitCallback is the Go version of
// https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/contracts/ibc-reflect/src/msg.rs#L17-L22
type InitCallback struct {
	ID           string `json:"id"`
	ContractAddr string `json:"contract_addr"`
}

type IBCPacketMsg struct {
	Dispatch *DispatchMsg `json:"dispatch,omitempty"`
}

type DispatchMsg struct {
	Msgs []types.CosmosMsg `json:"msgs"`
}

type IBCQueryMsg struct {
	ListAccounts *struct{} `json:"list_accounts,omitempty"`
}

type ListAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
}

type AccountInfo struct {
	Account   string `json:"account"`
	ChannelID string `json:"channel_id"`
}

// We just check if an error is returned or not
type AcknowledgeDispatch struct {
	Err string `json:"error"`
}

func toBytes(t *testing.T, v interface{}) []byte {
	bz, err := json.Marshal(v)
	require.NoError(t, err)
	return bz
}

const IBC_VERSION = "ibc-reflect-v1"

func TestIBCHandshake(t *testing.T) {
	// code id of the reflect contract
	const REFLECT_ID uint64 = 101
	// channel id for handshake
	const CHANNEL_ID = "channel-432"

	vm := withVM(t)
	checksum := createTestContract(t, vm, IBC_TEST_CONTRACT)
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := IBCInstantiateMsg{
		ReflectCodeID: REFLECT_ID,
	}
	ires, _, err := vm.Instantiate(checksum, env, info, toBytes(t, msg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	// fails on bad version
	channel := api.MockIBCChannel(CHANNEL_ID, types.Ordered, "random-garbage")
	_, err = vm.IBCChannelOpen(checksum, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.Error(t, err)
	// passes on good version
	channel = api.MockIBCChannel(CHANNEL_ID, types.Ordered, IBC_VERSION)
	channel.CounterpartyVersion = types.NewOptionalStringUnset()
	_, err = vm.IBCChannelOpen(checksum, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	env = api.MockEnv()
	// completes and dispatches message to create reflect contract
	channel = api.MockIBCChannel(CHANNEL_ID, types.Ordered, IBC_VERSION)
	res, _, err := vm.IBCChannelConnect(checksum, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
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

func TestIBCPacketDispatch(t *testing.T) {
	// code id of the reflect contract
	const REFLECT_ID uint64 = 77
	// address of first reflect contract instance that we created
	const REFLECT_ADDR = "reflect-acct-1"
	// channel id for handshake
	const CHANNEL_ID = "channel-234"

	// setup
	vm := withVM(t)
	checksum := createTestContract(t, vm, IBC_TEST_CONTRACT)
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{
		ReflectCodeID: REFLECT_ID,
	}
	_, _, err := vm.Instantiate(checksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	channel := api.MockIBCChannel(CHANNEL_ID, types.Ordered, IBC_VERSION)
	channel.CounterpartyVersion = types.NewOptionalStringUnset()
	_, err = vm.IBCChannelOpen(checksum, env, channel, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	// completes and dispatches message to create reflect contract
	channel = api.MockIBCChannel(CHANNEL_ID, types.Ordered, IBC_VERSION)
	_, _, err = vm.IBCChannelConnect(checksum, env, channel, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// mock reflect init callback (to store address)
	gasMeter4 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter4)
	handleMsg := IBCExecuteMsg{
		InitCallback: InitCallback{
			ID:           CHANNEL_ID,
			ContractAddr: REFLECT_ADDR,
		},
	}
	info = api.MockInfo(REFLECT_ADDR, nil)
	_, _, err = vm.Execute(checksum, env, info, toBytes(t, handleMsg), store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// ensure the channel is registered
	queryMsg := IBCQueryMsg{
		ListAccounts: &struct{}{},
	}
	qres, _, err := vm.Query(checksum, env, toBytes(t, queryMsg), store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	var accounts ListAccountsResponse
	err = json.Unmarshal(qres, &accounts)
	require.Equal(t, 1, len(accounts.Accounts))
	require.Equal(t, CHANNEL_ID, accounts.Accounts[0].ChannelID)
	require.Equal(t, REFLECT_ADDR, accounts.Accounts[0].Account)

	// process message received on this channel
	gasMeter5 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter5)
	ibcMsg := IBCPacketMsg{
		Dispatch: &DispatchMsg{
			Msgs: []types.CosmosMsg{{
				Bank: &types.BankMsg{Send: &types.SendMsg{
					ToAddress: "my-friend",
					Amount:    types.Coins{types.NewCoin(12345678, "uatom")},
				}},
			}},
		},
	}
	packet := api.MockIBCPacket(CHANNEL_ID, toBytes(t, ibcMsg))
	pres, _, err := vm.IBCPacketReceive(checksum, env, packet, store, *goapi, querier, gasMeter5, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	// assert app-level success
	var ack AcknowledgeDispatch
	err = json.Unmarshal(pres.Acknowledgement, &ack)
	require.Empty(t, ack.Err)

	// error on message from another channel
	packet2 := api.MockIBCPacket("no-such-channel", toBytes(t, ibcMsg))
	pres2, _, err := vm.IBCPacketReceive(checksum, env, packet2, store, *goapi, querier, gasMeter5, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// assert app-level failure
	var ack2 AcknowledgeDispatch
	err = json.Unmarshal(pres2.Acknowledgement, &ack2)
	require.Equal(t, "invalid packet: cosmwasm_std::addresses::Addr not found", ack2.Err)
}

func TestAnalyzeCode(t *testing.T) {
	vm := withVM(t)

	// instantiate non-ibc contract
	wasm, err := ioutil.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)
	checksum, err := vm.Create(wasm)
	require.NoError(t, err)
	// and analyze
	report, err := vm.AnalyzeCode(checksum)
	require.NoError(t, err)
	require.False(t, report.HasIBCEntryPoints)

	// instantiate ibc contract
	wasm2, err := ioutil.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)
	checksum2, err := vm.Create(wasm2)
	require.NoError(t, err)
	// and analyze
	report2, err := vm.AnalyzeCode(checksum2)
	require.NoError(t, err)
	require.True(t, report2.HasIBCEntryPoints)
}
