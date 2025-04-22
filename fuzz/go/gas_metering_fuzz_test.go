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

// Operation types for gas testing
type GasOperation byte

const (
	OpInstantiate GasOperation = iota
	OpExecute
	OpQuery
	OpMigrate
	OpAnalyze
)

// GasPattern defines a sequence of operations with varying gas limits
type GasPattern struct {
	// Operation to perform
	Op GasOperation
	// Gas limit for this operation (0 = use default)
	GasLimit uint64
	// Message to use (nil = use default)
	CustomMsg []byte
}

func FuzzGasMetering(f *testing.F) {
	// Add seed corpus with different gas limits and patterns
	// Basic single operation patterns
	f.Add(uint64(1), uint64(0), uint8(0), []byte{}, []byte{})                       // Minimum gas
	f.Add(uint64(100_000), uint64(TESTING_GAS_LIMIT), uint8(1), []byte{}, []byte{}) // Low gas + high gas
	f.Add(TESTING_GAS_LIMIT/2, TESTING_GAS_LIMIT*2, uint8(2), []byte{}, []byte{})   // Medium gas variations
	f.Add(TESTING_GAS_LIMIT, TESTING_GAS_LIMIT, uint8(3), []byte{}, []byte{})       // Normal gas
	f.Add(TESTING_GAS_LIMIT*10, TESTING_GAS_LIMIT/10, uint8(4), []byte{}, []byte{}) // High gas + low gas

	// Add custom message scenarios
	f.Add(TESTING_GAS_LIMIT, TESTING_GAS_LIMIT, uint8(2),
		[]byte(`{"cpu_loop":{}}`), []byte(`{"verifier":{}}`)) // Gas-intensive + simple query
	f.Add(TESTING_GAS_LIMIT*2, TESTING_GAS_LIMIT*2, uint8(3),
		[]byte(`{"cpu_loop":{"iterations":100}}`), []byte(`{}`)) // More specific gas-intensive op

	f.Fuzz(func(t *testing.T,
		gasLimit1 uint64,
		gasLimit2 uint64,
		operationPattern uint8,
		customExecMsg []byte,
		customQueryMsg []byte,
	) {
		// Skip zero gas limit as it's trivially going to fail
		if gasLimit1 == 0 && gasLimit2 == 0 {
			return
		}

		// Ensure gas limits are usable but not extreme (to avoid timeouts)
		if gasLimit1 > TESTING_GAS_LIMIT*100 {
			gasLimit1 = TESTING_GAS_LIMIT * 100
		}
		if gasLimit2 > TESTING_GAS_LIMIT*100 {
			gasLimit2 = TESTING_GAS_LIMIT * 100
		}

		// Ensure at least one gas limit is reasonable
		if gasLimit1 < 100_000 && gasLimit2 < 100_000 {
			gasLimit1 = 100_000
		}

		// Create a new VM
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return
		}
		defer vm.Cleanup()

		// Load cyberpunk contract for gas testing (has cpu_loop)
		cyberpunkWasm, err := os.ReadFile("../../testdata/cyberpunk.wasm")
		if err != nil {
			return
		}

		// Load hackatom for comparison
		hackatomWasm, err := os.ReadFile("../../testdata/hackatom.wasm")
		if err != nil {
			return
		}

		// Store both contracts - use standard gas limit for storing
		cyberpunkChecksum, _, err := vm.StoreCode(cyberpunkWasm, TESTING_GAS_LIMIT)
		if err != nil {
			return
		}

		hackatomChecksum, _, err := vm.StoreCode(hackatomWasm, TESTING_GAS_LIMIT)
		if err != nil {
			return
		}

		// Determine which operations to perform and in what order
		var operations []GasPattern

		// Base pattern on the operation pattern byte
		switch operationPattern % 5 {
		case 0:
			// Simple instantiate + execute
			operations = []GasPattern{
				{OpInstantiate, gasLimit1, nil},
				{OpExecute, gasLimit2, customExecMsg},
			}
		case 1:
			// Instantiate + query + execute
			operations = []GasPattern{
				{OpInstantiate, gasLimit1, nil},
				{OpQuery, gasLimit2, customQueryMsg},
				{OpExecute, gasLimit1, customExecMsg},
			}
		case 2:
			// Instantiate both contracts + execute + query
			operations = []GasPattern{
				{OpInstantiate, gasLimit1, nil}, // cyberpunk
				{OpInstantiate, gasLimit2, nil}, // hackatom
				{OpExecute, gasLimit1, customExecMsg},
				{OpQuery, gasLimit2, customQueryMsg},
			}
		case 3:
			// Complex pattern with multiple operations and changing gas limits
			operations = []GasPattern{
				{OpInstantiate, gasLimit1, nil},
				{OpExecute, gasLimit2, customExecMsg},
				{OpQuery, gasLimit1 / 2, customQueryMsg},
				{OpExecute, gasLimit2 * 2, customExecMsg},
				{OpAnalyze, gasLimit1, nil},
			}
		case 4:
			// Staggered gas limits (high-low-high-low)
			operations = []GasPattern{
				{OpInstantiate, gasLimit1, nil},
				{OpExecute, gasLimit2, customExecMsg},
				{OpQuery, gasLimit1, customQueryMsg},
				{OpExecute, gasLimit2, customExecMsg},
			}
		}

		// Execute the operations
		executeOperation := func(
			op GasPattern,
			checksum wasmvm.Checksum,
			store *api.Lookup,
			params api.ContractCallParams,
		) (api.ContractCallParams, error) {
			// Create a new gas meter with the specified limit
			gasMeter := api.NewMockGasMeter(op.GasLimit)
			store.SetGasMeter(gasMeter)

			// Create a properly typed GasMeter reference
			igasMeter := types.GasMeter(gasMeter)

			// Clone and update params
			newParams := params
			newParams.GasLimit = op.GasLimit
			newParams.GasMeter = &igasMeter

			// Set appropriate message
			switch op.Op {
			case OpInstantiate:
				if op.CustomMsg != nil {
					newParams.Msg = op.CustomMsg
				} else {
					// Default init message depends on the contract
					if checksum == cyberpunkChecksum {
						newParams.Msg = []byte(`{}`)
					} else {
						newParams.Msg = []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
					}
				}
				_, _ = vm.Instantiate(newParams)

			case OpExecute:
				if op.CustomMsg != nil && isValidJSON(op.CustomMsg) {
					newParams.Msg = op.CustomMsg
				} else {
					// Default execute depends on the contract
					if checksum == cyberpunkChecksum {
						newParams.Msg = []byte(`{"cpu_loop":{}}`)
					} else {
						newParams.Msg = []byte(`{"release":{}}`)
					}
				}
				_, _ = vm.Execute(newParams)

			case OpQuery:
				if op.CustomMsg != nil && isValidJSON(op.CustomMsg) {
					newParams.Msg = op.CustomMsg
				} else {
					// Default query depends on the contract
					if checksum == cyberpunkChecksum {
						newParams.Msg = []byte(`{}`)
					} else {
						newParams.Msg = []byte(`{"verifier":{}}`)
					}
				}
				_, _ = vm.Query(newParams)

			case OpAnalyze:
				// Analyze doesn't need a message, just the checksum
				_, _ = vm.AnalyzeCode(checksum)
			}

			return newParams, nil
		}

		// Set up common execution environment
		store1 := api.NewLookup(api.NewMockGasMeter(gasLimit1))
		goapi1 := api.NewMockAPI()
		balance1 := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
		querier1 := api.DefaultQuerier(api.MockContractAddr, balance1)

		// Marshal environment
		env := api.MockEnv()
		info := api.MockInfo("creator", nil)
		envBytes, err := json.Marshal(env)
		if err != nil {
			return
		}
		infoBytes, err := json.Marshal(info)
		if err != nil {
			return
		}

		// Initial gas meter and params for cyberpunk
		initialGasMeter1 := api.NewMockGasMeter(gasLimit1)
		// Create a properly typed GasMeter reference
		igasMeter1 := types.GasMeter(initialGasMeter1)

		params1 := api.ContractCallParams{
			Checksum:   cyberpunkChecksum.Bytes(),
			Env:        envBytes,
			Info:       infoBytes,
			GasMeter:   &igasMeter1,
			Store:      store1,
			API:        goapi1,
			Querier:    &querier1,
			GasLimit:   gasLimit1,
			PrintDebug: false,
		}

		// Initial setup for hackatom
		store2 := api.NewLookup(api.NewMockGasMeter(gasLimit2))
		goapi2 := api.NewMockAPI()
		balance2 := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
		querier2 := api.DefaultQuerier(api.MockContractAddr, balance2)

		initialGasMeter2 := api.NewMockGasMeter(gasLimit2)
		// Create a properly typed GasMeter reference
		igasMeter2 := types.GasMeter(initialGasMeter2)

		params2 := api.ContractCallParams{
			Checksum:   hackatomChecksum.Bytes(),
			Env:        envBytes,
			Info:       infoBytes,
			GasMeter:   &igasMeter2,
			Store:      store2,
			API:        goapi2,
			Querier:    &querier2,
			GasLimit:   gasLimit2,
			PrintDebug: false,
		}

		// Execute operations alternating between contracts when appropriate
		useHackatom := false
		for i, op := range operations {
			// Toggle between contracts for multi-contract patterns
			if i > 0 && (operationPattern%5 == 2 || operationPattern%5 == 3) {
				useHackatom = !useHackatom
			}

			var err error
			if useHackatom {
				params2, err = executeOperation(op, hackatomChecksum, store2, params2)
			} else {
				params1, err = executeOperation(op, cyberpunkChecksum, store1, params1)
			}

			if err != nil {
				// If one operation fails (e.g., out of gas), just continue with the next
				continue
			}
		}

		// Check gas reports at the end
		metrics, _ := vm.GetMetrics()
		_ = metrics
	})
}
