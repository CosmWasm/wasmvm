package gofuzz

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v2"
	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func FuzzContractExecution(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(`{"verifier": "fred", "beneficiary": "bob"}`), []byte(`{"release":{}}`)) // Valid hackatom instantiate/execute
	f.Add([]byte(`{}`), []byte(`{}`))                                                     // Empty JSON objects
	f.Add([]byte(`{"random": true}`), []byte(`{"random": 123}`))                          // Random JSON

	f.Fuzz(func(t *testing.T, instantiateMsg, executeMsg []byte) {
		// Skip if the messages are not valid JSON
		if !isValidJSON(instantiateMsg) || !isValidJSON(executeMsg) {
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

		// Create environment
		env := api.MockEnv()
		info := api.MockInfo("creator", nil)

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

		// Use ContractCallParams for instantiation
		instantiateParams := api.ContractCallParams{
			Checksum:   checksum.Bytes(),
			Env:        envBytes,
			Info:       infoBytes,
			Msg:        instantiateMsg,
			GasMeter:   &igasMeter,
			Store:      store,
			API:        goapi,
			Querier:    &querier,
			GasLimit:   TESTING_GAS_LIMIT,
			PrintDebug: false,
		}

		// Try instantiate - ignore errors since fuzzing will generate many invalid inputs
		_, err = vm.Instantiate(instantiateParams)
		if err != nil {
			return
		}

		// Execute with a new gas meter
		gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
		// Create a properly typed GasMeter reference
		igasMeter2 := types.GasMeter(gasMeter2)
		store.SetGasMeter(gasMeter2)

		// Change sender for execution
		info = api.MockInfo("fred", nil)
		infoBytes, err = json.Marshal(info)
		if err != nil {
			return
		}

		// Use ContractCallParams for execution
		executeParams := api.ContractCallParams{
			Checksum:   checksum.Bytes(),
			Env:        envBytes,
			Info:       infoBytes,
			Msg:        executeMsg,
			GasMeter:   &igasMeter2,
			Store:      store,
			API:        goapi,
			Querier:    &querier,
			GasLimit:   TESTING_GAS_LIMIT,
			PrintDebug: false,
		}

		// Execute - ignore errors since fuzzing will generate many invalid inputs
		_, _ = vm.Execute(executeParams)
	})
}

// Helper function to check if a byte slice is valid JSON
func isValidJSON(data []byte) bool {
	var js any
	return json.Unmarshal(data, &js) == nil
}
