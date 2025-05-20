//go:build wazero

package cosmwasm

import (
	"os"
	"testing"

	"github.com/CosmWasm/wasmvm/v3/internal/api"
	"github.com/CosmWasm/wasmvm/v3/types"
)

func TestWazeroInstantiateExecute(t *testing.T) {
	vm, err := NewVM("", TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG, TESTING_CACHE_SIZE)
	if err != nil {
		t.Fatal(err)
	}
	defer vm.Cleanup()

	wasm, err := os.ReadFile("testdata/hackatom.wasm")
	if err != nil {
		t.Fatal(err)
	}
	checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
	if err != nil {
		t.Fatalf("store error: %v", err)
	}

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	store := api.NewLookup(api.NewMockGasMeter(TESTING_GAS_LIMIT))
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, nil)

	res, _, err := vm.Instantiate(checksum, env, info, []byte(`{"verifier":"fred","beneficiary":"bob"}`), store, *api.NewMockAPI(), querier, api.NewMockGasMeter(TESTING_GAS_LIMIT), TESTING_GAS_LIMIT, types.UFraction{1, 1})
	t.Log("instantiate", res, err)

	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	execRes, _, err := vm.Execute(checksum, env, info, []byte(`{"release":{}}`), store, *api.NewMockAPI(), querier, api.NewMockGasMeter(TESTING_GAS_LIMIT), TESTING_GAS_LIMIT, types.UFraction{1, 1})
	t.Log("execute", execRes, err)
}
