package cosmwasm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	TESTING_PRINT_DEBUG  = true
	TESTING_GAS_LIMIT    = uint64(500_000_000_000) // ~0.5ms
	TESTING_MEMORY_LIMIT = 64                      // MiB
	TESTING_CACHE_SIZE   = 100                     // MiB
)

var TESTING_CAPABILITIES = []string{
	"staking",
	"stargate",
	"iterator",
	"cosmwasm_1_1",
	"cosmwasm_1_2",
	"cosmwasm_1_3",
	"cosmwasm_1_4",
	"cosmwasm_2_0",
	"cosmwasm_2_1",
	"cosmwasm_2_2",
}

const (
	CYBERPUNK_TEST_CONTRACT = "./testdata/cyberpunk.wasm"
	HACKATOM_TEST_CONTRACT  = "./testdata/hackatom.wasm"
)

func withVM(t *testing.T) *VM {
	t.Helper()
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	vm, err := NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG, TESTING_CACHE_SIZE)
	require.NoError(t, err)

	t.Cleanup(func() {
		vm.Cleanup()
		os.RemoveAll(tmpdir)
	})
	return vm
}

func createTestContract(t *testing.T, vm *VM, path string) Checksum {
	t.Helper()
	wasm, err := os.ReadFile(path)
	require.NoError(t, err)
	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	return checksum
}

func TestStoreCode(t *testing.T) {
	vm := withVM(t)

	hackatom, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	specs := map[string]struct {
		wasm        []byte
		expectedErr string
		expectOk    bool
	}{
		"valid wasm contract": {
			wasm:     hackatom,
			expectOk: true,
		},
		"nil bytes": {
			wasm:        nil,
			expectedErr: "Null/Nil argument: wasm",
			expectOk:    false,
		},
		"empty bytes": {
			wasm:        []byte{},
			expectedErr: "Wasm bytecode could not be deserialized",
			expectOk:    false,
		},
		"invalid wasm - random bytes": {
			wasm:        []byte("random invalid data"),
			expectedErr: "Wasm bytecode could not be deserialized",
			expectOk:    false,
		},
		"invalid wasm - corrupted header": {
			// First 8 bytes of a valid wasm file, followed by random data
			wasm:        append([]byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}, []byte("corrupted content")...),
			expectedErr: "Wasm bytecode could not be deserialized",
			expectOk:    false,
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			checksum, _, err := vm.StoreCode(spec.wasm, TESTING_GAS_LIMIT)
			if spec.expectOk {
				require.NoError(t, err)
				require.NotEmpty(t, checksum, "checksum should not be empty on success")
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), spec.expectedErr)
				require.Empty(t, checksum, "checksum should be empty on error")
			}
		})
	}
}

func TestSimulateStoreCode(t *testing.T) {
	vm := withVM(t)

	hackatom, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	specs := map[string]struct {
		wasm        []byte
		expectedErr string
		expectOk    bool
	}{
		"valid wasm contract": {
			wasm:     hackatom,
			expectOk: true,
		},
		"nil bytes": {
			wasm:        nil,
			expectedErr: "Null/Nil argument: wasm",
			expectOk:    false,
		},
		"empty bytes": {
			wasm:        []byte{},
			expectedErr: "Wasm bytecode could not be deserialized",
			expectOk:    false,
		},
		"invalid wasm - random bytes": {
			wasm:        []byte("random invalid data"),
			expectedErr: "Wasm bytecode could not be deserialized",
			expectOk:    false,
		},
		"invalid wasm - corrupted header": {
			// First 8 bytes of a valid wasm file, followed by random data
			wasm:        append([]byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}, []byte("corrupted content")...),
			expectedErr: "Wasm bytecode could not be deserialized",
			expectOk:    false,
		},
		"invalid wasm - no memory section": {
			// Minimal valid wasm module without memory section
			wasm:        []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00},
			expectedErr: "Error during static Wasm validation: Wasm contract must contain exactly one memory",
			expectOk:    false,
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			checksum, _, err := vm.SimulateStoreCode(spec.wasm, TESTING_GAS_LIMIT)
			if spec.expectOk {
				require.NoError(t, err)
				require.NotEmpty(t, checksum, "checksum should not be empty on success")

				// Verify the code was not actually stored
				_, err = vm.GetCode(checksum)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Error opening Wasm file for reading")
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), spec.expectedErr)
				require.Empty(t, checksum, "checksum should be empty on error")
			}
		})
	}
}

func TestStoreCodeAndGet(t *testing.T) {
	vm := withVM(t)

	wasm, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	code, err := vm.GetCode(checksum)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

func TestRemoveCode(t *testing.T) {
	vm := withVM(t)

	wasm, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)

	err = vm.RemoveCode(checksum)
	require.NoError(t, err)

	err = vm.RemoveCode(checksum)
	require.ErrorContains(t, err, "Wasm file does not exist")
}

