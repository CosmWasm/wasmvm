//go:build cgo && !nolink_libwasmvm

package cosmwasm

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v3/internal/api"
	"github.com/CosmWasm/wasmvm/v3/types"
)

const (
	testingPrintDebug  = false
	testingGasLimit    = uint64(500_000_000_000) // ~0.5ms
	testingMemoryLimit = 32                      // MiB
	testingCacheSize   = 100                     // MiB
)

var testingCapabilities = []string{"staking", "stargate", "iterator"}

const (
	cyberpunkTestContract = "./testdata/cyberpunk.wasm"
	hackatomTestContract  = "./testdata/hackatom.wasm"
)

func withVM(t *testing.T) *VM {
	t.Helper()
	tmpdir := t.TempDir()
	vm, err := NewVM(tmpdir, testingCapabilities, testingMemoryLimit, testingPrintDebug, testingCacheSize)
	require.NoError(t, err)

	t.Cleanup(func() {
		vm.Cleanup()
	})
	return vm
}

func createTestContract(t *testing.T, vm *VM, path string) Checksum {
	t.Helper()
	wasm, err := os.ReadFile(path)
	require.NoError(t, err)
	checksum, _, err := vm.StoreCode(wasm, testingGasLimit)
	require.NoError(t, err)
	return checksum
}

func TestStoreCode(t *testing.T) {
	vm := withVM(t)

	// Valid hackatom contract
	{
		wasm, err := os.ReadFile(hackatomTestContract)
		require.NoError(t, err)
		_, _, err = vm.StoreCode(wasm, testingGasLimit)
		require.NoError(t, err)
	}

	// Valid cyberpunk contract
	{
		wasm, err := os.ReadFile(cyberpunkTestContract)
		require.NoError(t, err)
		_, _, err = vm.StoreCode(wasm, testingGasLimit)
		require.NoError(t, err)
	}

	// Valid Wasm with no exports
	{
		// echo '(module)' | wat2wasm - -o empty.wasm
		// hexdump -C < empty.wasm

		wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
		_, _, err := vm.StoreCode(wasm, testingGasLimit)
		require.ErrorContains(t, err, "Error during static Wasm validation: Wasm contract must contain exactly one memory")
	}

	// No Wasm
	{
		wasm := []byte("foobar")
		_, _, err := vm.StoreCode(wasm, testingGasLimit)
		require.ErrorContains(t, err, "Wasm bytecode could not be deserialized")
	}

	// Empty
	{
		wasm := []byte("")
		_, _, err := vm.StoreCode(wasm, testingGasLimit)
		require.ErrorContains(t, err, "Wasm bytecode could not be deserialized")
	}

	// Nil
	{
		var wasm []byte = nil
		_, _, err := vm.StoreCode(wasm, testingGasLimit)
		require.ErrorContains(t, err, "Null/Nil argument: wasm")
	}
}

func TestSimulateStoreCode(t *testing.T) {
	vm := withVM(t)

	hackatom, err := os.ReadFile(hackatomTestContract)
	require.NoError(t, err)

	specs := map[string]struct {
		wasm []byte
		err  string
	}{
		"valid hackatom contract": {
			wasm: hackatom,
		},
		"no wasm": {
			wasm: []byte("foobar"),
			err:  "Wasm bytecode could not be deserialized",
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			checksum, _, err := vm.SimulateStoreCode(spec.wasm, testingGasLimit)

			if spec.err != "" {
				assert.ErrorContains(t, err, spec.err)
			} else {
				require.NoError(t, err)
				_, err = vm.GetCode(checksum)
				require.ErrorContains(t, err, "Error opening Wasm file for reading")
			}
		})
	}
}

