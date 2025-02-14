package api

import (
	"fmt"
	"os"
	"path/filepath"

	wasm "github.com/CosmWasm/wasmvm/v2/internal/runtime/wasm"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func init() {
	// Create a new Wazero runtime instance and assign it to currentRuntime
	r, err := wasm.NewWazeroRuntime()
	if err != nil {
		panic(fmt.Sprintf("Failed to create wazero runtime: %v", err))
	}
	currentRuntime = r
}

type Cache struct {
	handle   any
	lockfile os.File
}

// currentRuntime should be initialized with an instance of WazeroRuntime or another runtime.
var currentRuntime wasm.WazeroRuntime

// InitCache initializes a new cache for the WASM runtime.
func InitCache(config types.VMConfig) (Cache, error) {
	err := os.MkdirAll(config.Cache.BaseDir, 0o755)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not create base directory: %w", err)
	}

	lockPath := filepath.Join(config.Cache.BaseDir, "exclusive.lock")
	lockfile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not open exclusive.lock: %w", err)
	}

	// Write the lockfile content
	_, err = lockfile.WriteString("lock")
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("Could not write to exclusive.lock: %w", err)
	}

	return Cache{
		handle:   currentRuntime.InitCache(config),
		lockfile: *lockfile,
	}, nil
}

// ReleaseCache releases resources associated with the cache.
func ReleaseCache(cache Cache) {
	cache.lockfile.Close()
	currentRuntime.ReleaseCache(cache.handle)
}

// StoreCode stores the given WASM code in the cache.
func StoreCode(cache Cache, wasm []byte, persist bool) ([]byte, error) {
	checksum, err := currentRuntime.StoreCode(wasm, persist)
	if err != nil {
		return nil, fmt.Errorf("Failed to store code: %w", err)
	}
	return checksum, nil
}

// StoreCodeUnchecked stores the given WASM code without performing checks.
func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	checksum, err := currentRuntime.StoreCodeUnchecked(wasm)
	if err != nil {
		return nil, fmt.Errorf("Failed to store code unchecked: %w", err)
	}
	return checksum, nil
}

// GetCode retrieves the WASM code for the given checksum.
func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	code, err := currentRuntime.GetCode(checksum)
	if err != nil {
		return nil, fmt.Errorf("Failed to get code: %w", err)
	}
	return code, nil
}

// RemoveCode removes the WASM code for the given checksum.
func RemoveCode(cache Cache, checksum []byte) error {
	return currentRuntime.RemoveCode(checksum)
}

// Pin pins the given checksum in the cache.
func Pin(cache Cache, checksum []byte) error {
	return currentRuntime.Pin(checksum)
}

// Unpin unpins the given checksum in the cache.
func Unpin(cache Cache, checksum []byte) error {
	return currentRuntime.Unpin(checksum)
}

// AnalyzeCode analyzes the given checksum for metrics.
func AnalyzeCode(cache Cache, checksum []byte) (*types.AnalysisReport, error) {
	report, err := currentRuntime.AnalyzeCode(checksum)
	if err != nil {
		return nil, fmt.Errorf("Failed to analyze code: %w", err)
	}
	return report, nil
}

// Instantiate instantiates a contract with the given parameters.
func Instantiate(
	cache Cache,
	checksum []byte,
	env []byte,
	info []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.Instantiate(checksum, env, info, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

// Execute executes a contract with the given parameters.
func Execute(
	cache Cache,
	checksum []byte,
	env []byte,
	info []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.Execute(checksum, env, info, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

// Query queries a contract with the given parameters.
func Query(
	cache Cache,
	checksum []byte,
	env []byte,
	query []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.Query(checksum, env, query, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCChannelOpen handles IBC channel opening.
func IBCChannelOpen(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCChannelOpen(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCChannelConnect handles IBC channel connection.
func IBCChannelConnect(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCChannelConnect(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCPacketReceive handles IBC packet reception.
func IBCPacketReceive(
	cache Cache,
	checksum []byte,
	env []byte,
	packet []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCPacketReceive(checksum, env, packet, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCPacketAck handles IBC packet acknowledgment.
func IBCPacketAck(
	cache Cache,
	checksum []byte,
	env []byte,
	ack []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCPacketAck(checksum, env, ack, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCPacketTimeout handles IBC packet timeout.
func IBCPacketTimeout(
	cache Cache,
	checksum []byte,
	env []byte,
	packet []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCPacketTimeout(checksum, env, packet, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCSourceCallback handles IBC source callbacks.
func IBCSourceCallback(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCSourceCallback(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

// IBCDestinationCallback handles IBC destination callbacks.
func IBCDestinationCallback(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.IBCDestinationCallback(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}
