//go:build cgo && !nolink_libwasmvm

package cosmwasm

import (
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// InstantiateWithConfig is a compatibility method that uses the VMConfig type
// Converts the old-style config to the new ContractCallParams format
func (vm *VM) InstantiateWithConfig(config VMConfig) (types.ContractResult, uint64, error) {
	// Marshal env and info to []byte as required by ContractCallParams
	envBytes, err := json.Marshal(config.Env)
	if err != nil {
		return types.ContractResult{}, 0, fmt.Errorf("failed to marshal env: %w", err)
	}

	infoBytes, err := json.Marshal(config.Info)
	if err != nil {
		return types.ContractResult{}, 0, fmt.Errorf("failed to marshal info: %w", err)
	}

	// Create a GasMeter interface pointer for ContractCallParams
	var gasMeter types.GasMeter = config.GasMeter

	// Convert GoAPI to pointer form
	goapi := &config.GoAPI

	// Create ContractCallParams
	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   config.Checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        config.Msg,
		GasMeter:   &gasMeter,
		Store:      config.Store,
		API:        goapi,
		Querier:    &config.Querier,
		GasLimit:   config.GasLimit,
		PrintDebug: vm.printDebug,
	}

	// Call the actual Instantiate function
	data, gasReport, err := api.Instantiate(params)
	if err != nil {
		return types.ContractResult{}, gasReport.UsedInternally, err
	}

	// Deserialize the result
	var result types.ContractResult
	err = DeserializeResponse(config.GasLimit, config.DeserCost, &gasReport, data, &result)
	if err != nil {
		return types.ContractResult{}, gasReport.UsedInternally, err
	}

	return result, gasReport.UsedInternally, nil
}

// ExecuteWithOldParams is a compatibility method for the old-style Execute function
func (vm *VM) ExecuteWithOldParams(checksum types.Checksum, env types.Env, info types.MessageInfo,
	msg []byte, store KVStore, goapi GoAPI, querier Querier,
	gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (types.ContractResult, uint64, error) {

	// Convert to the new style
	envBytes, err := json.Marshal(env)
	if err != nil {
		return types.ContractResult{}, 0, fmt.Errorf("failed to marshal env: %w", err)
	}

	infoBytes, err := json.Marshal(info)
	if err != nil {
		return types.ContractResult{}, 0, fmt.Errorf("failed to marshal info: %w", err)
	}

	// Type conversion for interface
	var gasMeterInterface types.GasMeter = gasMeter

	params := api.ContractCallParams{
		Cache:      vm.cache,
		Checksum:   checksum.Bytes(),
		Env:        envBytes,
		Info:       infoBytes,
		Msg:        msg,
		GasMeter:   &gasMeterInterface,
		Store:      store,
		API:        &goapi,
		Querier:    &querier,
		GasLimit:   gasLimit,
		PrintDebug: vm.printDebug,
	}

	data, gasReport, err := api.Execute(params)
	if err != nil {
		return types.ContractResult{}, gasReport.UsedInternally, err
	}

	// Deserialize the result
	var result types.ContractResult
	err = DeserializeResponse(gasLimit, deserCost, &gasReport, data, &result)
	if err != nil {
		return types.ContractResult{}, gasReport.UsedInternally, err
	}

	return result, gasReport.UsedInternally, nil
}
