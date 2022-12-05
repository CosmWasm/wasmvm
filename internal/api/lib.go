package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"fmt"
	"runtime"
	"syscall"

	"github.com/CosmWasm/wasmvm/types"
)

// Value types
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

// Pointers
type cu8_ptr = *C.uint8_t

type Cache struct {
	ptr *C.cache_t
}

type Querier = types.Querier

func InitCache(dataDir string, supportedFeatures string, cacheSize uint32, instanceMemoryLimit uint32) (Cache, error) {
	dataDirBytes := []byte(dataDir)
	supportedFeaturesBytes := []byte(supportedFeatures)

	d := makeView(dataDirBytes)
	defer runtime.KeepAlive(dataDirBytes)
	f := makeView(supportedFeaturesBytes)
	defer runtime.KeepAlive(supportedFeaturesBytes)

	errmsg := newUnmanagedVector(nil)

	ptr, err := C.init_cache(d, f, cu32(cacheSize), cu32(instanceMemoryLimit), &errmsg)
	if err != nil {
		return Cache{}, ffiErrorWithMessage(err, errmsg)
	}
	return Cache{ptr: ptr}, nil
}

func ReleaseCache(cache Cache) {
	C.release_cache(cache.ptr) // No error case that needs handling
}

/// StoreCode stored the Wasm blob and returns the checksum
func StoreCode(cache Cache, wasm []byte) ([]byte, error) {
	w := makeView(wasm)
	defer runtime.KeepAlive(wasm)
	errmsg := newUnmanagedVector(nil)
	out := newUnmanagedVector(nil)
	err := C.save_wasm(cache.ptr, w, &errmsg, &out)
	if err != 0 {
		return nil, ffiErrorWithMessage2(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(out), nil
}

func RemoveCode(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := newUnmanagedVector(nil)
	err := C.remove_wasm(cache.ptr, cs, &errmsg)
	if err != 0 {
		return ffiErrorWithMessage2(err, errmsg)
	}
	return nil
}

func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := newUnmanagedVector(nil)
	out := newUnmanagedVector(nil)
	err := C.load_wasm(cache.ptr, cs, &errmsg, &out)
	if err != 0 {
		return nil, ffiErrorWithMessage2(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(out), nil
}

func Pin(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := newUnmanagedVector(nil)
	err := C.pin(cache.ptr, cs, &errmsg)
	if err != 0 {
		return ffiErrorWithMessage2(err, errmsg)
	}
	return nil
}

func Unpin(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := newUnmanagedVector(nil)
	err := C.unpin(cache.ptr, cs, &errmsg)
	if err != 0 {
		return ffiErrorWithMessage2(err, errmsg)
	}
	return nil
}

func AnalyzeCode(cache Cache, checksum []byte) (*types.AnalysisReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := newUnmanagedVector(nil)
	out := C.AnalysisReport{}
	err := C.analyze_code(cache.ptr, cs, &errmsg, &out)
	if err != 0 {
		return nil, ffiErrorWithMessage2(err, errmsg)
	}
	requiredCapabilities := string(copyAndDestroyUnmanagedVector(out.required_capabilities))
	res := types.AnalysisReport{
		HasIBCEntryPoints:    bool(out.has_ibc_entry_points),
		RequiredFeatures:     requiredCapabilities,
		RequiredCapabilities: requiredCapabilities,
	}
	return &res, nil
}

func GetMetrics(cache Cache) (*types.Metrics, error) {
	errmsg := newUnmanagedVector(nil)
	metrics, err := C.get_metrics(cache.ptr, &errmsg)
	if err != nil {
		return nil, ffiErrorWithMessage(err, errmsg)
	}

	return &types.Metrics{
		HitsPinnedMemoryCache:     uint32(metrics.hits_pinned_memory_cache),
		HitsMemoryCache:           uint32(metrics.hits_memory_cache),
		HitsFsCache:               uint32(metrics.hits_fs_cache),
		Misses:                    uint32(metrics.misses),
		ElementsPinnedMemoryCache: uint64(metrics.elements_pinned_memory_cache),
		ElementsMemoryCache:       uint64(metrics.elements_memory_cache),
		SizePinnedMemoryCache:     uint64(metrics.size_pinned_memory_cache),
		SizeMemoryCache:           uint64(metrics.size_memory_cache),
	}, nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	i := makeView(info)
	defer runtime.KeepAlive(info)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.instantiate(cache.ptr, cs, e, i, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	i := makeView(info)
	defer runtime.KeepAlive(info)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.execute(cache.ptr, cs, e, i, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.migrate(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.sudo(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	r := makeView(reply)
	defer runtime.KeepAlive(reply)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.reply(cache.ptr, cs, e, r, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.query(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.ibc_channel_open(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.ibc_channel_connect(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.ibc_channel_close(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(packet)
	defer runtime.KeepAlive(packet)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.ibc_packet_receive(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	ac := makeView(ack)
	defer runtime.KeepAlive(ack)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.ibc_packet_ack(cache.ptr, cs, e, ac, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
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
) ([]byte, uint64, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(packet)
	defer runtime.KeepAlive(packet)

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed cu64
	errmsg := newUnmanagedVector(nil)

	res, err := C.ibc_packet_timeout(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), ffiErrorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), uint64(gasUsed), nil
}

// ffiErrorWithMessage takes a error, tries to read the error message
// written by the Rust code and returns a readable error.
//
// This function must only be called on `ffiErr`s of type syscall.Errno
// that are no 0 (no error).
func ffiErrorWithMessage(ffiErr error, errMsg C.UnmanagedVector) error {
	errno := ffiErr.(syscall.Errno) // panics if conversion fails

	if errno == 0 {
		panic("Called ffiErrorWithMessage with a 0 errno (no error)")
	}

	// Checks for out of gas as a special case. See "ErrnoValue" in libwasmvm/src/error/rust.rs.
	if errno == 2 {
		return types.OutOfGasError{}
	}

	msg := copyAndDestroyUnmanagedVector(errMsg)
	if msg == nil || len(msg) == 0 {
		return fmt.Errorf("FFI call errored with errno %#v and no error message.", errno)
	}
	return fmt.Errorf("%s", string(msg))
}

// ffiErrorWithMessage2 takes a error number, tries to read the error message
// written by the Rust code and returns a readable error.
//
// This function must only be called on non-zero error numbers.
func ffiErrorWithMessage2(errno cint, errMsg C.UnmanagedVector) error {
	if errno == 0 {
		panic("Called ffiErrorWithMessage2 with a 0 error number (no error)")
	}

	// Checks for out of gas as a special case. See "ErrnoValue" in libwasmvm/src/error/rust.rs.
	if errno == 2 {
		return types.OutOfGasError{}
	}

	msg := copyAndDestroyUnmanagedVector(errMsg)
	if msg == nil || len(msg) == 0 {
		return fmt.Errorf("FFI call errored with errno %#v and no error message.", errno)
	}
	return fmt.Errorf("%s", string(msg))
}
