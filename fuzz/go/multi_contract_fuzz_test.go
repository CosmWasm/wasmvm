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

// ContractInstance tracks an instantiated contract
type ContractInstance struct {
	Checksum wasmvm.Checksum
	Params   api.ContractCallParams
	Store    *api.Lookup
	GasMeter api.MockGasMeter
}

// ContractInteractionPlan defines how contracts interact
type ContractInteractionPlan struct {
	// Sequence of contract indexes to call (0, 1, 2, etc.)
	Sequence []byte
	// Type of operation to perform (0=execute, 1=query)
	Operations []byte
	// Message complexity (0=simple, 1=medium, 2=complex)
	Complexity []byte
}

func FuzzMultiContract(f *testing.F) {
	// Add seed corpus with execute message and interaction plans
	// Basic contract interaction patterns
	f.Add([]byte{0, 1, 0, 1}, []byte{0, 0, 0, 0}, []byte{0, 0, 0, 0}, uint8(3))                   // Simple alternating calls between contracts
	f.Add([]byte{0, 0, 0, 1, 1, 1}, []byte{0, 0, 0, 0, 0, 0}, []byte{0, 1, 2, 0, 1, 2}, uint8(2)) // Multiple calls to first then second
	f.Add([]byte{0, 1, 0, 1}, []byte{0, 1, 0, 1}, []byte{0, 1, 1, 2}, uint8(4))                   // Mix of execute and query operations
	f.Add([]byte{0, 1, 2, 0, 1, 2}, []byte{0, 0, 0, 1, 1, 1}, []byte{2, 2, 2, 2, 2, 2}, uint8(1)) // Three contracts with complex messages

	f.Fuzz(func(t *testing.T,
		sequenceData []byte,
		operationsData []byte,
		complexityData []byte,
		contractCount uint8,
	) {
		// Cap max contract count to avoid excessive test time
		if contractCount == 0 {
			contractCount = 1
		}
		if contractCount > 5 {
			contractCount = 5
		}

		// Ensure at least some operations
		if len(sequenceData) == 0 || len(operationsData) == 0 || len(complexityData) == 0 {
			return
		}

		// Create a new VM for each test
		tmpdir := t.TempDir()
		vm, err := wasmvm.NewVM(tmpdir, TESTING_CAPABILITIES, TESTING_MEMORY_LIMIT, false, TESTING_CACHE_SIZE)
		if err != nil {
			return
		}
		defer vm.Cleanup()

		// Load available contract types
		wasmFiles := []string{
			"../../testdata/hackatom.wasm",
			"../../testdata/cyberpunk.wasm",
		}

		if len(wasmFiles) == 0 {
			t.Skip("No contract files available")
		}

		// Store all contracts
		var contractCode [][]byte
		for _, path := range wasmFiles {
			wasm, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			contractCode = append(contractCode, wasm)
		}

		if len(contractCode) == 0 {
			return
		}

		// Store the contracts
		var storedContracts []wasmvm.Checksum
		for _, wasm := range contractCode {
			checksum, _, err := vm.StoreCode(wasm, TESTING_GAS_LIMIT)
			if err != nil {
				continue
			}
			storedContracts = append(storedContracts, checksum)
		}

		if len(storedContracts) == 0 {
			return
		}

		// Create instances based on requested count (cycling through available contracts)
		var instances []ContractInstance
		for i := uint8(0); i < contractCount; i++ {
			// Select contract code (cycle through available contracts)
			checksumIndex := int(i) % len(storedContracts)
			checksum := storedContracts[checksumIndex]

			// Create environment specific to this instance
			gasMeter := api.NewMockGasMeter(TESTING_GAS_LIMIT)
			store := api.NewLookup(gasMeter)
			goapi := api.NewMockAPI()
			balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
			querier := api.DefaultQuerier(api.MockContractAddr, balance)

			// Create unique actor name for this contract
			actorName := "actor-" + string(rune(65+i)) // A, B, C, etc.

			// Env and info for the contract
			env := api.MockEnv()
			info := api.MockInfo(actorName, nil)

			// Marshal env and info
			envBytes, err := json.Marshal(env)
			if err != nil {
				continue
			}
			infoBytes, err := json.Marshal(info)
			if err != nil {
				continue
			}

			// Set up contract params
			igasMeter := types.GasMeter(gasMeter)
			params := api.ContractCallParams{
				Checksum:   checksum.Bytes(),
				Env:        envBytes,
				Info:       infoBytes,
				GasMeter:   &igasMeter,
				Store:      store,
				API:        goapi,
				Querier:    &querier,
				GasLimit:   TESTING_GAS_LIMIT,
				PrintDebug: false,
			}

			// Create appropriate init message based on contract type
			var initMsg []byte
			if checksumIndex%2 == 0 {
				// For hackatom
				initMsg = []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
			} else {
				// For cyberpunk
				initMsg = []byte(`{}`)
			}

			// Instantiate the contract
			params.Msg = initMsg
			_, err = vm.Instantiate(params)
			if err != nil {
				continue
			}

			// Save the instance
			instances = append(instances, ContractInstance{
				Checksum: checksum,
				Params:   params,
				Store:    store,
				GasMeter: gasMeter,
			})
		}

		// Need at least two contract instances for meaningful interactions
		if len(instances) < 1 {
			return
		}

		// Parse interaction plan
		plan := ContractInteractionPlan{
			Sequence:   sequenceData,
			Operations: operationsData,
			Complexity: complexityData,
		}

		// Execute the interaction plan
		maxOps := minOf4(len(plan.Sequence), len(plan.Operations), len(plan.Complexity), 20) // Cap at 20 operations max

		for i := 0; i < maxOps; i++ {
			// Select the contract based on sequence
			contractIdx := int(plan.Sequence[i]) % len(instances)
			instance := &instances[contractIdx]

			// Get operation type (0=execute, 1=query)
			isQuery := plan.Operations[i]%2 == 1

			// Get message complexity (0=simple, 1=medium, 2=complex)
			complexity := plan.Complexity[i] % 3

			// Generate appropriate message
			var msg []byte
			if isContractType(instance.Checksum, storedContracts, 0) {
				// Hackatom messages
				if isQuery {
					switch complexity {
					case 0:
						msg = []byte(`{"verifier":{}}`)
					case 1:
						msg = []byte(`{"raw_state":{"key":"config"}}`)
					case 2:
						msg = []byte(`{"other_msg":{"some_key":"some_value","nested":{"field1":"value1","field2":123}}}`)
					}
				} else {
					switch complexity {
					case 0:
						msg = []byte(`{"release":{}}`)
					case 1:
						msg = []byte(`{"change_owner":{"owner":"new_owner"}}`)
					case 2:
						msg = []byte(`{"complex_msg":{"nested":{"field1":"value1","field2":123},"array":[1,2,3]}}`)
					}
				}
			} else {
				// Cyberpunk or other messages
				if isQuery {
					switch complexity {
					case 0:
						msg = []byte(`{}`)
					case 1:
						msg = []byte(`{"config":{}}`)
					case 2:
						msg = []byte(`{"complex_query":{"nested":{"field1":"value1","field2":123},"array":[1,2,3]}}`)
					}
				} else {
					switch complexity {
					case 0:
						// Simple execution
						msg = []byte(`{"simple_action":{}}`)
					case 1:
						// CPU-intensive
						msg = []byte(`{"cpu_loop":{"iterations": 5}}`)
					case 2:
						// Complex execution
						msg = []byte(`{"complex_action":{"nested":{"field1":"value1","field2":123},"array":[1,2,3]}}`)
					}
				}
			}

			// Create a new gas meter for this operation
			gasMeter := api.NewMockGasMeter(TESTING_GAS_LIMIT)
			instance.GasMeter = gasMeter
			instance.Store.SetGasMeter(gasMeter)

			// Clone params for this operation
			params := instance.Params
			// Create a properly typed GasMeter reference
			igasMeter := types.GasMeter(gasMeter)
			params.GasMeter = &igasMeter
			params.Msg = msg

			// Execute or query based on operation type
			if isQuery {
				_, _ = vm.Query(params)
			} else {
				_, _ = vm.Execute(params)
			}
		}

		// Final metrics check
		_, _ = vm.GetMetrics()
	})
}

// Helper to determine if a checksum is of a specific contract type
func isContractType(checksum wasmvm.Checksum, checksums []wasmvm.Checksum, typeIdx int) bool {
	if typeIdx >= len(checksums) {
		return false
	}

	// Simple equality check
	return checksum == checksums[typeIdx]
}

// Helper to find minimum value of 4 integers
func minOf4(a, b, c, d int) int {
	if a < b {
		if a < c {
			if a < d {
				return a
			}
			return d
		}
		if c < d {
			return c
		}
		return d
	}
	if b < c {
		if b < d {
			return b
		}
		return d
	}
	if c < d {
		return c
	}
	return d
}
