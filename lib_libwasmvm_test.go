//go:build cgo && !nolink_libwasmvm

package wasmvm

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
	TESTING_PRINT_DEBUG  = false
	TESTING_GAS_LIMIT    = uint64(500_000_000_000) // ~0.5ms
	TESTING_MEMORY_LIMIT = 32                      // MiB
	TESTING_CACHE_SIZE   = 100                     // MiB
)

var TESTING_CAPABILITIES = []string{"staking", "stargate", "iterator"}

const (
	CYBERPUNK_TEST_CONTRACT = "./testdata/cyberpunk.wasm"
	HACKATOM_TEST_CONTRACT  = "./testdata/hackatom.wasm"
)

func withVM(t *testing.T) *VM {
	t.Helper()
	tmpdir := t.TempDir()
	vm, err := NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG, TESTING_CACHE_SIZE)
	require.NoError(t, err)

	t.Cleanup(func() {
		vm.Cleanup()
	})
	return vm
}

func createTestContract(t *testing.T, vm *VM, path string) Checksum {
	t.Helper()
	// #nosec G304 -- This is test code using hardcoded test files
	wasm, err := os.ReadFile(path)
	require.NoError(t, err)
	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	require.NoError(t, err)
	return checksum
}

func TestStoreCode(t *testing.T) {
	vm := withVM(t)

	// Valid hackatom contract
	{
		wasm, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
		require.NoError(t, err)
		_, _, err = vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.NoError(t, err)
	}

	// Valid cyberpunk contract
	{
		wasm, err := os.ReadFile(CYBERPUNK_TEST_CONTRACT)
		require.NoError(t, err)
		_, _, err = vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.NoError(t, err)
	}

	// Valid Wasm with no exports
	{
		// echo '(module)' | wat2wasm - -o empty.wasm
		// hexdump -C < empty.wasm
		wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
		_, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.ErrorContains(t, err, "Wasm contract must contain exactly one memory")
	}

	// No Wasm
	{
		wasm := []byte("foobar")
		_, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.ErrorContains(t, err, "could not be deserialized")
	}

	// Empty
	{
		wasm := []byte("")
		_, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.ErrorContains(t, err, "could not be deserialized")
	}

	// Nil
	{
		var wasm []byte
		var err error
		_, _, err = vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		require.ErrorContains(t, err, "Null/Nil argument")
	}
}

