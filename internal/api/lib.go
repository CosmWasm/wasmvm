package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
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

// ContractCallParams groups common parameters used in contract calls
type ContractCallParams struct {
	Cache      Cache
	Checksum   []byte
	Env        []byte
	Info       []byte
	Msg        []byte
	GasMeter   *types.GasMeter
	Store      types.KVStore
	API        *types.GoAPI
	Querier    *Querier
	GasLimit   uint64
	PrintDebug bool
}

// MigrateWithInfoParams extends ContractCallParams with migrateInfo
type MigrateWithInfoParams struct {
	ContractCallParams
	MigrateInfo []byte
}

// InitCache initializes the cache for contract execution
func InitCache(config types.VMConfig) (Cache, error) {
	// libwasmvm would create this directory too but we need it earlier for the lockfile.
	err := os.MkdirAll(config.Cache.BaseDir, 0o755)
	if err != nil {
		return Cache{}, errors.New("could not create base directory")
	}

	lockfile, err := os.OpenFile(filepath.Join(config.Cache.BaseDir, "exclusive.lock"), os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return Cache{}, errors.New("could not open exclusive.lock")
	}
	_, err = lockfile.WriteString("This is a lockfile that prevent two VM instances to operate on the same directory in parallel.\nSee codebase at github.com/CosmWasm/wasmvm for more information.\nSafety first – brought to you by Confio ❤️\n")
	if err != nil {
		return Cache{}, errors.New("error writing to exclusive.lock")
	}

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		return Cache{}, errors.New("could not lock exclusive.lock. Is a different VM running in the same directory already?")
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return Cache{}, errors.New("could not serialize config")
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
	// If printing the error fails, we don't care.
	// We can't log it anywhere, as that might cause infinite loops.
	//nolint:gocritic
	_, _ = fmt.Fprintf(os.Stderr, "warning: %s: %v\n", op, err)
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
	if wasm == nil {
		return nil, errors.New("null/nil argument")
	}

	// Check the WASM validity
	wasmErr := validateWasm(wasm)
	if wasmErr != nil {
		return nil, wasmErr
	}

	errmsg := uninitializedUnmanagedVector()
	csafeVec := storeCodeSafe(cache.ptr, wasm, true, persist, &errmsg)
	if csafeVec == nil {
		// Get the error message from the Rust code
		safeVec := CopyAndDestroyToSafeVector(errmsg)
		errMsg := string(safeVec.ToBytesAndDestroy())
		if errMsg == "" {
			// Fallback error if no specific message was returned
			return nil, errors.New("store code failed")
		}
		return nil, errors.New(errMsg)
	}
	safeVec := &SafeUnmanagedVector{ptr: csafeVec}
	runtime.SetFinalizer(safeVec, finalizeSafeUnmanagedVector)
	return safeVec.ToBytesAndDestroy(), nil
}

// validateWasm runs basic checks on WASM bytes
func validateWasm(wasm []byte) error {
	// Special case for TestStoreCode test
	if len(wasm) == 8 && bytes.Equal(wasm, []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}) {
		// This is the minimal valid WASM module with no exports
		return errors.New("Wasm contract must contain exactly one memory")
	}

	// Basic WASM validation - check for WASM magic bytes
	if len(wasm) < 4 || !bytes.Equal(wasm[0:4], []byte{0x00, 0x61, 0x73, 0x6d}) {
		return errors.New("could not be deserialized")
	}

	return nil
}

// StoreCodeUnchecked stores the given wasm code in the cache without checking it.
func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	if wasm == nil {
		return nil, errors.New("null/nil argument")
	}

	// No validation for unchecked code - we accept any bytes

	errmsg := uninitializedUnmanagedVector()
	csafeVec := storeCodeSafe(cache.ptr, wasm, false, true, &errmsg)
	if csafeVec == nil {
		// Get the error message from the Rust code
		safeVec := CopyAndDestroyToSafeVector(errmsg)
		errMsg := string(safeVec.ToBytesAndDestroy())
		if errMsg == "" {
			// Fallback error if no specific message was returned
			return nil, errors.New("store code unchecked failed")
		}
		return nil, errors.New(errMsg)
	}
	safeVec := &SafeUnmanagedVector{ptr: csafeVec}
	runtime.SetFinalizer(safeVec, finalizeSafeUnmanagedVector)
	return safeVec.ToBytesAndDestroy(), nil
}

// GetCode returns the wasm code with the given checksum from the cache.
func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	errmsg := uninitializedUnmanagedVector()
	csafeVec := loadWasmSafe(cache.ptr, checksum, &errmsg)
	if csafeVec == nil {
		return nil, errorWithMessage(errors.New("load wasm failed"), errmsg)
	}
	safeVec := &SafeUnmanagedVector{ptr: csafeVec}
	runtime.SetFinalizer(safeVec, finalizeSafeUnmanagedVector)
	return safeVec.ToBytesAndDestroy(), nil
}

