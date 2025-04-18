package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unsafe"

	"github.com/CosmWasm/wasmvm/v2/internal/ffi"
	"github.com/CosmWasm/wasmvm/v2/types"
	"golang.org/x/sys/unix"
)

// ByteSliceView represents a C struct ByteSliceView
type ByteSliceView struct {
	IsNil uint8
	Ptr   *uint8
	Len   uintptr
}

// UnmanagedVector represents a C struct UnmanagedVector
type UnmanagedVector struct {
	IsNone uint8
	Ptr    *uint8
	Len    uintptr
	Cap    uintptr
}

// GasReport represents a C struct GasReport
type GasReport struct {
	Limit          uint64
	Remaining      uint64
	UsedExternally uint64
	UsedInternally uint64
}

// CacheT is an opaque type for struct cache_t
type CacheT struct{}

// Cache holds the cache pointer and lockfile
type Cache struct {
	ptr      *CacheT
	lockfile os.File
}

type Querier = types.Querier

// Helper functions

func makeByteSliceView(data []byte) ffi.ByteSliceView {
	if data == nil {
		return ffi.ByteSliceView{IsNil: true}
	}
	return ffi.MakeByteSliceView(data)
}

func uninitializedUnmanagedVector() ffi.UnmanagedVector {
	return ffi.UnmanagedVector{IsNone: true}
}

func copyAndDestroyUnmanagedVector(v ffi.UnmanagedVector) []byte {
	if v.IsNone {
		return nil
	}
	data := make([]byte, v.Len)
	copy(data, unsafe.Slice(v.Ptr, v.Len))
	ffi.DestroyUnmanagedVector(v)
	return data
}

func convertGasReport(report ffi.GasReport) types.GasReport {
	return types.GasReport{
		Limit:          report.Limit,
		Remaining:      report.Remaining,
		UsedExternally: report.UsedExternally,
		UsedInternally: report.UsedInternally,
	}
}

// InitCache initializes the cache with the given configuration
func InitCache(config types.VMConfig) (Cache, error) {
	ffi.EnsureBindingsLoaded()

	err := os.MkdirAll(config.Cache.BaseDir, 0o755)
	if err != nil {
		return Cache{}, fmt.Errorf("could not create base directory: %v", err)
	}

	lockfile, err := os.OpenFile(filepath.Join(config.Cache.BaseDir, "exclusive.lock"), os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return Cache{}, fmt.Errorf("could not open exclusive.lock: %v", err)
	}
	_, err = lockfile.WriteString("This is a lockfile that prevent two VM instances to operate on the same directory in parallel.\nSee codebase at github.com/CosmWasm/wasmvm for more information.\nSafety first – brought to you by Confio ❤️\n")
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("error writing to exclusive.lock: %v", err)
	}

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("could not lock exclusive.lock. Is a different VM running in the same directory already? %v", err)
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("could not serialize config: %v", err)
	}
	configView := makeByteSliceView(configBytes)
	defer runtime.KeepAlive(configBytes)

	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	cachePtr := (*CacheT)(unsafe.Pointer(ffi.InitCache(uintptr(unsafe.Pointer(&configView)), uintptr(unsafe.Pointer(&errmsg)))))
	if cachePtr == nil {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		lockfile.Close()
		return Cache{}, fmt.Errorf("%s", string(msg))
	}
	return Cache{ptr: cachePtr, lockfile: *lockfile}, nil
}

// ReleaseCache releases the cache resources
func ReleaseCache(cache Cache) {
	ffi.ReleaseCache(uintptr(unsafe.Pointer(cache.ptr)))
	cache.lockfile.Close()
}

// StoreCode stores WASM code in the cache
func StoreCode(cache Cache, wasm []byte, checked bool, persist bool) ([]byte, error) {
	w := makeByteSliceView(wasm)
	defer runtime.KeepAlive(wasm)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	checksumVecPtr := ffi.StoreCode(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&w)), checked, persist, uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, fmt.Errorf("%s", string(msg))
	}
	checksumVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(checksumVecPtr))
	return copyAndDestroyUnmanagedVector(checksumVec), nil
}

// StoreCodeUnchecked stores WASM code without checking
func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	w := makeByteSliceView(wasm)
	defer runtime.KeepAlive(wasm)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	checksumVecPtr := ffi.StoreCode(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&w)), false, true, uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, fmt.Errorf("%s", string(msg))
	}
	checksumVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(checksumVecPtr))
	return copyAndDestroyUnmanagedVector(checksumVec), nil
}