func TestHappyPath(t *testing.T) {
	vm := withVM(t)
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	i, _, err := vm.Instantiate(checksum, env, info, msg, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires := i.Ok
	require.Empty(t, ires.Messages)

	// execute
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	h, _, err := vm.Execute(checksum, env, info, []byte(`{"release":{}}`), store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, h.Ok)
	hres := h.Ok
	require.Len(t, hres.Messages, 1)

	// make sure it read the balance properly and we got 250 atoms
	dispatch := hres.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "bob", send.ToAddress)
	assert.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	assert.Equal(t, expectedData, hres.Data)
}

func TestEnv(t *testing.T) {
	vm := withVM(t)
	checksum := createTestContract(t, vm, CYBERPUNK_TEST_CONTRACT)

	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	i, _, err := vm.Instantiate(checksum, env, info, []byte(`{}`), store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires := i.Ok
	require.Empty(t, ires.Messages)

	// Execute mirror env without Transaction
	env = types.Env{
		Block: types.BlockInfo{
			Height:  444,
			Time:    1955939743_123456789,
			ChainID: "nice-chain",
		},
		Contract: types.ContractInfo{
			Address: "wasm10dyr9899g6t0pelew4nvf4j5c3jcgv0r5d3a5l",
		},
		Transaction: nil,
	}
	info = api.MockInfo("creator", nil)
	msg := []byte(`{"mirror_env": {}}`)
	i, _, err = vm.Execute(checksum, env, info, msg, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	expected, _ := json.Marshal(env)
	require.Equal(t, expected, ires.Data)

	// Execute mirror env with Transaction
	env = types.Env{
		Block: types.BlockInfo{
			Height:  444,
			Time:    1955939743_123456789,
			ChainID: "nice-chain",
		},
		Contract: types.ContractInfo{
			Address: "wasm10dyr9899g6t0pelew4nvf4j5c3jcgv0r5d3a5l",
		},
		Transaction: &types.TransactionInfo{
			Index: 18,
		},
	}
	info = api.MockInfo("creator", nil)
	msg = []byte(`{"mirror_env": {}}`)
	i, _, err = vm.Execute(checksum, env, info, msg, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	expected, _ = json.Marshal(env)
	require.Equal(t, expected, ires.Data)
}

func TestGetMetrics(t *testing.T) {
	vm := withVM(t)

	// GetMetrics 1
	metrics, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Create contract
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	deserCost := types.UFraction{Numerator: 1, Denominator: 1}

	// GetMetrics 2
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	i, _, err := vm.Instantiate(checksum, env, info, msg1, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires := i.Ok
	require.Empty(t, ires.Messages)

	// GetMetrics 3
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	t.Log(metrics.SizeMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	i, _, err = vm.Instantiate(checksum, env, info, msg2, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	require.Empty(t, ires.Messages)

	// GetMetrics 4
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Pin
	err = vm.Pin(checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	i, _, err = vm.Instantiate(checksum, env, info, msg3, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	require.Empty(t, ires.Messages)

	// GetMetrics 6
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Unpin
	err = vm.Unpin(checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 4
	msg4 := []byte(`{"verifier": "fred", "beneficiary": "jeff"}`)
	i, _, err = vm.Instantiate(checksum, env, info, msg4, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	require.Empty(t, ires.Messages)

	// GetMetrics 8
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)
}

func TestLongPayloadDeserialization(t *testing.T) {
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	gasReport := types.GasReport{}

	// Create a valid payload
	validPayload := make([]byte, 128*1024)
	validPayloadJSON, err := json.Marshal(validPayload)
	require.NoError(t, err)
	resultJson := []byte(fmt.Sprintf(`{"ok":{"messages":[{"id":0,"msg":{"bank":{"send":{"to_address":"bob","amount":[{"denom":"ATOM","amount":"250"}]}}},"payload":%s,"reply_on":"never"}],"data":"8Auq","attributes":[],"events":[]}}`, validPayloadJSON))

	// Test that a valid payload can be deserialized
	var result types.ContractResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &result)
	require.NoError(t, err)
	require.Equal(t, validPayload, result.Ok.Messages[0].Payload)

	// Create an invalid payload (too large)
	invalidPayload := make([]byte, 128*1024+1)
	invalidPayloadJSON, err := json.Marshal(invalidPayload)
	require.NoError(t, err)
	resultJson = []byte(fmt.Sprintf(`{"ok":{"messages":[{"id":0,"msg":{"bank":{"send":{"to_address":"bob","amount":[{"denom":"ATOM","amount":"250"}]}}},"payload":%s,"reply_on":"never"}],"attributes":[],"events":[]}}`, invalidPayloadJSON))

	// Test that an invalid payload cannot be deserialized
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payload")

	// Test that an invalid payload cannot be deserialized to IBCBasicResult
	var ibcResult types.IBCBasicResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &ibcResult)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payload")

	// Test that an invalid payload cannot be deserialized to IBCReceiveResult
	var ibcReceiveResult types.IBCReceiveResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &ibcReceiveResult)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payload")
}