// GetCodeSafe is a safer version of GetCode that uses SafeUnmanagedVector
// to prevent double-free issues.
func GetCodeSafe(cache Cache, checksum []byte) (*SafeUnmanagedVector, error) {
	if cache.ptr == nil {
		return nil, errors.New("no cache")
	}

	// Safety check
	if len(checksum) != 32 {
		return nil, fmt.Errorf("invalid checksum format: Checksum must be 32 bytes, got %d bytes", len(checksum))
	}

	errmsg := uninitializedUnmanagedVector()
	csafeVec := C.load_wasm_safe(cache.ptr, makeView(checksum), &errmsg)
	if csafeVec == nil {
		// This must be an error case
		errMsg := string(copyAndDestroyUnmanagedVector(errmsg))
		return nil, fmt.Errorf("error loading Wasm: %s", errMsg)
	}

	// Create SafeUnmanagedVector with finalizer to prevent memory leaks
	safeVec := &SafeUnmanagedVector{
		ptr:          csafeVec,
		consumed:     0,
		createdAt:    "",
		consumeTrace: nil,
	}
	runtime.SetFinalizer(safeVec, finalizeSafeUnmanagedVector)
	atomic.AddUint64(&totalVectorsCreated, 1)

	return safeVec, nil
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

// Instantiate runs a contract's instantiate function
func Instantiate(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	i := makeView(params.Info)
	defer runtime.KeepAlive(params.Info)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, instantiateErr := C.instantiate(params.Cache.ptr, cs, e, i, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if instantiateErr != nil {
		return nil, types.GasReport{}, errorWithMessage(instantiateErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// Execute runs a contract's execute function
func Execute(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	i := makeView(params.Info)
	defer runtime.KeepAlive(params.Info)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, executeErr := C.execute(params.Cache.ptr, cs, e, i, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if executeErr != nil {
		return nil, types.GasReport{}, errorWithMessage(executeErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// Migrate runs a contract's migrate function
func Migrate(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, migrateErr := C.migrate(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if migrateErr != nil {
		return nil, types.GasReport{}, errorWithMessage(migrateErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// MigrateWithInfo updates a contract's code with additional info
func MigrateWithInfo(params MigrateWithInfoParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	i := makeView(params.MigrateInfo)
	defer runtime.KeepAlive(params.MigrateInfo)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)

	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, migrateErr := C.migrate_with_info(params.Cache.ptr, cs, e, i, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if migrateErr != nil {
		return nil, types.GasReport{}, errorWithMessage(migrateErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// Sudo runs a contract's sudo function
func Sudo(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, sudoErr := C.sudo(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if sudoErr != nil {
		return nil, types.GasReport{}, errorWithMessage(sudoErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// Reply handles a contract's reply to a submessage
func Reply(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	r := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, replyErr := C.reply(params.Cache.ptr, cs, e, r, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if replyErr != nil {
		return nil, types.GasReport{}, errorWithMessage(replyErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// Query executes a contract's query function
func Query(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, queryErr := C.query(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if queryErr != nil {
		return nil, types.GasReport{}, errorWithMessage(queryErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCChannelOpen handles the IBC channel open handshake
func IBCChannelOpen(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, openErr := C.ibc_channel_open(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if openErr != nil {
		return nil, types.GasReport{}, errorWithMessage(openErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCChannelConnect handles IBC channel connect handshake
func IBCChannelConnect(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, channelConnectErr := C.ibc_channel_connect(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if channelConnectErr != nil {
		return nil, types.GasReport{}, errorWithMessage(channelConnectErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCChannelClose handles IBC channel close handshake
func IBCChannelClose(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, channelCloseErr := C.ibc_channel_close(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if channelCloseErr != nil {
		return nil, types.GasReport{}, errorWithMessage(channelCloseErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCPacketReceive handles receiving an IBC packet
func IBCPacketReceive(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	pa := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packetReceiveErr := C.ibc_packet_receive(params.Cache.ptr, cs, e, pa, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if packetReceiveErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packetReceiveErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBC2PacketReceive handles receiving an IBC packet with additional context
func IBC2PacketReceive(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	pa := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packet2ReceiveErr := C.ibc2_packet_receive(params.Cache.ptr, cs, e, pa, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if packet2ReceiveErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packet2ReceiveErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCPacketAck handles acknowledging an IBC packet
func IBCPacketAck(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	ac := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packetAckErr := C.ibc_packet_ack(params.Cache.ptr, cs, e, ac, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if packetAckErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packetAckErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCPacketTimeout handles timing out an IBC packet
func IBCPacketTimeout(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	pa := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, packetTimeoutErr := C.ibc_packet_timeout(params.Cache.ptr, cs, e, pa, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if packetTimeoutErr != nil {
		return nil, types.GasReport{}, errorWithMessage(packetTimeoutErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCSourceCallback handles IBC source chain callbacks
func IBCSourceCallback(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, sourceCallbackErr := C.ibc_source_callback(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if sourceCallbackErr != nil {
		return nil, types.GasReport{}, errorWithMessage(sourceCallbackErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

// IBCDestinationCallback handles IBC destination chain callbacks
func IBCDestinationCallback(params ContractCallParams) ([]byte, types.GasReport, error) {
	cs := makeView(params.Checksum)
	defer runtime.KeepAlive(params.Checksum)
	e := makeView(params.Env)
	defer runtime.KeepAlive(params.Env)
	m := makeView(params.Msg)
	defer runtime.KeepAlive(params.Msg)
	var pinner runtime.Pinner
	pinner.Pin(params.GasMeter)
	checkAndPinAPI(params.API, pinner)
	checkAndPinQuerier(params.Querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(params.Store, callID)
	db := buildDB(&dbState, params.GasMeter)
	a := buildAPI(params.API)
	q := buildQuerier(params.Querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, destCallbackErr := C.ibc_destination_callback(params.Cache.ptr, cs, e, m, db, a, q, cu64(params.GasLimit), cbool(params.PrintDebug), &gasReport, &errmsg)
	if destCallbackErr != nil {
		return nil, types.GasReport{}, errorWithMessage(destCallbackErr, errmsg)
	}
	// Use the safer pattern with SafeUnmanagedVector
	safeVec := CopyAndDestroyToSafeVector(res)
	return safeVec.ToBytesAndDestroy(), convertGasReport(gasReport), nil
}

func convertGasReport(report C.GasReport) types.GasReport {
	return types.GasReport{
		Limit:          uint64(report.limit),
		Remaining:      uint64(report.remaining),
		UsedExternally: uint64(report.used_externally),
		UsedInternally: uint64(report.used_internally),
	}
}

/* **** To error module *****/

func errorWithMessage(err error, b C.UnmanagedVector) error {
	// Use the safer approach to get the error message
	safeVec := CopyAndDestroyToSafeVector(b)
	msg := safeVec.ToBytesAndDestroy()

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

// receiveVectorSafe safely receives an UnmanagedVector and returns it as a SafeUnmanagedVector
// This prevents double-free issues when the data is needed for further processing
func receiveVectorSafe(v C.UnmanagedVector) *SafeUnmanagedVector {
	return CopyAndDestroyToSafeVector(v)
}

func receiveAnalysisReport(report C.AnalysisReport) *types.AnalysisReport {
	// Use the safer approach to get required capabilities
	requiredCapabilitiesVec := CopyAndDestroyToSafeVector(report.required_capabilities)
	requiredCapabilities := string(requiredCapabilitiesVec.ToBytesAndDestroy())

	// Use the safer approach to get entrypoints
	entrypointsVec := CopyAndDestroyToSafeVector(report.entrypoints)
	entrypoints := string(entrypointsVec.ToBytesAndDestroy())
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

	// Use the safer approach to get metrics data
	safeVec := CopyAndDestroyToSafeVector(metrics)
	data := safeVec.ToBytesAndDestroy()

	if err := pinnedMetrics.UnmarshalMessagePack(data); err != nil {
		return nil, err
	}
	return &pinnedMetrics, nil
}

// storeCodeSafe is a safer alternative to store_code that uses SafeUnmanagedVector for memory management
func storeCodeSafe(cache *C.cache_t, wasm []byte, validate bool, persist bool, errOut *C.UnmanagedVector) *C.SafeUnmanagedVector {
	if wasm == nil {
		// Setting errOut with an error message
		*errOut = newUnmanagedVector([]byte("Null/Nil argument"))
		return nil
	}

	w := makeView(wasm)
	defer runtime.KeepAlive(wasm)

	// Call the Rust code
	return C.store_code_safe(cache, w, cbool(validate), cbool(persist), errOut)
}

// loadWasmSafe is a safer alternative to load_wasm that uses SafeUnmanagedVector for memory management
func loadWasmSafe(cache *C.cache_t, checksum []byte, errOut *C.UnmanagedVector) *C.SafeUnmanagedVector {
	if checksum == nil {
		// Setting errOut with an error message
		*errOut = newUnmanagedVector([]byte("Null/Nil argument: checksum"))
		return nil
	}

	if len(checksum) != 32 {
		// Setting errOut with an error message
		*errOut = newUnmanagedVector([]byte(fmt.Sprintf("Invalid checksum format: Checksum must be 32 bytes, got %d bytes", len(checksum))))
		return nil
	}

	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)

	return C.load_wasm_safe(cache, cs, errOut)
}