func TestSimulateStoreCode(t *testing.T) {
	vm := withVM(t)

	hackatom, err := os.ReadFile(HACKATOM_TEST_CONTRACT)
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
			err:  "magic header not detected: bad magic number",
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			checksum, _, err := vm.SimulateStoreCode(spec.wasm, TESTING_GAS_LIMIT)

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

	// instantiate it with this store
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MockContractAddr, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	// Marshal env and info
	envBytes, err := json.Marshal(env)
	require.NoError(t, err, "Failed to marshal env")
	infoBytes, err := json.Marshal(info)
	require.NoError(t, err, "Failed to marshal info")

	// Convert to types.GasMeter
	igasMeter1 := types.GasMeter(gasMeter1)
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msg,
		GasMeter:   &igasMeter1,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: vm.printDebug,
	}

	// Update this line to capture all 4 return values (the ContractResult is new)
	resBytes, contractResult, gasReport, err := vm.Instantiate(params)
	require.NoError(t, err)
	require.Greater(t, gasReport.UsedInternally, uint64(0), "Expected some gas to be used during instantiation")

	// We've already got the unmarshaled result, so we can skip this step
	// (But if you want to keep it for clarity, that's fine too)
	var initResult types.ContractResult
	if resBytes != nil {
		err = json.Unmarshal(resBytes, &initResult)
		require.NoError(t, err)
		require.Equal(t, contractResult, initResult)
	} else {
		initResult = contractResult
	}

	require.Empty(t, initResult.Err, "Contract error should be empty")
	require.NotNil(t, initResult.Ok)
	ires := initResult.Ok
	require.Empty(t, ires.Messages)

	// execute
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	executeMsg := []byte(`{"release":{}}`)

	// Marshal new env and info
	envBytes, err = json.Marshal(env)
	require.NoError(t, err, "Failed to marshal env")
	infoBytes, err = json.Marshal(info)
	require.NoError(t, err, "Failed to marshal info")

	executeParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        executeMsg,
		GasMeter:   &igasMeter2,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: vm.printDebug,
	}

	// Update to get all 4 return values
	execResBytes, execResult, gasReport, err := vm.Execute(executeParams)
	require.NoError(t, err)
	require.Greater(t, gasReport.UsedInternally, uint64(0), "Expected some gas to be used during execution")

	// Verify that raw response bytes correctly unmarshal to match execResult
	var parsedResult types.ContractResult
	err = json.Unmarshal(execResBytes, &parsedResult)
	require.NoError(t, err)
	require.Equal(t, execResult, parsedResult, "VM-parsed result should match manually parsed result")

	// Rest of the function remains the same
	// No need to unmarshal unless you want to validate
	require.Empty(t, execResult.Err, "Contract error should be empty")
	require.NotNil(t, execResult.Ok)
	hres := execResult.Ok
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

	// Initialize all variables needed for instantiation
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	querier := api.DefaultQuerier(api.MockContractAddr, nil)

	// Prepare env and info
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{}`)

	// Marshal env and info
	envBytes, err := json.Marshal(env)
	require.NoError(t, err)
	infoBytes, err := json.Marshal(info)
	require.NoError(t, err)

	// Create params for instantiate
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msg,
		GasMeter:   &igasMeter1,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: vm.printDebug,
	}

	// Call instantiate with all 4 return values
	_, result, _, err := vm.Instantiate(params)
	require.NoError(t, err)
	require.Empty(t, result.Err, "Contract error should be empty")
	require.NotNil(t, result.Ok)
	ires := result.Ok
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

	// Create new info and message for the execute call
	info = api.MockInfo("creator", nil)
	msg = []byte(`{"mirror_env": {}}`)

	// Marshal updated env and info
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	infoBytes, err = json.Marshal(info)
	require.NoError(t, err)

	// Create execute params
	executeParams := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msg,
		GasMeter:   &igasMeter1,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: vm.printDebug,
	}

	// Execute with 4 return values
	_, result, _, err = vm.Execute(executeParams)
	require.NoError(t, err)
	require.Empty(t, result.Err, "Contract error should be empty")
	require.NotNil(t, result.Ok)
	ires = result.Ok

	// Verify result matches expected env
	expected, err := json.Marshal(env)
	require.NoError(t, err, "Failed to marshal expected env")
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

	// Update the env in executeParams
	envBytes, err = json.Marshal(env)
	require.NoError(t, err)
	executeParams.Env = envBytes

	// Execute again
	_, result, _, err = vm.Execute(executeParams)
	require.NoError(t, err)
	require.Empty(t, result.Err, "Contract error should be empty")
	require.NotNil(t, result.Ok)
	ires = result.Ok

	// Verify again
	expected, err = json.Marshal(env)
	require.NoError(t, err, "Failed to marshal expected env")
	require.Equal(t, expected, ires.Data)
}

func TestGetMetrics(t *testing.T) {
	vm := withVM(t)

	// Initial state - verify empty metrics
	initialMetrics, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, initialMetrics, "Initial metrics should be empty")

	// Create contract - this should cause a file cache hit when checking code
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	// Verify metrics still empty (only code store happened, no cache hits yet)
	afterStoreMetrics, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, afterStoreMetrics, "Metrics should be empty after code store")

	// Prepare for contract instantiation
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MockContractAddr, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	envBytes, err := json.Marshal(env)
	require.NoError(t, err, "Failed to marshal env")
	infoBytes, err := json.Marshal(info)
	require.NoError(t, err, "Failed to marshal info")

	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msg1,
		GasMeter:   &igasMeter1,
		Store:      store,
		API:        goapi,
		Querier:    &querier,
		GasLimit:   TESTING_GAS_LIMIT,
		PrintDebug: vm.printDebug,
	}

	// First instantiation - expected to cause file cache hit
	resBytes, result, gasReport, err := vm.Instantiate(params)
	require.NoError(t, err, "Instantiation should succeed")
	require.NotNil(t, resBytes, "Response bytes should not be nil")
	require.Empty(t, result.Err, "Contract error should be empty")
	require.NotNil(t, result.Ok, "Contract result should not be nil")
	require.Greater(t, gasReport.UsedInternally, uint64(0), "Gas should be consumed")
	ires := result.Ok
	require.Empty(t, ires.Messages, "No messages should be returned")

	// Verify file cache hit for first instantiation
	metricsAfterFirstInstantiate, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, uint32(0), metricsAfterFirstInstantiate.HitsMemoryCache, "No memory cache hit expected")
	assert.Equal(t, uint32(1), metricsAfterFirstInstantiate.HitsFsCache, "Expected 1 file cache hit")
	assert.Equal(t, uint64(1), metricsAfterFirstInstantiate.ElementsMemoryCache, "Expected 1 item in memory cache")
	require.InEpsilon(t, 3700000, metricsAfterFirstInstantiate.SizeMemoryCache, 0.25, "Memory cache size should be around 3.7MB")

	// Second instantiation - expected to cause memory cache hit
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	params.Msg = msg2
	resBytes, result, gasReport, err = vm.Instantiate(params)
	require.NoError(t, err, "Second instantiation should succeed")
	require.NotNil(t, resBytes, "Response bytes should not be nil")
	require.Empty(t, result.Err, "Contract error should be empty")
	require.NotNil(t, result.Ok, "Contract result should not be nil")
	require.Greater(t, gasReport.UsedInternally, uint64(0), "Gas should be consumed")
	ires = result.Ok
	require.Empty(t, ires.Messages, "No messages should be returned")

	// Verify memory cache hit
	metricsAfterSecondInstantiate, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), metricsAfterSecondInstantiate.HitsMemoryCache, "Expected 1 memory cache hit")
	assert.Equal(t, uint32(1), metricsAfterSecondInstantiate.HitsFsCache, "File cache hits should remain at 1")
	assert.Equal(t, uint64(1), metricsAfterSecondInstantiate.ElementsMemoryCache, "Expected 1 item in memory cache")
	assert.Equal(t, metricsAfterFirstInstantiate.SizeMemoryCache, metricsAfterSecondInstantiate.SizeMemoryCache, "Memory cache size should be unchanged")

	// Pin the contract - should copy from memory cache to pinned cache
	err = vm.Pin(checksum)
	require.NoError(t, err, "Pinning should succeed")

	// Verify pin metrics
	metricsAfterPin, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), metricsAfterPin.HitsMemoryCache, "Memory cache hits should remain at 1")
	assert.Equal(t, uint32(2), metricsAfterPin.HitsFsCache, "Expected 2 file cache hits") // One more for pinning
	assert.Equal(t, uint64(1), metricsAfterPin.ElementsPinnedMemoryCache, "Expected 1 item in pinned cache")
	assert.Equal(t, uint64(1), metricsAfterPin.ElementsMemoryCache, "Expected 1 item in memory cache")
	assert.Greater(t, metricsAfterPin.SizePinnedMemoryCache, uint64(0), "Pinned cache size should be non-zero")
	assert.InEpsilon(t, metricsAfterSecondInstantiate.SizeMemoryCache, metricsAfterPin.SizePinnedMemoryCache, 0.01, "Pinned cache size should match memory cache size")

	// Third instantiation - expected to use pinned cache
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	params.Msg = msg3
	resBytes, result, gasReport, err = vm.Instantiate(params)
	require.NoError(t, err, "Third instantiation should succeed")
	require.NotNil(t, resBytes, "Response bytes should not be nil")
	require.Empty(t, result.Err, "Contract error should be empty")
	require.NotNil(t, result.Ok, "Contract result should not be nil")
	require.Greater(t, gasReport.UsedInternally, uint64(0), "Gas should be consumed")
	ires = result.Ok
	require.Empty(t, ires.Messages, "No messages should be returned")

	// Verify pinned cache hit
	metricsAfterThirdInstantiate, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), metricsAfterThirdInstantiate.HitsPinnedMemoryCache, "Expected 1 pinned cache hit")
	assert.Equal(t, uint32(1), metricsAfterThirdInstantiate.HitsMemoryCache, "Memory cache hits should remain at 1")
	assert.Equal(t, uint32(2), metricsAfterThirdInstantiate.HitsFsCache, "File cache hits should remain at 2")
	assert.Equal(t, uint64(1), metricsAfterThirdInstantiate.ElementsPinnedMemoryCache, "Expected 1 item in pinned cache")
	assert.Equal(t, uint64(1), metricsAfterThirdInstantiate.ElementsMemoryCache, "Expected 1 item in memory cache")

	// Unpin the contract - should remove from pinned cache
	err = vm.Unpin(checksum)
	require.NoError(t, err, "Unpinning should succeed")

	// Verify unpin metrics
	metricsAfterUnpin, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), metricsAfterUnpin.HitsPinnedMemoryCache, "Pinned cache hits should remain at 1")
	assert.Equal(t, uint32(1), metricsAfterUnpin.HitsMemoryCache, "Memory cache hits should remain at 1")
	assert.Equal(t, uint32(2), metricsAfterUnpin.HitsFsCache, "File cache hits should remain at 2")
	assert.Equal(t, uint64(0), metricsAfterUnpin.ElementsPinnedMemoryCache, "Pinned cache should be empty")
	assert.Equal(t, uint64(1), metricsAfterUnpin.ElementsMemoryCache, "Expected 1 item in memory cache")
	assert.Equal(t, uint64(0), metricsAfterUnpin.SizePinnedMemoryCache, "Pinned cache size should be zero")
}

func TestLongPayloadDeserialization(t *testing.T) {
	deserCost := types.UFraction{Numerator: 1, Denominator: 1}
	gasReport := types.GasReport{}

	// Create a valid payload
	validPayload := make([]byte, 1024*1024) // 1 MiB
	validPayloadJSON, err := json.Marshal(validPayload)
	require.NoError(t, err, "Failed to marshal valid payload")
	var resultJson []byte
	resultJson = fmt.Appendf(resultJson, `{"ok":{"messages":[{"id":0,"msg":{"bank":{"send":{"to_address":"bob","amount":[{"denom":"ATOM","amount":"250"}]}}},"payload":%s,"reply_on":"never"}],"data":"8Auq","attributes":[],"events":[]}}`, validPayloadJSON)

	// Test that a valid payload can be deserialized
	var result types.ContractResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &result)
	require.NoError(t, err)
	require.NotNil(t, result.Ok, "Expected valid ContractResult.Ok")
	require.Len(t, result.Ok.Messages, 1, "Expected one message in ContractResult.Ok")
	require.Equal(t, validPayload, result.Ok.Messages[0].Payload)

	// Create a larger payload (20 MiB) - this is now supported as well
	largerPayload := make([]byte, 20*1024*1024) // 20 MiB
	largerPayloadJSON, err := json.Marshal(largerPayload)
	require.NoError(t, err, "Failed to marshal larger payload")
	resultJson = fmt.Appendf(resultJson[:0], `{"ok":{"messages":[{"id":0,"msg":{"bank":{"send":{"to_address":"bob","amount":[{"denom":"ATOM","amount":"250"}]}}},"payload":%s,"reply_on":"never"}],"attributes":[],"events":[]}}`, largerPayloadJSON)

	// Test that a larger payload can also be deserialized
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &result)
	require.NoError(t, err)
	require.NotNil(t, result.Ok, "Expected valid ContractResult.Ok")
	require.Len(t, result.Ok.Messages, 1, "Expected one message in ContractResult.Ok")
	require.Equal(t, largerPayload, result.Ok.Messages[0].Payload)

	// Test with IBCBasicResult
	var ibcResult types.IBCBasicResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &ibcResult)
	require.NoError(t, err)

	// Test with IBCReceiveResult
	var ibcReceiveResult types.IBCReceiveResult
	err = DeserializeResponse(math.MaxUint64, deserCost, &gasReport, resultJson, &ibcReceiveResult)
	require.NoError(t, err)
}
