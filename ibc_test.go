//go:build cgo && !nolink_libwasmvm

package cosmwasm

import (
	"encoding/json"
	"os"
	"runtime"
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

func toBytes(t *testing.T, v interface{}) []byte {
	t.Helper()
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
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	init_msg := IBCInstantiateMsg{
		ReflectCodeID: REFLECT_ID,
	}
	i, _, err := vm.Instantiate(checksum, env, info, toBytes(t, init_msg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	assert.NotNil(t, i.Ok)
	iResponse := i.Ok
	require.Empty(t, iResponse.Messages)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	openMsg := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, IBC_VERSION)
	o, _, err := vm.IBCChannelOpen(checksum, env, openMsg, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, o.Ok)
	oResponse := o.Ok
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, oResponse)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	env = api.MockEnv()
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, IBC_VERSION)
	conn, _, err := vm.IBCChannelConnect(checksum, env, connectMsg, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, conn.Ok)
	connResponse := conn.Ok
	require.Len(t, connResponse.Messages, 1)

	// check for the expected custom event
	expected_events := []types.Event{{
		Type: "ibc",
		Attributes: []types.EventAttribute{{
			Key:   "channel",
			Value: "connect",
		}},
	}}
	require.Equal(t, expected_events, connResponse.Events)

	// make sure it read the balance properly and we got 250 atoms
	dispatch := connResponse.Messages[0].Msg
	require.NotNil(t, dispatch.Wasm, "%#v", dispatch)
	require.NotNil(t, dispatch.Wasm.Instantiate, "%#v", dispatch)
	init := dispatch.Wasm.Instantiate
	assert.Equal(t, REFLECT_ID, init.CodeID)
	assert.Empty(t, init.Funds)
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
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{
		ReflectCodeID: REFLECT_ID,
	}
	_, _, err := vm.Instantiate(checksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	openMsg := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, IBC_VERSION)
	o, _, err := vm.IBCChannelOpen(checksum, env, openMsg, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, o.Ok)
	oResponse := o.Ok
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, oResponse)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, IBC_VERSION)
	conn, _, err := vm.IBCChannelConnect(checksum, env, connectMsg, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, conn.Ok)
	connResponse := conn.Ok
	require.Len(t, connResponse.Messages, 1)
	id := connResponse.Messages[0].ID

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
	_, _, err = vm.Reply(checksum, env, reply, store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)

	// ensure the channel is registered
	queryMsg := IBCQueryMsg{
		ListAccounts: &struct{}{},
	}
	q, _, err := vm.Query(checksum, env, toBytes(t, queryMsg), store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, q.Ok)
	qResponse := q.Ok
	var accounts ListAccountsResponse
	err = json.Unmarshal(qResponse, &accounts)
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
	pr, _, err := vm.IBCPacketReceive(checksum, env, msg, store, *goapi, querier, gasMeter5, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	assert.NotNil(t, pr.Ok)
	prResponse := pr.Ok

	// assert app-level success
	var ack AcknowledgeDispatch
	err = json.Unmarshal(prResponse.Acknowledgement, &ack)
	require.NoError(t, err)
	require.Empty(t, ack.Err)

	// error on message from another channel
	msg2 := api.MockIBCPacketReceive("no-such-channel", toBytes(t, ibcMsg))
	pr2, _, err := vm.IBCPacketReceive(checksum, env, msg2, store, *goapi, querier, gasMeter5, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	assert.NotNil(t, pr.Ok)
	prResponse2 := pr2.Ok
	// assert app-level failure
	var ack2 AcknowledgeDispatch
	err = json.Unmarshal(prResponse2.Acknowledgement, &ack2)
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
	require.Equal(t, expected_events, prResponse2.Events)
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
	require.Equal(t, "", report.RequiredCapabilities)
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

// (Original tests such as TestIBC, TestIBCHandshake, etc. remain unchanged.)
// … [Original tests omitted for brevity] …

// -----------------------------------------------------------------------------
// Memory Leak Test Helpers
// -----------------------------------------------------------------------------

// measureMemoryLeak runs the passed function 'f' for 'iterations' times,
// forcing garbage collection before and after and then logging the average
// increase in Allocated memory per iteration. If the average exceeds a given
// threshold (in bytes), the test will fail.
func measureMemoryLeak(t *testing.T, iterations int, testName string, f func()) {
	// Run one round to “warm up” (optional)
	f()

	runtime.GC()
	var mBefore, mAfter runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	for i := 0; i < iterations; i++ {
		f()
	}

	runtime.GC()
	runtime.ReadMemStats(&mAfter)

	// Calculate the average difference in allocated bytes per iteration.
	diff := mAfter.Alloc - mBefore.Alloc
	avg := diff / uint64(iterations)
	t.Logf("%s: %d iterations, total alloc diff: %d bytes, average diff: %d bytes/iter", testName, iterations, diff, avg)

	// Optionally assert that the average increase is below a threshold.
	// (Adjust maxAvgAllocBytes as needed; here we set an example threshold of 2KB.)
	const maxAvgAllocBytes = 2048
	if avg > maxAvgAllocBytes {
		t.Errorf("%s: memory leak suspected, average allocation per iteration %d bytes exceeds threshold (%d bytes)",
			testName, avg, maxAvgAllocBytes)
	}
}

// -----------------------------------------------------------------------------
// Helpers to run complete IBC transactions in one “iteration”
// -----------------------------------------------------------------------------

// runStoreAndGetCode performs a store code and retrieval transaction.
func runStoreAndGetCode(t *testing.T, wasmPath string) {
	vm := withVM(t)
	wasm, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	code, err := vm.GetCode(checksum)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

// runIBCHandshake performs the full handshake (instantiate, channel open, connect, reply)
// sequence. Note that we use hard-coded values here (e.g. IBC_VERSION) as in the original test.
func runIBCHandshake(t *testing.T, wasmPath string, reflectID uint64, channelID string) {
	vm := withVM(t)

	_, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Store the contract code.
	checksum := createTestContract(t, vm, IBC_TEST_CONTRACT)

	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// Instantiate the contract.
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{ReflectCodeID: reflectID}
	i, _, err := vm.Instantiate(checksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)

	// Channel open.
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	openMsg := api.MockIBCChannelOpenInit(channelID, types.Ordered, IBC_VERSION)
	o, _, err := vm.IBCChannelOpen(checksum, env, openMsg, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, o.Ok)
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: IBC_VERSION}, o.Ok)

	// Channel connect.
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	env = api.MockEnv()
	connectMsg := api.MockIBCChannelConnectAck(channelID, types.Ordered, IBC_VERSION)
	conn, _, err := vm.IBCChannelConnect(checksum, env, connectMsg, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, conn.Ok)
	require.Len(t, conn.Ok.Messages, 1)

	// Simulate reply for the reflect init callback.
	gasMeter4 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter4)
	reply := types.Reply{
		ID: conn.Ok.Messages[0].ID,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: types.Array[types.Event]{
					{
						Type: "instantiate",
						Attributes: types.Array[types.EventAttribute]{
							{Key: "_contract_address", Value: "dummy-address"},
						},
					},
				},
				Data: nil,
			},
		},
	}
	_, _, err = vm.Reply(checksum, env, reply, store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
}

