//go:build cgo && !nolink_libwasmvm

package wasmvm

import (
	"encoding/json"
	"os"
	"testing"

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
	const REFLECT_ID uint64 = 101
	// channel id for handshake
	const CHANNEL_ID = "channel-432"

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
		ReflectCodeID: REFLECT_ID,
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
	openMsg := api.MockIBCChannelOpenInit(CHANNEL_ID, types.Ordered, IBC_VERSION)
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
	o, _, err := api.IBCChannelOpen(params)
	require.NoError(t, err)
	var oResponse types.IBCChannelOpenResult
	err = json.Unmarshal(o, &oResponse)
	require.NoError(t, err)
	require.NotNil(t, oResponse.Ok)
	require.Equal(t, &types.IBC3ChannelOpenResponse{Version: "ibc-reflect-v1"}, oResponse.Ok)

	// channel connect
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter3)
	gasMeter = gasMeter3
	env = api.MockEnv()
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	// completes and dispatches message to create reflect contract
	connectMsg := api.MockIBCChannelConnectAck(CHANNEL_ID, types.Ordered, IBC_VERSION)
	connectMsgBytes, err := json.Marshal(connectMsg)
	require.NoError(t, err)
	params = api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum[:],
		Env:        envBytes,
		Msg:        connectMsgBytes,
		GasMeter:   &gasMeter,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: false,
	}
	conn, _, err := api.IBCChannelConnect(params)
	require.NoError(t, err)
	var connResponse types.IBCBasicResult
	err = json.Unmarshal(conn, &connResponse)
	require.NoError(t, err)
	require.NotNil(t, connResponse.Ok)
	require.Len(t, connResponse.Ok.Messages, 1)

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
	require.Equal(t, REFLECT_ID, dispatch.Wasm.Instantiate.CodeID)
}
