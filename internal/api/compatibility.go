package api

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// WrapInstantiate is a helper function to call Instantiate with ContractCallParams
func WrapInstantiate(params ContractCallParams) ([]byte, types.GasReport, error) {
	// Note: Removed the internal creation of params, now it's passed directly
	return Instantiate(params)
}

// WrapExecute is a helper function to call Execute with ContractCallParams
func WrapExecute(params ContractCallParams) ([]byte, types.GasReport, error) {
	return Execute(params)
}

// WrapMigrate is a helper function to call Migrate with ContractCallParams
func WrapMigrate(params ContractCallParams) ([]byte, types.GasReport, error) {
	return Migrate(params)
}

// WrapSudo is a helper function to call Sudo with ContractCallParams
func WrapSudo(params ContractCallParams) ([]byte, types.GasReport, error) {
	return Sudo(params)
}

// WrapReply is a helper function to call Reply with ContractCallParams
func WrapReply(params ContractCallParams) ([]byte, types.GasReport, error) {
	return Reply(params)
}

// WrapQuery is a helper function to call Query with ContractCallParams
func WrapQuery(params ContractCallParams) ([]byte, types.GasReport, error) {
	return Query(params)
}