// runIBCPacketDispatch performs the full packet dispatch transaction
// (including instantiation, handshake, reply, query, and IBC packet receive).
func runIBCPacketDispatch(t *testing.T, wasmPath string, reflectID uint64, channelID string, reflectAddr string) {
	vm := withVM(t)

	_, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Store the contract code.
	checksum := createTestContract(t, vm, IBC_TEST_CONTRACT)

	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// Instantiate the contract.
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	initMsg := IBCInstantiateMsg{ReflectCodeID: reflectID}
	_, _, err = vm.Instantiate(checksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)

	// Channel open.
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	openMsg := api.MockIBCChannelOpenInit(channelID, types.Ordered, IBC_VERSION)
	o, _, err := vm.IBCChannelOpen(checksum, env, openMsg, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, o.Ok)

	// Channel connect.
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	connectMsg := api.MockIBCChannelConnectAck(channelID, types.Ordered, IBC_VERSION)
	conn, _, err := vm.IBCChannelConnect(checksum, env, connectMsg, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, conn.Ok)
	require.Len(t, conn.Ok.Messages, 1)
	id := conn.Ok.Messages[0].ID

	// Simulate reply for the reflect init callback.
	gasMeter4 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter4)
	reply := types.Reply{
		ID: id,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: types.Array[types.Event]{
					{
						Type: "instantiate",
						Attributes: types.Array[types.EventAttribute]{
							{Key: "_contract_address", Value: reflectAddr},
						},
					},
				},
				Data: nil,
			},
		},
	}
	_, _, err = vm.Reply(checksum, env, reply, store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)

	// Query the list of accounts.
	gasMeterQuery := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeterQuery)
	queryMsg := IBCQueryMsg{ListAccounts: &struct{}{}}
	q, _, err := vm.Query(checksum, env, toBytes(t, queryMsg), store, *goapi, querier, gasMeterQuery, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	var accounts ListAccountsResponse
	err = json.Unmarshal(q.Ok, &accounts)
	require.NoError(t, err)
	require.Len(t, accounts.Accounts, 1)
	require.Equal(t, channelID, accounts.Accounts[0].ChannelID)
	require.Equal(t, reflectAddr, accounts.Accounts[0].Account)

	// Process a valid IBC packet receive.
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
	msg := api.MockIBCPacketReceive(channelID, toBytes(t, ibcMsg))
	pr, _, err := vm.IBCPacketReceive(checksum, env, msg, store, *goapi, querier, gasMeter5, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	var ack AcknowledgeDispatch
	err = json.Unmarshal(pr.Ok.Acknowledgement, &ack)
	require.NoError(t, err)
	require.Empty(t, ack.Err)

	// Process an IBC packet receive with an invalid channel.
	msg2 := api.MockIBCPacketReceive("no-such-channel", toBytes(t, ibcMsg))
	pr2, _, err := vm.IBCPacketReceive(checksum, env, msg2, store, *goapi, querier, gasMeter5, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	var ack2 AcknowledgeDispatch
	err = json.Unmarshal(pr2.Ok.Acknowledgement, &ack2)
	require.NoError(t, err)
	require.Equal(t, "invalid packet: cosmwasm_std::addresses::Addr not found", ack2.Err)
}

// -----------------------------------------------------------------------------
// Memory Leak Test Functions
// -----------------------------------------------------------------------------

// TestMemoryLeakStoreAndGetCode repeatedly runs StoreCode and GetCode
// to ensure no unexpected memory accumulation occurs.
func TestMemoryLeakStoreAndGetCode(t *testing.T) {
	const iterations = 1000
	measureMemoryLeak(t, iterations, "StoreAndGetCode", func() {
		runStoreAndGetCode(t, IBC_TEST_CONTRACT)
	})
}

// TestMemoryLeakIBCHandshake repeatedly runs the full handshake process.
func TestMemoryLeakIBCHandshake(t *testing.T) {
	const iterations = 1000
	const reflectID = 101
	const channelID = "channel-432"
	measureMemoryLeak(t, iterations, "IBCHandshake", func() {
		runIBCHandshake(t, IBC_TEST_CONTRACT, reflectID, channelID)
	})
}

// TestMemoryLeakIBCPacketDispatch repeatedly runs the packet dispatch process.
func TestMemoryLeakIBCPacketDispatch(t *testing.T) {
	const iterations = 1000
	const reflectID = 77
	const channelID = "channel-234"
	const reflectAddr = "reflect-acct-1"
	measureMemoryLeak(t, iterations, "IBCPacketDispatch", func() {
		runIBCPacketDispatch(t, IBC_TEST_CONTRACT, reflectID, channelID, reflectAddr)
	})
}

// TestMemoryLeakAnalyzeCode repeatedly calls AnalyzeCode on stored contracts.
func TestMemoryLeakAnalyzeCode(t *testing.T) {
	const iterations = 1000

	vm := withVM(t)
	//	_ := types.UFraction{Numerator: 1, Denominator: 1}

	// For a non-IBC contract.
	wasm, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)
	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	measureMemoryLeak(t, iterations, "AnalyzeCodeNonIBC", func() {
		report, err := vm.AnalyzeCode(checksum)
		require.NoError(t, err)
		require.False(t, report.HasIBCEntryPoints)
	})

	// For an IBC contract.
	wasm2, err := os.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)
	checksum2, _, err := vm.StoreCode(wasm2, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	measureMemoryLeak(t, iterations, "AnalyzeCodeIBC", func() {
		report2, err := vm.AnalyzeCode(checksum2)
		require.NoError(t, err)
		require.True(t, report2.HasIBCEntryPoints)
	})
}
