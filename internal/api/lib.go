package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// Package api provides the core functionality for interacting with the wasmvm.

// Value types.
type (
	cint   = C.int
	cbool  = C.bool
	cusize = C.size_t
	cu8    = C.uint8_t
	cu32   = C.uint32_t
	cu64   = C.uint64_t
	ci8    = C.int8_t
	ci32   = C.int32_t
	ci64   = C.int64_t
)

// Pointers.
type (
	cu8_ptr = *C.uint8_t
)

// Cache represents a cache for storing and retrieving wasm code.
type Cache struct {
	ptr      *C.cache_t
	lockfile os.File
}

// Querier represents a type that can query the state of the blockchain.
type Querier = types.Querier

// cu8Ptr represents a pointer to an unsigned 8-bit integer.
type cu8Ptr = *C.uint8_t

func InitCache(config types.VMConfig) (Cache, error) {
	// libwasmvm would create this directory too but we need it earlier for the lockfile.
	err := os.MkdirAll(config.Cache.BaseDir, 0o755)
	if err != nil {
		return Cache{}, fmt.Errorf("could not create base directory")
	}

	lockfile, err := os.OpenFile(filepath.Join(config.Cache.BaseDir, "exclusive.lock"), os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return Cache{}, fmt.Errorf("could not open exclusive.lock")
	}
	_, err = lockfile.WriteString("This is a lockfile that prevent two VM instances to operate on the same directory in parallel.\nSee codebase at github.com/CosmWasm/wasmvm for more information.\nSafety first – brought to you by Confio ❤️\n")
	if err != nil {
		return Cache{}, fmt.Errorf("error writing to exclusive.lock")
	}

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		return Cache{}, fmt.Errorf("could not lock exclusive.lock. Is a different VM running in the same directory already?")
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return Cache{}, fmt.Errorf("could not serialize config")
	}
	configView := makeView(configBytes)
	defer runtime.KeepAlive(configBytes)

	errmsg := uninitializedUnmanagedVector()

	ptr, err := C.init_cache(configView, &errmsg)
	if err != nil {
		return Cache{}, errorWithMessage(err, errmsg)
	}
	return Cache{ptr: ptr, lockfile: *lockfile}, nil
}

// logCleanupError logs errors that occur during cleanup operations.
// These errors are not critical as cleanup will happen when the process exits anyway.
func logCleanupError(op string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", op, err)
	}
}

// ReleaseCache releases the resources associated with the cache.
func ReleaseCache(cache Cache) {
	// First close the lockfile to release the lock
	err := cache.lockfile.Close()
	if err != nil {
		logCleanupError("failed to close lockfile", err)
		return
	}
	// Only release the cache if the lockfile was closed successfully
	C.release_cache(cache.ptr)
}

// StoreCode stores the given wasm code in the cache.
func StoreCode(cache Cache, wasm []byte, persist bool) ([]byte, error) {
	w := makeView(wasm)
	defer runtime.KeepAlive(wasm)
	errmsg := uninitializedUnmanagedVector()

	checksum, storeErr := C.store_code(cache.ptr, w, cbool(true), cbool(persist), &errmsg)
	if storeErr != nil {
		return nil, errorWithMessage(storeErr, errmsg)
	}
	return receiveVector(checksum), nil
}

// StoreCodeUnchecked stores the given wasm code in the cache without checking it.
func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	w := makeView(wasm)
	defer runtime.KeepAlive(wasm)
	errmsg := uninitializedUnmanagedVector()

	checksum, storeErr := C.store_code(cache.ptr, w, cbool(false), cbool(true), &errmsg)
	if storeErr != nil {
		return nil, errorWithMessage(storeErr, errmsg)
	}
	return receiveVector(checksum), nil
}

// RemoveCode removes the wasm code with the given checksum from the cache.
func RemoveCode(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()

	_, removeErr := C.remove_wasm(cache.ptr, cs, &errmsg)
	if removeErr != nil {
		return errorWithMessage(removeErr, errmsg)
	}
	return nil
}

// GetCode returns the wasm code with the given checksum from the cache.
func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()

	wasm, loadErr := C.load_wasm(cache.ptr, cs, &errmsg)
	if loadErr != nil {
		return nil, errorWithMessage(loadErr, errmsg)
	}
	return receiveVector(wasm), nil
}

// Pin pins the wasm code with the given checksum in the cache.
func Pin(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()

	_, pinErr := C.pin(cache.ptr, cs, &errmsg)
	if pinErr != nil {
		return errorWithMessage(pinErr, errmsg)
	}
	return nil
}

