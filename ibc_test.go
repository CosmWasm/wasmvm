//go:build cgo && !nolink_libwasmvm

// Package wasmvm contains integration tests for the wasmvm package.
package wasmvm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

const IBC_TEST_CONTRACT = "./testdata/ibc_reflect.wasm"

func TestIBC(t *testing.T) {
	vm := withVM(t)

	wasm, err := os.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)

	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
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

func toBytes(t *testing.T, v any) []byte {
	t.Helper()
	bz, err := json.Marshal(v)
	require.NoError(t, err)
	return bz
}

const IBC_VERSION = "ibc-reflect-v1"

func TestIBCHandshake(t *testing.T) {
	// code id of the reflect contract
	const reflectID uint64 = 101
	// channel id for handshake
	const channelID = "channel-432"

	vm := withVM(t)
	checksum := createTestContract(t, vm, IBC_TEST_CONTRACT)
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	mockQuerier := api.DefaultQuerier(api.MockContractAddr, types.Array[types.Coin]{})
	querier := mockQuerier
	var gasMeter types.GasMeter = gasMeter1

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	init_msg := IBCInstantiateMsg{
		ReflectCodeID: reflectID,
	}
	envBytes, err := json.Marshal(env)
	require.NoError(t, err)
	infoBytes, err := json.Marshal(info)
	require.NoError(t, err)
	msgBytes := toBytes(t, init_msg)
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msgBytes,
		GasMeter:   &gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	i, _, err := api.Instantiate(params)
	require.NoError(t, err)
	var iResponse types.IBCBasicResult
	err = json.Unmarshal(i, &iResponse)
	require.NoError(t, err)
	require.NotNil(t, iResponse.Ok)
	require.Empty(t, iResponse.Ok.Messages)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	gasMeter = gasMeter2
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	openMsg := api.MockIBCChannelOpenInit(channelID, types.Ordered, IBC_VERSION)
	openMsgBytes, err := json.Marshal(openMsg)
	require.NoError(t, err)
	params = api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        openMsgBytes,
		GasMeter:   &gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	openResult, _, err := vm.IBCChannelOpen(params)
	require.NoError(t, err)
	var oResponse types.IBCChannelOpenResult
	err = json.Unmarshal(openResult, &oResponse)
	require.NoError(t, err)
	require.NotNil(t, oResponse.Ok)
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, oResponse.Ok)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(channelID, types.Ordered, IBC_VERSION)
	connectMsgBytes, err := json.Marshal(connectMsg)
	require.NoError(t, err)
	var gasMeter3GasMeter types.GasMeter = gasMeter3
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	connectParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        connectMsgBytes,
		GasMeter:   &gasMeter3GasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	conn, _, err := vm.IBCChannelConnect(connectParams)
	require.NoError(t, err)
	var connResponse types.IBCBasicResult
	err = json.Unmarshal(conn, &connResponse)
	require.NoError(t, err)
	require.NotNil(t, connResponse.Ok)
	require.Len(t, connResponse.Ok.Messages, 1)
	_ = connResponse.Ok.Messages[0].ID // We don't use this ID in TestIBCHandshake

	// check for the expected custom event
	expected_events := []types.Event{{
		Type: "ibc",
		Attributes: []types.EventAttribute{{
			Key:   "channel",
			Value: "connect",
		}},
	}}
	require.Equal(t, expected_events, connResponse.Ok.Events)

	// make sure it read the balance properly and we got 250 atoms
	dispatch := connResponse.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Wasm, "%#v", dispatch)
	require.NotNil(t, dispatch.Wasm.Instantiate, "%#v", dispatch)
	require.Equal(t, reflectID, dispatch.Wasm.Instantiate.CodeID)
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
	balance := types.Array[types.Coin]{}
	querier := api.DefaultQuerier(api.MockContractAddr, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{
		ReflectCodeID: REFLECT_ID,
	}
	envBytes, err := json.Marshal(env)
	require.NoError(t, err)
	infoBytes, err := json.Marshal(info)
	require.NoError(t, err)
	msgBytes := toBytes(t, initMsg)
	var gasMeter types.GasMeter = gasMeter1
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msgBytes,
		GasMeter:   &gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	_, err = vm.Instantiate(params)
	require.NoError(t, err)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	openMsg := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, IBC_VERSION)
	openMsgBytes, err := json.Marshal(openMsg)
	require.NoError(t, err)
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	var gasMeter2GasMeter types.GasMeter = gasMeter2
	channelParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        openMsgBytes,
		GasMeter:   &gasMeter2GasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	openResult, _, err := vm.IBCChannelOpen(channelParams)
	require.NoError(t, err)
	var oResponse types.IBCChannelOpenResult
	err = json.Unmarshal(openResult, &oResponse)
	require.NoError(t, err)
	require.NotNil(t, oResponse.Ok)
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, oResponse.Ok)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, IBC_VERSION)
	connectMsgBytes, err := json.Marshal(connectMsg)
	require.NoError(t, err)
	var gasMeter3GasMeter types.GasMeter = gasMeter3
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	connectParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        connectMsgBytes,
		GasMeter:   &gasMeter3GasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	conn, _, err := vm.IBCChannelConnect(connectParams)
	require.NoError(t, err)
	var connResponse types.IBCBasicResult
	err = json.Unmarshal(conn, &connResponse)
	require.NoError(t, err)
	require.NotNil(t, connResponse.Ok)
	require.Len(t, connResponse.Ok.Messages, 1)
	id := connResponse.Ok.Messages[0].ID

	// mock reflect init callback (to store address)
	gasMeter4 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter4)
	reply := types.Reply{
		ID: id,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: types.Array[types.Event]{{
					Type: "instantiate",
					Attributes: types.Array[types.EventAttribute]{
						{
							Key:   "_contract_address",
							Value: REFLECT_ADDR,
						},
					},
				}},
				Data: nil,
			},
		},
	}
	replyBytes, err := json.Marshal(reply)
	require.NoError(t, err)
	var gasMeter4GasMeter types.GasMeter = gasMeter4
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	replyParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        replyBytes,
		GasMeter:   &gasMeter4GasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	_, _, err = vm.Reply(replyParams)
	require.NoError(t, err)

	// ensure the channel is registered
	queryMsg := IBCQueryMsg{
		ListAccounts: &struct{}{},
	}
	queryBytes := toBytes(t, queryMsg)
	var gasMeter4QueryGasMeter types.GasMeter = gasMeter4
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	queryParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        queryBytes,
		GasMeter:   &gasMeter4QueryGasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	queryResult, err := vm.Query(queryParams)
	require.NoError(t, err)
	require.NotNil(t, queryResult.Result.Ok)
	var accounts ListAccountsResponse
	err = json.Unmarshal(queryResult.Result.Ok, &accounts)
	require.NoError(t, err)
	require.Len(t, accounts.Accounts, 1)
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
					Amount:    types.Array[types.Coin]{types.NewCoin(12345678, "uatom")},
				}},
			}},
		},
	}
	msg := api.MockIBCPacketReceive(CHANNEL_ID, toBytes(t, ibcMsg))
	msgBytes, err = json.Marshal(msg)
	require.NoError(t, err)
	var gasMeter5GasMeter types.GasMeter = gasMeter5
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	packetParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        msgBytes,
		GasMeter:   &gasMeter5GasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	packetResult, _, err := vm.IBCPacketReceive(packetParams)
	require.NoError(t, err)
	var ackResult types.IBCReceiveResult
	err = json.Unmarshal(packetResult, &ackResult)
	require.NoError(t, err)
	assert.NotNil(t, ackResult.Ok)

	// assert app-level success
	var ack AcknowledgeDispatch
	err = json.Unmarshal(ackResult.Ok.Acknowledgement, &ack)
	require.NoError(t, err)
	require.Empty(t, ack.Err)

	// error on message from another channel
	msg2 := api.MockIBCPacketReceive("no-such-channel", toBytes(t, ibcMsg))
	msg2Bytes, err := json.Marshal(msg2)
	require.NoError(t, err)
	packet2Params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        msg2Bytes,
		GasMeter:   &gasMeter5GasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	packet2Result, _, err := vm.IBCPacketReceive(packet2Params)
	require.NoError(t, err)
	var ack2Result types.IBCReceiveResult
	err = json.Unmarshal(packet2Result, &ack2Result)
	require.NoError(t, err)
	assert.NotNil(t, ack2Result.Ok)

	// assert app-level failure
	var ack2 AcknowledgeDispatch
	err = json.Unmarshal(ack2Result.Ok.Acknowledgement, &ack2)
	require.NoError(t, err)
	require.Equal(t, "invalid packet: cosmwasm_std::addresses::Addr not found", ack2.Err)

	// check for the expected custom event
	expected_events := []types.Event{{
		Type: "ibc",
		Attributes: []types.EventAttribute{{
			Key:   "packet",
			Value: "receive",
		}},
	}}
	require.Equal(t, expected_events, ack2Result.Ok.Events)
}

