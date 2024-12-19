package api

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func init() {
	// Create a new wazero runtime instance and assign it to currentRuntime
	r, err := runtime.NewWazeroRuntime()
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
var currentRuntime runtime.WasmRuntime

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

	_, err = lockfile.WriteString("This is a lockfile that prevents two VM instances from operating on the same directory in parallel.\nSee codebase at github.com/CosmWasm/wasmvm for more information.\nSafety first – brought to you by Confio ❤️\n")
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("Error writing to exclusive.lock: %w", err)
	}

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("Could not lock exclusive.lock: %w", err)
	}

	handle, err := currentRuntime.InitCache(config)
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("failed to init cache: %w", err)
	}

	return Cache{handle: handle, lockfile: *lockfile}, nil
}

func ReleaseCache(cache Cache) {
	currentRuntime.ReleaseCache(cache.handle)
}

func StoreCode(cache Cache, wasm []byte, persist bool) ([]byte, error) {
	if cache.handle == nil {
		return nil, fmt.Errorf("cache handle is nil")
	}
	checksum, err, _ := currentRuntime.StoreCode(wasm)
	return checksum, err
}

func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	checksum, err := currentRuntime.StoreCodeUnchecked(wasm)
	return checksum, err
}

func RemoveCode(cache Cache, checksum []byte) error {
	return currentRuntime.RemoveCode(checksum)
}

func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	return currentRuntime.GetCode(checksum)
}

func Pin(cache Cache, checksum []byte) error {
	return currentRuntime.Pin(checksum)
}

func Unpin(cache Cache, checksum []byte) error {
	return currentRuntime.Unpin(checksum)
}

func AnalyzeCode(cache Cache, checksum []byte) (*types.AnalysisReport, error) {
	return currentRuntime.AnalyzeCode(checksum)
}

func GetMetrics(cache Cache) (*types.Metrics, error) {
	return currentRuntime.GetMetrics()
}

func GetPinnedMetrics(cache Cache) (*types.PinnedMetrics, error) {
	return currentRuntime.GetPinnedMetrics()
}

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

func Migrate(
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
	return currentRuntime.Migrate(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func MigrateWithInfo(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	migrateInfo []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.MigrateWithInfo(checksum, env, msg, migrateInfo, gasMeter, store, api, querier, gasLimit, printDebug)
}

func Sudo(
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
	return currentRuntime.Sudo(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

func Reply(
	cache Cache,
	checksum []byte,
	env []byte,
	reply []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *types.Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	return currentRuntime.Reply(checksum, env, reply, gasMeter, store, api, querier, gasLimit, printDebug)
}

func Query(
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
	return currentRuntime.Query(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

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

func IBCChannelClose(
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
	return currentRuntime.IBCChannelClose(checksum, env, msg, gasMeter, store, api, querier, gasLimit, printDebug)
}

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
