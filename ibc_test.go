package cosmwasm

import (
	"encoding/json"
	"fmt"
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

	t.Run("Store and retrieve IBC contract", func(t *testing.T) {
		wasm, err := os.ReadFile(IBC_TEST_CONTRACT)
		require.NoError(t, err)

		checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.NoError(t, err)

		code, err := vm.GetCode(checksum)
		require.NoError(t, err)
		require.Equal(t, WasmCode(wasm), code)
	})

	t.Run("Analyze stored IBC contract", func(t *testing.T) {
		// Re-read the same wasm file and store it again, or retrieve the same checksum from above
		wasm, err := os.ReadFile(IBC_TEST_CONTRACT)
		require.NoError(t, err)

		checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.NoError(t, err)

		// Now run the analyzer
		report, err := vm.AnalyzeCode(checksum)
		require.NoError(t, err)

		// We expect IBC entry points to be present in this contract
		require.True(t, report.HasIBCEntryPoints, "IBC contract should have IBC entry points")

		// You can also assert/update checks regarding capabilities or migration versions:
		require.Contains(t, report.RequiredCapabilities, "iterator", "Expected 'iterator' capability for this contract")
		require.Contains(t, report.RequiredCapabilities, "stargate", "Expected 'stargate' capability for this contract")

		// Optionally check if the contract has a migrate version or not
		if report.ContractMigrateVersion != nil {
			t.Logf("Contract declares a migration version: %d", *report.ContractMigrateVersion)
		} else {
			t.Log("Contract does not declare a migration version")
		}
	})
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
	fmt.Printf("DEBUG: JSON being sent to contract: %s\n", string(bz))
	// Pretty print the struct for debugging
	prettyJSON, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	fmt.Printf("DEBUG: Message structure:\n%s\n", string(prettyJSON))
	return bz
}

const IBC_VERSION = "ibc-reflect-v1"

// channel id for handshake
const CHANNEL_ID = "channel-432"

func TestIBCHandshake(t *testing.T) {
	vm := withVM(t)

	// First store the reflect contract
	reflectWasm, err := os.ReadFile("./testdata/reflect.wasm")
	require.NoError(t, err)
	reflectChecksum, _, err := vm.StoreCode(reflectWasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// Store code ID mapping
	err = vm.Pin(reflectChecksum)
	require.NoError(t, err)

	fmt.Printf("DEBUG: Reflect contract details:\n")
	fmt.Printf("  Checksum (hex): %x\n", reflectChecksum)

	// Then store the IBC contract
	ibcWasm, err := os.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)
	ibcChecksum, _, err := vm.StoreCode(ibcWasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// Store code ID mapping
	err = vm.Pin(ibcChecksum)
	require.NoError(t, err)

	fmt.Printf("DEBUG: IBC contract details:\n")
	fmt.Printf("  Checksum (hex): %x\n", ibcChecksum)
	checksum := ibcChecksum
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
		ReflectCodeID: 1, // We'll use 1 as the code ID since this is a test environment
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
	assert.Equal(t, 1, init.CodeID)
	assert.Empty(t, init.Funds)
}

func TestIBCPacketDispatch(t *testing.T) {
	// address of first reflect contract instance that we created
	const REFLECT_ADDR = "reflect-acct-1"
	// channel id for handshake
	const CHANNEL_ID = "channel-234"

	// setup
	vm := withVM(t)

	// First store the reflect contract
	reflectWasm, err := os.ReadFile("./testdata/reflect.wasm")
	require.NoError(t, err)
	reflectChecksum, _, err := vm.StoreCode(reflectWasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// Store code ID mapping
	err = vm.Pin(reflectChecksum)
	require.NoError(t, err)

	fmt.Printf("DEBUG: Reflect contract details:\n")
	fmt.Printf("  Checksum (hex): %x\n", reflectChecksum)

	// Then store the IBC contract
	ibcWasm, err := os.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)
	ibcChecksum, _, err := vm.StoreCode(ibcWasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	// Store code ID mapping
	err = vm.Pin(ibcChecksum)
	require.NoError(t, err)

	fmt.Printf("DEBUG: IBC contract details:\n")
	fmt.Printf("  Checksum (hex): %x\n", ibcChecksum)

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
		ReflectCodeID: 1, // We'll use 1 as the code ID since this is a test environment
	}
	_, _, err = vm.Instantiate(ibcChecksum, env, info, toBytes(t, initMsg), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)

	// channel open
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	openMsg := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, IBC_VERSION)
	o, _, err := vm.IBCChannelOpen(ibcChecksum, env, openMsg, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, o.Ok)
	oResponse := o.Ok
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, oResponse)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, IBC_VERSION)
	conn, _, err := vm.IBCChannelConnect(ibcChecksum, env, connectMsg, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT, deserCost)
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
	_, _, err = vm.Reply(ibcChecksum, env, reply, store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)

	// ensure the channel is registered
	queryMsg := IBCQueryMsg{
		ListAccounts: &struct{}{},
	}
	q, _, err := vm.Query(ibcChecksum, env, toBytes(t, queryMsg), store, *goapi, querier, gasMeter4, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, q.Ok)
	qResponse := q.Ok
	var accounts ListAccountsResponse
	err = json.Unmarshal(qResponse, &accounts)
	require.NoError(t, err)
	require.Len(t, accounts.Accounts, 1)
	require.Equal(t, CHANNEL_ID, accounts.Accounts[0].ChannelID)
	require.Equal(t, REFLECT_ADDR, accounts.Accounts[0].Account)
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
