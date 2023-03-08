//go:build cgo

package cosmwasm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/internal/api"
	"github.com/CosmWasm/wasmvm/types"
)

const IBCTestContract = "./testdata/ibc_reflect.wasm"

func TestIBC(t *testing.T) {
	vm := withVM(t)

	wasm, err := os.ReadFile(IBCTestContract)
	require.NoError(t, err)

	checksum, err := vm.StoreCode(wasm)
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

const IBCVersion = "ibc-reflect-v1"

func TestIBCHandshake(t *testing.T) {
	// code id of the reflect contract
	const ReflectID uint64 = 101
	// channel id for handshake
	const ChannelID = "channel-432"

	vm := withVM(t)
	checksum := createTestContract(t, vm, IBCTestContract)
	gasMeter1 := api.NewMockGasMeter(TestingGasLimit)
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MockContractAddr, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{
		ReflectCodeID: ReflectID,
	}
	ires, _, err := vm.Instantiate(checksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TestingGasLimit, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// channel open
	gasMeter2 := api.NewMockGasMeter(TestingGasLimit)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	openMsg := api.MockIBCChannelOpenInit(ChannelID, types.Ordered, IBCVersion)
	ores, _, err := vm.IBCChannelOpen(checksum, env, openMsg, store, *goapi, querier, gasMeter2, TestingGasLimit, deserCost)
	require.NoError(t, err)
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, ores)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TestingGasLimit)
	store.SetGasMeter(gasMeter3)
	env = api.MockEnv()
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(ChannelID, types.Ordered, IBCVersion)
	res, _, err := vm.IBCChannelConnect(checksum, env, connectMsg, store, *goapi, querier, gasMeter2, TestingGasLimit, deserCost)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Messages))

	// check for the expected custom event
	expectedEvents := []types.Event{{
		Type: "ibc",
		Attributes: []types.EventAttribute{{
			Key:   "channel",
			Value: "connect",
		}},
	}}
	require.Equal(t, expectedEvents, res.Events)

	// make sure it read the balance properly and we got 250 atoms
	dispatch := res.Messages[0].Msg
	require.NotNil(t, dispatch.Wasm, "%#v", dispatch)
	require.NotNil(t, dispatch.Wasm.Instantiate, "%#v", dispatch)
	init := dispatch.Wasm.Instantiate
	assert.Equal(t, ReflectID, init.CodeID)
	assert.Empty(t, init.Funds)
}

func TestIBCPacketDispatch(t *testing.T) {
	// code id of the reflect contract
	const ReflectID uint64 = 77
	// address of first reflect contract instance that we created
	const ReflectAddress = "reflect-acct-1"
	// channel id for handshake
	const ChannelID = "channel-234"

	// setup
	vm := withVM(t)
	checksum := createTestContract(t, vm, IBCTestContract)
	gasMeter1 := api.NewMockGasMeter(TestingGasLimit)
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MockContractAddr, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{
		ReflectCodeID: ReflectID,
	}
	_, _, err := vm.Instantiate(checksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TestingGasLimit, deserCost)
	require.NoError(t, err)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TestingGasLimit)
	store.SetGasMeter(gasMeter2)
	openMsg := api.MockIBCChannelOpenInit(ChannelID, types.Ordered, IBCVersion)
	ores, _, err := vm.IBCChannelOpen(checksum, env, openMsg, store, *goapi, querier, gasMeter2, TestingGasLimit, deserCost)
	require.NoError(t, err)
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, ores)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TestingGasLimit)
	store.SetGasMeter(gasMeter3)
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(ChannelID, types.Ordered, IBCVersion)
	res, _, err := vm.IBCChannelConnect(checksum, env, connectMsg, store, *goapi, querier, gasMeter3, TestingGasLimit, deserCost)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Messages))
	id := res.Messages[0].ID

	// mock reflect init callback (to store address)
	gasMeter4 := api.NewMockGasMeter(TestingGasLimit)
	store.SetGasMeter(gasMeter4)
	reply := types.Reply{
		ID: id,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: types.Events{{
					Type: "instantiate",
					Attributes: types.EventAttributes{
						{
							Key:   "_contract_address",
							Value: ReflectAddress,
						},
					},
				}},
				Data: nil,
			},
		},
	}
	_, _, err = vm.Reply(checksum, env, reply, store, *goapi, querier, gasMeter4, TestingGasLimit, deserCost)
	require.NoError(t, err)

	// ensure the channel is registered
	queryMsg := IBCQueryMsg{
		ListAccounts: &struct{}{},
	}
	qres, _, err := vm.Query(checksum, env, toBytes(t, queryMsg), store, *goapi, querier, gasMeter4, TestingGasLimit, deserCost)
	require.NoError(t, err)
	var accounts ListAccountsResponse
	err = json.Unmarshal(qres, &accounts)
	require.NoError(t, err)
	require.Equal(t, 1, len(accounts.Accounts))
	require.Equal(t, ChannelID, accounts.Accounts[0].ChannelID)
	require.Equal(t, ReflectAddress, accounts.Accounts[0].Account)

	// process message received on this channel
	gasMeter5 := api.NewMockGasMeter(TestingGasLimit)
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
	msg := api.MockIBCPacketReceive(ChannelID, toBytes(t, ibcMsg))
	pr, _, err := vm.IBCPacketReceive(checksum, env, msg, store, *goapi, querier, gasMeter5, TestingGasLimit, deserCost)
	require.NoError(t, err)
	assert.NotNil(t, pr.Ok)
	pres := pr.Ok

	// assert app-level success
	var ack AcknowledgeDispatch
	err = json.Unmarshal(pres.Acknowledgement, &ack)
	require.NoError(t, err)
	require.Empty(t, ack.Err)

	// error on message from another channel
	msg2 := api.MockIBCPacketReceive("no-such-channel", toBytes(t, ibcMsg))
	pr2, _, err := vm.IBCPacketReceive(checksum, env, msg2, store, *goapi, querier, gasMeter5, TestingGasLimit, deserCost)
	require.NoError(t, err)
	assert.NotNil(t, pr.Ok)
	pres2 := pr2.Ok
	// assert app-level failure
	var ack2 AcknowledgeDispatch
	_ = json.Unmarshal(pres2.Acknowledgement, &ack2)
	require.Equal(t, "invalid packet: cosmwasm_std::addresses::Addr not found", ack2.Err)

	// check for the expected custom event
	expectedEvents := []types.Event{{
		Type: "ibc",
		Attributes: []types.EventAttribute{{
			Key:   "packet",
			Value: "receive",
		}},
	}}
	require.Equal(t, expectedEvents, pres2.Events)
}