// RemoveCode removes WASM code from the cache
func RemoveCode(cache Cache, checksum []byte) error {
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	ffi.RemoveWasm(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&cs)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return fmt.Errorf("%s", string(msg))
	}
	return nil
}

// GetCode retrieves WASM code from the cache
func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	wasmVecPtr := ffi.LoadWasm(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&cs)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, fmt.Errorf("%s", string(msg))
	}
	wasmVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(wasmVecPtr))
	return copyAndDestroyUnmanagedVector(wasmVec), nil
}

// Pin pins WASM code in the cache
func Pin(cache Cache, checksum []byte) error {
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	ffi.Pin(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&cs)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return fmt.Errorf("%s", string(msg))
	}
	return nil
}

// Unpin unpins WASM code from the cache
func Unpin(cache Cache, checksum []byte) error {
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	ffi.Unpin(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&cs)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return fmt.Errorf("%s", string(msg))
	}
	return nil
}

// AnalyzeCode analyzes WASM code in the cache
func AnalyzeCode(cache Cache, checksum []byte) (*types.AnalysisReport, error) {
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	// Note: analyze_code returns an UnmanagedVector containing serialized data
	reportVecPtr := ffi.AnalyzeCode(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&cs)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, fmt.Errorf("%s", string(msg))
	}
	reportVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(reportVecPtr))
	// For simplicity, assume the Rust side serializes required_capabilities and entrypoints as strings
	data := copyAndDestroyUnmanagedVector(reportVec)
	// Placeholder: Parse the serialized data (this depends on Rust's serialization)
	// Here, we'll simulate the original behavior
	requiredCapabilities := string(data) // Adjust based on actual format
	entrypoints := "instantiate,execute" // Placeholder
	entrypointsArray := strings.Split(entrypoints, ",")
	hasIBC2EntryPoints := slices.Contains(entrypointsArray, "ibc2_packet_receive")

	res := types.AnalysisReport{
		HasIBCEntryPoints:    false, // Adjust based on actual data
		HasIBC2EntryPoints:   hasIBC2EntryPoints,
		RequiredCapabilities: requiredCapabilities,
		Entrypoints:          entrypointsArray,
		// ContractMigrateVersion omitted for simplicity
	}
	return &res, nil
}

// GetMetrics retrieves cache metrics
func GetMetrics(cache Cache) (*types.Metrics, error) {
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	metricsVecPtr := ffi.GetMetrics(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, fmt.Errorf("%s", string(msg))
	}
	metricsVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(metricsVecPtr))
	_ = copyAndDestroyUnmanagedVector(metricsVec) // Consume the vector, ignore data for now
	// Placeholder: Parse metrics (adjust based on actual serialization)
	return &types.Metrics{
		HitsPinnedMemoryCache:     0, // Adjust based on actual data
		HitsMemoryCache:           0,
		HitsFsCache:               0,
		Misses:                    0,
		ElementsPinnedMemoryCache: 0,
		ElementsMemoryCache:       0,
		SizePinnedMemoryCache:     0,
		SizeMemoryCache:           0,
	}, nil
}

// GetPinnedMetrics retrieves pinned cache metrics
func GetPinnedMetrics(cache Cache) (*types.PinnedMetrics, error) {
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	metricsVecPtr := ffi.GetPinnedMetrics(uintptr(unsafe.Pointer(cache.ptr)), uintptr(unsafe.Pointer(&errmsg)))
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, fmt.Errorf("%s", string(msg))
	}
	metricsVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(metricsVecPtr))
	var pinnedMetrics types.PinnedMetrics
	data := copyAndDestroyUnmanagedVector(metricsVec)
	if err := pinnedMetrics.UnmarshalMessagePack(data); err != nil {
		return nil, err
	}
	return &pinnedMetrics, nil
}

