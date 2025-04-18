package api

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// WrapInstantiate is a helper function to convert from old param style to new ContractCallParams style
func WrapInstantiate(cache Cache, checksum []byte, env []byte, info []byte, msg []byte, gasMeter *types.GasMeter,
	store *Lookup, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool,
) ([]byte, types.GasReport, error) {
	params := ContractCallParams{
		Cache:      cache,
		Checksum:   checksum,
		Env:        env,
		Info:       info,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        api,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}

	return Instantiate(params)
}

// WrapExecute is a helper function to convert from old param style to new ContractCallParams style
func WrapExecute(cache Cache, checksum []byte, env []byte, info []byte, msg []byte, gasMeter *types.GasMeter,
	store *Lookup, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool,
) ([]byte, types.GasReport, error) {
	params := ContractCallParams{
		Cache:      cache,
		Checksum:   checksum,
		Env:        env,
		Info:       info,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        api,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}

	return Execute(params)
}

// WrapMigrate is a helper function to convert from old param style to new ContractCallParams style
func WrapMigrate(cache Cache, checksum []byte, env []byte, msg []byte, gasMeter *types.GasMeter,
	store *Lookup, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool,
) ([]byte, types.GasReport, error) {
	params := ContractCallParams{
		Cache:      cache,
		Checksum:   checksum,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        api,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}

	return Migrate(params)
}

// WrapSudo is a helper function to convert from old param style to new ContractCallParams style
func WrapSudo(cache Cache, checksum []byte, env []byte, msg []byte, gasMeter *types.GasMeter,
	store *Lookup, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool,
) ([]byte, types.GasReport, error) {
	params := ContractCallParams{
		Cache:      cache,
		Checksum:   checksum,
		Env:        env,
		Msg:        msg,
		GasMeter:   gasMeter,
		Store:      store,
		API:        api,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}

	return Sudo(params)
}

// WrapReply is a helper function to convert from old param style to new ContractCallParams style
func WrapReply(cache Cache, checksum []byte, env []byte, reply []byte, gasMeter *types.GasMeter,
	store *Lookup, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool,
) ([]byte, types.GasReport, error) {
	params := ContractCallParams{
		Cache:      cache,
		Checksum:   checksum,
		Env:        env,
		Msg:        reply,
		GasMeter:   gasMeter,
		Store:      store,
		API:        api,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}

	return Reply(params)
}

// WrapQuery is a helper function to convert from old param style to new ContractCallParams style
func WrapQuery(cache Cache, checksum []byte, env []byte, query []byte, gasMeter *types.GasMeter,
	store *Lookup, api *types.GoAPI, querier *types.Querier, gasLimit uint64, printDebug bool,
) ([]byte, types.GasReport, error) {
	params := ContractCallParams{
		Cache:      cache,
		Checksum:   checksum,
		Env:        env,
		Msg:        query,
		GasMeter:   gasMeter,
		Store:      store,
		API:        api,
		Querier:    querier,
		GasLimit:   gasLimit,
		PrintDebug: printDebug,
	}

	return Query(params)
}
