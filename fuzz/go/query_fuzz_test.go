//go:build go1.18

package gofuzz

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v2"
	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func FuzzQuery(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(`{"verifier": {}}`))                         // Valid hackatom query
	f.Add([]byte(`{}`))                                       // Empty JSON object
	f.Add([]byte(`{"random": true}`))                         // Random JSON
	f.Add([]byte(`{"balance": {"address": "some-address"}}`)) // Common query pattern

	f.Fuzz(func(t *testing.T, queryMsg []byte) {
		// Skip if the message is not valid JSON
		if !isValidJSON(queryMsg) {
			return
		}

		// Setup VM
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return
		}
		defer vm.Cleanup()

		// Load hackatom contract
		wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
		if err != nil {
			t.Fatal(err)
		}

		// Store code
		checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
		if err != nil {
			return
		}

		// Set up execution environment
		gasMeter := api.NewMockGasMeter(TESTING_GAS_LIMIT)
		store := api.NewLookup(gasMeter)
		goapi := api.NewMockAPI()
		balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
		querier := api.DefaultQuerier(api.MockContractAddr, balance)

		// Instantiate contract first
		env := api.MockEnv()
		info := api.MockInfo("creator", nil)

		initMsg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

		// Marshal env and info
		envBytes, err := json.Marshal(env)
		if err != nil {
			return
		}
		infoBytes, err := json.Marshal(info)
		if err != nil {
			return
		}

		// Convert to types.GasMeter - create a properly typed reference
		igasMeter := types.GasMeter(gasMeter)

		// Instantiate the contract first
		instantiateParams := api.ContractCallParams{
			Checksum:   checksum.Bytes(),
			Env:        envBytes,
			Info:       infoBytes,
			Msg:        initMsg,
			GasMeter:   &igasMeter,
			Store:      store,
			API:        goapi,
			Querier:    &querier,
			GasLimit:   TESTING_GAS_LIMIT,
			PrintDebug: false,
		}

		_, err = vm.Instantiate(instantiateParams)
		if err != nil {
			return
		}

		// Now query the contract with fuzzing input
		queryParams := api.ContractCallParams{
			Checksum:   checksum.Bytes(),
			Env:        envBytes,
			Msg:        queryMsg,
			GasMeter:   &igasMeter,
			Store:      store,
			API:        goapi,
			Querier:    &querier,
			GasLimit:   TESTING_GAS_LIMIT,
			PrintDebug: false,
		}

		// We're ignoring any errors - the purpose is to catch panics, overflows, etc.
		_, _ = vm.Query(queryParams)
	})
}