// Instantiate executes the instantiate entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	i := makeByteSliceView(info)
	defer runtime.KeepAlive(info)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Instantiate(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&i)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// Execute executes the execute entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	i := makeByteSliceView(info)
	defer runtime.KeepAlive(info)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Execute(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&i)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// Migrate executes the migrate entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Migrate(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// MigrateWithInfo executes the migrate entry point with additional info
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
	defer runtime.KeepAlive(msg)
	i := makeByteSliceView(migrateInfo)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.MigrateWithInfo(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&i)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// Sudo executes the sudo entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Sudo(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// Reply executes the reply entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	r := makeByteSliceView(reply)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Reply(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&r)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// Query executes the query entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Query(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCChannelOpen executes the IBC channel open entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcChannelOpen(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCChannelConnect executes the IBC channel connect entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcChannelConnect(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCChannelClose executes the IBC channel close entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcChannelClose(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCPacketReceive executes the IBC packet receive entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	pa := makeByteSliceView(packet)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcPacketReceive(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&pa)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBC2PacketReceive executes the IBC2 packet receive entry point
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
	cs := makeByteSliceView(checksum)
	var pinner runtime.Pinner
	if cs.Ptr != nil {
		pinner.Pin(checksum)
	}
	defer pinner.Unpin()
	e := makeByteSliceView(env)
	if e.Ptr != nil {
		pinner.Pin(env)
	}
	pa := makeByteSliceView(payload)
	if pa.Ptr != nil {
		pinner.Pin(payload)
	}
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.Ibc2PacketReceive(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&pa)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCPacketAck executes the IBC packet acknowledge entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	ac := makeByteSliceView(ack)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcPacketAck(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&ac)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCPacketTimeout executes the IBC packet timeout entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	pa := makeByteSliceView(packet)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcPacketTimeout(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&pa)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCSourceCallback executes the IBC source callback entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcSourceCallback(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// IBCDestinationCallback executes the IBC destination callback entry point
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
	cs := makeByteSliceView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeByteSliceView(env)
	defer runtime.KeepAlive(env)
	m := makeByteSliceView(msg)
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
	var gasReport ffi.GasReport
	var errmsg ffi.UnmanagedVector
	errmsg.IsNone = true

	resVecPtr := ffi.IbcDestinationCallback(
		uintptr(unsafe.Pointer(cache.ptr)),
		uintptr(unsafe.Pointer(&cs)),
		uintptr(unsafe.Pointer(&e)),
		uintptr(unsafe.Pointer(&m)),
		uintptr(unsafe.Pointer(&db)),
		uintptr(unsafe.Pointer(&a)),
		uintptr(unsafe.Pointer(&q)),
		gasLimit,
		printDebug,
		uintptr(unsafe.Pointer(&gasReport)),
		uintptr(unsafe.Pointer(&errmsg)),
	)
	if !errmsg.IsNone {
		msg := copyAndDestroyUnmanagedVector(errmsg)
		return nil, convertGasReport(gasReport), fmt.Errorf("%s", string(msg))
	}
	resVec := *(*ffi.UnmanagedVector)(unsafe.Pointer(resVecPtr))
	res := copyAndDestroyUnmanagedVector(resVec)
	return res, convertGasReport(gasReport), nil
}

// checkAndPinAPI checks and pins the API
func checkAndPinAPI(api *types.GoAPI, pinner runtime.Pinner) {
	if api == nil {
		panic("API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}
	if api.HumanizeAddress == nil {
		panic("HumanizeAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}
	if api.CanonicalizeAddress == nil {
		panic("CanonicalizeAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}
	if api.ValidateAddress == nil {
		panic("ValidateAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}
	pinner.Pin(api)
}

// checkAndPinQuerier checks and pins the querier
func checkAndPinQuerier(querier *Querier, pinner runtime.Pinner) {
	if querier == nil {
		panic("Querier must not be nil. If you don't want to provide querier functionality, please create an instance that returns an error on every call to Query().")
	}
	pinner.Pin(querier)
}

// buildDBState, buildDB, buildAPI, buildQuerier need implementations (likely using ffi types/vtables)
// These are placeholders based on the cgo version logic
type DBState struct { // Consider moving to ffi or using a different approach
	Store  types.KVStore
	CallID uint64
}

func buildDBState(store types.KVStore, callID uint64) DBState {
	return DBState{Store: store, CallID: callID}
}

func buildDB(state *DBState, gm *types.GasMeter) ffi.Db {
	// Needs actual implementation using ffi vtables and types
	return ffi.Db{}
}

func buildAPI(api *types.GoAPI) ffi.GoApi {
	// Needs actual implementation using ffi vtables and types
	return ffi.GoApi{}
}

func buildQuerier(querier *Querier) ffi.GoQuerier {
	// Needs actual implementation using ffi vtables and types
	return ffi.GoQuerier{}
}