func TestStoreCodeAndGet(t *testing.T) {
	vm := withVM(t)

	wasm, err := os.ReadFile(hackatomTestContract)
	require.NoError(t, err)

	checksum, _, err := vm.StoreCode(wasm, testingGasLimit)
	require.NoError(t, err)

	code, err := vm.GetCode(checksum)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

func TestRemoveCode(t *testing.T) {
	vm := withVM(t)

	wasm, err := os.ReadFile(hackatomTestContract)
	require.NoError(t, err)

	checksum, _, err := vm.StoreCode(wasm, testingGasLimit)
	require.NoError(t, err)

	err = vm.RemoveCode(checksum)
	require.NoError(t, err)

	err = vm.RemoveCode(checksum)
	require.ErrorContains(t, err, "Wasm file does not exist")
}

func TestHappyPath(t *testing.T) {
	vm := withVM(t)
	checksum := createTestContract(t, vm, hackatomTestContract)

	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	gasMeter1 := api.NewMockGasMeter(testingGasLimit)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	i, _, err := vm.Instantiate(checksum, env, info, msg, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires := i.Ok
	require.Empty(t, ires.Messages)

	// execute
	gasMeter2 := api.NewMockGasMeter(testingGasLimit)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	h, _, err := vm.Execute(checksum, env, info, []byte(`{"release":{}}`), store, *goapi, querier, gasMeter2, testingGasLimit, deserCost)
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
	checksum := createTestContract(t, vm, cyberpunkTestContract)

	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	gasMeter1 := api.NewMockGasMeter(testingGasLimit)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	i, _, err := vm.Instantiate(checksum, env, info, []byte(`{}`), store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
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
	i, _, err = vm.Execute(checksum, env, info, msg, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	expected, _ := json.Marshal(env)
	require.Equal(t, expected, ires.Data)

	tx_hash, _ := hex.DecodeString("AABBCCDDEEFF0011AABBCCDDEEFF0011AABBCCDDEEFF0011AABBCCDDEEFF0011")

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
			Hash:  tx_hash,
		},
	}
	info = api.MockInfo("creator", nil)
	msg = []byte(`{"mirror_env": {}}`)
	i, _, err = vm.Execute(checksum, env, info, msg, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
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
	checksum := createTestContract(t, vm, hackatomTestContract)

	deserCost := types.UFraction{Numerator: 1, Denominator: 1}

	// GetMetrics 2
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter1 := api.NewMockGasMeter(testingGasLimit)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	i, _, err := vm.Instantiate(checksum, env, info, msg1, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires := i.Ok
	require.Empty(t, ires.Messages)

	// GetMetrics 3
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	i, _, err = vm.Instantiate(checksum, env, info, msg2, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
	require.NoError(t, err)
	require.NotNil(t, i.Ok)
	ires = i.Ok
	require.Empty(t, ires.Messages)

	// GetMetrics 4
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
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
	i, _, err = vm.Instantiate(checksum, env, info, msg3, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
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
	i, _, err = vm.Instantiate(checksum, env, info, msg4, store, *goapi, querier, gasMeter1, testingGasLimit, deserCost)
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
	resultJson := fmt.Appendf(nil, `{"ok":{"messages":[{"id":0,"msg":{"bank":{"send":{"to_address":"bob","amount":[{"denom":"ATOM","amount":"250"}]}}},"payload":%s,"reply_on":"never"}],"data":"8Auq","attributes":[],"events":[]}}`, validPayloadJSON)

	// Test that a valid payload can be deserialized
	var result types.ContractResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &result)
	require.NoError(t, err)
	require.Equal(t, validPayload, result.Ok.Messages[0].Payload)

	// Create an invalid payload (too large)
	invalidPayload := make([]byte, 128*1024+1)
	invalidPayloadJSON, err := json.Marshal(invalidPayload)
	require.NoError(t, err)
	resultJson = fmt.Appendf(nil, `{"ok":{"messages":[{"id":0,"msg":{"bank":{"send":{"to_address":"bob","amount":[{"denom":"ATOM","amount":"250"}]}}},"payload":%s,"reply_on":"never"}],"attributes":[],"events":[]}}`, invalidPayloadJSON)

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

func TestPinUnpin(t *testing.T) {
	vm := withVM(t)

	// Get pinned metrics
	pinnedMetrics, err := vm.GetPinnedMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.PinnedMetrics{PerModule: []types.PerModuleEntry{}}, pinnedMetrics)

	// Create contract 1
	checksumHackatom := createTestContract(t, vm, hackatomTestContract)

	// Pin contract 1
	err = vm.Pin(checksumHackatom)
	require.NoError(t, err)

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Len(t, pinnedMetrics.PerModule, 1)
	require.Equal(t, uint32(0), pinnedMetrics.PerModule[0].Metrics.Hits)
	require.InEpsilon(t, 4000000, pinnedMetrics.PerModule[0].Metrics.Size, 0.25)

	// Create contract 2
	checksumCyberpunk := createTestContract(t, vm, cyberpunkTestContract)

	// Pin contract 2
	err = vm.Pin(checksumCyberpunk)
	require.NoError(t, err)

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Len(t, pinnedMetrics.PerModule, 2)
	require.Equal(t, uint32(0), pinnedMetrics.PerModule[0].Metrics.Hits)
	require.InEpsilon(t, 4000000, pinnedMetrics.PerModule[0].Metrics.Size, 0.25)
	require.Equal(t, uint32(0), pinnedMetrics.PerModule[1].Metrics.Hits)
	require.InEpsilon(t, 4000000, pinnedMetrics.PerModule[1].Metrics.Size, 0.25)

	// Unpin contract 1
	err = vm.Unpin(checksumHackatom)
	require.NoError(t, err)

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Len(t, pinnedMetrics.PerModule, 1)
	require.Equal(t, uint32(0), pinnedMetrics.PerModule[0].Metrics.Hits)
	require.InEpsilon(t, 4000000, pinnedMetrics.PerModule[0].Metrics.Size, 0.25)

	// Unpin contract 2
	err = vm.Unpin(checksumCyberpunk)
	require.NoError(t, err)

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Empty(t, pinnedMetrics.PerModule)
}

func TestPinNotExisting(t *testing.T) {
	vm := withVM(t)

	// Get pinned metrics
	pinnedMetrics, err := vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Empty(t, pinnedMetrics.PerModule)

	// Create contract 1, get correct checksum
	checksum := createTestContract(t, vm, hackatomTestContract)
	// Malform the checksum
	checksum[0] = checksum[0] + 1

	// Try to pin not existing contract
	err = vm.Pin(checksum)
	require.ErrorContains(t, err, "Error opening Wasm file for reading")

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Empty(t, pinnedMetrics.PerModule)
}

func TestUnpinNotExisting(t *testing.T) {
	vm := withVM(t)

	// Get pinned metrics
	pinnedMetrics, err := vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Empty(t, pinnedMetrics.PerModule)

	// Create contract 1, get correct checksum
	checksum := createTestContract(t, vm, hackatomTestContract)

	// Pin contract 1
	err = vm.Pin(checksum)
	require.NoError(t, err)

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Len(t, pinnedMetrics.PerModule, 1)

	// Malform the checksum
	checksum[0] = checksum[0] + 1

	// Try to unpin not existing contract
	err = vm.Unpin(checksum)
	// Unpin just ignores unpinning non-existing codes
	require.NoError(t, err)

	// Get pinned metrics
	pinnedMetrics, err = vm.GetPinnedMetrics()
	require.NoError(t, err)
	require.Len(t, pinnedMetrics.PerModule, 1)
}