// Unpin unpins the wasm code with the given checksum from the cache.
func Unpin(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()

	_, unpinErr := C.unpin(cache.ptr, cs, &errmsg)
	if unpinErr != nil {
		return errorWithMessage(unpinErr, errmsg)
	}
	return nil
}

// AnalyzeCode analyzes the wasm code with the given checksum.
func AnalyzeCode(cache Cache, checksum []byte) (*types.AnalysisReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()

	report, analyzeErr := C.analyze_code(cache.ptr, cs, &errmsg)
	if analyzeErr != nil {
		return nil, errorWithMessage(analyzeErr, errmsg)
	}
	return receiveAnalysisReport(report), nil
}

// GetMetrics returns the metrics for the cache.
func GetMetrics(cache Cache) (*types.Metrics, error) {
	errmsg := uninitializedUnmanagedVector()

	metrics, metricsErr := C.get_metrics(cache.ptr, &errmsg)
	if metricsErr != nil {
		return nil, errorWithMessage(metricsErr, errmsg)
	}
	return receiveMetrics(metrics), nil
}

// GetPinnedMetrics returns the metrics for pinned wasm code in the cache.
func GetPinnedMetrics(cache Cache) (*types.PinnedMetrics, error) {
	errmsg := uninitializedUnmanagedVector()

	metrics, metricsErr := C.get_pinned_metrics(cache.ptr, &errmsg)
	if metricsErr != nil {
		return nil, errorWithMessage(metricsErr, errmsg)
	}
	pinnedMetrics, err := receivePinnedMetrics(metrics)
	if err != nil {
		return nil, err
	}
	return pinnedMetrics, nil
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
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	i := makeView(info)
	defer runtime.KeepAlive(info)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, instantiateErr := C.instantiate(cache.ptr, cs, e, i, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if instantiateErr != nil {
		return nil, types.GasReport{}, errorWithMessage(instantiateErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
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
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	i := makeView(info)
	defer runtime.KeepAlive(info)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, executeErr := C.execute(cache.ptr, cs, e, i, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if executeErr != nil {
		return nil, types.GasReport{}, errorWithMessage(executeErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Migrate(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, migrateErr := C.migrate(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if migrateErr != nil {
		return nil, types.GasReport{}, errorWithMessage(migrateErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
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
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	i := makeView(migrateInfo)
	defer runtime.KeepAlive(migrateInfo)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, migrateInfoErr := C.migrate_with_info(cache.ptr, cs, e, m, i, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if migrateInfoErr != nil {
		return nil, types.GasReport{}, errorWithMessage(migrateInfoErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Sudo(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, sudoErr := C.sudo(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if sudoErr != nil {
		return nil, types.GasReport{}, errorWithMessage(sudoErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Reply(
	cache Cache,
	checksum []byte,
	env []byte,
	reply []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	r := makeView(reply)
	defer runtime.KeepAlive(reply)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, replyErr := C.reply(cache.ptr, cs, e, r, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if replyErr != nil {
		return nil, types.GasReport{}, errorWithMessage(replyErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Query(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, queryErr := C.query(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if queryErr != nil {
		return nil, types.GasReport{}, errorWithMessage(queryErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCChannelOpen(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, channelOpenErr := C.ibc_channel_open(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if channelOpenErr != nil {
		return nil, types.GasReport{}, errorWithMessage(channelOpenErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCChannelConnect(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, channelConnectErr := C.ibc_channel_connect(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if channelConnectErr != nil {
		return nil, types.GasReport{}, errorWithMessage(channelConnectErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCChannelClose(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, channelCloseErr := C.ibc_channel_close(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if channelCloseErr != nil {
		return nil, types.GasReport{}, errorWithMessage(channelCloseErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCPacketReceive(
	cache Cache,
	checksum []byte,
	env []byte,
	packet []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(packet)
	defer runtime.KeepAlive(packet)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packetReceiveErr := C.ibc_packet_receive(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if packetReceiveErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packetReceiveErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBC2PacketReceive(
	cache Cache,
	checksum []byte,
	env []byte,
	payload []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(payload)
	defer runtime.KeepAlive(payload)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packet2ReceiveErr := C.ibc2_packet_receive(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if packet2ReceiveErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packet2ReceiveErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCPacketAck(
	cache Cache,
	checksum []byte,
	env []byte,
	ack []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	ac := makeView(ack)
	defer runtime.KeepAlive(ack)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packetAckErr := C.ibc_packet_ack(cache.ptr, cs, e, ac, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if packetAckErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packetAckErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCPacketTimeout(
	cache Cache,
	checksum []byte,
	env []byte,
	packet []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(packet)
	defer runtime.KeepAlive(packet)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packetTimeoutErr := C.ibc_packet_timeout(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if packetTimeoutErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packetTimeoutErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCSourceCallback(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, sourceCallbackErr := C.ibc_source_callback(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if sourceCallbackErr != nil {
		return nil, types.GasReport{}, errorWithMessage(sourceCallbackErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCDestinationCallback(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, destCallbackErr := C.ibc_destination_callback(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if destCallbackErr != nil {
		return nil, types.GasReport{}, errorWithMessage(destCallbackErr, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func convertGasReport(report C.GasReport) types.GasReport {
	return types.GasReport{
		Limit:          uint64(report.limit),
		Remaining:      uint64(report.remaining),
		UsedExternally: uint64(report.used_externally),
		UsedInternally: uint64(report.used_internally),
	}
}

/**** To error module ***/

func errorWithMessage(err error, b C.UnmanagedVector) error {
	// we always destroy the unmanaged vector to avoid a memory leak
	msg := copyAndDestroyUnmanagedVector(b)

	// this checks for out of gas as a special case
	if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
		return types.OutOfGasError{}
	}
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}

// checkAndPinAPI checks and pins the API and relevant pointers inside of it.
// All errors will result in panics as they indicate misuse of the wasmvm API and are not expected
// to be caused by user data.
func checkAndPinAPI(api *types.GoAPI, pinner runtime.Pinner) {
	if api == nil {
		panic("API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	// func cHumanizeAddress assumes this is set
	if api.HumanizeAddress == nil {
		panic("HumanizeAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	// func cCanonicalizeAddress assumes this is set
	if api.CanonicalizeAddress == nil {
		panic("CanonicalizeAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	// func cValidateAddress assumes this is set
	if api.ValidateAddress == nil {
		panic("ValidateAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	pinner.Pin(api) // this pointer is used in Rust (`state` in `C.GoApi`) and must not change
}

// checkAndPinQuerier checks and pins the querier.
// All errors will result in panics as they indicate misuse of the wasmvm API and are not expected
// to be caused by user data.
func checkAndPinQuerier(querier *Querier, pinner runtime.Pinner) {
	if querier == nil {
		panic("Querier must not be nil. If you don't want to provide querier functionality, please create an instance that returns an error on every call to Query().")
	}

	pinner.Pin(querier) // this pointer is used in Rust (`state` in `C.GoQuerier`) and must not change
}

func receiveVector(v C.UnmanagedVector) []byte {
	return copyAndDestroyUnmanagedVector(v)
}

func receiveAnalysisReport(report C.AnalysisReport) *types.AnalysisReport {
	requiredCapabilities := string(copyAndDestroyUnmanagedVector(report.required_capabilities))
	entrypoints := string(copyAndDestroyUnmanagedVector(report.entrypoints))
	entrypoints_array := strings.Split(entrypoints, ",")
	hasIBC2EntryPoints := slices.Contains(entrypoints_array, "ibc2_packet_receive")

	res := types.AnalysisReport{
		HasIBCEntryPoints:      bool(report.has_ibc_entry_points),
		HasIBC2EntryPoints:     hasIBC2EntryPoints,
		RequiredCapabilities:   requiredCapabilities,
		Entrypoints:            entrypoints_array,
		ContractMigrateVersion: optionalU64ToPtr(report.contract_migrate_version),
	}
	return &res
}

func receiveMetrics(metrics C.Metrics) *types.Metrics {
	return &types.Metrics{
		HitsPinnedMemoryCache:     uint32(metrics.hits_pinned_memory_cache),
		HitsMemoryCache:           uint32(metrics.hits_memory_cache),
		HitsFsCache:               uint32(metrics.hits_fs_cache),
		Misses:                    uint32(metrics.misses),
		ElementsPinnedMemoryCache: uint64(metrics.elements_pinned_memory_cache),
		ElementsMemoryCache:       uint64(metrics.elements_memory_cache),
		SizePinnedMemoryCache:     uint64(metrics.size_pinned_memory_cache),
		SizeMemoryCache:           uint64(metrics.size_memory_cache),
	}
}

func receivePinnedMetrics(metrics C.UnmanagedVector) (*types.PinnedMetrics, error) {
	var pinnedMetrics types.PinnedMetrics
	if err := pinnedMetrics.UnmarshalMessagePack(copyAndDestroyUnmanagedVector(metrics)); err != nil {
		return nil, err
	}
	return &pinnedMetrics, nil
}