func TestAnalyzeCode(t *testing.T) {
	vm := withVM(t)

	// Store non-IBC contract
	wasm, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)
	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// and analyze
	report, err := vm.AnalyzeCode(checksum)
	require.NoError(t, err)
	require.False(t, report.HasIBCEntryPoints)
	require.Empty(t, report.RequiredCapabilities)
	require.Equal(t, uint64(42), *report.ContractMigrateVersion)

	// Store IBC contract
	wasm2, err := os.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)
	checksum2, _, err := vm.StoreCode(wasm2, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// and analyze
	report2, err := vm.AnalyzeCode(checksum2)
	require.NoError(t, err)
	require.True(t, report2.HasIBCEntryPoints)
	require.Equal(t, "iterator,stargate", report2.RequiredCapabilities)
	require.Nil(t, report2.ContractMigrateVersion)
}

func TestIBCMsgGetChannel(t *testing.T) {
	const CHANNEL_ID = "channel-432"

	msg1 := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, "random-garbage")
	msg2 := api.MockIBCChannelOpenTry(CHANNEL_ID, types.Ordered, "random-garbage")
	msg3 := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, "random-garbage")
	msg4 := api.MockIBCChannelConnectConfirm(CHANNEL_ID, types.Ordered, "random-garbage")
	msg5 := api.MockIBCChannelCloseInit(CHANNEL_ID, types.Ordered, "random-garbage")
	msg6 := api.MockIBCChannelCloseConfirm(CHANNEL_ID, types.Ordered, "random-garbage")

	require.Equal(t, msg1.GetChannel(), msg2.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg3.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg4.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg5.GetChannel())
	require.Equal(t, msg1.GetChannel(), msg6.GetChannel())
	require.Equal(t, CHANNEL_ID, msg1.GetChannel().Endpoint.ChannelID)
}

func TestIBCMsgGetCounterVersion(t *testing.T) {
	const CHANNEL_ID = "channel-432"
	const VERSION = "random-garbage"

	msg1 := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, VERSION)
	_, ok := msg1.GetCounterVersion()
	require.False(t, ok)

	msg2 := api.MockIBCChannelOpenTry(CHANNEL_ID, types.Ordered, VERSION)
	v, ok := msg2.GetCounterVersion()
	require.True(t, ok)
	require.Equal(t, VERSION, v)

	msg3 := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, VERSION)
	v, ok = msg3.GetCounterVersion()
	require.True(t, ok)
	require.Equal(t, VERSION, v)

	msg4 := api.MockIBCChannelConnectConfirm(CHANNEL_ID, types.Ordered, VERSION)
	_, ok = msg4.GetCounterVersion()
	require.False(t, ok)
}