func TestAnalyzeCode(t *testing.T) {
	vm := withVM(t)

	// Store non-IBC contract
	wasm, err := os.ReadFile(HackatomTestContract)
	require.NoError(t, err)
	checksum, err := vm.StoreCode(wasm)
	require.NoError(t, err)
	// and analyze
	report, err := vm.AnalyzeCode(checksum)
	require.NoError(t, err)
	require.False(t, report.HasIBCEntryPoints)
	require.Equal(t, "", report.RequiredFeatures) //nolint:staticcheck // ignore deprecation
	require.Equal(t, "", report.RequiredCapabilities)

	// Store IBC contract
	wasm2, err := os.ReadFile(IBCTestContract)
	require.NoError(t, err)
	checksum2, err := vm.StoreCode(wasm2)
	require.NoError(t, err)
	// and analyze
	report2, err := vm.AnalyzeCode(checksum2)
	require.NoError(t, err)
	require.True(t, report2.HasIBCEntryPoints)
	require.Equal(t, "iterator,stargate", report2.RequiredFeatures) //nolint:staticcheck // ignore deprecation
	require.Equal(t, "iterator,stargate", report2.RequiredCapabilities)
}

func TestIBCMsgGetChannel(t *testing.T) {
	const ChannelID = "channel-432"

	msg1 := api.MockIBCChannelOpenInit(ChannelID, types.Ordered, "random-garbage")
	msg2 := api.MockIBCChannelOpenTry(ChannelID, types.Ordered, "random-garbage")
	msg3 := api.MockIBCChannelConnectAck(ChannelID, types.Ordered, "random-garbage")
	msg4 := api.MockIBCChannelConnectConfirm(ChannelID, types.Ordered, "random-garbage")
	msg5 := api.MockIBCChannelCloseInit(ChannelID, types.Ordered, "random-garbage")
	msg6 := api.MockIBCChannelCloseConfirm(ChannelID, types.Ordered, "random-garbage")

	require.Equal(t, msg1.GetChannel(), msg2.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg3.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg4.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg5.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg6.GetChannel())
	require.Equal(t, msg1.GetChannel().Endpoint.ChannelID, ChannelID)
}

func TestIBCMsgGetCounterVersion(t *testing.T) {
	const ChannelID = "channel-432"
	const VERSION = "random-garbage"

	msg1 := api.MockIBCChannelOpenInit(ChannelID, types.Ordered, VERSION)
	_, ok := msg1.GetCounterVersion()
	require.False(t, ok)

	msg2 := api.MockIBCChannelOpenTry(ChannelID, types.Ordered, VERSION)
	v, ok := msg2.GetCounterVersion()
	require.True(t, ok)
	require.Equal(t, VERSION, v)

	msg3 := api.MockIBCChannelConnectAck(ChannelID, types.Ordered, VERSION)
	v, ok = msg3.GetCounterVersion()
	require.True(t, ok)
	require.Equal(t, VERSION, v)

	msg4 := api.MockIBCChannelConnectConfirm(ChannelID, types.Ordered, VERSION)
	_, ok = msg4.GetCounterVersion()
	require.False(t, ok)
}
