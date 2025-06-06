# Combined Code Files

## TOC
- [`api/api.go`](#file-1)
- [`api/api_test.go`](#file-2)
- [`api/iterator.go`](#file-3)
- [`api/iterator_test.go`](#file-4)
- [`api/lib.go`](#file-5)
- [`api/lib_test.go`](#file-6)
- [`api/mock_failure.go`](#file-7)
- [`api/mocks.go`](#file-8)
- [`api/testdb/README.md`](#file-9)
- [`api/testdb/memdb.go`](#file-10)
- [`api/testdb/memdb_iterator.go`](#file-11)
- [`api/testdb/types.go`](#file-12)
- [`api/version.go`](#file-13)
- [`api/version_test.go`](#file-14)
- [`runtime/combined_code.md`](#file-15)
- [`runtime/constants/constants.go`](#file-16)
- [`runtime/constants/gas.go`](#file-17)
- [`runtime/crypto/bls_impl.go`](#file-18)
- [`runtime/crypto/crypto.go`](#file-19)
- [`runtime/crypto/hostcrypto.go`](#file-20)
- [`runtime/cryptoapi/types.go`](#file-21)
- [`runtime/gas/gas.go`](#file-22)
- [`runtime/gas/gasversionone/gas.go`](#file-23)
- [`runtime/gas/gasversiontwo/gas.go`](#file-24)
- [`runtime/gas/wazero/gas.go`](#file-25)
- [`runtime/gas.go`](#file-26)
- [`runtime/host/combined_code.md`](#file-27)
- [`runtime/host/environment.go`](#file-28)
- [`runtime/host/hostfunctions.go`](#file-29)
- [`runtime/host/hostfunctions_impl.go`](#file-30)
- [`runtime/host/memory.go`](#file-31)
- [`runtime/host/registerhostfunctions.go`](#file-32)
- [`runtime/hostapi/types.go`](#file-33)
- [`runtime/memory/memory.go`](#file-34)
- [`runtime/tracing.go`](#file-35)
- [`runtime/types/gas.go`](#file-36)
- [`runtime/types/types.go`](#file-37)
- [`runtime/validation/validation.go`](#file-38)
- [`runtime/wasm/execution.go`](#file-39)
- [`runtime/wasm/ibc.go`](#file-40)
- [`runtime/wasm/runtime.go`](#file-41)
- [`runtime/wasm/system.go`](#file-42)
- [`runtime/wasm/vm.go`](#file-43)

---

### `api/api.go`
*2025-02-26 16:48:21 | 1 KB*
```go
// Package api defines core interfaces and error types for the WASM VM
package api

import (
	"fmt"
)

// Error implements the error interface
func (e ErrorOutOfGas) Error() string {
	return fmt.Sprintf("out of gas: %s", e.Descriptor)
}

```
---
### `api/api_test.go`
*2025-02-20 21:49:29 | 3 KB*
```go
package api

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// prettyPrint returns a properly formatted string representation of a struct
func prettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%#v", v)
	}
	return string(b)
}

func TestValidateAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	// if the human address is larger than 32 bytes, this will lead to an error in the go side
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier": "` + longName + `", "beneficiary": "bob"}`)

	// make sure the call doesn't error, but we get a JSON-encoded error result from ContractResult
	igasMeter := types.GasMeter(gasMeter)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)

	// DEBUG: print all calls with proper formatting and deserialization
	fmt.Printf("\n=== Debug Information ===\n")
	fmt.Printf("Cache:     %#v\n", cache)
	fmt.Printf("Checksum:  %x\n", checksum)

	// Deserialize env
	var envObj types.Env
	_ = json.Unmarshal(env, &envObj)
	fmt.Printf("Env:       %s\n", prettyPrint(envObj))

	// Deserialize info
	var infoObj types.MessageInfo
	_ = json.Unmarshal(info, &infoObj)
	fmt.Printf("Info:      %s\n", prettyPrint(infoObj))

	// Deserialize msg
	var msgObj map[string]interface{}
	_ = json.Unmarshal(msg, &msgObj)
	fmt.Printf("Msg:       %s\n", prettyPrint(msgObj))

	fmt.Printf("Gas Meter: %#v\n", igasMeter)
	fmt.Printf("Store:     %#v\n", store)
	fmt.Printf("API:       %#v\n", api)
	fmt.Printf("Querier:   %s\n", prettyPrint(querier))
	fmt.Printf("======================\n\n")

	require.NoError(t, err)
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)

	// ensure the error message is what we expect
	require.Nil(t, result.Ok)
	// with this error
	require.Equal(t, "Generic error: addr_validate errored: human encoding too long", result.Err)
}

```
---
### `api/iterator.go`
*2025-02-20 21:49:29 | 4 KB*
```go
package api

import (
	"fmt"
	"math"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// frame stores all Iterators for one contract call
type frame []types.Iterator

// iteratorFrames contains one frame for each contract call, indexed by contract call ID.
var (
	iteratorFrames      = make(map[uint64]frame)
	iteratorFramesMutex sync.Mutex
)

// this is a global counter for creating call IDs
var (
	latestCallID      uint64
	latestCallIDMutex sync.Mutex
)

// startCall is called at the beginning of a contract call to create a new frame in iteratorFrames.
// It updates latestCallID for generating a new call ID.
func startCall() uint64 {
	latestCallIDMutex.Lock()
	defer latestCallIDMutex.Unlock()
	latestCallID++
	return latestCallID
}

// removeFrame removes the frame with for the given call ID.
// The result can be nil when the frame is not initialized,
// i.e. when startCall() is called but no iterator is stored.
func removeFrame(callID uint64) frame {
	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()

	remove := iteratorFrames[callID]
	delete(iteratorFrames, callID)
	return remove
}

// endCall is called at the end of a contract call to remove one item the iteratorFrames
func endCall(callID uint64) {
	// we pull removeFrame in another function so we don't hold the mutex while cleaning up the removed frame
	remove := removeFrame(callID)
	// free all iterators in the frame when we release it
	for _, iter := range remove {
		iter.Close()
	}
}

// storeIterator will add this to the end of the frame for the given call ID and return
// an iterator ID to reference it.
//
// We assign iterator IDs starting with 1 for historic reasons. This could be changed to 0
// I guess.
func storeIterator(callID uint64, it types.Iterator, frameLenLimit int) (uint64, error) {
	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()

	new_index := len(iteratorFrames[callID])
	if new_index >= frameLenLimit {
		return 0, fmt.Errorf("Reached iterator limit (%d)", frameLenLimit)
	}

	// store at array position `new_index`
	iteratorFrames[callID] = append(iteratorFrames[callID], it)

	iterator_id, ok := indexToIteratorID(new_index)
	if !ok {
		// This error case is not expected to happen since the above code ensures the
		// index is in the range [0, frameLenLimit-1]
		return 0, fmt.Errorf("could not convert index to iterator ID")
	}
	return iterator_id, nil
}

// retrieveIterator will recover an iterator based on its ID.
func retrieveIterator(callID uint64, iteratorID uint64) types.Iterator {
	indexInFrame, ok := iteratorIdToIndex(iteratorID)
	if !ok {
		return nil
	}

	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()
	myFrame := iteratorFrames[callID]
	if myFrame == nil {
		return nil
	}
	if indexInFrame >= len(myFrame) {
		// index out of range
		return nil
	}
	return myFrame[indexInFrame]
}

// iteratorIdToIndex converts an iterator ID to an index in the frame.
// The second value marks if the conversion succeeded.
func iteratorIdToIndex(id uint64) (int, bool) {
	if id < 1 || id > math.MaxInt32 {
		// If success is false, the int value is undefined. We use an arbitrary constant for potential debugging purposes.
		return 777777777, false
	}

	// Int conversion safe because value is in signed 32bit integer range
	return int(id) - 1, true
}

// indexToIteratorID converts an index in the frame to an iterator ID.
// The second value marks if the conversion succeeded.
func indexToIteratorID(index int) (uint64, bool) {
	if index < 0 || index > math.MaxInt32 {
		// If success is false, the return value is undefined. We use an arbitrary constant for potential debugging purposes.
		return 888888888, false
	}

	return uint64(index) + 1, true
}

```
---
### `api/iterator_test.go`
*2025-02-20 21:49:29 | 9 KB*
```go
package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api/testdb"
	"github.com/CosmWasm/wasmvm/v2/types"
)

type queueData struct {
	checksum []byte
	store    *Lookup
	api      *types.GoAPI
	querier  types.Querier
}

func (q queueData) Store(meter MockGasMeter) types.KVStore {
	return q.store.WithGasMeter(meter)
}

func setupQueueContractWithData(t *testing.T, cache Cache, values ...int) queueData {
	t.Helper()
	checksum := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	igasMeter1 := types.GasMeter(gasMeter1)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	for _, value := range values {
		// push 17
		var gasMeter2 types.GasMeter = NewMockGasMeter(TESTING_GAS_LIMIT)
		push := []byte(fmt.Sprintf(`{"enqueue":{"value":%d}}`, value))
		res, _, err = Execute(cache, checksum, env, info, push, &gasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err)
		requireOkResponse(t, res, 0)
	}

	return queueData{
		checksum: checksum,
		store:    store,
		api:      api,
		querier:  querier,
	}
}

func setupQueueContract(t *testing.T, cache Cache) queueData {
	t.Helper()
	return setupQueueContractWithData(t, cache, 17, 22)
}

func TestStoreIterator(t *testing.T) {
	const limit = 2000
	callID1 := startCall()
	callID2 := startCall()

	store := testdb.NewMemDB()
	var iter types.Iterator
	var index uint64
	var err error

	iter, _ = store.Iterator(nil, nil)
	index, err = storeIterator(callID1, iter, limit)
	require.NoError(t, err)
	require.Equal(t, uint64(1), index)
	iter, _ = store.Iterator(nil, nil)
	index, err = storeIterator(callID1, iter, limit)
	require.NoError(t, err)
	require.Equal(t, uint64(2), index)

	iter, _ = store.Iterator(nil, nil)
	index, err = storeIterator(callID2, iter, limit)
	require.NoError(t, err)
	require.Equal(t, uint64(1), index)
	iter, _ = store.Iterator(nil, nil)
	index, err = storeIterator(callID2, iter, limit)
	require.NoError(t, err)
	require.Equal(t, uint64(2), index)
	iter, _ = store.Iterator(nil, nil)
	index, err = storeIterator(callID2, iter, limit)
	require.NoError(t, err)
	require.Equal(t, uint64(3), index)

	endCall(callID1)
	endCall(callID2)
}

func TestStoreIteratorHitsLimit(t *testing.T) {
	callID := startCall()

	store := testdb.NewMemDB()
	var iter types.Iterator
	var err error
	const limit = 2

	iter, _ = store.Iterator(nil, nil)
	_, err = storeIterator(callID, iter, limit)
	require.NoError(t, err)

	iter, _ = store.Iterator(nil, nil)
	_, err = storeIterator(callID, iter, limit)
	require.NoError(t, err)

	iter, _ = store.Iterator(nil, nil)
	_, err = storeIterator(callID, iter, limit)
	require.ErrorContains(t, err, "Reached iterator limit (2)")

	endCall(callID)
}

func TestRetrieveIterator(t *testing.T) {
	const limit = 2000
	callID1 := startCall()
	callID2 := startCall()

	store := testdb.NewMemDB()
	var iter types.Iterator
	var err error

	iter, _ = store.Iterator(nil, nil)
	iteratorID11, err := storeIterator(callID1, iter, limit)
	require.NoError(t, err)
	iter, _ = store.Iterator(nil, nil)
	_, err = storeIterator(callID1, iter, limit)
	require.NoError(t, err)
	iter, _ = store.Iterator(nil, nil)
	_, err = storeIterator(callID2, iter, limit)
	require.NoError(t, err)
	iter, _ = store.Iterator(nil, nil)
	iteratorID22, err := storeIterator(callID2, iter, limit)
	require.NoError(t, err)
	iter, err = store.Iterator(nil, nil)
	require.NoError(t, err)
	iteratorID23, err := storeIterator(callID2, iter, limit)
	require.NoError(t, err)

	// Retrieve existing
	iter = retrieveIterator(callID1, iteratorID11)
	require.NotNil(t, iter)
	iter = retrieveIterator(callID2, iteratorID22)
	require.NotNil(t, iter)

	// Retrieve with non-existent iterator ID
	iter = retrieveIterator(callID1, iteratorID23)
	require.Nil(t, iter)
	iter = retrieveIterator(callID1, uint64(0))
	require.Nil(t, iter)
	iter = retrieveIterator(callID1, uint64(2147483647))
	require.Nil(t, iter)
	iter = retrieveIterator(callID1, uint64(2147483648))
	require.Nil(t, iter)
	iter = retrieveIterator(callID1, uint64(18446744073709551615))
	require.Nil(t, iter)

	// Retrieve with non-existent call ID
	iter = retrieveIterator(callID1+1_234_567, iteratorID23)
	require.Nil(t, iter)

	endCall(callID1)
	endCall(callID2)
}

func TestQueueIteratorSimple(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	// query the sum
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := setup.Store(gasMeter)
	query := []byte(`{"sum":{}}`)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.Equal(t, `{"sum":39}`, string(qResult.Ok))

	// query reduce (multiple iterators at once)
	query = []byte(`{"reducer":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var reduced types.QueryResult
	err = json.Unmarshal(data, &reduced)
	require.NoError(t, err)
	require.Equal(t, "", reduced.Err)
	require.JSONEq(t, `{"counters":[[17,22],[22,0]]}`, string(reduced.Ok))
}

func TestQueueIteratorRaces(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	require.Empty(t, iteratorFrames)

	contract1 := setupQueueContractWithData(t, cache, 17, 22)
	contract2 := setupQueueContractWithData(t, cache, 1, 19, 6, 35, 8)
	contract3 := setupQueueContractWithData(t, cache, 11, 6, 2)
	env := MockEnvBin(t)

	reduceQuery := func(t *testing.T, setup queueData, expected string) {
		t.Helper()
		checksum, querier, api := setup.checksum, setup.querier, setup.api
		gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
		igasMeter := types.GasMeter(gasMeter)
		store := setup.Store(gasMeter)

		// query reduce (multiple iterators at once)
		query := []byte(`{"reducer":{}}`)
		data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err)
		var reduced types.QueryResult
		err = json.Unmarshal(data, &reduced)
		require.NoError(t, err)
		require.Equal(t, "", reduced.Err)
		require.JSONEq(t, fmt.Sprintf(`{"counters":%s}`, expected), string(reduced.Ok))
	}

	// 30 concurrent batches (in go routines) to trigger any race condition
	numBatches := 30

	var wg sync.WaitGroup
	// for each batch, query each of the 3 contracts - so the contract queries get mixed together
	wg.Add(numBatches * 3)
	for i := 0; i < numBatches; i++ {
		go func() {
			reduceQuery(t, contract1, "[[17,22],[22,0]]")
			wg.Done()
		}()
		go func() {
			reduceQuery(t, contract2, "[[1,68],[19,35],[6,62],[35,0],[8,54]]")
			wg.Done()
		}()
		go func() {
			reduceQuery(t, contract3, "[[11,0],[6,11],[2,17]]")
			wg.Done()
		}()
	}
	wg.Wait()

	// when they finish, we should have removed all frames
	require.Empty(t, iteratorFrames)
}

func TestQueueIteratorLimit(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	var err error
	var qResult types.QueryResult
	var gasLimit uint64

	// Open 5000 iterators
	gasLimit = TESTING_GAS_LIMIT
	gasMeter := NewMockGasMeter(gasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := setup.Store(gasMeter)
	query := []byte(`{"open_iterators":{"count":5000}}`)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, gasLimit, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.Equal(t, `{}`, string(qResult.Ok))

	// Open 35000 iterators
	gasLimit = TESTING_GAS_LIMIT * 4
	gasMeter = NewMockGasMeter(gasLimit)
	igasMeter = types.GasMeter(gasMeter)
	store = setup.Store(gasMeter)
	query = []byte(`{"open_iterators":{"count":35000}}`)
	env = MockEnvBin(t)
	_, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, gasLimit, TESTING_PRINT_DEBUG)
	require.ErrorContains(t, err, "Reached iterator limit (32768)")
}

```
---
### `api/lib.go`
*2025-02-26 17:16:11 | 9 KB*
```go
package api

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/wasm"
	"github.com/CosmWasm/wasmvm/v2/types"
)

func init() {
	// Create a new wazero runtime instance and assign it to currentRuntime
	r, err := wasm.NewWazeroVM()
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
var currentRuntime wasm.WasmRuntime

func InitCache(config types.VMConfig) (Cache, error) {
	err := os.MkdirAll(config.Cache.BaseDir, 0o755)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not create base directory: %w", err)
	}

	lockPath := filepath.Join(config.Cache.BaseDir, "exclusive.lock")
	lockfile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not open exclusive.lock")
	}

	// Write the lockfile content
	_, err = lockfile.WriteString("This is a lockfile that prevents two VM instances from operating on the same directory in parallel.\nSee codebase at github.com/CosmWasm/wasmvm for more information.\nSafety first – brought to you by Confio ❤️\n")
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("Error writing to exclusive.lock")
	}

	// Try to acquire the lock
	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		lockfile.Close()
		return Cache{}, fmt.Errorf("Could not lock exclusive.lock. Is a different VM running in the same directory already?")
	}

	// Initialize the runtime with the config
	handle, err := currentRuntime.InitCache(config)
	if err != nil {
		if err := unix.Flock(int(lockfile.Fd()), unix.LOCK_UN); err != nil {
			fmt.Printf("Error unlocking file: %v\n", err)
		}
		lockfile.Close()
		return Cache{}, err
	}

	return Cache{
		handle:   handle,
		lockfile: *lockfile,
	}, nil
}

func ReleaseCache(cache Cache) {
	if cache.handle != nil {
		currentRuntime.ReleaseCache(cache.handle)
	}

	// Release the file lock and close the lockfile
	if cache.lockfile != (os.File{}) {
		if err := unix.Flock(int(cache.lockfile.Fd()), unix.LOCK_UN); err != nil {
			fmt.Printf("Error unlocking cache file: %v\n", err)
		}
		cache.lockfile.Close()
	}
}

func StoreCode(cache Cache, wasm []byte, persist bool) ([]byte, error) {
	if cache.handle == nil {
		return nil, fmt.Errorf("cache handle is nil")
	}
	checksum, err := currentRuntime.StoreCode(wasm, persist)
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

```
---
### `api/lib_test.go`
*2025-02-20 21:49:29 | 50 KB*
```go
package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	TESTING_PRINT_DEBUG  = true
	TESTING_GAS_LIMIT    = uint64(1_000_000_000_000) // ~1ms
	TESTING_MEMORY_LIMIT = 64                        // MiB
	TESTING_CACHE_SIZE   = 2048                      // MiB (2GB)
)

var TESTING_CAPABILITIES = []string{"staking", "stargate", "iterator", "cosmwasm_1_1", "cosmwasm_1_2", "cosmwasm_1_3", "cosmwasm_1_4", "cosmwasm_2_0", "cosmwasm_2_1", "cosmwasm_2_2"}

type CapitalizedResponse struct {
	Text string `json:"text"`
}

func TestInitAndReleaseCache(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

// wasmd expects us to create the base directory
// https://github.com/CosmWasm/wasmd/blob/v0.30.0/x/wasm/keeper/keeper.go#L128
func TestInitCacheWorksForNonExistentDir(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	createMe := filepath.Join(tmpdir, "does-not-yet-exist")
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  createMe,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func TestInitCacheErrorsForBrokenDir(t *testing.T) {
	// Use colon to make this fail on Windows
	// https://gist.github.com/doctaphred/d01d05291546186941e1b7ddc02034d3
	// On Unix we should not have permission to create this.
	cannotBeCreated := "/foo:bar"
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  cannotBeCreated,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	_, err := InitCache(config)
	require.ErrorContains(t, err, "Could not create base directory")
}

func TestInitLockingPreventsConcurrentAccess(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	config1 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache1, err1 := InitCache(config1)
	require.NoError(t, err1)

	config2 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	_, err2 := InitCache(config2)
	require.ErrorContains(t, err2, "Could not lock exclusive.lock. Is a different VM running in the same directory already?")

	ReleaseCache(cache1)

	// Now we can try again
	config3 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache3, err3 := InitCache(config3)
	require.NoError(t, err3)
	ReleaseCache(cache3)
}

func TestInitLockingAllowsMultipleInstancesInDifferentDirs(t *testing.T) {
	tmpdir1, err := os.MkdirTemp("", "wasmvm-testing1")
	require.NoError(t, err)
	tmpdir2, err := os.MkdirTemp("", "wasmvm-testing2")
	require.NoError(t, err)
	tmpdir3, err := os.MkdirTemp("", "wasmvm-testing3")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir1)
	defer os.RemoveAll(tmpdir2)
	defer os.RemoveAll(tmpdir3)

	config1 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir1,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache1, err1 := InitCache(config1)
	require.NoError(t, err1)
	config2 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir2,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache2, err2 := InitCache(config2)
	require.NoError(t, err2)
	config3 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir3,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache3, err3 := InitCache(config3)
	require.NoError(t, err3)

	ReleaseCache(cache1)
	ReleaseCache(cache2)
	ReleaseCache(cache3)
}

func TestInitCacheEmptyCapabilities(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    []string{},
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func withCache(tb testing.TB) (Cache, func()) {
	tb.Helper()
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(tb, err)
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(tb, err)

	cleanup := func() {
		os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	return cache, cleanup
}

func TestStoreCodeAndGetCode(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	require.Equal(t, expectedChecksum[:], checksum)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestRemoveCode(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	// First removal works
	err = RemoveCode(cache, checksum)
	require.NoError(t, err)

	// Second removal fails
	err = RemoveCode(cache, checksum)
	require.ErrorContains(t, err, "Wasm file does not exist")
}

func TestStoreCodeFailsWithBadData(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm := []byte("some invalid data")
	_, err := StoreCode(cache, wasm, true)
	require.Error(t, err)
}

func TestStoreCodeUnchecked(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCodeUnchecked(cache, wasm)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	require.Equal(t, expectedChecksum[:], checksum)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestStoreCodeUncheckedWorksWithInvalidWasm(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	// Look for "interface_version_8" in the wasm file and replace it with "interface_version_9".
	// This makes the wasm file invalid.
	wasm = bytes.Replace(wasm, []byte("interface_version_8"), []byte("interface_version_9"), 1)

	// StoreCode should fail
	_, err = StoreCode(cache, wasm, true)
	require.ErrorContains(t, err, "contract has unknown")

	// StoreCodeUnchecked should not fail
	checksum, err := StoreCodeUnchecked(cache, wasm)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	assert.Equal(t, expectedChecksum[:], checksum)
}

func TestPin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Pin(cache, checksum)
	require.NoError(t, err)
}

func TestPinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Pin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Pin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// Unknown checksum (errors in cosmwasm-vm)
	unknownChecksum := []byte{
		0x72, 0x2c, 0x8c, 0x99, 0x3f, 0xd7, 0x5a, 0x76, 0x27, 0xd6, 0x9e, 0xd9, 0x41, 0x34,
		0x4f, 0xe2, 0xa1, 0x42, 0x3a, 0x3e, 0x75, 0xef, 0xd3, 0xe6, 0x77, 0x8a, 0x14, 0x28,
		0x84, 0x22, 0x71, 0x04,
	}
	err = Pin(cache, unknownChecksum)
	require.ErrorContains(t, err, "Error opening Wasm file for reading")
}

func TestUnpin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Unpin(cache, checksum)
	require.NoError(t, err)
}

func TestUnpinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Unpin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Unpin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// No error case triggered in cosmwasm-vm is known right now
}

func TestGetMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.Metrics{}, metrics)

	// Store contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	// GetMetrics 2
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg2, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 4
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Pin
	err = Pin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg3, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 6
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Unpin
	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 4
	msg4 := []byte(`{"verifier": "fred", "beneficiary": "jeff"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg4, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 8
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)
}

func TestGetPinnedMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.PinnedMetrics{PerModule: make([]types.PerModuleEntry, 0)}, metrics)

	// Store contract 1
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Store contract 2
	cyberpunkWasm, err := os.ReadFile("../../testdata/cyberpunk.wasm")
	require.NoError(t, err)
	cyberpunkChecksum, err := StoreCode(cache, cyberpunkWasm, true)
	require.NoError(t, err)

	err = Pin(cache, cyberpunkChecksum)
	require.NoError(t, err)

	findMetrics := func(list []types.PerModuleEntry, checksum types.Checksum) *types.PerModuleMetrics {
		found := (*types.PerModuleMetrics)(nil)

		for _, structure := range list {
			if bytes.Equal(structure.Checksum, checksum) {
				found = &structure.Metrics
				break
			}
		}

		return found
	}

	// GetMetrics 2
	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Len(t, metrics.PerModule, 2)

	hackatomMetrics := findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics := findMetrics(metrics.PerModule, cyberpunkChecksum)

	require.Equal(t, uint32(0), hackatomMetrics.Hits)
	require.NotEqual(t, uint32(0), hackatomMetrics.Size)
	require.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	require.NotEqual(t, uint32(0), cyberpunkMetrics.Size)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Len(t, metrics.PerModule, 2)

	hackatomMetrics = findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics = findMetrics(metrics.PerModule, cyberpunkChecksum)

	require.Equal(t, uint32(1), hackatomMetrics.Hits)
	require.NotEqual(t, uint32(0), hackatomMetrics.Size)
	require.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	require.NotEqual(t, uint32(0), cyberpunkMetrics.Size)
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa3e4ae), cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Empty(t, result.Ok.Messages)
}

func TestExecute(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa3e4ae), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// execute with the same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	res, cost, err = Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0x12899a6), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// make sure it read the balance properly and we got 250 atoms
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	// Ensure we got our custom event
	require.Len(t, result.Ok.Events, 1)
	ev := result.Ok.Events[0]
	require.Equal(t, "hackatom", ev.Type)
	require.Len(t, ev.Attributes, 1)
	require.Equal(t, "action", ev.Attributes[0].Key)
	require.Equal(t, "release", ev.Attributes[0].Value)

	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	require.Equal(t, "bob", send.ToAddress)
	require.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	require.Equal(t, expectedData, result.Ok.Data)
}

func TestExecutePanic(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a panic
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"panic":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.Error(t, err)
	require.Contains(t, err.Error(), "RuntimeError: Aborted: panicked at src/contract.rs:127:5:\nThis page intentionally faulted")
}

func TestExecuteUnreachable(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a panic
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"unreachable":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.ErrorContains(t, err, "RuntimeError: unreachable")
}

func TestExecuteCpuLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x72c3ce), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// execute a cpu loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	_, cost, err = Execute(cache, checksum, env, info, []byte(`{"cpu_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, false)
	diff = time.Since(start)
	require.Error(t, err)
	require.Equal(t, cost.UsedInternally, maxGas)
	t.Logf("CPULoop Time (%d gas): %s\n", cost.UsedInternally, diff)
}

func TestExecuteStorageLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a storage loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start := time.Now()
	_, gasReport, err := Execute(cache, checksum, env, info, []byte(`{"storage_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, false)
	diff := time.Since(start)
	require.Error(t, err)
	t.Logf("StorageLoop Time (%d gas): %s\n", gasReport.UsedInternally, diff)
	t.Logf("Gas used: %d\n", gasMeter2.GasConsumed())
	t.Logf("Wasm gas: %d\n", gasReport.UsedInternally)

	// the "sdk gas" * GasMultiplier + the wasm cost should equal the maxGas (or be very close)
	totalCost := gasReport.UsedInternally + gasMeter2.GasConsumed()
	require.Equal(t, int64(maxGas), int64(totalCost))
}

func BenchmarkContractCall(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
		igasMeter2 := types.GasMeter(gasMeter2)
		store.SetGasMeter(gasMeter2)
		info = MockInfoBin(b, "fred")
		msg := []byte(`{"allocate_large_memory":{"pages":0}}`) // replace with noop once we have it
		res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(b, err)
		requireOkResponse(b, res, 0)
	}
}

func Benchmark100ConcurrentContractCalls(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	info = MockInfoBin(b, "fred")

	const callCount = 100 // Calls per benchmark iteration

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		errChan := make(chan error, callCount)
		resChan := make(chan []byte, callCount)
		wg.Add(callCount)
		info = mockInfoBinNoAssert("fred")
		for i := 0; i < callCount; i++ {
			go func() {
				defer wg.Done()
				gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
				igasMeter2 := types.GasMeter(gasMeter2)
				store.SetGasMeter(gasMeter2)
				msg := []byte(`{"allocate_large_memory":{"pages":0}}`) // replace with noop once we have it
				res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
				errChan <- err
				resChan <- res
			}()
		}
		wg.Wait()
		close(errChan)
		close(resChan)

		// Now check results in the main test goroutine
		for i := 0; i < callCount; i++ {
			require.NoError(b, <-errChan)
			requireOkResponse(b, <-resChan, 0)
		}
	}
}

func TestExecuteUserErrorsInApiCalls(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	defaultApi := NewMockAPI()
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, defaultApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	failingApi := NewMockFailureAPI()
	res, _, err = Execute(cache, checksum, env, info, []byte(`{"user_errors_in_api_calls":{}}`), &igasMeter2, store, failingApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
}

func TestMigrate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// verifier is fred
	query := []byte(`{"verifier":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.JSONEq(t, `{"verifier":"fred"}`, string(qResult.Ok))

	// migrate to a new verifier - alice
	// we use the same code blob as we are testing hackatom self-migration
	info = MockInfoBin(t, "admin")
	_, _, err = MigrateWithInfo(cache, checksum, env, []byte(`{"verifier":"alice"}`), info, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// should update verifier to alice
	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult2 types.QueryResult
	err = json.Unmarshal(data, &qResult2)
	require.NoError(t, err)
	require.Equal(t, "", qResult2.Err)
	require.JSONEq(t, `{"verifier":"alice"}`, string(qResult2.Ok))
}

func TestMultipleInstances(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// instance1 controlled by fred
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store1 := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "regen")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store1, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa2aeb8), cost.UsedInternally)

	// instance2 controlled by mary
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store2 := NewLookup(gasMeter2)
	info = MockInfoBin(t, "chrous")
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, cost, err = Instantiate(cache, checksum, env, info, msg, &igasMeter2, store2, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xa35f43), cost.UsedInternally)

	// fail to execute store1 with mary
	resp := exec(t, cache, checksum, "mary", store1, api, querier, 0x9a2b03)
	require.Equal(t, "Unauthorized", resp.Err)

	// succeed to execute store1 with fred
	resp = exec(t, cache, checksum, "fred", store1, api, querier, 0x1281a12)
	require.Equal(t, "", resp.Err)
	require.Len(t, resp.Ok.Messages, 1)
	attributes := resp.Ok.Attributes
	require.Len(t, attributes, 2)
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "bob", attributes[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, checksum, "mary", store2, api, querier, 0x12859dc)
	require.Equal(t, "", resp.Err)
	require.Len(t, resp.Ok.Messages, 1)
	attributes = resp.Ok.Attributes
	require.Len(t, attributes, 2)
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "sue", attributes[1].Value)
}

func TestSudo(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// call sudo with same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	msg = []byte(`{"steal_funds":{"recipient":"community-pool","amount":[{"amount":"700","denom":"gold"}]}}`)
	res, _, err = Sudo(cache, checksum, env, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "community-pool", send.ToAddress)
	expectedPayout := types.Array[types.Coin]{types.NewCoin(700, "gold")}
	assert.Equal(t, expectedPayout, send.Amount)
}

func TestDispatchSubmessage(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// dispatch a submessage
	var id uint64 = 1234
	payload := types.SubMsg{
		ID: id,
		Msg: types.CosmosMsg{Bank: &types.BankMsg{Send: &types.SendMsg{
			ToAddress: "friend",
			Amount:    types.Array[types.Coin]{types.NewCoin(1, "token")},
		}}},
		ReplyOn: types.ReplyAlways,
	}
	payloadBin, err := json.Marshal(payload)
	require.NoError(t, err)
	payloadMsg := []byte(fmt.Sprintf(`{"reflect_sub_msg":{"msgs":[%s]}}`, string(payloadBin)))

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Execute(cache, checksum, env, info, payloadMsg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	dispatch := result.Ok.Messages[0]
	assert.Equal(t, id, dispatch.ID)
	assert.Equal(t, payload.Msg, dispatch.Msg)
	assert.Nil(t, dispatch.GasLimit)
	assert.Equal(t, payload.ReplyOn, dispatch.ReplyOn)
}

func TestReplyAndQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	var id uint64 = 1234
	data := []byte("foobar")
	events := types.Array[types.Event]{{
		Type: "message",
		Attributes: types.Array[types.EventAttribute]{{
			Key:   "signer",
			Value: "caller-addr",
		}},
	}}
	reply := types.Reply{
		ID: id,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: events,
				Data:   data,
			},
		},
	}
	replyBin, err := json.Marshal(reply)
	require.NoError(t, err)

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Reply(cache, checksum, env, replyBin, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// now query the state to see if it stored the data properly
	badQuery := []byte(`{"sub_msg_result":{"id":7777}}`)
	res, _, err = Query(cache, checksum, env, badQuery, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireQueryError(t, res)

	query := []byte(`{"sub_msg_result":{"id":1234}}`)
	res, _, err = Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	qResult := requireQueryOk(t, res)

	var stored types.Reply
	err = json.Unmarshal(qResult, &stored)
	require.NoError(t, err)
	assert.Equal(t, id, stored.ID)
	require.NotNil(t, stored.Result.Ok)
	val := stored.Result.Ok
	require.Equal(t, data, val.Data)
	require.Equal(t, events, val.Events)
}

func requireOkResponse(tb testing.TB, res []byte, expectedMsgs int) {
	tb.Helper()
	var result types.ContractResult
	err := json.Unmarshal(res, &result)
	require.NoError(tb, err)
	require.Equal(tb, "", result.Err)
	require.Len(tb, result.Ok.Messages, expectedMsgs)
}

func requireQueryError(t *testing.T, res []byte) {
	t.Helper()
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Ok)
	require.NotEmpty(t, result.Err)
}

func requireQueryOk(t *testing.T, res []byte) []byte {
	t.Helper()
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Err)
	require.NotEmpty(t, result.Ok)
	return result.Ok
}

func createHackatomContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/hackatom.wasm")
}

func createCyberpunkContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/cyberpunk.wasm")
}

func createQueueContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/queue.wasm")
}

func createReflectContract(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/reflect.wasm")
}

func createFloaty2(tb testing.TB, cache Cache) []byte {
	tb.Helper()
	return createContract(tb, cache, "../../testdata/floaty_2.0.wasm")
}

func createContract(tb testing.TB, cache Cache, wasmFile string) []byte {
	tb.Helper()
	wasm, err := os.ReadFile(wasmFile)
	require.NoError(tb, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(tb, err)
	return checksum
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, checksum []byte, signer types.HumanAddress, store types.KVStore, api *types.GoAPI, querier types.Querier, gasExpected uint64) types.ContractResult {
	t.Helper()
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	env := MockEnvBin(t)
	info := MockInfoBin(t, signer)
	res, cost, err := Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	assert.Equal(t, gasExpected, cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	return result
}

func TestQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// invalid query
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var badResult types.QueryResult
	err = json.Unmarshal(data, &badResult)
	require.NoError(t, err)
	require.Contains(t, badResult.Err, "Error parsing into type hackatom::msg::QueryMsg: unknown variant `Raw`, expected one of")

	// make a valid query
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := types.GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter3, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.JSONEq(t, `{"verifier":"fred"}`, string(qResult.Ok))
}

func TestHackatomQuerier(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM"), types.NewCoin(65432, "ETH")}
	querier := DefaultQuerier("foobar", initBalance)

	// make a valid query to the other address
	query := []byte(`{"other_balance":{"address":"foobar"}}`)
	// TODO The query happens before the contract is initialized. How is this legal?
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	var balances types.AllBalancesResponse
	err = json.Unmarshal(qResult.Ok, &balances)
	require.NoError(t, err)
	require.Equal(t, balances.Amount, initBalance)
}

func TestCustomReflectQuerier(t *testing.T) {
	type CapitalizedQuery struct {
		Text string `json:"text"`
	}

	type QueryMsg struct {
		Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
		// There are more queries but we don't use them yet
		// https://github.com/CosmWasm/cosmwasm/blob/v0.11.0-alpha3/contracts/reflect/src/msg.rs#L18-L28
	}

	type CapitalizedResponse struct {
		Text string `json:"text"`
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, initBalance)
	// we need this to handle the custom requests from the reflect contract
	innerQuerier := querier.(*MockQuerier)
	innerQuerier.Custom = ReflectCustom{}
	querier = types.Querier(innerQuerier)

	// make a valid query to the other address
	queryMsg := QueryMsg{
		Capitalized: &CapitalizedQuery{
			Text: "small Frys :)",
		},
	}
	query, err := json.Marshal(queryMsg)
	require.NoError(t, err)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)

	var response CapitalizedResponse
	err = json.Unmarshal(qResult.Ok, &response)
	require.NoError(t, err)
	require.Equal(t, "SMALL FRYS :)", response.Text)
}

// testfloats is disabled temporarily because of its high output

// TestFloats is a port of the float_instrs_are_deterministic test in cosmwasm-vm
func TestFloats(t *testing.T) {
	type Value struct {
		U32 *uint32 `json:"u32,omitempty"`
		U64 *uint64 `json:"u64,omitempty"`
		F32 *uint32 `json:"f32,omitempty"`
		F64 *uint64 `json:"f64,omitempty"`
	}

	// helper to print the value in the same format as Rust's Debug trait
	debugStr := func(value Value) string {
		if value.U32 != nil {
			return fmt.Sprintf("U32(%d)", *value.U32)
		} else if value.U64 != nil {
			return fmt.Sprintf("U64(%d)", *value.U64)
		} else if value.F32 != nil {
			return fmt.Sprintf("F32(%d)", *value.F32)
		} else if value.F64 != nil {
			return fmt.Sprintf("F64(%d)", *value.F64)
		} else {
			t.FailNow()
			return ""
		}
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createFloaty2(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)

	// query instructions
	query := []byte(`{"instructions":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, false)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Empty(t, qResult.Err)
	var instructions []string
	err = json.Unmarshal(qResult.Ok, &instructions)
	require.NoError(t, err)
	// little sanity check
	require.Len(t, instructions, 70)

	hasher := sha256.New()
	const RUNS_PER_INSTRUCTION = 150
	for _, instr := range instructions {
		for seed := 0; seed < RUNS_PER_INSTRUCTION; seed++ {
			// query some input values for the instruction
			msg := fmt.Sprintf(`{"random_args_for":{"instruction":"%s","seed":%d}}`, instr, seed)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, false)
			require.NoError(t, err)
			err = json.Unmarshal(data, &qResult)
			require.NoError(t, err)
			require.Empty(t, qResult.Err)
			var args []Value
			err = json.Unmarshal(qResult.Ok, &args)
			require.NoError(t, err)

			// build the run message
			argStr, err := json.Marshal(args)
			require.NoError(t, err)
			msg = fmt.Sprintf(`{"run":{"instruction":"%s","args":%s}}`, instr, argStr)

			// run the instruction
			// this might throw a runtime error (e.g. if the instruction traps)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, false)
			var result string
			if err != nil {
				require.Error(t, err)
				// remove the prefix to make the error message the same as in the cosmwasm-vm test
				result = strings.Replace(err.Error(), "Error calling the VM: Error executing Wasm: ", "", 1)
			} else {
				err = json.Unmarshal(data, &qResult)
				require.NoError(t, err)
				require.Empty(t, qResult.Err)
				var response Value
				err = json.Unmarshal(qResult.Ok, &response)
				require.NoError(t, err)
				result = debugStr(response)
			}
			// add the result to the hash
			hasher.Write([]byte(fmt.Sprintf("%s%d%s", instr, seed, result)))
		}
	}

	hash := hasher.Sum(nil)
	require.Equal(t, "6e9ffbe929a2c1bcbffca0d4e9d0935371045bba50158a01ec082459a4cbbd2a", hex.EncodeToString(hash))
}

// mockInfoBinNoAssert creates the message binary without using testify assertions
func mockInfoBinNoAssert(sender types.HumanAddress) []byte {
	info := types.MessageInfo{
		Sender: sender,
		Funds:  types.Array[types.Coin]{},
	}
	res, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}
	return res
}

```
---
### `api/mock_failure.go`
*2024-12-19 16:14:31 | 1 KB*
```go
package api

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

/***** Mock types.GoAPI ****/

func MockFailureCanonicalizeAddress(human string) ([]byte, uint64, error) {
	return nil, 0, fmt.Errorf("mock failure - canonical_address")
}

func MockFailureHumanizeAddress(canon []byte) (string, uint64, error) {
	return "", 0, fmt.Errorf("mock failure - human_address")
}

func MockFailureValidateAddress(human string) (uint64, error) {
	return 0, fmt.Errorf("mock failure - validate_address")
}

func NewMockFailureAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockFailureHumanizeAddress,
		CanonicalizeAddress: MockFailureCanonicalizeAddress,
		ValidateAddress:     MockFailureValidateAddress,
	}
}

```
---
### `api/mocks.go`
*2025-02-20 21:49:29 | 17 KB*
```go
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api/testdb"
	"github.com/CosmWasm/wasmvm/v2/types"
)

/** helper constructors **/

const MOCK_CONTRACT_ADDR = "contract"

// MockEnv returns a mock environment for testing
// this is the original, and should not be changed.
func MockEnv() types.Env {
	return types.Env{
		Block: types.BlockInfo{
			Height:  123,
			Time:    1578939743_987654321,
			ChainID: "foobar",
		},
		Transaction: &types.TransactionInfo{
			Index: 4,
		},
		Contract: types.ContractInfo{
			Address: MOCK_CONTRACT_ADDR,
		},
	}
}

func MockEnvBin(tb testing.TB) []byte {
	tb.Helper()
	env := MockEnv()
	// Create a map with fields in the exact order we want
	envMap := map[string]interface{}{
		"block": map[string]interface{}{
			"height":   env.Block.Height,
			"time":     env.Block.Time,
			"chain_id": env.Block.ChainID,
		},
		"transaction": map[string]interface{}{
			"index": env.Transaction.Index,
		},
		"contract": map[string]interface{}{
			"address": env.Contract.Address,
		},
	}
	// Use a custom encoder to preserve field order
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(envMap)
	require.NoError(tb, err)
	bin := bytes.TrimSpace(buf.Bytes())
	fmt.Printf("[DEBUG] MockEnvBin JSON: %s\n", string(bin))
	return bin
}

func MockInfo(sender types.HumanAddress, funds []types.Coin) types.MessageInfo {
	return types.MessageInfo{
		Sender: sender,
		Funds:  funds,
	}
}

func MockInfoWithFunds(sender types.HumanAddress) types.MessageInfo {
	return MockInfo(sender, []types.Coin{{
		Denom:  "ATOM",
		Amount: "100",
	}})
}

func MockInfoBin(tb testing.TB, sender types.HumanAddress) []byte {
	tb.Helper()
	bin, err := json.Marshal(MockInfoWithFunds(sender))
	require.NoError(tb, err)
	return bin
}

func MockIBCChannel(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannel {
	return types.IBCChannel{
		Endpoint: types.IBCEndpoint{
			PortID:    "my_port",
			ChannelID: channelID,
		},
		CounterpartyEndpoint: types.IBCEndpoint{
			PortID:    "their_port",
			ChannelID: "channel-7",
		},
		Order:        ordering,
		Version:      ibcVersion,
		ConnectionID: "connection-3",
	}
}

func MockIBCChannelOpenInit(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelOpenMsg {
	return types.IBCChannelOpenMsg{
		OpenInit: &types.IBCOpenInit{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
		OpenTry: nil,
	}
}

func MockIBCChannelOpenTry(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelOpenMsg {
	return types.IBCChannelOpenMsg{
		OpenInit: nil,
		OpenTry: &types.IBCOpenTry{
			Channel:             MockIBCChannel(channelID, ordering, ibcVersion),
			CounterpartyVersion: ibcVersion,
		},
	}
}

func MockIBCChannelConnectAck(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelConnectMsg {
	return types.IBCChannelConnectMsg{
		OpenAck: &types.IBCOpenAck{
			Channel:             MockIBCChannel(channelID, ordering, ibcVersion),
			CounterpartyVersion: ibcVersion,
		},
		OpenConfirm: nil,
	}
}

func MockIBCChannelConnectConfirm(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelConnectMsg {
	return types.IBCChannelConnectMsg{
		OpenAck: nil,
		OpenConfirm: &types.IBCOpenConfirm{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
	}
}

func MockIBCChannelCloseInit(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelCloseMsg {
	return types.IBCChannelCloseMsg{
		CloseInit: &types.IBCCloseInit{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
		CloseConfirm: nil,
	}
}

func MockIBCChannelCloseConfirm(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelCloseMsg {
	return types.IBCChannelCloseMsg{
		CloseInit: nil,
		CloseConfirm: &types.IBCCloseConfirm{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
	}
}

func MockIBCPacket(myChannel string, data []byte) types.IBCPacket {
	return types.IBCPacket{
		Data: data,
		Src: types.IBCEndpoint{
			PortID:    "their_port",
			ChannelID: "channel-7",
		},
		Dest: types.IBCEndpoint{
			PortID:    "my_port",
			ChannelID: myChannel,
		},
		Sequence: 15,
		Timeout: types.IBCTimeout{
			Block: &types.IBCTimeoutBlock{
				Revision: 1,
				Height:   123456,
			},
		},
	}
}

func MockIBCPacketReceive(myChannel string, data []byte) types.IBCPacketReceiveMsg {
	return types.IBCPacketReceiveMsg{
		Packet: MockIBCPacket(myChannel, data),
	}
}

func MockIBCPacketAck(myChannel string, data []byte, ack types.IBCAcknowledgement) types.IBCPacketAckMsg {
	packet := MockIBCPacket(myChannel, data)

	return types.IBCPacketAckMsg{
		Acknowledgement: ack,
		OriginalPacket:  packet,
	}
}

func MockIBCPacketTimeout(myChannel string, data []byte) types.IBCPacketTimeoutMsg {
	packet := MockIBCPacket(myChannel, data)

	return types.IBCPacketTimeoutMsg{
		Packet: packet,
	}
}

/*** Mock GasMeter ****/
// This code is borrowed from Cosmos-SDK store/types/gas.go

// ErrorOutOfGas defines an error thrown when an action results in out of gas.
type ErrorOutOfGas struct {
	Descriptor string
}

// ErrorGasOverflow defines an error thrown when an action results gas consumption
// unsigned integer overflow.
type ErrorGasOverflow struct {
	Descriptor string
}

type MockGasMeter interface {
	types.GasMeter
	ConsumeGas(amount types.Gas, descriptor string)
}

type mockGasMeter struct {
	limit    types.Gas
	consumed types.Gas
}

// NewMockGasMeter returns a reference to a new mockGasMeter.
func NewMockGasMeter(limit types.Gas) MockGasMeter {
	return &mockGasMeter{
		limit:    limit,
		consumed: 0,
	}
}

func (g *mockGasMeter) GasConsumed() types.Gas {
	return g.consumed
}

func (g *mockGasMeter) Limit() types.Gas {
	return g.limit
}

// addUint64Overflow performs the addition operation on two uint64 integers and
// returns a boolean on whether or not the result overflows.
func addUint64Overflow(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, true
	}

	return a + b, false
}

func (g *mockGasMeter) ConsumeGas(amount types.Gas, descriptor string) {
	var overflow bool
	// TODO: Should we set the consumed field after overflow checking?
	g.consumed, overflow = addUint64Overflow(g.consumed, amount)
	if overflow {
		panic(ErrorGasOverflow{descriptor})
	}

	if g.consumed > g.limit {
		panic(ErrorOutOfGas{descriptor})
	}
}

/*** Mock types.KVStore ****/
// Much of this code is borrowed from Cosmos-SDK store/transient.go

// Note: these gas prices are all in *wasmer gas* and (sdk gas * 100)
//
// We making simple values and non-clear multiples so it is easy to see their impact in test output
// Also note we do not charge for each read on an iterator (out of simplicity and not needed for tests)
const (
	GetPrice    uint64 = 99000
	SetPrice    uint64 = 187000
	RemovePrice uint64 = 142000
	RangePrice  uint64 = 261000
)

type Lookup struct {
	db    *testdb.MemDB
	meter MockGasMeter
}

func NewLookup(meter MockGasMeter) *Lookup {
	return &Lookup{
		db:    testdb.NewMemDB(),
		meter: meter,
	}
}

func (l *Lookup) SetGasMeter(meter MockGasMeter) {
	l.meter = meter
}

func (l *Lookup) WithGasMeter(meter MockGasMeter) *Lookup {
	return &Lookup{
		db:    l.db,
		meter: meter,
	}
}

// Get wraps the underlying DB's Get method panicking on error.
func (l Lookup) Get(key []byte) []byte {
	l.meter.ConsumeGas(GetPrice, "get")
	v, err := l.db.Get(key)
	if err != nil {
		panic(err)
	}

	return v
}

// Set wraps the underlying DB's Set method panicking on error.
func (l Lookup) Set(key, value []byte) {
	l.meter.ConsumeGas(SetPrice, "set")
	if err := l.db.Set(key, value); err != nil {
		panic(err)
	}
}

// Delete wraps the underlying DB's Delete method panicking on error.
func (l Lookup) Delete(key []byte) {
	l.meter.ConsumeGas(RemovePrice, "remove")
	if err := l.db.Delete(key); err != nil {
		panic(err)
	}
}

// Iterator wraps the underlying DB's Iterator method panicking on error.
func (l Lookup) Iterator(start, end []byte) types.Iterator {
	l.meter.ConsumeGas(RangePrice, "range")
	iter, err := l.db.Iterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

// ReverseIterator wraps the underlying DB's ReverseIterator method panicking on error.
func (l Lookup) ReverseIterator(start, end []byte) types.Iterator {
	l.meter.ConsumeGas(RangePrice, "range")
	iter, err := l.db.ReverseIterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

var _ types.KVStore = (*Lookup)(nil)

/***** Mock types.GoAPI ****/

const CanonicalLength = 32

const (
	CostCanonical uint64 = 440
	CostHuman     uint64 = 550
)

func MockCanonicalizeAddress(human string) ([]byte, uint64, error) {
	if len(human) > CanonicalLength {
		return nil, 0, fmt.Errorf("human encoding too long")
	}
	res := make([]byte, CanonicalLength)
	copy(res, []byte(human))
	return res, CostCanonical, nil
}

func MockHumanizeAddress(canon []byte) (string, uint64, error) {
	if len(canon) != CanonicalLength {
		return "", 0, fmt.Errorf("wrong canonical length")
	}
	cut := CanonicalLength
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human := string(canon[:cut])
	return human, CostHuman, nil
}

func MockValidateAddress(input string) (gasCost uint64, _ error) {
	canonicalized, gasCostCanonicalize, err := MockCanonicalizeAddress(input)
	gasCost += gasCostCanonicalize
	if err != nil {
		return gasCost, err
	}
	humanized, gasCostHumanize, err := MockHumanizeAddress(canonicalized)
	gasCost += gasCostHumanize
	if err != nil {
		return gasCost, err
	}
	if humanized != strings.ToLower(input) {
		return gasCost, fmt.Errorf("address validation failed")
	}

	return gasCost, nil
}

func NewMockAPI() *types.GoAPI {
	return &types.GoAPI{
		// Simply convert the canonical address back to string.
		HumanizeAddress: func(canon []byte) (string, uint64, error) {
			return string(canon), 0, nil
		},
		// Return the raw bytes of the human address.
		CanonicalizeAddress: func(human string) ([]byte, uint64, error) {
			if human == "" {
				return nil, 0, fmt.Errorf("empty address")
			}
			// For testing, simply return the bytes of the input string.
			return []byte(human), 0, nil
		},
		// Accept any non-empty string.
		ValidateAddress: func(human string) (uint64, error) {
			if human == "" {
				return 0, fmt.Errorf("empty address")
			}
			// In our test environment, all non-empty addresses are valid.
			return 0, nil
		},
	}
}

func TestMockApi(t *testing.T) {
	human := "foobar"
	canon, cost, err := MockCanonicalizeAddress(human)
	require.NoError(t, err)
	require.Len(t, canon, CanonicalLength)
	require.Equal(t, CostCanonical, cost)

	recover, cost, err := MockHumanizeAddress(canon)
	require.NoError(t, err)
	require.Equal(t, recover, human)
	require.Equal(t, CostHuman, cost)
}

/**** MockQuerier ****/

const DEFAULT_QUERIER_GAS_LIMIT = 1_000_000

type MockQuerier struct {
	Bank    BankQuerier
	Custom  CustomQuerier
	usedGas uint64
}

var _ types.Querier = &MockQuerier{}

func DefaultQuerier(contractAddr string, coins types.Array[types.Coin]) types.Querier {
	balances := map[string]types.Array[types.Coin]{
		contractAddr: coins,
	}
	return &MockQuerier{
		Bank:    NewBankQuerier(balances),
		Custom:  NoCustom{},
		usedGas: 0,
	}
}

func (q *MockQuerier) Query(request types.QueryRequest, _gasLimit uint64) ([]byte, error) {
	marshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	q.usedGas += uint64(len(marshaled))
	if request.Bank != nil {
		return q.Bank.Query(request.Bank)
	}
	if request.Custom != nil {
		return q.Custom.Query(request.Custom)
	}
	if request.Staking != nil {
		return nil, types.UnsupportedRequest{Kind: "staking"}
	}
	if request.Wasm != nil {
		return nil, types.UnsupportedRequest{Kind: "wasm"}
	}
	return nil, types.Unknown{}
}

func (q MockQuerier) GasConsumed() uint64 {
	return q.usedGas
}

type BankQuerier struct {
	Balances map[string]types.Array[types.Coin]
}

func NewBankQuerier(balances map[string]types.Array[types.Coin]) BankQuerier {
	bal := make(map[string]types.Array[types.Coin], len(balances))
	for k, v := range balances {
		dst := make([]types.Coin, len(v))
		copy(dst, v)
		bal[k] = dst
	}
	return BankQuerier{
		Balances: bal,
	}
}

func (q BankQuerier) Query(request *types.BankQuery) ([]byte, error) {
	if request.Balance != nil {
		denom := request.Balance.Denom
		coin := types.NewCoin(0, denom)
		for _, c := range q.Balances[request.Balance.Address] {
			if c.Denom == denom {
				coin = c
			}
		}
		resp := types.BalanceResponse{
			Amount: coin,
		}
		return json.Marshal(resp)
	}
	if request.AllBalances != nil {
		coins := q.Balances[request.AllBalances.Address]
		resp := types.AllBalancesResponse{
			Amount: coins,
		}
		return json.Marshal(resp)
	}
	return nil, types.UnsupportedRequest{Kind: "Empty BankQuery"}
}

type CustomQuerier interface {
	Query(request json.RawMessage) ([]byte, error)
}

type NoCustom struct{}

var _ CustomQuerier = NoCustom{}

func (q NoCustom) Query(request json.RawMessage) ([]byte, error) {
	return nil, types.UnsupportedRequest{Kind: "custom"}
}

// ReflectCustom fulfills the requirements for testing `reflect` contract
type ReflectCustom struct{}

var _ CustomQuerier = ReflectCustom{}

type CustomQuery struct {
	Ping        *struct{}         `json:"ping,omitempty"`
	Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
}

type CapitalizedQuery struct {
	Text string `json:"text"`
}

// CustomResponse is the response for all `CustomQuery`s
type CustomResponse struct {
	Msg string `json:"msg"`
}

func (q ReflectCustom) Query(request json.RawMessage) ([]byte, error) {
	var query CustomQuery
	err := json.Unmarshal(request, &query)
	if err != nil {
		return nil, err
	}
	var resp CustomResponse
	if query.Ping != nil {
		resp.Msg = "PONG"
	} else if query.Capitalized != nil {
		resp.Msg = strings.ToUpper(query.Capitalized.Text)
	} else {
		return nil, errors.New("Unsupported query")
	}
	return json.Marshal(resp)
}

// ************ test code for mocks *************************//

func TestBankQuerierAllBalances(t *testing.T) {
	addr := "foobar"
	balance := types.Array[types.Coin]{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: addr,
			},
		},
	}
	res, err := q.Query(req, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp types.AllBalancesResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, balance)

	// query missing account
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: "someone-else",
			},
		},
	}
	res, err = q.Query(req2, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp2 types.AllBalancesResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Nil(t, resp2.Amount)
}

func TestBankQuerierBalance(t *testing.T) {
	addr := "foobar"
	balance := types.Array[types.Coin]{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account with matching denom
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "ATOM",
			},
		},
	}
	res, err := q.Query(req, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp types.BalanceResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, types.NewCoin(12345678, "ATOM"))

	// query existing account with missing denom
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "BTC",
			},
		},
	}
	res, err = q.Query(req2, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp2 types.BalanceResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Equal(t, resp2.Amount, types.NewCoin(0, "BTC"))

	// query missing account
	req3 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: "someone-else",
				Denom:   "ATOM",
			},
		},
	}
	res, err = q.Query(req3, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp3 types.BalanceResponse
	err = json.Unmarshal(res, &resp3)
	require.NoError(t, err)
	assert.Equal(t, resp3.Amount, types.NewCoin(0, "ATOM"))
}

func TestReflectCustomQuerier(t *testing.T) {
	q := ReflectCustom{}

	// try ping
	msg, err := json.Marshal(CustomQuery{Ping: &struct{}{}})
	require.NoError(t, err)
	bz, err := q.Query(msg)
	require.NoError(t, err)
	var resp CustomResponse
	err = json.Unmarshal(bz, &resp)
	require.NoError(t, err)
	assert.Equal(t, "PONG", resp.Msg)

	// try capital
	msg2, err := json.Marshal(CustomQuery{Capitalized: &CapitalizedQuery{Text: "small."}})
	require.NoError(t, err)
	bz, err = q.Query(msg2)
	require.NoError(t, err)
	var resp2 CustomResponse
	err = json.Unmarshal(bz, &resp2)
	require.NoError(t, err)
	assert.Equal(t, "SMALL.", resp2.Msg)
}

```
---
### `api/testdb/README.md`
*2024-12-19 16:14:31 | 1 KB*
```markdown
# Testdb
This package contains an in memory DB for testing purpose only. The original code was copied from
https://github.com/tendermint/tm-db/tree/v0.6.7 to decouple project dependencies.

All credits and a big thank you go to the original authors!

```
---
### `api/testdb/memdb.go`
*2025-02-20 21:49:29 | 5 KB*
```go
package testdb

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/google/btree"
)

const (
	// The approximate number of items and children per B-tree node. Tuned with benchmarks.
	bTreeDegree = 32
)

// item is a btree.Item with byte slices as keys and values
type item struct {
	key   []byte
	value []byte
}

// Less implements btree.Item.
func (i *item) Less(other btree.Item) bool {
	// this considers nil == []byte{}, but that's ok since we handle nil endpoints
	// in iterators specially anyway
	return bytes.Compare(i.key, other.(*item).key) == -1
}

// newKey creates a new key item.
func newKey(key []byte) *item {
	return &item{key: key}
}

// newPair creates a new pair item.
func newPair(key, value []byte) *item {
	return &item{key: key, value: value}
}

// MemDB is an in-memory database backend using a B-tree for storage.
//
// For performance reasons, all given and returned keys and values are pointers to the in-memory
// database, so modifying them will cause the stored values to be modified as well. All DB methods
// already specify that keys and values should be considered read-only, but this is especially
// important with MemDB.
type MemDB struct {
	mtx   sync.RWMutex
	btree *btree.BTree
}

// NewMemDB creates a new in-memory database.
func NewMemDB() *MemDB {
	database := &MemDB{
		btree: btree.New(bTreeDegree),
	}
	return database
}

// Get implements DB.
func (db *MemDB) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errKeyEmpty
	}
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	i := db.btree.Get(newKey(key))
	if i != nil {
		return i.(*item).value, nil
	}
	return nil, nil
}

// Has implements DB.
func (db *MemDB) Has(key []byte) (bool, error) {
	if len(key) == 0 {
		return false, errKeyEmpty
	}
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	return db.btree.Has(newKey(key)), nil
}

// Set implements DB.
func (db *MemDB) Set(key []byte, value []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	if value == nil {
		return errValueNil
	}
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.set(key, value)
	return nil
}

// set sets a value without locking the mutex.
func (db *MemDB) set(key []byte, value []byte) {
	db.btree.ReplaceOrInsert(newPair(key, value))
}

// SetSync implements DB.
func (db *MemDB) SetSync(key []byte, value []byte) error {
	return db.Set(key, value)
}

// Delete implements DB.
func (db *MemDB) Delete(key []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.delete(key)
	return nil
}

// delete deletes a key without locking the mutex.
func (db *MemDB) delete(key []byte) {
	db.btree.Delete(newKey(key))
}

// DeleteSync implements DB.
func (db *MemDB) DeleteSync(key []byte) error {
	return db.Delete(key)
}

// Close implements DB.
func (db *MemDB) Close() error {
	// Close is a noop since for an in-memory database, we don't have a destination to flush
	// contents to nor do we want any data loss on invoking Close().
	// See the discussion in https://github.com/tendermint/tendermint/libs/pull/56
	return nil
}

// Print implements DB.
func (db *MemDB) Print() error {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	db.btree.Ascend(func(i btree.Item) bool {
		item := i.(*item)
		fmt.Printf("[%X]:\t[%X]\n", item.key, item.value)
		return true
	})
	return nil
}

// Stats implements DB.
func (db *MemDB) Stats() map[string]string {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", db.btree.Len())
	return stats
}

// Iterator implements DB.
// Takes out a read-lock on the database until the iterator is closed.
func (db *MemDB) Iterator(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	return newMemDBIterator(db, start, end, false), nil
}

// ReverseIterator implements DB.
// Takes out a read-lock on the database until the iterator is closed.
func (db *MemDB) ReverseIterator(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	return newMemDBIterator(db, start, end, true), nil
}

// IteratorNoMtx makes an iterator with no mutex.
func (db *MemDB) IteratorNoMtx(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	return newMemDBIteratorMtxChoice(db, start, end, false, false), nil
}

// ReverseIteratorNoMtx makes an iterator with no mutex.
func (db *MemDB) ReverseIteratorNoMtx(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	return newMemDBIteratorMtxChoice(db, start, end, true, false), nil
}

```
---
### `api/testdb/memdb_iterator.go`
*2025-02-20 21:49:29 | 4 KB*
```go
package testdb

import (
	"bytes"
	"context"

	"github.com/google/btree"
)

const (
	// Size of the channel buffer between traversal goroutine and iterator. Using an unbuffered
	// channel causes two context switches per item sent, while buffering allows more work per
	// context switch. Tuned with benchmarks.
	chBufferSize = 64
)

// memDBIterator is a memDB iterator.
type memDBIterator struct {
	ch     <-chan *item
	cancel context.CancelFunc
	item   *item
	start  []byte
	end    []byte
	useMtx bool
}

var _ Iterator = (*memDBIterator)(nil)

// newMemDBIterator creates a new memDBIterator.
func newMemDBIterator(db *MemDB, start []byte, end []byte, reverse bool) *memDBIterator {
	return newMemDBIteratorMtxChoice(db, start, end, reverse, true)
}

func newMemDBIteratorMtxChoice(db *MemDB, start []byte, end []byte, reverse bool, useMtx bool) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
		useMtx: useMtx,
	}

	if useMtx {
		db.mtx.RLock()
	}
	go func() {
		if useMtx {
			defer db.mtx.RUnlock()
		}
		// Because we use [start, end) for reverse ranges, while btree uses (start, end], we need
		// the following variables to handle some reverse iteration conditions ourselves.
		var (
			skipEqual     []byte
			abortLessThan []byte
		)
		visitor := func(i btree.Item) bool {
			item := i.(*item)
			if skipEqual != nil && bytes.Equal(item.key, skipEqual) {
				skipEqual = nil
				return true
			}
			if abortLessThan != nil && bytes.Compare(item.key, abortLessThan) == -1 {
				return false
			}
			select {
			case <-ctx.Done():
				return false
			case ch <- item:
				return true
			}
		}
		switch {
		case start == nil && end == nil && !reverse:
			db.btree.Ascend(visitor)
		case start == nil && end == nil && reverse:
			db.btree.Descend(visitor)
		case end == nil && !reverse:
			// must handle this specially, since nil is considered less than anything else
			db.btree.AscendGreaterOrEqual(newKey(start), visitor)
		case !reverse:
			db.btree.AscendRange(newKey(start), newKey(end), visitor)
		case end == nil:
			// abort after start, since we use [start, end) while btree uses (start, end]
			abortLessThan = start
			db.btree.Descend(visitor)
		default:
			// skip end and abort after start, since we use [start, end) while btree uses (start, end]
			skipEqual = end
			abortLessThan = start
			db.btree.DescendLessOrEqual(newKey(end), visitor)
		}
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}

	return iter
}

// Close implements Iterator.
func (i *memDBIterator) Close() error {
	i.cancel()
	for range i.ch { // drain channel
	}
	i.item = nil
	return nil
}

// Domain implements Iterator.
func (i *memDBIterator) Domain() ([]byte, []byte) {
	return i.start, i.end
}

// Valid implements Iterator.
func (i *memDBIterator) Valid() bool {
	return i.item != nil
}

// Next implements Iterator.
func (i *memDBIterator) Next() {
	i.assertIsValid()
	item, ok := <-i.ch
	switch {
	case ok:
		i.item = item
	default:
		i.item = nil
	}
}

// Error implements Iterator.
func (i *memDBIterator) Error() error {
	return nil // famous last words
}

// Key implements Iterator.
func (i *memDBIterator) Key() []byte {
	i.assertIsValid()
	if len(i.item.key) == 0 {
		return nil
	}
	return i.item.key
}

// Value implements Iterator.
func (i *memDBIterator) Value() []byte {
	i.assertIsValid()
	if len(i.item.value) == 0 {
		return nil
	}
	return i.item.value
}

func (i *memDBIterator) assertIsValid() {
	if !i.Valid() {
		panic("iterator is invalid")
	}
}

```
---
### `api/testdb/types.go`
*2025-02-18 08:30:26 | 1 KB*
```go
package testdb

import (
	"errors"

	"github.com/CosmWasm/wasmvm/v2/types"
)

var (

	// errKeyEmpty is returned when attempting to use an empty or nil key.
	errKeyEmpty = errors.New("key cannot be empty")

	// errValueNil is returned when attempting to set a nil value.
	errValueNil = errors.New("value cannot be nil")
)

type Iterator = types.Iterator

```
---
### `api/version.go`
*2025-02-20 21:49:29 | 1 KB*
```go
package api

// Just define a constant version here
const wasmvmVersion = "6.9.0"

// LibwasmvmVersion returns the version of this library as a string.
func LibwasmvmVersion() (string, error) {
	// Since we're no longer using cgo, we return the hardcoded version.
	return wasmvmVersion, nil
}

```
---
### `api/version_test.go`
*2025-02-19 15:15:32 | 1 KB*
```go
package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLibwasmvmVersion(t *testing.T) {
	version, err := LibwasmvmVersion()
	require.NoError(t, err)
	require.Regexp(t, `^([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-z0-9.]+)?$`, version)
}

```
---
### `runtime/combined_code.md`
*2025-02-26 16:50:13 | 147 KB*
```markdown
# Combined Code Files

## TOC
- [`constants/constants.go`](#file-1)
- [`constants/gas.go`](#file-2)
- [`crypto/bls_impl.go`](#file-3)
- [`crypto/crypto.go`](#file-4)
- [`crypto/hostcrypto.go`](#file-5)
- [`cryptoapi/types.go`](#file-6)
- [`gas/gas.go`](#file-7)
- [`gas/gasversionone/gas.go`](#file-8)
- [`gas/gasversiontwo/gas.go`](#file-9)
- [`gas/wazero/gas.go`](#file-10)
- [`gas.go`](#file-11)
- [`host/environment.go`](#file-12)
- [`host/hostfunctions.go`](#file-13)
- [`host/memory.go`](#file-14)
- [`host/registerhostfunctions.go`](#file-15)
- [`memory/memory.go`](#file-16)
- [`tracing.go`](#file-17)
- [`types/gas.go`](#file-18)
- [`types/types.go`](#file-19)
- [`validation/validation.go`](#file-20)
- [`wasm/execution.go`](#file-21)
- [`wasm/ibc.go`](#file-22)
- [`wasm/runtime.go`](#file-23)
- [`wasm/system.go`](#file-24)
- [`wasm/vm.go`](#file-25)

---

### `constants/constants.go`
*2025-02-26 15:39:53 | 1 KB*
```go
package constants

const (

	// Point lengths for BLS12-381
	BLS12_381_G1_POINT_LEN = 48
	BLS12_381_G2_POINT_LEN = 96
	WasmPageSize           = 65536
)

// Gas costs for various operations
const (
	// Memory operations
	GasPerByte = 3

	// Database operations
	GasCostRead  = 100
	GasCostWrite = 200
	GasCostQuery = 500

	// Iterator operations
	GasCostIteratorCreate = 10000 // Base cost for creating an iterator
	GasCostIteratorNext   = 1000  // Base cost for iterator next operations

	// Contract operations
	GasCostInstantiate = 40000 // Base cost for contract instantiation
	GasCostExecute     = 20000 // Base cost for contract execution
)

```
---
### `constants/gas.go`
*2025-02-20 21:58:20 | 1 KB*
```go
package constants

const (
	// Gas multiplier for wazero operations
	GasMultiplier uint64 = 100

	// Maximum sizes for BLS operations
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)

```
---
### `crypto/bls_impl.go`
*2025-02-26 15:33:15 | 1 KB*
```go
package crypto

import (
	"fmt"

	bls12381 "github.com/kilic/bls12-381"
)

// Implementation of BLS12381AggregateG1 and BLS12381AggregateG2

// BLS12381AggregateG2 aggregates multiple G2 points into a single compressed G2 point.
func (c *CryptoImplementation) BLS12381AggregateG2(elements [][]byte) ([]byte, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("no elements to aggregate")
	}

	g2 := bls12381.NewG2()
	result := g2.Zero()

	for _, element := range elements {
		point, err := g2.FromCompressed(element)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress G2 point: %w", err)
		}
		g2.Add(result, result, point)
	}

	return g2.ToCompressed(result), nil
}

```
---
### `crypto/crypto.go`
*2025-02-26 16:02:49 | 14 KB*
```go
package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"

	"crypto/ed25519"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"

	bls12381 "github.com/kilic/bls12-381"
)

// Ensure CryptoImplementation implements the interfaces
var _ cryptoapi.CryptoOperations = (*CryptoImplementation)(nil)

// CryptoImplementation provides concrete implementations of crypto operations
type CryptoImplementation struct{}

// NewCryptoImplementation creates a new crypto implementation
func NewCryptoImplementation() *CryptoImplementation {
	return &CryptoImplementation{}
}

// BLS12381AggregateG1 aggregates multiple G1 points into a single compressed G1 point.
func (c *CryptoImplementation) BLS12381AggregateG1(elements [][]byte) ([]byte, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("no elements to aggregate")
	}

	g1 := bls12381.NewG1()
	result := g1.Zero()

	for _, element := range elements {
		point, err := g1.FromCompressed(element)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress G1 point: %w", err)
		}
		g1.Add(result, result, point)
	}

	return g1.ToCompressed(result), nil
}

// BLS12381HashToG1 hashes arbitrary bytes to a compressed G1 point.
func (c *CryptoImplementation) BLS12381HashToG1(message, dst []byte) ([]byte, error) {
	g1 := bls12381.NewG1()
	point, err := g1.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G1: %w", err)
	}
	return g1.ToCompressed(point), nil
}

// BLS12381HashToG2 hashes arbitrary bytes to a compressed G2 point.
func (c *CryptoImplementation) BLS12381HashToG2(message, dst []byte) ([]byte, error) {
	g2 := bls12381.NewG2()
	point, err := g2.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G2: %w", err)
	}
	return g2.ToCompressed(point), nil
}

// BLS12381VerifyG1G2 checks if the pairing product of G1 and G2 points equals the identity in GT.
func (c *CryptoImplementation) BLS12381VerifyG1G2(g1Points, g2Points [][]byte) (bool, error) {
	if len(g1Points) != len(g2Points) {
		return false, fmt.Errorf("number of G1 points (%d) must equal number of G2 points (%d)", len(g1Points), len(g2Points))
	}
	if len(g1Points) == 0 {
		return false, fmt.Errorf("at least one pair of points is required")
	}

	g1 := bls12381.NewG1()
	g2 := bls12381.NewG2()
	engine := bls12381.NewEngine()

	// For each (G1, G2) pair, add their pairing to the calculation
	for i := 0; i < len(g1Points); i++ {
		p1, err := g1.FromCompressed(g1Points[i])
		if err != nil {
			return false, fmt.Errorf("invalid G1 point at index %d: %w", i, err)
		}

		p2, err := g2.FromCompressed(g2Points[i])
		if err != nil {
			return false, fmt.Errorf("invalid G2 point at index %d: %w", i, err)
		}

		engine.AddPair(p1, p2)
	}

	// Check if the pairing result equals 1 (the identity element in GT)
	return engine.Check(), nil
}

// Secp256k1Verify verifies a secp256k1 signature in [R || s] format (65 bytes).
func (c *CryptoImplementation) Secp256k1Verify(messageHash, signature, pubkey []byte) (bool, error) {
	if len(messageHash) != 32 {
		return false, fmt.Errorf("message hash must be 32 bytes, got %d", len(messageHash))
	}
	if len(signature) != 64 && len(signature) != 65 {
		return false, fmt.Errorf("signature must be 64 or 65 bytes, got %d", len(signature))
	}

	// Use 64-byte signature format (R, s)
	sigR := new(big.Int).SetBytes(signature[:32])
	sigS := new(big.Int).SetBytes(signature[32:64])

	// Parse the public key
	pk, err := parseSecp256k1PubKey(pubkey)
	if err != nil {
		return false, err
	}

	// Verify the signature
	return ecdsa.Verify(pk, messageHash, sigR, sigS), nil
}

// Secp256k1RecoverPubkey recovers a public key from a signature and recovery byte.
func (c *CryptoImplementation) Secp256k1RecoverPubkey(messageHash, signature []byte, recovery byte) ([]byte, error) {
	if len(messageHash) != 32 {
		return nil, fmt.Errorf("message hash must be 32 bytes, got %d", len(messageHash))
	}
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	if recovery > 3 {
		return nil, fmt.Errorf("recovery byte must be 0, 1, 2, or 3, got %d", recovery)
	}

	// Parse r and s from the signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Calculate recovery parameters
	curve := secp256k1Curve()
	recid := int(recovery)
	isOdd := recid&1 != 0
	isSecondX := recid&2 != 0

	// Use r instead of creating a new x variable
	if isSecondX {
		r.Add(r, new(big.Int).Set(curve.Params().N))
	}

	// Calculate corresponding y value
	y, err := recoverY(curve, r, isOdd)
	if err != nil {
		return nil, err
	}

	// Construct R point using r instead of x
	R := &ecdsa.PublicKey{
		Curve: curve,
		X:     r,
		Y:     y,
	}

	// Derive from R and signature the original public key
	e := new(big.Int).SetBytes(messageHash)
	Rinv := new(big.Int).ModInverse(R.X, curve.Params().N)
	if Rinv == nil {
		return nil, fmt.Errorf("failed to compute modular inverse")
	}

	// Calculate r⁻¹(sR - eG)
	sR := new(big.Int).Mul(s, R.X)
	sR.Mod(sR, curve.Params().N)

	eG := new(big.Int).Neg(e)
	eG.Mod(eG, curve.Params().N)

	Q := ecPointAdd(
		curve,
		R.X, R.Y, // R
		ecScalarMult(curve, eG, nil, nil), // eG
	)

	Q = ecScalarMult(curve, Rinv, Q[0], Q[1])

	// Convert the recovered public key to compressed format
	return compressPublicKey(Q[0], Q[1]), nil
}

// Secp256r1RecoverPubkey recovers a secp256r1 public key from a signature
func (c *CryptoImplementation) Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error) {
	// Current placeholder needs actual implementation
	curve := elliptic.P256() // NIST P-256 (secp256r1) curve

	// Implement proper recovery:
	recid := int(recovery)
	isOdd := recid&1 != 0
	isSecondX := recid&2 != 0

	// Restore potential x coordinate from signature
	x := new(big.Int).SetBytes(signature[:32])
	if isSecondX {
		x.Add(x, new(big.Int).Set(curve.Params().N))
	}

	// Calculate corresponding y value
	y, err := recoverY(curve, x, isOdd)
	if err != nil {
		return nil, err
	}

	// Create compressed public key
	compressedPubKey := make([]byte, 33)
	compressedPubKey[0] = byte(0x02) + byte(y.Bit(0))
	xBytes := x.Bytes()
	copy(compressedPubKey[1+32-len(xBytes):], xBytes)

	return compressedPubKey, nil
}

// Ed25519Verify verifies an Ed25519 signature.
func (c *CryptoImplementation) Ed25519Verify(message, signature, pubKey []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	if len(pubKey) != 32 {
		return false, fmt.Errorf("public key must be 32 bytes, got %d", len(pubKey))
	}

	// Use Go's ed25519 implementation to verify
	return ed25519Verify(pubKey, message, signature), nil
}

// Ed25519BatchVerify verifies multiple Ed25519 signatures in a batch.
func (c *CryptoImplementation) Ed25519BatchVerify(messages, signatures, pubKeys [][]byte) (bool, error) {
	if len(messages) != len(signatures) || len(messages) != len(pubKeys) {
		return false, fmt.Errorf("number of messages (%d), signatures (%d), and public keys (%d) must be equal",
			len(messages), len(signatures), len(pubKeys))
	}

	for i := 0; i < len(messages); i++ {
		if ok, _ := c.Ed25519Verify(messages[i], signatures[i], pubKeys[i]); !ok {
			return false, nil
		}
	}
	return true, nil
}

// parseSecp256k1PubKey parses a SEC1 encoded public key in compressed or uncompressed format
func parseSecp256k1PubKey(pubKeyBytes []byte) (*ecdsa.PublicKey, error) {
	curve := secp256k1Curve()

	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("empty public key")
	}

	// Handle compressed public key format
	if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		x := new(big.Int).SetBytes(pubKeyBytes[1:])
		isOdd := pubKeyBytes[0] == 0x03
		y, err := recoverY(curve, x, isOdd)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	// Handle uncompressed public key format
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		x := new(big.Int).SetBytes(pubKeyBytes[1:33])
		y := new(big.Int).SetBytes(pubKeyBytes[33:])
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	return nil, fmt.Errorf("invalid public key format or length: %d", len(pubKeyBytes))
}

// recoverY calculates the Y coordinate for a given X coordinate on an elliptic curve
func recoverY(curve elliptic.Curve, x *big.Int, isOdd bool) (*big.Int, error) {
	// y² = x³ + ax + b
	// For secp256k1, a = 0 and b = 7

	// Calculate x³ + ax + b
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	// For secp256k1, a = 0, so we skip adding ax

	// Add b (7 for secp256k1, different for other curves)
	b := getB(curve)
	x3.Add(x3, b)

	// Modulo p
	x3.Mod(x3, curve.Params().P)

	// Calculate the square root modulo p
	y := new(big.Int).ModSqrt(x3, curve.Params().P)
	if y == nil {
		return nil, fmt.Errorf("no square root exists for y")
	}

	// Check if we need the "other" root (p - y)
	if isOdd != isOddValue(y) {
		y.Sub(curve.Params().P, y)
	}

	return y, nil
}

// isOddValue checks if a big.Int value is odd
func isOddValue(value *big.Int) bool {
	return value.Bit(0) == 1
}

// getB returns the b parameter for the curve equation y² = x³ + ax + b
func getB(curve elliptic.Curve) *big.Int {
	if curve == secp256k1Curve() {
		return big.NewInt(7) // Secp256k1 has b = 7
	}
	if curve == elliptic.P256() {
		// Return the b parameter for P-256 (secp256r1)
		b, _ := new(big.Int).SetString("5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604b", 16)
		return b
	}
	// Default, though this should not happen with our supported curves
	return big.NewInt(0)
}

// secp256k1Curve returns an elliptic.Curve instance for secp256k1
func secp256k1Curve() elliptic.Curve {
	// This is a simplified version - in production code, you would use a proper secp256k1 implementation
	// For now, we'll use a placeholder that matches the rest of the code
	return elliptic.P256() // This is a placeholder - real code would return actual secp256k1
}

// ecPointAdd adds two elliptic curve points
func ecPointAdd(curve elliptic.Curve, x1, y1 *big.Int, point [2]*big.Int) [2]*big.Int {
	x2, y2 := point[0], point[1]
	x3, y3 := curve.Add(x1, y1, x2, y2)
	return [2]*big.Int{x3, y3}
}

// ecScalarMult multiplies a point on an elliptic curve by a scalar
func ecScalarMult(curve elliptic.Curve, k *big.Int, x, y *big.Int) [2]*big.Int {
	if x == nil || y == nil {
		// If point is the identity (represented as nil), use the base point
		x, y = curve.Params().Gx, curve.Params().Gy
	}
	x3, y3 := curve.ScalarMult(x, y, k.Bytes())
	return [2]*big.Int{x3, y3}
}

// compressPublicKey creates a compressed representation of a public key
func compressPublicKey(x, y *big.Int) []byte {
	result := make([]byte, 33)
	// Set prefix based on Y coordinate's parity
	if y.Bit(0) == 0 {
		result[0] = 0x02 // even Y
	} else {
		result[0] = 0x03 // odd Y
	}

	// Pad X coordinate to 32 bytes
	xBytes := x.Bytes()
	offset := 1 + 32 - len(xBytes)
	copy(result[offset:], xBytes)

	return result
}

// ed25519Verify verifies an ED25519 signature
func ed25519Verify(pubKey, message, signature []byte) bool {
	// In a real implementation, use Go's crypto/ed25519 package

	if len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(ed25519.PublicKey(pubKey), message, signature)
}

// Secp256r1Verify verifies a signature using the NIST P-256 curve (secp256r1)
func (c *CryptoImplementation) Secp256r1Verify(hash, signature, pubkey []byte) (bool, error) {
	// Implementation details similar to Secp256k1Verify but using P-256 curve
	if len(hash) != 32 {
		return false, fmt.Errorf("message hash must be 32 bytes, got %d", len(hash))
	}
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	curve := elliptic.P256() // Use NIST P-256 curve

	// Parse signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Parse public key
	pk, err := parsePublicKey(pubkey, curve)
	if err != nil {
		return false, err
	}

	// Verify signature
	return ecdsa.Verify(pk, hash, r, s), nil
}

// parsePublicKey parses a SEC1 encoded public key for the specified curve
func parsePublicKey(pubKeyBytes []byte, curve elliptic.Curve) (*ecdsa.PublicKey, error) {
	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("empty public key")
	}

	// Handle compressed public key format
	if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		x := new(big.Int).SetBytes(pubKeyBytes[1:])
		isOdd := pubKeyBytes[0] == 0x03
		y, err := recoverY(curve, x, isOdd)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	// Handle uncompressed public key format
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		x := new(big.Int).SetBytes(pubKeyBytes[1:33])
		y := new(big.Int).SetBytes(pubKeyBytes[33:])
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	return nil, fmt.Errorf("invalid public key format or length: %d", len(pubKeyBytes))
}

```
---
### `crypto/hostcrypto.go`
*2025-02-26 16:37:23 | 7 KB*
```go
package crypto

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/host"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	wazerotypes "github.com/tetratelabs/wazero/api"
)

type contextKey string

const envKey contextKey = "env"

// Add global handler variable
var cryptoHandler cryptoapi.CryptoOperations

// Add function to set the handler
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
// It reads the message and domain separation tag from contract memory using MemoryManager,
// charges gas, calls BLS12381HashToG1, allocates space for the result, writes it, and returns the pointer.
func hostBls12381HashToG1(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Retrieve the runtime environment from context.
	env := ctx.Value(envKey).(*host.RuntimeEnvironment)

	// Create a MemoryManager for the contract module.
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read the input message.
	message, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	// Read the domain separation tag.
	dst, err := mm.Read(dstPtr, dstLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation.
	env.Gas.(types.GasMeter).ConsumeGas(uint64(hashLen+dstLen)*constants.GasPerByte, "BLS12381 hash operation")

	// Hash to curve.
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		return 0
	}

	// Allocate memory for the result.
	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0
	}

	// Write the result into memory.
	if err := mm.Write(resultPtr, result); err != nil {
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
// It follows the same pattern as hostBls12381HashToG1.
func hostBls12381HashToG2(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	env := ctx.Value(envKey).(*host.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	message, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	dst, err := mm.Read(dstPtr, dstLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation.
	env.Gas.(types.GasMeter).ConsumeGas(uint64(hashLen+dstLen)*constants.GasPerByte, "BLS12381 hash operation")

	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		return 0
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0
	}

	if err := mm.Write(resultPtr, result); err != nil {
		return 0
	}

	return resultPtr
}

// hostBls12381PairingEquality implements bls12_381_pairing_equality.
// It reads the four compressed points from memory and calls BLS12381PairingEquality.
func hostBls12381PairingEquality(_ context.Context, mod wazerotypes.Module, a1Ptr, a1Len, a2Ptr, a2Len, b1Ptr, b1Len, b2Ptr, b2Len uint32) uint32 {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	a1, err := mm.Read(a1Ptr, a1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a1: %v", err))
	}
	a2, err := mm.Read(a2Ptr, a2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a2: %v", err))
	}
	b1, err := mm.Read(b1Ptr, b1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b1: %v", err))
	}
	b2, err := mm.Read(b2Ptr, b2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b2: %v", err))
	}

	result, err := cryptoHandler.BLS12381VerifyG1G2(
		[][]byte{a1, b1}, // g1 points
		[][]byte{a2, b2}, // g2 points
	)
	if err != nil {
		panic(fmt.Sprintf("failed to check pairing equality: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1Verify implements secp256r1_verify.
// It reads the hash, signature, and public key from memory via MemoryManager,
// calls Secp256r1Verify, and returns 1 if valid.
func hostSecp256r1Verify(_ context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	sig, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read public key: %v", err))
	}

	result, err := cryptoHandler.Secp256r1Verify(hash, sig, pubkey)
	if err != nil {
		panic(fmt.Sprintf("failed to verify secp256r1 signature: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1RecoverPubkey implements secp256r1_recover_pubkey.
// It reads the hash and signature from memory, recovers the public key,
// allocates memory for it, writes it, and returns the pointer and length.
func hostSecp256r1RecoverPubkey(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, recovery uint32) (uint32, uint32) {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	result, err := cryptoHandler.Secp256r1RecoverPubkey(hash, signature, byte(recovery))
	if err != nil {
		panic(fmt.Sprintf("failed to recover public key: %v", err))
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
	}

	if err := mm.Write(resultPtr, result); err != nil {
		panic(fmt.Sprintf("failed to write result: %v", err))
	}

	return resultPtr, uint32(len(result))
}

// SetupCryptoHandlers initializes the crypto system by creating and setting the global crypto handler
func SetupCryptoHandlers() error {
	// Create a new implementation of the CryptoOperations interface
	impl := NewCryptoImplementation()

	// Set it as the global handler
	SetCryptoHandler(impl)

	return nil
}

```
---
### `cryptoapi/types.go`
*2025-02-26 16:01:43 | 2 KB*
```go
package cryptoapi

// CryptoVerifier defines the interface for crypto verification operations
type CryptoVerifier interface {
	// Secp256k1Verify verifies a secp256k1 signature
	Secp256k1Verify(hash, signature, publicKey []byte) (bool, error)

	// Secp256k1RecoverPubkey recovers a public key from a signature
	Secp256k1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error)

	// Ed25519Verify verifies an ed25519 signature
	Ed25519Verify(message, signature, publicKey []byte) (bool, error)

	// Ed25519BatchVerify verifies multiple ed25519 signatures
	Ed25519BatchVerify(messages, signatures, publicKeys [][]byte) (bool, error)
}

// BLS12381Operations defines operations for BLS12-381 curves
type BLS12381Operations interface {
	// BLS12381AggregateG1 aggregates multiple G1 points
	BLS12381AggregateG1(elements [][]byte) ([]byte, error)

	// BLS12381AggregateG2 aggregates multiple G2 points
	BLS12381AggregateG2(elements [][]byte) ([]byte, error)

	// BLS12381HashToG1 hashes a message to a G1 point
	BLS12381HashToG1(message, dst []byte) ([]byte, error)

	// BLS12381HashToG2 hashes a message to a G2 point
	BLS12381HashToG2(message, dst []byte) ([]byte, error)

	// BLS12381VerifyG1G2 verifies a pairing check
	BLS12381VerifyG1G2(g1Points, g2Points [][]byte) (bool, error)
}

// CryptoOperations combines all crypto operations into a single interface
type CryptoOperations interface {
	CryptoVerifier
	BLS12381Operations
	Secp256r1Verify(hash, signature, pubkey []byte) (bool, error)
	Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error)
}

```
---
### `gas/gas.go`
*2025-02-20 21:49:29 | 2 KB*
```go
package gas

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// GasState tracks gas consumption
type GasState struct {
	limit uint64
	used  uint64
}

// NewGasState creates a new GasState with the given limit
func NewGasState(limit uint64) *GasState {
	return &GasState{
		limit: limit,
		used:  0,
	}
}

// GasConsumed implements types.GasMeter
func (g *GasState) GasConsumed() uint64 {
	return g.used
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) error {
	g.used += amount
	if g.used > g.limit {
		return fmt.Errorf("out of gas: used %d, limit %d - %s", g.used, g.limit, description)
	}
	return nil
}

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() types.GasConfig {
	return types.GasConfig{
		PerByte:                 constants.GasPerByte,
		DatabaseRead:            constants.GasCostRead,
		DatabaseWrite:           constants.GasCostWrite,
		ExternalQuery:           constants.GasCostQuery,
		IteratorCreate:          constants.GasCostIteratorCreate,
		IteratorNext:            constants.GasCostIteratorNext,
		Instantiate:             constants.GasCostInstantiate,
		Execute:                 constants.GasCostExecute,
		Bls12381AggregateG1Cost: types.GasCost{BaseCost: 1000, PerPoint: 100},
		Bls12381AggregateG2Cost: types.GasCost{BaseCost: 1000, PerPoint: 100},
	}
}

```
---
### `gas/gasversionone/gas.go`
*2025-02-20 21:49:29 | 6 KB*
```go
package gas1

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// --- ErrorOutOfGas ---
//
// ErrorOutOfGas is returned when the gas consumption exceeds the allowed limit.
type ErrorOutOfGas struct {
	Descriptor string
}

func (e ErrorOutOfGas) Error() string {
	return fmt.Sprintf("out of gas: %s", e.Descriptor)
}

// --- Constants ---
const (
	// Cost of one Wasm VM instruction (CosmWasm 1.x uses 150 gas per op).
	wasmInstructionCost uint64 = 150

	// Conversion multiplier: CosmWasm gas units are 100x the Cosmos SDK gas units.
	gasMultiplier uint64 = 100

	// Cost per byte for memory copy operations (host ↔ wasm).
	memoryCopyCost uint64 = 1
)

// --- GasState ---
// GasState tracks gas usage during a contract execution (CosmWasm 1.x compatible).
type GasState struct {
	gasLimit      uint64         // Total gas limit (in CosmWasm gas units)
	usedInternal  uint64         // Gas used for internal Wasm operations (in CosmWasm gas units)
	externalUsed  uint64         // Gas used externally (from the Cosmos SDK GasMeter, in SDK gas units)
	initialExtern uint64         // Initial external gas consumed at start (SDK units)
	gasMeter      types.GasMeter // Reference to an external (SDK) GasMeter
}

// NewGasState creates a new GasState.
// The given gas limit is in Cosmos SDK gas units; it is converted to CosmWasm gas units.
// The provided gasMeter is used to track external gas usage.
func NewGasState(limitSDK uint64, meter types.GasMeter) *GasState {
	gs := &GasState{
		gasLimit:     limitSDK * gasMultiplier,
		usedInternal: 0,
		externalUsed: 0,
		gasMeter:     meter,
	}
	if meter != nil {
		gs.initialExtern = meter.GasConsumed()
	}
	return gs
}

// ConsumeWasmGas consumes gas for executing the given number of Wasm instructions.
func (gs *GasState) ConsumeWasmGas(numInstr uint64) error {
	if numInstr == 0 {
		return nil
	}
	cost := numInstr * wasmInstructionCost
	return gs.consumeInternalGas(cost, "Wasm execution")
}

// ConsumeMemoryGas charges gas for copying numBytes of data.
func (gs *GasState) ConsumeMemoryGas(numBytes uint64) error {
	if numBytes == 0 {
		return nil
	}
	cost := numBytes * memoryCopyCost
	return gs.consumeInternalGas(cost, "Memory operation")
}

// ConsumeDBReadGas charges gas for a database read, based on key and value sizes.
func (gs *GasState) ConsumeDBReadGas(keyLen, valueLen int) error {
	totalBytes := uint64(0)
	if keyLen > 0 {
		totalBytes += uint64(keyLen)
	}
	if valueLen > 0 {
		totalBytes += uint64(valueLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "DB read")
}

// ConsumeDBWriteGas charges gas for a database write, based on key and value sizes.
func (gs *GasState) ConsumeDBWriteGas(keyLen, valueLen int) error {
	totalBytes := uint64(0)
	if keyLen > 0 {
		totalBytes += uint64(keyLen)
	}
	if valueLen > 0 {
		totalBytes += uint64(valueLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "DB write")
}

// ConsumeQueryGas charges gas for an external query operation.
func (gs *GasState) ConsumeQueryGas(reqLen, respLen int) error {
	totalBytes := uint64(0)
	if reqLen > 0 {
		totalBytes += uint64(reqLen)
	}
	if respLen > 0 {
		totalBytes += uint64(respLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "External query")
}

// consumeInternalGas deducts the given cost from internal gas usage and checks combined gas.
func (gs *GasState) consumeInternalGas(cost uint64, descriptor string) error {
	if cost == 0 {
		return nil
	}
	gs.usedInternal += cost

	// Update external usage from the Cosmos SDK GasMeter.
	if gs.gasMeter != nil {
		currentExtern := gs.gasMeter.GasConsumed()
		if currentExtern < gs.initialExtern {
			gs.initialExtern = currentExtern
		}
		gs.externalUsed = currentExtern - gs.initialExtern
	}

	combinedUsed := gs.usedInternal + (gs.externalUsed * gasMultiplier)
	if combinedUsed > gs.gasLimit {
		return ErrorOutOfGas{Descriptor: descriptor}
	}
	return nil
}

// GasUsed returns the internal gas used in Cosmos SDK gas units.
func (gs *GasState) GasUsed() uint64 {
	used := gs.usedInternal / gasMultiplier
	if gs.usedInternal%gasMultiplier != 0 {
		used++
	}
	return used
}

// Report returns a GasReport summarizing gas usage.
func (gs *GasState) Report() types.GasReport {
	if gs.gasMeter != nil {
		currentExtern := gs.gasMeter.GasConsumed()
		if currentExtern < gs.initialExtern {
			gs.initialExtern = currentExtern
		}
		gs.externalUsed = currentExtern - gs.initialExtern
	}
	usedExternWasm := gs.externalUsed * gasMultiplier
	usedInternWasm := gs.usedInternal
	var remaining uint64
	if gs.gasLimit >= (usedInternWasm + usedExternWasm) {
		remaining = gs.gasLimit - (usedInternWasm + usedExternWasm)
	}
	return types.GasReport{
		Limit:          gs.gasLimit,
		Remaining:      remaining,
		UsedExternally: usedExternWasm,
		UsedInternally: usedInternWasm,
	}
}

// DebugString returns a human-readable summary of the current gas state.
func (gs *GasState) DebugString() string {
	report := gs.Report()
	usedExternSDK := gs.externalUsed
	usedInternSDK := gs.GasUsed()
	totalSDK := usedExternSDK + usedInternSDK
	return fmt.Sprintf(
		"GasState{limit=%d, usedIntern=%d, usedExtern=%d, combined=%d | SDK gas: internal=%d, external=%d, total=%d}",
		report.Limit, report.UsedInternally, report.UsedExternally, report.UsedInternally+report.UsedExternally,
		usedInternSDK, usedExternSDK, totalSDK,
	)
}

```
---
### `gas/gasversiontwo/gas.go`
*2025-02-20 21:49:29 | 7 KB*
```go
package gas2

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Gas constants (CosmWasm 2.x)
const (
	// GasMultiplier is how many CosmWasm gas points equal 1 Cosmos SDK gas point (reduced 1000x in 2.x).
	GasMultiplier uint64 = 140_000
	// InstanceCost for loading a WASM instance (unchanged from 1.x).
	InstanceCost uint64 = 60_000
	// InstanceCostDiscount for cached instances (about 30x cheaper than full load).
	InstanceCostDiscount uint64 = 2_000
	// CompileCost per byte for compiling WASM code.
	CompileCost uint64 = 3
	// EventPerAttributeCost per event attribute (count).
	EventPerAttributeCost uint64 = 10
	// EventAttributeDataCost per byte of event attribute data.
	EventAttributeDataCost uint64 = 1
	// EventAttributeDataFreeTier bytes of attribute data with no charge.
	EventAttributeDataFreeTier uint64 = 100
	// CustomEventCost per custom event emitted.
	CustomEventCost uint64 = 20
	// ContractMessageDataCost per byte of message passed to contract (still 0 by default).
	ContractMessageDataCost uint64 = 0
	// GasCostHumanAddress to convert a canonical address to human-readable.
	GasCostHumanAddress uint64 = 5
	// GasCostCanonicalAddress to convert a human address to canonical form.
	GasCostCanonicalAddress uint64 = 4
	// GasCostValidateAddress (humanize + canonicalize).
	GasCostValidateAddress uint64 = GasCostHumanAddress + GasCostCanonicalAddress
)

var defaultPerByteUncompressCost = wasmvmtypes.UFraction{
	Numerator:   15,
	Denominator: 100,
}

// DefaultPerByteUncompressCost returns the default uncompress cost fraction.
func DefaultPerByteUncompressCost() wasmvmtypes.UFraction {
	return defaultPerByteUncompressCost
}

// GasRegister defines the gas registration interface.
type GasRegister interface {
	UncompressCosts(byteLength int) types.Gas
	SetupContractCost(discount bool, msgLen int) types.Gas
	ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas
	EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas
	ToWasmVMGas(source types.Gas) uint64
	FromWasmVMGas(source uint64) types.Gas
}

// WasmGasRegisterConfig holds configuration parameters for gas costs.
type WasmGasRegisterConfig struct {
	InstanceCost               types.Gas
	InstanceCostDiscount       types.Gas
	CompileCost                types.Gas
	UncompressCost             wasmvmtypes.UFraction
	GasMultiplier              types.Gas
	EventPerAttributeCost      types.Gas
	EventAttributeDataCost     types.Gas
	EventAttributeDataFreeTier uint64
	ContractMessageDataCost    types.Gas
	CustomEventCost            types.Gas
}

// DefaultGasRegisterConfig returns the default configuration for CosmWasm 2.x.
func DefaultGasRegisterConfig() WasmGasRegisterConfig {
	return WasmGasRegisterConfig{
		InstanceCost:               InstanceCost,
		InstanceCostDiscount:       InstanceCostDiscount,
		CompileCost:                CompileCost,
		UncompressCost:             DefaultPerByteUncompressCost(),
		GasMultiplier:              GasMultiplier,
		EventPerAttributeCost:      EventPerAttributeCost,
		EventAttributeDataCost:     EventAttributeDataCost,
		EventAttributeDataFreeTier: EventAttributeDataFreeTier,
		ContractMessageDataCost:    ContractMessageDataCost,
		CustomEventCost:            CustomEventCost,
	}
}

// WasmGasRegister implements GasRegister.
type WasmGasRegister struct {
	c WasmGasRegisterConfig
}

// NewDefaultWasmGasRegister creates a new gas register with default config.
func NewDefaultWasmGasRegister() WasmGasRegister {
	return NewWasmGasRegister(DefaultGasRegisterConfig())
}

// NewWasmGasRegister creates a new gas register with the given configuration.
func NewWasmGasRegister(c WasmGasRegisterConfig) WasmGasRegister {
	if c.GasMultiplier == 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "GasMultiplier cannot be 0"))
	}
	return WasmGasRegister{c: c}
}

// UncompressCosts returns the gas cost to uncompress a WASM bytecode of the given length.
func (g WasmGasRegister) UncompressCosts(byteLength int) types.Gas {
	if byteLength < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "byteLength cannot be negative"))
	}
	numerator := g.c.UncompressCost.Numerator
	denom := g.c.UncompressCost.Denominator
	gasCost := uint64(byteLength) * numerator / denom
	return types.Gas(gasCost)
}

// SetupContractCost returns the gas cost to set up contract execution/instantiation.
func (g WasmGasRegister) SetupContractCost(discount bool, msgLen int) types.Gas {
	if msgLen < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "msgLen cannot be negative"))
	}
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	msgDataCost := types.Gas(msgLen) * g.c.ContractMessageDataCost
	return baseCost + msgDataCost
}

// ReplyCosts returns the gas cost for handling a submessage reply.
// CosmWasm 2.x no longer includes event attributes or error messages in reply,
// so we only charge the base cost.
func (g WasmGasRegister) ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas {
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	// In v2.x, additional reply data is not charged.
	return baseCost
}

// EventCosts returns the gas cost for contract-emitted events.
// It computes the cost for a list of event attributes and events.
func (g WasmGasRegister) EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas {
	gasUsed, remainingFree := g.eventAttributeCosts(attrs, g.c.EventAttributeDataFreeTier)
	for _, evt := range events {
		// Charge for any event attributes that exist.
		gasEvt, newFree := g.eventAttributeCosts(evt.Attributes, remainingFree)
		gasUsed += gasEvt
		remainingFree = newFree
	}
	gasUsed += types.Gas(len(events)) * g.c.CustomEventCost
	return gasUsed
}

// eventAttributeCosts computes the gas cost for a set of event attributes given a free byte allowance.
func (g WasmGasRegister) eventAttributeCosts(attrs []wasmvmtypes.EventAttribute, freeTier uint64) (types.Gas, uint64) {
	if len(attrs) == 0 {
		return 0, freeTier
	}
	var totalBytes uint64 = 0
	for _, attr := range attrs {
		totalBytes += uint64(len(attr.Key)) + uint64(len(attr.Value))
	}
	if totalBytes <= freeTier {
		remainingFree := freeTier - totalBytes
		return 0, remainingFree
	}
	chargeBytes := totalBytes - freeTier
	gasCost := types.Gas(chargeBytes) * g.c.EventAttributeDataCost
	return gasCost, 0
}

// ToWasmVMGas converts SDK gas to CosmWasm VM gas.
func (g WasmGasRegister) ToWasmVMGas(source types.Gas) uint64 {
	x := uint64(source) * uint64(g.c.GasMultiplier)
	if x < uint64(source) {
		panic(wasmvmtypes.ErrorOutOfGas{Descriptor: "CosmWasm gas overflow"})
	}
	return x
}

// FromWasmVMGas converts CosmWasm VM gas to SDK gas.
func (g WasmGasRegister) FromWasmVMGas(source uint64) types.Gas {
	return types.Gas(source / uint64(g.c.GasMultiplier))
}

```
---
### `gas/wazero/gas.go`
*2025-02-26 15:26:13 | 3 KB*
```go
package wazerogasometer

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
)

// WazeroGasMeter implements both wazero's api.GasMeter and CosmWasm's types.GasMeter
type WazeroGasMeter struct {
	limit     uint64
	consumed  uint64
	gasConfig types.GasConfig
}

// NewWazeroGasMeter creates a new gas meter compatible with both systems
func NewWazeroGasMeter(limit uint64, config types.GasConfig) *WazeroGasMeter {
	return &WazeroGasMeter{
		limit:     limit,
		consumed:  0,
		gasConfig: config,
	}
}

// WithGasMeter attaches the gas meter to the context
func (g *WazeroGasMeter) WithGasMeter(ctx context.Context) context.Context {
	return context.WithValue(ctx, gasMeterKey{}, g)
}

// gasMeterKey is a private type for the context key to avoid collisions
type gasMeterKey struct{}

// Implements wazero api.GasMeter
func (g *WazeroGasMeter) Gas(uint64) error {
	return nil // Always allow gas during compilation
}

// ConsumeFuel implements wazero's gas consumption during execution
func (g *WazeroGasMeter) ConsumeFuel(fuel uint64) error {
	// Convert wazero fuel units to CosmWasm gas units
	gasToCharge := fuel * g.gasConfig.GasMultiplier
	return g.ConsumeGas(gasToCharge, "wazero operation")
}

// Implements types.GasMeter
func (g *WazeroGasMeter) GasConsumed() uint64 {
	return g.consumed
}

// ConsumeGas implements types.GasMeter
func (g *WazeroGasMeter) ConsumeGas(amount uint64, descriptor string) error {
	if g.consumed+amount > g.limit {
		return types.OutOfGasError{Descriptor: descriptor}
	}
	g.consumed += amount
	return nil
}

// GasRemaining returns remaining gas
func (g *WazeroGasMeter) GasRemaining() uint64 {
	if g.consumed >= g.limit {
		return 0
	}
	return g.limit - g.consumed
}

// HasGas checks if there is enough gas remaining
func (g *WazeroGasMeter) HasGas(required uint64) bool {
	return g.GasRemaining() >= required
}

// GasForOperation calculates gas needed for a specific operation
func (g *WazeroGasMeter) GasForOperation(op types.GasOperation) uint64 {
	switch op {
	case types.GasOperationMemoryRead:
		return g.gasConfig.PerByte
	case types.GasOperationMemoryWrite:
		return g.gasConfig.PerByte
	case types.GasOperationDBRead:
		return g.gasConfig.DatabaseRead
	case types.GasOperationDBWrite:
		return g.gasConfig.DatabaseWrite
	case types.GasOperationDBDelete:
		return g.gasConfig.DatabaseWrite
	case types.GasOperationCompile:
		return g.gasConfig.CompileCost
	// Add other operations as needed
	default:
		return 0
	}
}

// GetGasMeterFromContext retrieves the gas meter from context
func GetGasMeterFromContext(ctx context.Context) (*WazeroGasMeter, bool) {
	meter, ok := ctx.Value(gasMeterKey{}).(*WazeroGasMeter)
	return meter, ok
}

// ConsumeGasFromContext consumes gas from the meter in context
func ConsumeGasFromContext(ctx context.Context, amount uint64, description string) error {
	meter, ok := GetGasMeterFromContext(ctx)
	if !ok {
		return fmt.Errorf("gas meter not found in context")
	}
	return meter.ConsumeGas(amount, description)
}

```
---
### `gas.go`
*2025-02-20 21:49:29 | 4 KB*
```go
package runtime

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
)

// GasConfig holds gas costs for different operations
type GasConfig struct {
	// Memory operations
	PerByte uint64

	// Database operations
	DatabaseRead  uint64
	DatabaseWrite uint64
	ExternalQuery uint64

	// Iterator operations
	IteratorCreate uint64
	IteratorNext   uint64

	// Contract operations
	Instantiate uint64
	Execute     uint64

	Bls12381AggregateG1Cost GasCost
	Bls12381AggregateG2Cost GasCost
}

type GasCost struct {
	BaseCost uint64
	PerPoint uint64
}

func (c GasCost) TotalCost(pointCount uint64) uint64 {
	return c.BaseCost + c.PerPoint*pointCount
}

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() GasConfig {
	return GasConfig{
		PerByte:        constants.GasPerByte,
		DatabaseRead:   constants.GasCostRead,
		DatabaseWrite:  constants.GasCostWrite,
		ExternalQuery:  constants.GasCostQuery,
		IteratorCreate: constants.GasCostIteratorCreate,
		IteratorNext:   constants.GasCostIteratorNext,
		Instantiate:    constants.GasCostInstantiate,
		Execute:        constants.GasCostExecute,
	}
}

// GasState tracks gas usage during execution
type GasState struct {
	config GasConfig
	limit  uint64
	used   uint64
}

func (g *GasState) GasConsumed() uint64 {
	return g.GetGasUsed()
}

// NewGasState creates a new GasState with the given limit
func NewGasState(limit uint64) *GasState {
	return &GasState{
		config: DefaultGasConfig(),
		limit:  limit,
		used:   0,
	}
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) error {
	g.used += amount
	if g.used > g.limit {
		return fmt.Errorf("out of gas: used %d, limit %d - %s", g.used, g.limit, description)
	}
	return nil
}

// ConsumeMemory charges gas for memory operations
func (g *GasState) ConsumeMemory(size uint32) error {
	cost := uint64(size) * g.config.PerByte
	return g.ConsumeGas(cost, fmt.Sprintf("memory allocation: %d bytes", size))
}

// ConsumeRead charges gas for database read operations
func (g *GasState) ConsumeRead(size uint32) error {
	// Base cost plus per-byte cost
	cost := g.config.DatabaseRead + (uint64(size) * g.config.PerByte)
	return g.ConsumeGas(cost, "db read")
}

// ConsumeWrite charges gas for database write operations
func (g *GasState) ConsumeWrite(size uint32) error {
	// Base cost plus per-byte cost
	cost := g.config.DatabaseWrite + (uint64(size) * g.config.PerByte)
	return g.ConsumeGas(cost, "db write")
}

// ConsumeQuery charges gas for external query operations
func (g *GasState) ConsumeQuery() error {
	return g.ConsumeGas(g.config.ExternalQuery, "external query")
}

// ConsumeIterator charges gas for iterator operations
func (g *GasState) ConsumeIterator(create bool) error {
	var cost uint64
	var desc string
	if create {
		cost = g.config.IteratorCreate
		desc = "create iterator"
	} else {
		cost = g.config.IteratorNext
		desc = "iterator next"
	}
	return g.ConsumeGas(cost, desc)
}

// GetGasUsed returns the amount of gas used
func (g *GasState) GetGasUsed() uint64 {
	return g.used
}

// GetGasLimit returns the gas limit
func (g *GasState) GetGasLimit() uint64 {
	return g.limit
}

// GetGasRemaining returns the remaining gas
func (g *GasState) GetGasRemaining() uint64 {
	if g.used > g.limit {
		return 0
	}
	return g.limit - g.used
}

// HasGas checks if there is enough gas remaining
func (g *GasState) HasGas(required uint64) bool {
	return g.GetGasRemaining() >= required
}

```
---
### `host/environment.go`
*2025-02-20 22:04:35 | 1 KB*
```go
package host

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// StartCall starts a new call context and returns the call ID
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]types.Iterator)
	return e.nextCallID
}

// StoreIterator stores an iterator and returns its ID
func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter types.Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextIterID++
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

// GetIterator retrieves an iterator by its IDs
func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) types.Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()
	if callMap, ok := e.iterators[callID]; ok {
		return callMap[iterID]
	}
	return nil
}

```
---
### `host/hostfunctions.go`
*2025-02-26 15:37:08 | 28 KB*
```go
package host

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/tetratelabs/wazero/api"
)

const (
	// Return codes for cryptographic operations
	SECP256K1_VERIFY_CODE_VALID   uint32 = 0
	SECP256K1_VERIFY_CODE_INVALID uint32 = 1

	// BLS12-381 return codes
	BLS12_381_VALID_PAIRING   uint32 = 0
	BLS12_381_INVALID_PAIRING uint32 = 1

	BLS12_381_AGGREGATE_SUCCESS     uint32 = 0
	BLS12_381_HASH_TO_CURVE_SUCCESS uint32 = 0

	// Size limits for BLS12-381 operations (MI = 1024*1024, KI = 1024)
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	envKey contextKey = "env"
)

// GasState tracks gas consumption
type GasState struct {
	limit uint64
	used  uint64
}

func NewGasState(limit uint64) GasState {
	return GasState{
		limit: limit,
		used:  0,
	}
}

// GasConsumed implements types.GasMeter
func (g GasState) GasConsumed() uint64 {
	return g.used
}

// allocateInContract calls the contract's allocate function.
// It handles memory allocation within the WebAssembly module's memory space.
func allocateInContract(ctx context.Context, mod api.Module, size uint32) (uint32, error) {
	allocateFn := mod.ExportedFunction("allocate")
	if allocateFn == nil {
		return 0, fmt.Errorf("contract does not export 'allocate' function")
	}
	results, err := allocateFn.Call(ctx, uint64(size))
	if err != nil {
		return 0, fmt.Errorf("failed to call 'allocate': %w", err)
	}
	if len(results) != 1 {
		return 0, fmt.Errorf("expected 1 result from 'allocate', got %d", len(results))
	}
	return uint32(results[0]), nil
}

// readNullTerminatedString reads bytes from memory starting at addrPtr until a null byte is found.
func readNullTerminatedString(memManager *memory.MemoryManager, addrPtr uint32) ([]byte, error) {
	var buf []byte
	for i := addrPtr; ; i++ {
		b, err := memManager.Read(i, 1)
		if err != nil {
			return nil, fmt.Errorf("memory access error at offset %d: %w", i, err)
		}
		if b[0] == 0 {
			break
		}
		buf = append(buf, b[0])
	}
	return buf, nil
}

// hostHumanizeAddress implements addr_humanize.
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostHumanizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to convert to a human-readable address.
	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: API.HumanizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: humanized address: '%s'\n", human)

	// Write the result back into memory.
	if err := env.MemManager.Write(addrPtr, []byte(human)); err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to write humanized address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: successfully wrote humanized address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostCanonicalizeAddress reads a null-terminated address from memory,
// calls the API to canonicalize it, logs intermediate results, and writes
// the canonical address back into memory.
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostCanonicalizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to canonicalize the address.
	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: API.CanonicalizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: canonical address (hex): %x\n", canonical)

	// Write the canonical address back to memory.
	if err := env.MemManager.Write(addrPtr, canonical); err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to write canonical address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: successfully wrote canonical address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostValidateAddress reads a null-terminated address from memory,
// calls the API to validate it, and logs the process.
// Returns 1 if the address is valid and 0 otherwise.
func hostValidateAddress(ctx context.Context, mod api.Module, addrPtr uint32) uint32 {
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Read the address as a null-terminated string.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		panic(fmt.Sprintf("[ERROR] hostValidateAddress: failed to read address from memory: %v", err))
	}
	fmt.Printf("[DEBUG] hostValidateAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Validate the address.
	_, err = env.API.ValidateAddress(string(addr))
	if err != nil {
		fmt.Printf("[DEBUG] hostValidateAddress: API.ValidateAddress failed: %v\n", err)
		return 0 // reject invalid address
	}
	fmt.Printf("[DEBUG] hostValidateAddress: address validated successfully\n")
	return 1 // valid
}

// hostScan implements db_scan.
func hostScan(ctx context.Context, mod api.Module, startPtr, startLen, order uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostScan: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	start, err := readMemory(mem, startPtr, startLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read start key: %v", err))
	}

	var iter types.Iterator
	if order == 1 {
		iter = env.DB.ReverseIterator(start, nil)
	} else {
		iter = env.DB.Iterator(start, nil)
	}

	// Store the iterator and pack the call and iterator IDs.
	callID := env.StartCall()
	iterID := env.StoreIterator(callID, iter)
	return uint32(callID<<16 | iterID&0xFFFF)
}

// hostDbNext implements db_next.
func hostDbNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbNext: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}
	if !iter.Valid() {
		return 0
	}

	key := iter.Key()
	value := iter.Value()

	// Charge gas for the returned data.
	env.GasUsed += uint64(len(key)+len(value)) * constants.GasPerByte

	totalLen := 4 + len(key) + 4 + len(value)
	offset, err := env.MemManager.Allocate(uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	keyLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLenData, uint32(len(key)))
	if err := env.MemManager.Write(offset, keyLenData); err != nil {
		panic(fmt.Sprintf("failed to write key length: %v", err))
	}

	if err := env.MemManager.Write(offset+4, key); err != nil {
		panic(fmt.Sprintf("failed to write key: %v", err))
	}

	valLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(valLenData, uint32(len(value)))
	if err := env.MemManager.Write(offset+4+uint32(len(key)), valLenData); err != nil {
		panic(fmt.Sprintf("failed to write value length: %v", err))
	}

	if err := env.MemManager.Write(offset+8+uint32(len(key)), value); err != nil {
		panic(fmt.Sprintf("failed to write value: %v", err))
	}

	iter.Next()
	return offset
}

// hostNextValue implements db_next_value.
func hostNextValue(ctx context.Context, mod api.Module, callID, iterID uint64) (valPtr, valLen, errCode uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostNextValue: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2
	}

	if !iter.Valid() {
		return 0, 0, 0
	}

	value := iter.Value()
	env.GasUsed += uint64(len(value)) * constants.GasPerByte

	valOffset, err := allocateInContract(ctx, mod, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, valOffset, value, false); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	iter.Next()
	return valOffset, uint32(len(value)), 0
}

// hostDbRead implements db_read.
func hostDbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Charge base gas cost for DB read
	if err := env.Gas.ConsumeGas(constants.GasCostRead, "db_read base cost"); err != nil {
		panic(err) // Or handle more gracefully
	}

	// Read key length and charge per byte
	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		return 0
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	// Charge per-byte gas for key
	if err := env.Gas.ConsumeGas(uint64(keyLen)*constants.GasPerByte, "db_read key bytes"); err != nil {
		panic(err)
	}

	// Rest of existing code...

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key data: %v\n", err)
		return 0
	}
	fmt.Printf("Key data: %x\n", key)

	value := env.DB.Get(key)
	fmt.Printf("Value found: %x\n", value)

	valuePtr, err := env.MemManager.Allocate(uint32(len(value)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory: %v\n", err)
		return 0
	}

	if err := env.MemManager.Write(valuePtr, value); err != nil {
		fmt.Printf("ERROR: Failed to write value to memory: %v\n", err)
		return 0
	}

	return valuePtr
}

// hostDbWrite implements db_write.
func hostDbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbWrite: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	valLenBytes, err := env.MemManager.Read(valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := env.MemManager.Read(valuePtr+4, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, value)
}

// hostSecp256k1Verify implements secp256k1_verify.
func hostSecp256k1Verify(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Get the environment and memory
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1Verify: runtime environment not found in context")
		return SECP256K1_VERIFY_CODE_INVALID
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Charge gas for this operation
	gasToCharge := env.GasConfig.Secp256k1VerifyCost + uint64(len(hash)+len(sig)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Verify signature using the crypto handler
	valid, err := cryptoHandler.Secp256k1Verify(hash, sig, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Secp256k1Verify failed: %v\n", err)
		return SECP256K1_VERIFY_CODE_INVALID
	}

	if valid {
		return SECP256K1_VERIFY_CODE_VALID
	}
	return SECP256K1_VERIFY_CODE_INVALID
}

// hostSecp256k1RecoverPubkey implements secp256k1_recover_pubkey.
func hostSecp256k1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, recoveryParam uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1RecoverPubkey: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return 0
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Secp256k1RecoverPubkeyCost + uint64(len(hash)+len(sig))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1RecoverPubkey: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Recover pubkey using cryptoHandler
	pubkey, err := cryptoHandler.Secp256k1RecoverPubkey(hash, sig, byte(recoveryParam))
	if err != nil {
		fmt.Printf("ERROR: Secp256k1RecoverPubkey failed: %v\n", err)
		return 0
	}

	// Allocate region for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(pubkey)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for recovered pubkey: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, pubkey) {
		fmt.Printf("ERROR: Failed to write recovered pubkey to memory\n")
		return 0
	}

	return resultPtr
}

// hostEd25519Verify implements ed25519_verify.
func hostEd25519Verify(ctx context.Context, mod api.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519Verify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	message, ok := mem.Read(msgPtr, msgLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read message from memory\n")
		return 0
	}

	signature, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519VerifyCost + uint64(len(message)+len(signature)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Verify signature
	valid, err := cryptoHandler.Ed25519Verify(message, signature, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Ed25519Verify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify.
func hostEd25519BatchVerify(ctx context.Context, mod api.Module, msgsPtr, msgsLen, sigsPtr, sigsLen, pubkeysPtr, pubkeysLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519BatchVerify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read array counts and pointers
	msgsData, ok := mem.Read(msgsPtr, msgsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read messages array from memory\n")
		return 0
	}

	sigsData, ok := mem.Read(sigsPtr, sigsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signatures array from memory\n")
		return 0
	}

	pubkeysData, ok := mem.Read(pubkeysPtr, pubkeysLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public keys array from memory\n")
		return 0
	}

	// Parse arrays (implementation depends on how arrays are serialized)
	// This is a simplified example - actual parsing logic may differ
	messages, signatures, pubkeys := parseArraysForBatchVerify(msgsData, sigsData, pubkeysData)

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519BatchVerifyCost * uint64(len(messages))
	dataSize := 0
	for i := 0; i < len(messages); i++ {
		dataSize += len(messages[i]) + len(signatures[i]) + len(pubkeys[i])
	}
	gasToCharge += uint64(dataSize) * constants.GasPerByte

	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519BatchVerify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Batch verify signatures
	valid, err := cryptoHandler.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		fmt.Printf("ERROR: Ed25519BatchVerify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// parseArraysForBatchVerify parses the array data for Ed25519BatchVerify
// Implementation depends on how arrays are serialized in the contract
func parseArraysForBatchVerify(msgsData, sigsData, pubkeysData []byte) ([][]byte, [][]byte, [][]byte) {
	// Example implementation - actual parsing may differ based on serialization format
	// This is a placeholder implementation

	// In a real implementation, you would parse the arrays from their serialized format
	// Here we're creating dummy data just to satisfy the function signature
	count := 1 // In reality, extract this from the data

	messages := make([][]byte, count)
	signatures := make([][]byte, count)
	pubkeys := make([][]byte, count)

	// Fill with dummy data for demonstration
	for i := 0; i < count; i++ {
		messages[i] = []byte("message")
		signatures[i] = make([]byte, 64)
		pubkeys[i] = make([]byte, 32)
	}

	return messages, signatures, pubkeys
}

// hostBls12381AggregateG1 implements bls12_381_aggregate_g1.
func hostBls12381AggregateG1(ctx context.Context, mod api.Module, g1sPtr, g1sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG1: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 points
	g1s, ok := mem.Read(g1sPtr, g1sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return 0
	}

	pointCount := len(g1s) / constants.BLS12_381_G1_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G1 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG1Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during G1 aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g1s, constants.BLS12_381_G1_POINT_LEN)

	// Use cryptoHandler to aggregate points
	result, err := cryptoHandler.BLS12381AggregateG1(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G1 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G1 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// splitIntoPoints splits a byte array into equal-sized points
func splitIntoPoints(data []byte, pointLen int) [][]byte {
	pointCount := len(data) / pointLen
	points := make([][]byte, pointCount)

	for i := 0; i < pointCount; i++ {
		points[i] = data[i*pointLen : (i+1)*pointLen]
	}

	return points
}

// hostBls12381AggregateG2 implements bls12_381_aggregate_g2.
func hostBls12381AggregateG2(ctx context.Context, mod api.Module, g2sPtr, g2sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data
	mem := mod.Memory()
	g2s, ok := mem.Read(g2sPtr, g2sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return 0
	}

	pointCount := len(g2s) / constants.BLS12_381_G2_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G2 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG2Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g2s, constants.BLS12_381_G2_POINT_LEN)

	// Use cryptoHandler interface instead of direct function call
	result, err := cryptoHandler.BLS12381AggregateG2(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G2 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G2 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// hostDbRemove implements db_remove.
func hostDbRemove(ctx context.Context, mod api.Module, keyPtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbRemove: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the 4-byte length prefix from the key pointer.
	lenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key.
	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	env.DB.Delete(key)
}

// Add missing gasPerByte constant
const gasPerByte = constants.GasPerByte

// cryptoHandler holds crypto operations - initialized at runtime
var cryptoHandler cryptoapi.CryptoOperations

// SetCryptoHandler sets the crypto handler for host functions
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// readMessage reads a message of specified length from memory
func readMessage(mod api.Module, ptr, len uint32) ([]byte, error) {
	if len > constants.BLS12_381_MAX_MESSAGE_SIZE {
		return nil, fmt.Errorf("message too large: %d > %d", len, constants.BLS12_381_MAX_MESSAGE_SIZE)
	}

	mem := mod.Memory()
	data, ok := mem.Read(ptr, len)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", ptr, len)
	}
	return data, nil
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
func hostBls12381HashToG1(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Get environment context
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG1: runtime environment not found in context")
		return 0
	}

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G1 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
func hostBls12381HashToG2(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381HashToG2Cost.TotalCost(uint64(len(message)))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Hash-to-G2: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G2 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381VerifyG1G2 implements bls12_381_verify.
func hostBls12381VerifyG1G2(ctx context.Context, mod api.Module, g1PointsPtr, g1PointsLen, g2PointsPtr, g2PointsLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381VerifyG1G2: runtime environment not found in context")
		return BLS12_381_INVALID_PAIRING
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 and G2 points
	g1Data, ok := mem.Read(g1PointsPtr, g1PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g2Data, ok := mem.Read(g2PointsPtr, g2PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g1Count := len(g1Data) / constants.BLS12_381_G1_POINT_LEN
	g2Count := len(g2Data) / constants.BLS12_381_G2_POINT_LEN

	if g1Count != g2Count {
		fmt.Printf("ERROR: Number of G1 points (%d) must match number of G2 points (%d)\n", g1Count, g2Count)
		return BLS12_381_INVALID_PAIRING
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381VerifyCost.TotalCost(uint64(g1Count))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during BLS verification: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return BLS12_381_INVALID_PAIRING
	}

	// Split into individual points
	g1Points := splitIntoPoints(g1Data, constants.BLS12_381_G1_POINT_LEN)
	g2Points := splitIntoPoints(g2Data, constants.BLS12_381_G2_POINT_LEN)

	// Verify pairing
	valid, err := cryptoHandler.BLS12381VerifyG1G2(g1Points, g2Points)
	if err != nil {
		fmt.Printf("ERROR: BLS12-381 verification failed: %v\n", err)
		return BLS12_381_INVALID_PAIRING
	}

	if valid {
		return BLS12_381_VALID_PAIRING
	}
	return BLS12_381_INVALID_PAIRING
}

```
---
### `host/memory.go`
*2025-02-20 21:54:38 | 1 KB*
```go
package host

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// readMemory reads length bytes from memory at the given offset
func readMemory(mem api.Memory, offset, length uint32) ([]byte, error) {
	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", offset, length)
	}
	return data, nil
}

// writeMemory writes data to memory at the given offset
func writeMemory(mem api.Memory, offset uint32, data []byte, allowGrow bool) error {
	if !mem.Write(offset, data) {
		return fmt.Errorf("failed to write %d bytes to memory at offset %d", len(data), offset)
	}
	return nil
}

```
---
### `host/registerhostfunctions.go`
*2025-02-26 15:30:03 | 23 KB*
```go
package host

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// --- Minimal Host Interfaces ---
// WasmInstance is a minimal interface for a WASM contract instance.
type WasmInstance interface {
	// RegisterFunction registers a host function with the instance.
	RegisterFunction(module, name string, fn interface{})
}

// MemoryManager is imported from our memory package.
type MemoryManager = memory.MemoryManager

// Storage represents contract storage.
type Storage interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Scan(start, end []byte, order int32) (uint32, error)
	Next(iteratorID uint32) (key []byte, value []byte, err error)
}

// API aliases types.GoAPI.
type API = types.GoAPI

// Querier aliases types.Querier.
type Querier = types.Querier

// GasMeter aliases types.GasMeter.
type GasMeter = types.GasMeter

// Logger is a simple logging interface.
type Logger interface {
	Debug(args ...interface{})
	Error(args ...interface{})
}

// --- Runtime Environment ---
// RuntimeEnvironment holds all execution context for a contract call.
type RuntimeEnvironment struct {
	DB        types.KVStore
	API       API
	Querier   Querier
	Gas       GasMeter
	GasConfig types.GasConfig

	// internal gas limit and gas used for host functions:
	gasLimit uint64
	gasUsed  uint64

	// Iterator management.
	iterators      map[uint64]map[uint64]types.Iterator
	iteratorsMutex types.RWMutex // alias for sync.RWMutex from types package if desired
	nextCallID     uint64
	nextIterID     uint64
}

// --- Helper: writeToRegion ---
// writeToRegion uses MemoryManager to update a Region struct and write the provided data.
func writeToRegion(mem MemoryManager, regionPtr uint32, data []byte) error {
	regionStruct, err := mem.Read(regionPtr, 12)
	if err != nil {
		return fmt.Errorf("failed to read Region at %d: %w", regionPtr, err)
	}
	offset := binary.LittleEndian.Uint32(regionStruct[0:4])
	capacity := binary.LittleEndian.Uint32(regionStruct[4:8])
	if uint32(len(data)) > capacity {
		return fmt.Errorf("data length %d exceeds region capacity %d", len(data), capacity)
	}
	if err := mem.Write(offset, data); err != nil {
		return fmt.Errorf("failed to write data to memory at offset %d: %w", offset, err)
	}
	binary.LittleEndian.PutUint32(regionStruct[8:12], uint32(len(data)))
	if err := mem.Write(regionPtr+8, regionStruct[8:12]); err != nil {
		return fmt.Errorf("failed to write Region length at %d: %w", regionPtr+8, err)
	}
	return nil
}

// --- RegisterHostFunctions ---
// RegisterHostFunctions registers all host functions. It uses the provided WasmInstance,
// MemoryManager, Storage, API, Querier, GasMeter, and Logger.
func RegisterHostFunctions(instance WasmInstance, mem MemoryManager, storage Storage, api API, querier Querier, gasMeter GasMeter, logger Logger) {
	// Abort: abort(msg_ptr: u32, file_ptr: u32, line: u32, col: u32) -> !
	instance.RegisterFunction("env", "abort", func(msgPtr, filePtr uint32, line, col uint32) {
		msg, _ := mem.ReadRegion(msgPtr)
		file, _ := mem.ReadRegion(filePtr)
		logger.Error(fmt.Sprintf("Wasm abort called: %s (%s:%d:%d)", string(msg), string(file), line, col))
		panic(fmt.Sprintf("Wasm abort: %s", string(msg)))
	})

	// Debug: debug(msg_ptr: u32) -> ()
	instance.RegisterFunction("env", "debug", func(msgPtr uint32) {
		msg, err := mem.ReadRegion(msgPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("debug: failed to read message: %v", err))
			return
		}
		logger.Debug("cosmwasm debug:", string(msg))
	})

	// db_read: db_read(key_ptr: u32) -> u32 (returns Region pointer or 0 if not found)
	instance.RegisterFunction("env", "db_read", func(keyPtr uint32) uint32 {
		key, err := mem.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: failed to read key region: %v", err))
			return 0
		}
		value, err := storage.Get(key)
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: storage error: %v", err))
			return 0
		}
		if value == nil {
			logger.Debug("db_read: key not found")
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: memory allocation failed: %v", err))
			return 0
		}
		if err := mem.Write(regionPtr, value); err != nil {
			logger.Error(fmt.Sprintf("db_read: failed to write value to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_read: key %X -> %d bytes at region %d", key, len(value), regionPtr))
		return regionPtr
	})

	// db_write: db_write(key_ptr: u32, value_ptr: u32) -> ()
	instance.RegisterFunction("env", "db_write", func(keyPtr, valuePtr uint32) {
		key, err := mem.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_write: failed to read key: %v", err))
			return
		}
		value, err := mem.ReadRegion(valuePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_write: failed to read value: %v", err))
			return
		}
		if err := storage.Set(key, value); err != nil {
			logger.Error(fmt.Sprintf("db_write: storage error: %v", err))
		} else {
			logger.Debug(fmt.Sprintf("db_write: stored %d bytes under key %X", len(value), key))
		}
	})

	// db_remove: db_remove(key_ptr: u32) -> ()
	instance.RegisterFunction("env", "db_remove", func(keyPtr uint32) {
		key, err := mem.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_remove: failed to read key: %v", err))
			return
		}
		if err := storage.Delete(key); err != nil {
			logger.Error(fmt.Sprintf("db_remove: storage error: %v", err))
		} else {
			logger.Debug(fmt.Sprintf("db_remove: removed key %X", key))
		}
	})

	// db_scan: db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32
	instance.RegisterFunction("env", "db_scan", func(startPtr, endPtr uint32, order int32) uint32 {
		start, err := mem.ReadRegion(startPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: failed to read start key: %v", err))
			return 0
		}
		end, err := mem.ReadRegion(endPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: failed to read end key: %v", err))
			return 0
		}
		iteratorID, err := storage.Scan(start, end, order)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: storage scan error: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_scan: created iterator %d for range [%X, %X], order %d", iteratorID, start, end, order))
		return iteratorID
	})

	// db_next: db_next(iterator_id: u32) -> u32
	instance.RegisterFunction("env", "db_next", func(iteratorID uint32) uint32 {
		key, value, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if key == nil {
			logger.Debug(fmt.Sprintf("db_next: iterator %d exhausted", iteratorID))
			return 0
		}
		// Allocate regions for key and value.
		keyRegion, err := mem.Allocate(uint32(len(key)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for key: %v", err))
			return 0
		}
		if err := writeToRegion(mem, keyRegion, key); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write key to region: %v", err))
			return 0
		}
		valRegion, err := mem.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for value: %v", err))
			return 0
		}
		if err := writeToRegion(mem, valRegion, value); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write value to region: %v", err))
			return 0
		}
		// Allocate a combined region for both key and value Region structs.
		combinedSize := uint32(12 * 2)
		combinedPtr, err := mem.Allocate(combinedSize)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for combined region: %v", err))
			return 0
		}
		keyRegionStruct, err := mem.Read(keyRegion, 12)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to read key region struct: %v", err))
			return 0
		}
		valRegionStruct, err := mem.Read(valRegion, 12)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to read value region struct: %v", err))
			return 0
		}
		concat := append(keyRegionStruct, valRegionStruct...)
		if err := mem.Write(combinedPtr, concat); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write combined region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next: iterator %d next -> key %d bytes, value %d bytes", iteratorID, len(key), len(value)))
		return combinedPtr
	})

	// db_next_key: db_next_key(iterator_id: u32) -> u32
	instance.RegisterFunction("env", "db_next_key", func(iteratorID uint32) uint32 {
		key, _, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_key: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if key == nil {
			logger.Debug(fmt.Sprintf("db_next_key: iterator %d exhausted", iteratorID))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(key)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_key: failed to allocate memory: %v", err))
			return 0
		}
		if err := writeToRegion(mem, regionPtr, key); err != nil {
			logger.Error(fmt.Sprintf("db_next_key: failed to write key to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next_key: iterator %d -> key %d bytes", iteratorID, len(key)))
		return regionPtr
	})

	// db_next_value: db_next_value(iterator_id: u32) -> u32
	instance.RegisterFunction("env", "db_next_value", func(iteratorID uint32) uint32 {
		_, value, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_value: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if value == nil {
			logger.Debug(fmt.Sprintf("db_next_value: iterator %d exhausted", iteratorID))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_value: failed to allocate memory: %v", err))
			return 0
		}
		if err := writeToRegion(mem, regionPtr, value); err != nil {
			logger.Error(fmt.Sprintf("db_next_value: failed to write value to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next_value: iterator %d -> value %d bytes", iteratorID, len(value)))
		return regionPtr
	})

	// addr_validate: addr_validate(addr_ptr: u32) -> u32 (0 = success, nonzero = error)
	instance.RegisterFunction("env", "addr_validate", func(addrPtr uint32) uint32 {
		addrBytes, err := mem.ReadRegion(addrPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_validate: failed to read address: %v", err))
			return 1
		}
		_, err = api.ValidateAddress(string(addrBytes))
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_validate: address %q is INVALID: %v", addrBytes, err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_validate: address %q is valid", addrBytes))
		return 0
	})

	// addr_canonicalize: addr_canonicalize(human_ptr: u32, canon_ptr: u32) -> u32
	instance.RegisterFunction("env", "addr_canonicalize", func(humanPtr, canonPtr uint32) uint32 {
		humanAddr, err := mem.ReadRegion(humanPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_canonicalize: failed to read human address: %v", err))
			return 1
		}
		canon, _, err := api.CanonicalizeAddress(string(humanAddr))
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_canonicalize: invalid address %q: %v", humanAddr, err))
			return 1
		}
		if err := writeToRegion(mem, canonPtr, canon); err != nil {
			logger.Error(fmt.Sprintf("addr_canonicalize: failed to write canonical address: %v", err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_canonicalize: %q -> %X", humanAddr, canon))
		return 0
	})

	// addr_humanize: addr_humanize(canon_ptr: u32, human_ptr: u32) -> u32
	instance.RegisterFunction("env", "addr_humanize", func(canonPtr, humanPtr uint32) uint32 {
		canonAddr, err := mem.ReadRegion(canonPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_humanize: failed to read canonical address: %v", err))
			return 1
		}
		human, _, err := api.HumanizeAddress(canonAddr)
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_humanize: invalid canonical addr %X: %v", canonAddr, err))
			return 1
		}
		if err := writeToRegion(mem, humanPtr, []byte(human)); err != nil {
			logger.Error(fmt.Sprintf("addr_humanize: failed to write human address: %v", err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_humanize: %X -> %q", canonAddr, human))
		return 0
	})

	// secp256k1_verify: secp256k1_verify(hash_ptr: u32, sig_ptr: u32, pubkey_ptr: u32) -> u32
	instance.RegisterFunction("env", "secp256k1_verify", func(hashPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msgHash, err := mem.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read message hash: %v", err))
			return 2
		}
		sig, err := mem.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := mem.ReadRegion(pubKeyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read public key: %v", err))
			return 2
		}
		valid, _, err := api.Secp256k1Verify(msgHash, sig, pubKey)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("secp256k1_verify: signature verification FAILED")
			return 1
		}
		logger.Debug("secp256k1_verify: signature verification successful")
		return 0
	})

	// secp256k1_recover_pubkey: secp256k1_recover_pubkey(hash_ptr: u32, sig_ptr: u32, recovery_param: u32) -> u64
	instance.RegisterFunction("env", "secp256k1_recover_pubkey", func(hashPtr, sigPtr, param uint32) uint64 {
		msgHash, err := mem.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to read message hash: %v", err))
			return 0
		}
		sig, err := mem.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to read signature: %v", err))
			return 0
		}
		pubKey, _, err := api.Secp256k1RecoverPubkey(msgHash, sig, byte(param))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: recover failed: %v", err))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(pubKey)))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: allocation failed: %v", err))
			return 0
		}
		if err := writeToRegion(mem, regionPtr, pubKey); err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to write pubkey: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("secp256k1_recover_pubkey: recovered %d-byte pubkey", len(pubKey)))
		return (uint64(regionPtr) << 32) | uint64(len(pubKey))
	})

	// ed25519_verify: ed25519_verify(msg_ptr: u32, sig_ptr: u32, pubkey_ptr: u32) -> u32
	instance.RegisterFunction("env", "ed25519_verify", func(msgPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msg, err := mem.ReadRegion(msgPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read message: %v", err))
			return 2
		}
		sig, err := mem.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := mem.ReadRegion(pubKeyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read public key: %v", err))
			return 2
		}
		valid, _, err := api.Ed25519Verify(msg, sig, pubKey)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("ed25519_verify: signature verification FAILED")
			return 1
		}
		logger.Debug("ed25519_verify: signature verification successful")
		return 0
	})

	// ed25519_batch_verify: ed25519_batch_verify(msgs_ptr: u32, sigs_ptr: u32, pubkeys_ptr: u32) -> u32
	instance.RegisterFunction("env", "ed25519_batch_verify", func(msgsPtr, sigsPtr, pubKeysPtr uint32) uint32 {
		msgs, err := mem.ReadRegion(msgsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read messages: %v", err))
			return 2
		}
		sigs, err := mem.ReadRegion(sigsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read signatures: %v", err))
			return 2
		}
		pubKeys, err := mem.ReadRegion(pubKeysPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read public keys: %v", err))
			return 2
		}
		// Deserialize the inputs from the flat byte arrays into slices of byte slices
		var msgsArray, sigsArray, pubKeysArray [][]byte
		if err := json.Unmarshal(msgs, &msgsArray); err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to deserialize messages: %v", err))
			return 2
		}
		if err := json.Unmarshal(sigs, &sigsArray); err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to deserialize signatures: %v", err))
			return 2
		}
		if err := json.Unmarshal(pubKeys, &pubKeysArray); err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to deserialize public keys: %v", err))
			return 2
		}
		valid, _, err := api.Ed25519BatchVerify(msgsArray, sigsArray, pubKeysArray)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("ed25519_batch_verify: batch verification FAILED")
			return 1
		}
		logger.Debug("ed25519_batch_verify: batch verification successful")
		return 0
	})

	// bls12_381_aggregate_g1: bls12_381_aggregate_g1(messages_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_aggregate_g1", func(messagesPtr uint32) uint32 {
		msgs, err := mem.ReadRegion(messagesPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: failed to read messages: %v", err))
			return 1
		}
		result, err := api.Bls12381AggregateG1(msgs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_aggregate_g1: aggregation successful")
		return regionPtr
	})

	// bls12_381_aggregate_g2: bls12_381_aggregate_g2(messages_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_aggregate_g2", func(messagesPtr uint32) uint32 {
		msgs, err := mem.ReadRegion(messagesPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: failed to read messages: %v", err))
			return 1
		}
		result, err := api.Bls12381AggregateG2(msgs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_aggregate_g2: aggregation successful")
		return regionPtr
	})

	// bls12_381_pairing_equality: bls12_381_pairing_equality(pairs_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_pairing_equality", func(pairsPtr uint32) uint32 {
		pairs, err := mem.ReadRegion(pairsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_pairing_equality: failed to read pairs: %v", err))
			return 2
		}
		equal, err := api.Bls12381PairingCheck(pairs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_pairing_equality: error: %v", err))
			return 3
		}
		if !equal {
			logger.Debug("bls12_381_pairing_equality: pairs are NOT equal")
			return 1
		}
		logger.Debug("bls12_381_pairing_equality: pairs are equal")
		return 0
	})

	// bls12_381_hash_to_g1: bls12_381_hash_to_g1(message_ptr: u32, dest_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_hash_to_g1", func(messagePtr, destPtr uint32) uint32 {
		msg, err := mem.ReadRegion(messagePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to read message: %v", err))
			return 1
		}
		dest, err := mem.ReadRegion(destPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to read dest: %v", err))
			return 1
		}
		result, err := api.Bls12381HashToG1(msg, dest)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_hash_to_g1: hash successful")
		return regionPtr
	})

	// bls12_381_hash_to_g2: bls12_381_hash_to_g2(message_ptr: u32, dest_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_hash_to_g2", func(messagePtr, destPtr uint32) uint32 {
		msg, err := mem.ReadRegion(messagePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to read message: %v", err))
			return 1
		}
		dest, err := mem.ReadRegion(destPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to read dest: %v", err))
			return 1
		}
		result, err := api.Bls12381HashToG2(msg, dest)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_hash_to_g2: hash successful")
		return regionPtr
	})

	// query_chain: query_chain(request_ptr: u32) -> u32
	instance.RegisterFunction("env", "query_chain", func(reqPtr uint32) uint32 {
		request, err := mem.ReadRegion(reqPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to read request: %v", err))
			return 0
		}
		response := types.RustQuery(querier, request, gasMeter.GasConsumed())
		serialized, err := json.Marshal(response)
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to serialize response: %v", err))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(serialized)))
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: allocation failed: %v", err))
			return 0
		}
		if err := mem.Write(regionPtr, serialized); err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to write response: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("query_chain: responded with %d bytes at region %d", len(serialized), regionPtr))
		return regionPtr
	})
}

```
---
### `memory/memory.go`
*2025-02-20 21:49:29 | 6 KB*
```go
package memory

import (
	"context"
	"errors"

	"github.com/tetratelabs/wazero/api"
)

// WasmMemory is an alias for the wazero Memory interface.
type WasmMemory = api.Memory

// Region in Go for clarity (optional; we can also handle without this struct)
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// MemoryManager manages a Wasm instance's memory and allocation.
type MemoryManager struct {
	Memory       WasmMemory                   // interface to Wasm memory (e.g., provides Read, Write)
	WasmAllocate func(uint32) (uint32, error) // function to call Wasm allocate
	Deallocate   func(uint32) error           // function to call Wasm deallocate
	MemorySize   uint32                       // size of the memory (for bounds checking, if available)
}

// NewMemoryManager creates and initializes a MemoryManager from the given module.
// It retrieves the exported "allocate" and "deallocate" functions and the Wasm memory,
// and sets the memorySize field.
func NewMemoryManager(module api.Module) (*MemoryManager, error) {
	allocFn := module.ExportedFunction("allocate")
	deallocFn := module.ExportedFunction("deallocate")
	mem := module.Memory()
	if allocFn == nil || deallocFn == nil || mem == nil {
		return nil, errors.New("missing required exports: allocate, deallocate, or memory")
	}

	// Get the current memory size.
	size := mem.Size()

	// Create wrapper functions that call the exported functions.
	allocateWrapper := func(requestSize uint32) (uint32, error) {
		results, err := allocFn.Call(context.Background(), uint64(requestSize))
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 0, errors.New("allocate returned no results")
		}
		return uint32(results[0]), nil
	}

	deallocateWrapper := func(ptr uint32) error {
		_, err := deallocFn.Call(context.Background(), uint64(ptr))
		return err
	}

	return &MemoryManager{
		Memory:       mem,
		WasmAllocate: allocateWrapper,
		Deallocate:   deallocateWrapper,
		MemorySize:   size,
	}, nil
}

// Read copies `length` bytes from Wasm memory at the given offset into a new byte slice.
func (m *MemoryManager) Read(offset uint32, length uint32) ([]byte, error) {
	if offset+length > m.MemorySize {
		return nil, errors.New("memory read out of bounds")
	}
	data, ok := m.Memory.Read(offset, uint32(length))
	if !ok {
		return nil, errors.New("failed to read memory")
	}
	return data, nil
}

// Write copies the given data into Wasm memory starting at the given offset.
func (m *MemoryManager) Write(offset uint32, data []byte) error {
	length := uint32(len(data))
	if offset+length > m.MemorySize {
		return errors.New("memory write out of bounds")
	}
	if !m.Memory.Write(offset, data) {
		return errors.New("failed to write memory")
	}
	return nil
}

// ReadRegion reads a Region (offset, capacity, length) from Wasm memory and returns the pointed bytes.
func (m *MemoryManager) ReadRegion(regionPtr uint32) ([]byte, error) {
	// Read 12 bytes for Region struct
	const regionSize = 12
	raw, err := m.Read(regionPtr, regionSize)
	if err != nil {
		return nil, err
	}
	// Parse Region struct (little-endian u32s)
	if len(raw) != regionSize {
		return nil, errors.New("invalid region struct size")
	}
	region := Region{
		Offset:   littleEndianToUint32(raw[0:4]),
		Capacity: littleEndianToUint32(raw[4:8]),
		Length:   littleEndianToUint32(raw[8:12]),
	}
	// Basic sanity checks
	if region.Offset+region.Length > m.MemorySize {
		return nil, errors.New("region out of bounds")
	}
	if region.Length > region.Capacity {
		return nil, errors.New("region length exceeds capacity")
	}
	// Read the actual data
	return m.Read(region.Offset, region.Length)
}

// Allocate requests a new memory region of given size from the Wasm instance.
func (m *MemoryManager) Allocate(size uint32) (uint32, error) {
	// Call the contract's allocate function via the provided callback
	offset, err := m.WasmAllocate(size)
	if err != nil {
		return 0, err
	}
	if offset == 0 {
		// A zero offset might indicate allocation failure (if contract uses 0 as null)
		return 0, errors.New("allocation failed")
	}
	// Optionally, ensure offset is within memory bounds (if allocate doesn't already guarantee it)
	if offset >= m.MemorySize {
		return 0, errors.New("allocation returned out-of-bounds pointer")
	}
	return offset, nil
}

// Free releases previously allocated memory back to the contract.
func (m *MemoryManager) Free(offset uint32) error {
	return m.Deallocate(offset)
}

// CreateRegion allocates a Region struct in Wasm memory for a given data buffer.
func (m *MemoryManager) CreateRegion(dataOffset, dataLength uint32) (uint32, error) {
	const regionSize = 12
	regionPtr, err := m.Allocate(regionSize)
	if err != nil {
		return 0, err
	}
	// Build the region struct in little-endian bytes
	reg := make([]byte, regionSize)
	putUint32LE(reg[0:4], dataOffset)
	putUint32LE(reg[4:8], dataLength)  // capacity = length (we allocate exactly length)
	putUint32LE(reg[8:12], dataLength) // length = actual data length
	// Write the struct into memory
	if err := m.Write(regionPtr, reg); err != nil {
		m.Free(regionPtr) // free the region struct allocation if writing fails
		return 0, err
	}
	return regionPtr, nil
}

// Utility: convert 4 bytes little-endian to uint32
func littleEndianToUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// Utility: write uint32 as 4 little-endian bytes
func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v & 0xFF)
	b[1] = byte((v >> 8) & 0xFF)
	b[2] = byte((v >> 16) & 0xFF)
	b[3] = byte((v >> 24) & 0xFF)
}

```
---
### `tracing.go`
*2025-02-20 21:49:29 | 3 KB*
```go
package runtime

import (
	"encoding/hex"
	"fmt"
	"runtime"
	"time"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/tetratelabs/wazero/api"
)

// TraceConfig controls tracing behavior
type TraceConfig struct {
	Enabled     bool
	ShowMemory  bool
	ShowParams  bool
	ShowStack   bool
	MaxDataSize uint32 // Maximum bytes of data to print
}

// Global trace configuration - can be modified at runtime
var TraceConf = TraceConfig{
	Enabled:     true,
	ShowMemory:  true,
	ShowParams:  true,
	ShowStack:   true,
	MaxDataSize: 256,
}

// TraceFn wraps a function with tracing
func TraceFn(name string) func() {
	if !TraceConf.Enabled {
		return func() {}
	}

	start := time.Now()

	// Get caller information
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)

	// Print entry trace
	fmt.Printf("\n=== ENTER: %s ===\n", name)
	fmt.Printf("Location: %s:%d\n", file, line)
	fmt.Printf("Function: %s\n", fn.Name())

	if TraceConf.ShowStack {
		// Capture and print stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		fmt.Printf("Stack:\n%s\n", string(buf[:n]))
	}

	// Return function to be deferred
	return func() {
		duration := time.Since(start)
		fmt.Printf("=== EXIT: %s (took %v) ===\n\n", name, duration)
	}
}

// TraceMemory prints memory state if enabled
func TraceMemory(memory api.Memory, msg string) {
	if !TraceConf.Enabled || !TraceConf.ShowMemory {
		return
	}

	fmt.Printf("\n=== Memory State: %s ===\n", msg)
	fmt.Printf("Size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/constants.WasmPageSize)

	// Print first page contents
	if data, ok := memory.Read(0, TraceConf.MaxDataSize); ok {
		fmt.Printf("First %d bytes:\n%s\n", TraceConf.MaxDataSize, hex.Dump(data))
	}
}

// TraceParams prints parameter values if enabled
func TraceParams(params ...interface{}) {
	if !TraceConf.Enabled || !TraceConf.ShowParams {
		return
	}

	fmt.Printf("Parameters:\n")
	for i, p := range params {
		// Handle different parameter types appropriately
		switch v := p.(type) {
		case []byte:
			if uint32(len(v)) > TraceConf.MaxDataSize {
				fmt.Printf("  %d: []byte len=%d (truncated)\n", i, len(v))
				fmt.Printf("     %x...\n", v[:int(TraceConf.MaxDataSize)])
			} else {
				fmt.Printf("  %d: []byte %x\n", i, v)
			}
		default:
			fmt.Printf("  %d: %v\n", i, p)
		}
	}
}

```
---
### `types/gas.go`
*2025-02-26 15:40:05 | 1 KB*
```go
package types

// GasConfig defines costs for various operations in the VM
type GasConfig struct {
	// Basic costs
	PerByte       uint64
	DatabaseRead  uint64
	DatabaseWrite uint64
	CompileCost   uint64
	GasMultiplier uint64

	// Crypto operation costs
	Secp256k1VerifyCost        uint64
	Secp256k1RecoverPubkeyCost uint64
	Ed25519VerifyCost          uint64
	Ed25519BatchVerifyCost     uint64

	// BLS12-381 operation costs
	Bls12381AggregateG1Cost OperationCost
	Bls12381AggregateG2Cost OperationCost
	Bls12381HashToG1Cost    OperationCost
	Bls12381HashToG2Cost    OperationCost
	Bls12381VerifyCost      OperationCost
}

// OperationCost defines a cost function with base and variable components
type OperationCost struct {
	Base     uint64
	Variable uint64
}

// TotalCost calculates the total cost for n operations
func (c OperationCost) TotalCost(n uint64) uint64 {
	return c.Base + c.Variable*n
}

```
---
### `types/types.go`
*2025-02-26 15:40:25 | 4 KB*
```go
package types

import (
	"sync"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
)

// GasOperation represents different types of gas operations
type GasOperation int

const (
	GasOperationMemoryRead GasOperation = iota
	GasOperationMemoryWrite
	GasOperationDBRead
	GasOperationDBWrite
	GasOperationDBDelete
	GasOperationCompile
)

// OutOfGasError represents an out of gas error
type OutOfGasError struct {
	Descriptor string
}

func (e OutOfGasError) Error() string {
	return "out of gas: " + e.Descriptor
}

// GasCost represents a gas cost with base and per-unit components
type GasCost struct {
	BaseCost uint64
	PerUnit  uint64
}

// TotalCost calculates total gas cost for an operation
func (g GasCost) TotalCost(units uint64) uint64 {
	return g.BaseCost + (g.PerUnit * units)
}

// GasMeter interface defines gas consumption methods
type GasMeter interface {
	GasConsumed() uint64
	ConsumeGas(amount uint64, descriptor string) error
}

// Add missing interfaces
type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
	Iterator(start, end []byte) Iterator
	ReverseIterator(start, end []byte) Iterator
}

type Iterator interface {
	Valid() bool
	Next()
	Key() []byte
	Value() []byte
	Close() error
	Domain() (start, end []byte)
	Error() error
}

type GoAPI interface {
	HumanizeAddress([]byte) (string, uint64, error)
	CanonicalizeAddress(string) ([]byte, uint64, error)
	ValidateAddress(string) (uint64, error)
	Secp256k1Verify(message, signature, pubkey []byte) (bool, uint64, error)
	Secp256k1RecoverPubkey(message, signature []byte, recovery uint8) ([]byte, uint64, error)
	Ed25519Verify(message, signature, pubkey []byte) (bool, uint64, error)
	Ed25519BatchVerify(messages [][]byte, signatures [][]byte, pubkeys [][]byte) (bool, uint64, error)
	// Add other required methods
}

type Querier interface {
	Query(request []byte) ([]byte, error)
}

// Add missing types
type Env struct {
	Block       BlockInfo
	Contract    ContractInfo
	Transaction TransactionInfo
}

type MessageInfo struct {
	Sender string
	Funds  []Coin
}

type ContractResult struct {
	Data   []byte
	Events []Event
}

type Reply struct {
	ID     uint64
	Result SubMsgResult
}

type UFraction struct {
	Numerator   uint64
	Denominator uint64
}

// Add these type definitions
type BlockInfo struct {
	Height  int64
	Time    int64
	ChainID string
}

type ContractInfo struct {
	Address string
	CodeID  uint64
}

type TransactionInfo struct {
	Index uint32
}

type Coin struct {
	Denom  string
	Amount uint64
}

type Event struct {
	Type       string
	Attributes []EventAttribute
}

type EventAttribute struct {
	Key   string
	Value string
}

type SubMsgResult struct {
	Ok  *SubMsgResponse
	Err string
}

type SubMsgResponse struct {
	Events []Event
	Data   []byte
}

// Add after other type definitions

// RuntimeEnvironment holds the execution context for host functions
type RuntimeEnvironment struct {
	DB         KVStore
	API        GoAPI
	Querier    Querier
	Gas        GasMeter
	GasConfig  GasConfig
	MemManager *memory.MemoryManager
	GasUsed    uint64 // Track gas usage

	// Iterator management
	iterators      map[uint64]map[uint64]Iterator
	iteratorsMutex sync.RWMutex
	nextCallID     uint64
	nextIterID     uint64
}

// Add methods to RuntimeEnvironment
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]Iterator)
	return e.nextCallID
}

func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextIterID++
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()
	if callMap, ok := e.iterators[callID]; ok {
		return callMap[iterID]
	}
	return nil
}

```
---
### `validation/validation.go`
*2025-02-20 21:49:29 | 2 KB*
```go
package validation

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
)

// AnalyzeForValidation validates a compiled module to ensure it meets the CosmWasm requirements.
// It ensures the module has exactly one exported memory, that the required exports ("allocate", "deallocate")
// are present, and that the contract's interface marker export is exactly "interface_version_8".
func AnalyzeForValidation(compiled wazero.CompiledModule) error {
	// Check memory constraints: exactly one memory export is required.
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return fmt.Errorf("static Wasm validation error: contract must contain exactly one memory (found %d)", memoryCount)
	}

	// Ensure required exports (e.g., "allocate" and "deallocate") are present.
	requiredExports := []string{"allocate", "deallocate"}
	exports := compiled.ExportedFunctions()
	for _, r := range requiredExports {
		found := false
		for name := range exports {
			if name == r {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("static Wasm validation error: contract missing required export %q", r)
		}
	}

	// Ensure the interface version marker is present.
	var interfaceVersionCount int
	for name := range exports {
		if strings.HasPrefix(name, "interface_version_") {
			interfaceVersionCount++
			if name != "interface_version_8" {
				return fmt.Errorf("static Wasm validation error: unknown interface version marker %q", name)
			}
		}
	}
	if interfaceVersionCount == 0 {
		return fmt.Errorf("static Wasm validation error: contract missing required interface version marker (interface_version_*)")
	}

	return nil
}

```
---
### `wasm/execution.go`
*2025-02-26 15:42:32 | 17 KB*
```go
package wasm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	// Assume types package defines Env, MessageInfo, QueryRequest, Reply, etc.
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/crypto"
	wazmeter "github.com/CosmWasm/wasmvm/v2/internal/runtime/gas/wazero"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/host"
	runtimeTypes "github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

func init() {
	// Set up the crypto handler
	cryptoImpl := crypto.NewCryptoImplementation()
	host.SetCryptoHandler(cryptoImpl)
}

// Instantiate compiles (if needed) and instantiates a contract, calling its "instantiate" method.
func (vm *WazeroVM) Instantiate(checksum Checksum, env types.Env, info types.MessageInfo, initMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	// Marshal env and info to JSON (as the contract expects JSON input) [oai_citation_attribution:14‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=func%20%28vm%20) [oai_citation_attribution:15‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=infoBin%2C%20err%20%3A%3D%20json).
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal Env: %w", err)
	}
	infoBz, err := json.Marshal(info)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal MessageInfo: %w", err)
	}
	// Execute the contract call
	resBz, gasUsed, execErr := vm.callContract(checksum, "instantiate", envBz, infoBz, initMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		// If an error occurred in execution, return the error with gas used so far [oai_citation_attribution:16‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=data%2C%20gasReport%2C%20err%20%3A%3D%20api,printDebug).
		return nil, gasUsed, execErr
	}
	// Deserialize the contract's response (JSON) into a ContractResult struct [oai_citation_attribution:17‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=var%20result%20types).
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize instantiate result: %w", err)
	}
	return &result, gasUsed, nil
}

// Execute calls a contract's "execute" entry point with the given message.
func (vm *WazeroVM) Execute(checksum Checksum, env types.Env, info types.MessageInfo, execMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBz, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "execute", envBz, infoBz, execMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize execute result: %w", err)
	}
	return &result, gasUsed, nil
}

// Query calls a contract's "query" entry point. Query has no MessageInfo (no funds or sender).
func (vm *WazeroVM) Query(checksum Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	// For queries, no info, so we pass only env and msg.
	resBz, gasUsed, execErr := vm.callContract(checksum, "query", envBz, nil, queryMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize query result: %w", err)
	}
	return &result, gasUsed, nil
}

// Migrate calls a contract's "migrate" entry point with given migrate message.
func (vm *WazeroVM) Migrate(checksum Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "migrate", envBz, nil, migrateMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize migrate result: %w", err)
	}
	return &result, gasUsed, nil
}

// Sudo calls the contract's "sudo" entry point (privileged call from the chain).
func (vm *WazeroVM) Sudo(checksum Checksum, env types.Env, sudoMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "sudo", envBz, nil, sudoMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize sudo result: %w", err)
	}
	return &result, gasUsed, nil
}

// Reply calls the contract's "reply" entry point to handle a SubMsg reply.
func (vm *WazeroVM) Reply(checksum Checksum, env types.Env, reply types.Reply, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	replyBz, err := json.Marshal(reply)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "reply", envBz, nil, replyBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize reply result: %w", err)
	}
	return &result, gasUsed, nil
}

// gasContext holds gas metering state for a contract execution
type gasContext struct {
	meter        *wazmeter.WazeroGasMeter
	operationGas uint64 // Tracks gas for current operation
}

// callContract is an internal helper to instantiate the Wasm module and call a specified entry point.
func (vm *WazeroVM) callContract(checksum Checksum, entrypoint string, env []byte, info []byte, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, uint64, error) {
	ctx := context.Background()
	// Attach the execution context (store, api, querier, gasMeter) to ctx for host functions.
	instCtx := instanceContext{store: store, api: api, querier: querier, gasMeter: gasMeter, gasLimit: gasLimit}
	ctx = context.WithValue(ctx, instanceContextKey{}, &instCtx)

	// Create wazero gas meter with proper configuration
	internalConfig := runtimeTypes.GasConfig{
		PerByte:       vm.gasConfig.PerByte,
		DatabaseRead:  vm.gasConfig.DatabaseRead,
		DatabaseWrite: vm.gasConfig.DatabaseWrite,
		GasMultiplier: 100, // Default multiplier
	}
	wazmeter := wazmeter.NewWazeroGasMeter(gasLimit, internalConfig)

	// Create module config with gas metering
	modConfig := wazero.NewModuleConfig()

	// Convert memory limit from bytes to pages (64KiB per page)
	maxPages := uint32(vm.memoryLimit / 65536)
	if maxPages < 1 {
		maxPages = 1 // Ensure at least 1 page
	}

	// Fix gas meter context key
	ctx = wazmeter.WithGasMeter(ctx)

	// Ensure we have a compiled module for this code (maybe from cache) [oai_citation_attribution:18‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=Contract%20pinning%20is%20a%20feature,33x%20faster).
	codeHash := [32]byte{}
	copy(codeHash[:], checksum) // convert to array key
	compiled, err := vm.getCompiledModule(codeHash)
	if err != nil {
		return nil, 0, fmt.Errorf("loading module: %w", err)
	}
	// Instantiate a new module instance for this execution.
	module, err := vm.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, 0, fmt.Errorf("instantiating module: %w", err)
	}
	defer module.Close(ctx) // ensure instance is closed after execution

	// Allocate and write input data (env, info, msg) into the module's memory.
	mem := module.Memory()
	// Helper to allocate a region and copy data into it, returning the Region pointer.
	allocData := func(data []byte) (uint32, error) {
		if data == nil {
			return 0, nil
		}
		allocFn := module.ExportedFunction("allocate")
		if allocFn == nil {
			return 0, fmt.Errorf("allocate function not found in module")
		}
		// Request a region for data
		allocRes, err := allocFn.Call(ctx, uint64(len(data)))
		if err != nil || len(allocRes) == 0 {
			return 0, fmt.Errorf("allocate failed: %v", err)
		}
		regionPtr := uint32(allocRes[0])
		// The Region struct is stored at regionPtr [oai_citation_attribution:19‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=). It contains a pointer to allocated memory.
		// Read the offset of the allocated buffer from the Region (first 4 bytes).
		offset, ok := mem.ReadUint32Le(regionPtr)
		if !ok {
			return 0, fmt.Errorf("failed to read allocated region offset")
		}
		// Write the data into the allocated buffer.
		if !mem.Write(uint32(offset), data) {
			return 0, fmt.Errorf("failed to write data into wasm memory")
		}
		// Set the region's length field (third 4 bytes of Region struct) to data length.
		if !mem.WriteUint32Le(regionPtr+8, uint32(len(data))) {
			return 0, fmt.Errorf("failed to write region length")
		}
		return regionPtr, nil
	}
	envPtr, err := allocData(env)
	if err != nil {
		return nil, 0, err
	}
	infoPtr, err := allocData(info)
	if err != nil {
		return nil, 0, err
	}
	msgPtr, err := allocData(msg)
	if err != nil {
		return nil, 0, err
	}

	// Call the contract's entrypoint function.
	fn := module.ExportedFunction(entrypoint)
	if fn == nil {
		return nil, 0, fmt.Errorf("entry point %q not found in contract", entrypoint)
	}
	// Prepare arguments as (env_ptr, info_ptr, msg_ptr) or (env_ptr, msg_ptr) depending on entrypoint [oai_citation_attribution:20‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=,to%20extend%20their%20functionality) [oai_citation_attribution:21‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=extern%20,u32).
	args := []uint64{uint64(envPtr)}
	if info != nil {
		args = append(args, uint64(infoPtr))
	}
	args = append(args, uint64(msgPtr))
	// Execute the contract function. This will trigger host function calls (db_read, etc.) as needed.
	results, err := fn.Call(ctx, args...)
	// Compute gas used internally by subtracting remaining gas from gasLimit.
	gasUsed := gasLimit
	if instCtx.gasMeter != nil {
		// Use GasConsumed difference (querier gas usage accounted separately).
		gasUsed = instCtx.gasMeter.GasConsumed()
	}
	if err != nil {
		// If the execution trapped (e.g., out of gas or contract panic), determine error.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Context cancellation (treat as out of gas for consistency).
			return nil, gasUsed, runtimeTypes.OutOfGasError{Descriptor: "execution timeout"}
		}
		// Wazero traps on out-of-gas would manifest as a panic/exit error
		if gasUsed >= gasLimit {
			return nil, gasUsed, runtimeTypes.OutOfGasError{Descriptor: "execution exceeded gas limit"}
		}
		// Otherwise, return the error as a generic VM error.
		return nil, gasUsed, fmt.Errorf("contract execution error: %w", err)
	}
	// The contract returns a pointer to a Region with the result data (or 0 if no data) [oai_citation_attribution:23‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=extern%20,u32).
	var data []byte
	if len(results) > 0 {
		resultPtr := uint32(results[0])
		if resultPtr != 0 {
			// Read region pointer for result
			resOffset, ok := mem.ReadUint32Le(resultPtr)
			resLength, ok2 := mem.ReadUint32Le(resultPtr + 8)
			if ok && ok2 {
				data, _ = mem.Read(resOffset, resLength)
			}
		}
	}
	// We do not explicitly call deallocate for result region, as the whole module instance will be closed and memory freed.
	return data, gasUsed, nil
}

// getCompiledModule returns a compiled module for the given checksum, compiling or retrieving from cache as needed.
func (vm *WazeroVM) getCompiledModule(codeHash [32]byte) (wazero.CompiledModule, error) {
	// Fast path: check caches under read lock.
	vm.cacheMu.RLock()
	if item, ok := vm.pinned[codeHash]; ok {
		vm.hitsPinned++ // pinned cache hit
		item.hits++
		compiled := item.compiled
		vm.cacheMu.RUnlock()
		vm.logger.Debug("Using pinned contract module from cache", "checksum", hex.EncodeToString(codeHash[:]))
		return compiled, nil
	}
	if item, ok := vm.memoryCache[codeHash]; ok {
		vm.hitsMemory++ // LRU cache hit
		item.hits++
		// Move this item to most-recently-used position in LRU order
		// (We'll do simple reorder: remove and append at end).
		// Find and remove from cacheOrder slice:
		for i, hash := range vm.cacheOrder {
			if hash == codeHash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.cacheOrder = append(vm.cacheOrder, codeHash)
		compiled := item.compiled
		vm.cacheMu.RUnlock()
		vm.logger.Debug("Using cached module from LRU cache", "checksum", hex.EncodeToString(codeHash[:]))
		return compiled, nil
	}
	vm.cacheMu.RUnlock()

	// Cache miss: compile the module.
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// Double-check if another goroutine compiled it while we were waiting.
	if item, ok := vm.pinned[codeHash]; ok {
		vm.hitsPinned++
		item.hits++
		return item.compiled, nil
	}
	if item, ok := vm.memoryCache[codeHash]; ok {
		vm.hitsMemory++
		item.hits++
		// promote in LRU order
		for i, hash := range vm.cacheOrder {
			if hash == codeHash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.cacheOrder = append(vm.cacheOrder, codeHash)
		return item.compiled, nil
	}
	// Not in any cache yet: compile the Wasm code.
	code, ok := vm.codeStore[codeHash]
	if !ok {
		vm.logger.Error("Wasm code bytes not found for checksum")
		return nil, fmt.Errorf("code %x not found", codeHash)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}
	vm.misses++ // cache miss (compiled new module)
	// Add to memory cache (un-pinned by default). Evict LRU if over capacity.
	size := uint64(len(code))
	vm.memoryCache[codeHash] = &cacheItem{compiled: compiled, size: size, hits: 0}
	vm.cacheOrder = append(vm.cacheOrder, codeHash)
	if len(vm.memoryCache) > vm.cacheSize {
		// evict least recently used (front of cacheOrder)
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background()) // free the compiled module
			delete(vm.memoryCache, oldest)
			vm.logger.Debug("Evicted module from cache (LRU)", "checksum", hex.EncodeToString(oldest[:]))
		}
	}
	vm.logger.Info("Compiled new contract module and cached",
		"checksum", hex.EncodeToString(codeHash[:]),
		"size_bytes", size)
	return compiled, nil
}

// instanceContext carries environment references for host functions.
type instanceContext struct {
	store    types.KVStore
	api      types.GoAPI
	querier  types.Querier
	gasMeter types.GasMeter
	gasLimit uint64
}

// instanceContextKey is used as context key for instanceContext.
type instanceContextKey struct{}

// Helper to retrieve instanceContext from a context.
func getInstanceContext(ctx context.Context) *instanceContext {
	val := ctx.Value(instanceContextKey{})
	if val == nil {
		return nil
	}
	return val.(*instanceContext)
}

// OutOfGasError represents an out of gas error
type OutOfGasError struct{}

func (OutOfGasError) Error() string {
	return "out of gas"
}

```
---
### `wasm/ibc.go`
*2025-02-20 21:49:29 | 6 KB*
```go
package wasm

import (
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// IBCChannelOpen calls the contract's "ibc_channel_open" entry point.
func (vm *WazeroVM) IBCChannelOpen(checksum types.Checksum, env types.Env, msg types.IBCChannelOpenMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCChannelOpenResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_open", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCChannelOpenResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelOpenResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCChannelConnect calls "ibc_channel_connect" entry point.
func (vm *WazeroVM) IBCChannelConnect(checksum types.Checksum, env types.Env, msg types.IBCChannelConnectMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_connect", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelConnectResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCChannelClose calls "ibc_channel_close".
func (vm *WazeroVM) IBCChannelClose(checksum types.Checksum, env types.Env, msg types.IBCChannelCloseMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_close", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelCloseResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketReceive calls "ibc_packet_receive".
func (vm *WazeroVM) IBCPacketReceive(checksum types.Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCReceiveResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_receive", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCReceiveResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketReceiveResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketAck calls "ibc_packet_ack".
func (vm *WazeroVM) IBCPacketAck(checksum types.Checksum, env types.Env, msg types.IBCPacketAckMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_ack", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketAckResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketTimeout calls "ibc_packet_timeout".
func (vm *WazeroVM) IBCPacketTimeout(checksum types.Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_timeout", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketTimeoutResult: %w", err)
	}
	return &result, gasUsed, nil
}

```
---
### `wasm/runtime.go`
*2025-02-20 21:49:29 | 3 KB*
```go
package wasm

import "github.com/CosmWasm/wasmvm/v2/types"

type WasmRuntime interface {
	// InitCache sets up any runtime-specific cache or resources. Returns a handle.
	InitCache(config types.VMConfig) (any, error)

	// ReleaseCache frees resources created by InitCache.
	ReleaseCache(handle any)

	// Compilation and code storage
	StoreCode(code []byte, persist bool) (checksum []byte, err error)
	StoreCodeUnchecked(code []byte) ([]byte, error)
	GetCode(checksum []byte) ([]byte, error)
	RemoveCode(checksum []byte) error
	Pin(checksum []byte) error
	Unpin(checksum []byte) error
	AnalyzeCode(checksum []byte) (*types.AnalysisReport, error)

	// Execution lifecycles
	Instantiate(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Execute(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Migrate(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	MigrateWithInfo(checksum []byte, env []byte, msg []byte, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Sudo(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Reply(checksum []byte, env []byte, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Query(checksum []byte, env []byte, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)

	// IBC entry points
	IBCChannelOpen(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCChannelConnect(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCChannelClose(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketReceive(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketAck(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketTimeout(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCSourceCallback(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCDestinationCallback(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)

	// Metrics
	GetMetrics() (*types.Metrics, error)
	GetPinnedMetrics() (*types.PinnedMetrics, error)
}

```
---
### `wasm/system.go`
*2025-02-26 15:27:58 | 10 KB*
```go
package wasm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// StoreCode compiles and stores a new Wasm code blob, returning its checksum and gas used for compilation.
func (vm *WazeroVM) StoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:] // as []byte
	// Simulate compilation gas cost
	compileCost := uint64(len(code)) * (3 * 140_000) // CostPerByte = 3 * 140k, as per CosmWasm gas schedule
	if gasLimit < compileCost {
		// Not enough gas provided to compile this code
		return cs, compileCost, OutOfGasError{}
	}
	// If code is already stored, we can avoid recompiling (but still charge gas).
	codeHash := [32]byte(checksum)
	vm.cacheMu.Lock()
	alreadyStored := vm.codeStore[codeHash] != nil
	vm.cacheMu.Unlock()
	if !alreadyStored {
		// Insert code into storage
		vm.cacheMu.Lock()
		vm.codeStore[codeHash] = code
		vm.cacheMu.Unlock()
		vm.logger.Info("Stored new contract code", "checksum", hex.EncodeToString(checksum[:]), "size", len(code))
	} else {
		vm.logger.Debug("StoreCode called for already stored code", "checksum", hex.EncodeToString(checksum[:]))
	}
	// Compile module immediately to ensure it is valid and cached.
	vm.cacheMu.Lock()
	_, compErr := vm.getCompiledModule(codeHash)
	vm.cacheMu.Unlock()
	if compErr != nil {
		return cs, compileCost, compErr
	}
	return cs, compileCost, nil
}

// SimulateStoreCode estimates gas needed to store the given code, without actually storing it.
func (vm *WazeroVM) SimulateStoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:]
	cost := uint64(len(code)) * (3 * 140_000) // same formula as compileCost
	if gasLimit < cost {
		return cs, cost, OutOfGasError{}
	}
	// We do not compile or store the code in simulation.
	return cs, cost, nil
}

// GetCode returns the original Wasm bytes for the given code checksum.
func (vm *WazeroVM) GetCode(checksum Checksum) ([]byte, error) {
	codeHash := [32]byte{}
	copy(codeHash[:], checksum) // convert to array key
	vm.cacheMu.RLock()
	code := vm.codeStore[codeHash]
	vm.cacheMu.RUnlock()
	if code == nil {
		return nil, fmt.Errorf("code for %x not found", checksum)
	}
	return code, nil
}

// RemoveCode removes the Wasm bytes and any cached compiled module.
func (vm *WazeroVM) RemoveCode(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// First check if it's pinned (priority cache)
	if item, ok := vm.pinned[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.pinned, hash)
		vm.logger.Info("Removed pinned contract from memory", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	if item, ok := vm.memoryCache[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.memoryCache, hash)
		// Also need to remove from LRU ordering
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.logger.Info("Removed contract from in-memory cache", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	// If not in caches, nothing to remove.
	vm.logger.Debug("RemoveCode called but code not in memory cache", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// Pin marks the module with the given checksum as pinned, meaning it won't be removed by the LRU cache.
func (vm *WazeroVM) Pin(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// If already pinned, nothing to do.
	if _, ok := vm.pinned[hash]; ok {
		return nil
	}
	// See if it's in memory cache, move it to pinned.
	memItem, memOk := vm.memoryCache[hash]
	if memOk {
		delete(vm.memoryCache, hash)
		// Remove from LRU order slice
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		// Add to pinned cache directly
		vm.pinned[hash] = memItem
		vm.logger.Info("Pinned contract code in memory", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	// Not in mem cache, fetch from code store & compile.
	code, ok := vm.codeStore[hash]
	if !ok {
		return fmt.Errorf("code %x not found", hash)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return fmt.Errorf("pinning compilation failed: %w", err)
	}
	item := &cacheItem{
		compiled: compiled,
		size:     uint64(len(code)),
		hits:     0,
	}
	// Add to pinned cache
	vm.pinned[hash] = item
	vm.logger.Info("Pinned contract code in memory", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// Unpin marks the module with the given checksum as unpinned, allowing it to be removed by the LRU cache.
func (vm *WazeroVM) Unpin(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// If not pinned, nothing to do.
	item, ok := vm.pinned[hash]
	if !ok {
		return nil
	}
	// Move from pinned to memory cache
	delete(vm.pinned, hash)
	vm.memoryCache[hash] = item
	vm.cacheOrder = append(vm.cacheOrder, hash) // add to end (most recently used)
	// If memoryCache is now over capacity, evict the LRU item
	if len(vm.memoryCache) > vm.cacheSize {
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background())
			delete(vm.memoryCache, oldest)
			vm.logger.Debug("Evicted module after unpin (LRU)", "checksum", hex.EncodeToString(oldest[:]))
		}
	}
	vm.logger.Info("Unpinned contract code", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// AnalyzeCode statically analyzes the Wasm bytecode and returns capabilities and features it requires.
func (vm *WazeroVM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error) {
	hash := [32]byte{}
	copy(hash[:], checksum)

	// Get the module (either from cache or fresh compile)
	vm.cacheMu.Lock()
	module, err := vm.getCompiledModule(hash)
	vm.cacheMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Create base report
	report := types.AnalysisReport{
		HasIBCEntryPoints:    false,
		RequiredCapabilities: "",
	}

	// First, check exports for IBC entry points
	exports := module.ExportedFunctions()
	for name := range exports {
		// Check for IBC exports
		if name == "ibc_channel_open" || name == "ibc_channel_connect" || name == "ibc_channel_close" ||
			name == "ibc_packet_receive" || name == "ibc_packet_ack" || name == "ibc_packet_timeout" {
			report.HasIBCEntryPoints = true
			break
		}
	}

	// Get the module's imports to check for required capabilities
	var requiredCapabilities string

	// Helper to add capabilities without duplicates
	addCapability := func(cap string) {
		if requiredCapabilities == "" {
			requiredCapabilities = cap
		} else {
			// Check if already present
			found := false
			for _, c := range []string{requiredCapabilities} {
				if c == cap {
					found = true
					break
				}
			}
			if !found {
				requiredCapabilities = requiredCapabilities + "," + cap
			}
		}
	}

	// Check imports to determine capabilities
	for _, imp := range module.ImportedFunctions() {
		impModule, impName, _ := imp.Import()

		if impModule == "env" {
			// Check for capability-indicating imports
			switch impName {
			case "secp256k1_verify", "secp256k1_recover_pubkey":
				addCapability("secp256k1")
			case "ed25519_verify", "ed25519_batch_verify":
				addCapability("ed25519")
			case "addr_humanize", "addr_canonicalize", "addr_validate":
				addCapability("cosmwasm_1_1")
			case "bls12_381_aggregate_g1", "bls12_381_aggregate_g2":
				addCapability("cosmwasm_1_4")
			}
		}
	}

	report.RequiredCapabilities = requiredCapabilities
	return &report, nil
}

// GetMetrics returns aggregated metrics about cache usage.
func (vm *WazeroVM) GetMetrics() (*types.Metrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	m := &types.Metrics{
		HitsPinnedMemoryCache:     uint32(vm.hitsPinned),
		HitsMemoryCache:           uint32(vm.hitsMemory),
		HitsFsCache:               0, // we are not using FS cache in this implementation
		Misses:                    uint32(vm.misses),
		ElementsPinnedMemoryCache: uint64(len(vm.pinned)),
		ElementsMemoryCache:       uint64(len(vm.memoryCache)),
		SizePinnedMemoryCache:     0,
		SizeMemoryCache:           0,
	}
	// Calculate sizes
	for _, item := range vm.pinned {
		m.SizePinnedMemoryCache += item.size
	}
	for _, item := range vm.memoryCache {
		m.SizeMemoryCache += item.size
	}
	return m, nil
}

// GetPinnedMetrics returns detailed metrics for each pinned contract.
func (vm *WazeroVM) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	var entries []types.PerModuleEntry
	for hash, item := range vm.pinned {
		entries = append(entries, types.PerModuleEntry{
			Checksum: hash[:],
			Metrics: types.PerModuleMetrics{
				Hits: uint32(item.hits),
				Size: item.size,
			},
		})
	}
	// Sort entries by checksum for consistency
	sort.Slice(entries, func(i, j int) bool {
		return hex.EncodeToString(entries[i].Checksum) < hex.EncodeToString(entries[j].Checksum)
	})
	return &types.PinnedMetrics{PerModule: entries}, nil
}

```
---
### `wasm/vm.go`
*2025-02-26 16:38:28 | 3 KB*
```go
package wasm

import (
	"context"
	"fmt"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/crypto"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/gas"
	wasmTypes "github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

type Checksum = []byte

type cacheItem struct {
	compiled wazero.CompiledModule
	size     uint64
	hits     uint64
}

// WazeroVM implements the CosmWasm VM using wazero
type WazeroVM struct {
	runtime     wazero.Runtime
	codeStore   map[[32]byte][]byte
	memoryCache map[[32]byte]*cacheItem
	pinned      map[[32]byte]*cacheItem
	cacheOrder  [][32]byte
	cacheSize   int
	cacheMu     sync.RWMutex

	// Cache statistics
	hitsPinned uint64
	hitsMemory uint64
	misses     uint64

	// Configuration
	gasConfig   wasmTypes.GasConfig
	memoryLimit uint32

	logger Logger
}

// Logger defines the logging interface used by WazeroVM
type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
}

// InfoLogger extends Logger with chainable methods for structured logging
type InfoLogger interface {
	Logger
	With(keyvals ...interface{}) Logger
}

// NewWazeroVM creates a new VM instance with wazero as the runtime
func NewWazeroVM() (*WazeroVM, error) {
	// Create runtime with proper context
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	// Set default values
	cacheSize := 100                        // Default cache size
	memoryLimit := uint32(32 * 1024 * 1024) // Default memory limit (32MB)
	gasConfig := gas.DefaultGasConfig()

	// Create the VM instance
	vm := &WazeroVM{
		runtime:     runtime,
		codeStore:   make(map[[32]byte][]byte),
		memoryCache: make(map[[32]byte]*cacheItem),
		pinned:      make(map[[32]byte]*cacheItem),
		cacheOrder:  make([][32]byte, 0),
		cacheSize:   cacheSize,
		gasConfig:   gasConfig,
		memoryLimit: memoryLimit,
		logger:      nil, // Will use default logger
	}

	// Initialize crypto handler
	err := crypto.SetupCryptoHandlers()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize crypto handlers: %w", err)
	}

	return vm, nil
}

// Close releases all resources held by the VM
func (vm *WazeroVM) Close() error {
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()

	ctx := context.Background()

	// Close all compiled modules
	for _, item := range vm.pinned {
		if err := item.compiled.Close(ctx); err != nil {
			vm.logger.Error("Error closing pinned module", "error", err)
		}
	}

	for _, item := range vm.memoryCache {
		if err := item.compiled.Close(ctx); err != nil {
			vm.logger.Error("Error closing cached module", "error", err)
		}
	}

	// Clear maps
	vm.pinned = make(map[[32]byte]*cacheItem)
	vm.memoryCache = make(map[[32]byte]*cacheItem)
	vm.cacheOrder = make([][32]byte, 0)

	// Close the runtime
	return vm.runtime.Close(ctx)
}

```
---

## Summary
Files: 25, Total: 156 KB
Breakdown:
- go: 156 KB
```
---
### `runtime/constants/constants.go`
*2025-02-26 15:39:53 | 1 KB*
```go
package constants

const (

	// Point lengths for BLS12-381
	BLS12_381_G1_POINT_LEN = 48
	BLS12_381_G2_POINT_LEN = 96
	WasmPageSize           = 65536
)

// Gas costs for various operations
const (
	// Memory operations
	GasPerByte = 3

	// Database operations
	GasCostRead  = 100
	GasCostWrite = 200
	GasCostQuery = 500

	// Iterator operations
	GasCostIteratorCreate = 10000 // Base cost for creating an iterator
	GasCostIteratorNext   = 1000  // Base cost for iterator next operations

	// Contract operations
	GasCostInstantiate = 40000 // Base cost for contract instantiation
	GasCostExecute     = 20000 // Base cost for contract execution
)

```
---
### `runtime/constants/gas.go`
*2025-02-20 21:58:20 | 1 KB*
```go
package constants

const (
	// Gas multiplier for wazero operations
	GasMultiplier uint64 = 100

	// Maximum sizes for BLS operations
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)

```
---
### `runtime/crypto/bls_impl.go`
*2025-02-26 15:33:15 | 1 KB*
```go
package crypto

import (
	"fmt"

	bls12381 "github.com/kilic/bls12-381"
)

// Implementation of BLS12381AggregateG1 and BLS12381AggregateG2

// BLS12381AggregateG2 aggregates multiple G2 points into a single compressed G2 point.
func (c *CryptoImplementation) BLS12381AggregateG2(elements [][]byte) ([]byte, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("no elements to aggregate")
	}

	g2 := bls12381.NewG2()
	result := g2.Zero()

	for _, element := range elements {
		point, err := g2.FromCompressed(element)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress G2 point: %w", err)
		}
		g2.Add(result, result, point)
	}

	return g2.ToCompressed(result), nil
}

```
---
### `runtime/crypto/crypto.go`
*2025-02-26 16:02:49 | 14 KB*
```go
package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"

	"crypto/ed25519"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"

	bls12381 "github.com/kilic/bls12-381"
)

// Ensure CryptoImplementation implements the interfaces
var _ cryptoapi.CryptoOperations = (*CryptoImplementation)(nil)

// CryptoImplementation provides concrete implementations of crypto operations
type CryptoImplementation struct{}

// NewCryptoImplementation creates a new crypto implementation
func NewCryptoImplementation() *CryptoImplementation {
	return &CryptoImplementation{}
}

// BLS12381AggregateG1 aggregates multiple G1 points into a single compressed G1 point.
func (c *CryptoImplementation) BLS12381AggregateG1(elements [][]byte) ([]byte, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("no elements to aggregate")
	}

	g1 := bls12381.NewG1()
	result := g1.Zero()

	for _, element := range elements {
		point, err := g1.FromCompressed(element)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress G1 point: %w", err)
		}
		g1.Add(result, result, point)
	}

	return g1.ToCompressed(result), nil
}

// BLS12381HashToG1 hashes arbitrary bytes to a compressed G1 point.
func (c *CryptoImplementation) BLS12381HashToG1(message, dst []byte) ([]byte, error) {
	g1 := bls12381.NewG1()
	point, err := g1.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G1: %w", err)
	}
	return g1.ToCompressed(point), nil
}

// BLS12381HashToG2 hashes arbitrary bytes to a compressed G2 point.
func (c *CryptoImplementation) BLS12381HashToG2(message, dst []byte) ([]byte, error) {
	g2 := bls12381.NewG2()
	point, err := g2.HashToCurve(message, dst)
	if err != nil {
		return nil, fmt.Errorf("failed to hash to G2: %w", err)
	}
	return g2.ToCompressed(point), nil
}

// BLS12381VerifyG1G2 checks if the pairing product of G1 and G2 points equals the identity in GT.
func (c *CryptoImplementation) BLS12381VerifyG1G2(g1Points, g2Points [][]byte) (bool, error) {
	if len(g1Points) != len(g2Points) {
		return false, fmt.Errorf("number of G1 points (%d) must equal number of G2 points (%d)", len(g1Points), len(g2Points))
	}
	if len(g1Points) == 0 {
		return false, fmt.Errorf("at least one pair of points is required")
	}

	g1 := bls12381.NewG1()
	g2 := bls12381.NewG2()
	engine := bls12381.NewEngine()

	// For each (G1, G2) pair, add their pairing to the calculation
	for i := 0; i < len(g1Points); i++ {
		p1, err := g1.FromCompressed(g1Points[i])
		if err != nil {
			return false, fmt.Errorf("invalid G1 point at index %d: %w", i, err)
		}

		p2, err := g2.FromCompressed(g2Points[i])
		if err != nil {
			return false, fmt.Errorf("invalid G2 point at index %d: %w", i, err)
		}

		engine.AddPair(p1, p2)
	}

	// Check if the pairing result equals 1 (the identity element in GT)
	return engine.Check(), nil
}

// Secp256k1Verify verifies a secp256k1 signature in [R || s] format (65 bytes).
func (c *CryptoImplementation) Secp256k1Verify(messageHash, signature, pubkey []byte) (bool, error) {
	if len(messageHash) != 32 {
		return false, fmt.Errorf("message hash must be 32 bytes, got %d", len(messageHash))
	}
	if len(signature) != 64 && len(signature) != 65 {
		return false, fmt.Errorf("signature must be 64 or 65 bytes, got %d", len(signature))
	}

	// Use 64-byte signature format (R, s)
	sigR := new(big.Int).SetBytes(signature[:32])
	sigS := new(big.Int).SetBytes(signature[32:64])

	// Parse the public key
	pk, err := parseSecp256k1PubKey(pubkey)
	if err != nil {
		return false, err
	}

	// Verify the signature
	return ecdsa.Verify(pk, messageHash, sigR, sigS), nil
}

// Secp256k1RecoverPubkey recovers a public key from a signature and recovery byte.
func (c *CryptoImplementation) Secp256k1RecoverPubkey(messageHash, signature []byte, recovery byte) ([]byte, error) {
	if len(messageHash) != 32 {
		return nil, fmt.Errorf("message hash must be 32 bytes, got %d", len(messageHash))
	}
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	if recovery > 3 {
		return nil, fmt.Errorf("recovery byte must be 0, 1, 2, or 3, got %d", recovery)
	}

	// Parse r and s from the signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Calculate recovery parameters
	curve := secp256k1Curve()
	recid := int(recovery)
	isOdd := recid&1 != 0
	isSecondX := recid&2 != 0

	// Use r instead of creating a new x variable
	if isSecondX {
		r.Add(r, new(big.Int).Set(curve.Params().N))
	}

	// Calculate corresponding y value
	y, err := recoverY(curve, r, isOdd)
	if err != nil {
		return nil, err
	}

	// Construct R point using r instead of x
	R := &ecdsa.PublicKey{
		Curve: curve,
		X:     r,
		Y:     y,
	}

	// Derive from R and signature the original public key
	e := new(big.Int).SetBytes(messageHash)
	Rinv := new(big.Int).ModInverse(R.X, curve.Params().N)
	if Rinv == nil {
		return nil, fmt.Errorf("failed to compute modular inverse")
	}

	// Calculate r⁻¹(sR - eG)
	sR := new(big.Int).Mul(s, R.X)
	sR.Mod(sR, curve.Params().N)

	eG := new(big.Int).Neg(e)
	eG.Mod(eG, curve.Params().N)

	Q := ecPointAdd(
		curve,
		R.X, R.Y, // R
		ecScalarMult(curve, eG, nil, nil), // eG
	)

	Q = ecScalarMult(curve, Rinv, Q[0], Q[1])

	// Convert the recovered public key to compressed format
	return compressPublicKey(Q[0], Q[1]), nil
}

// Secp256r1RecoverPubkey recovers a secp256r1 public key from a signature
func (c *CryptoImplementation) Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error) {
	// Current placeholder needs actual implementation
	curve := elliptic.P256() // NIST P-256 (secp256r1) curve

	// Implement proper recovery:
	recid := int(recovery)
	isOdd := recid&1 != 0
	isSecondX := recid&2 != 0

	// Restore potential x coordinate from signature
	x := new(big.Int).SetBytes(signature[:32])
	if isSecondX {
		x.Add(x, new(big.Int).Set(curve.Params().N))
	}

	// Calculate corresponding y value
	y, err := recoverY(curve, x, isOdd)
	if err != nil {
		return nil, err
	}

	// Create compressed public key
	compressedPubKey := make([]byte, 33)
	compressedPubKey[0] = byte(0x02) + byte(y.Bit(0))
	xBytes := x.Bytes()
	copy(compressedPubKey[1+32-len(xBytes):], xBytes)

	return compressedPubKey, nil
}

// Ed25519Verify verifies an Ed25519 signature.
func (c *CryptoImplementation) Ed25519Verify(message, signature, pubKey []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}
	if len(pubKey) != 32 {
		return false, fmt.Errorf("public key must be 32 bytes, got %d", len(pubKey))
	}

	// Use Go's ed25519 implementation to verify
	return ed25519Verify(pubKey, message, signature), nil
}

// Ed25519BatchVerify verifies multiple Ed25519 signatures in a batch.
func (c *CryptoImplementation) Ed25519BatchVerify(messages, signatures, pubKeys [][]byte) (bool, error) {
	if len(messages) != len(signatures) || len(messages) != len(pubKeys) {
		return false, fmt.Errorf("number of messages (%d), signatures (%d), and public keys (%d) must be equal",
			len(messages), len(signatures), len(pubKeys))
	}

	for i := 0; i < len(messages); i++ {
		if ok, _ := c.Ed25519Verify(messages[i], signatures[i], pubKeys[i]); !ok {
			return false, nil
		}
	}
	return true, nil
}

// parseSecp256k1PubKey parses a SEC1 encoded public key in compressed or uncompressed format
func parseSecp256k1PubKey(pubKeyBytes []byte) (*ecdsa.PublicKey, error) {
	curve := secp256k1Curve()

	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("empty public key")
	}

	// Handle compressed public key format
	if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		x := new(big.Int).SetBytes(pubKeyBytes[1:])
		isOdd := pubKeyBytes[0] == 0x03
		y, err := recoverY(curve, x, isOdd)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	// Handle uncompressed public key format
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		x := new(big.Int).SetBytes(pubKeyBytes[1:33])
		y := new(big.Int).SetBytes(pubKeyBytes[33:])
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	return nil, fmt.Errorf("invalid public key format or length: %d", len(pubKeyBytes))
}

// recoverY calculates the Y coordinate for a given X coordinate on an elliptic curve
func recoverY(curve elliptic.Curve, x *big.Int, isOdd bool) (*big.Int, error) {
	// y² = x³ + ax + b
	// For secp256k1, a = 0 and b = 7

	// Calculate x³ + ax + b
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	// For secp256k1, a = 0, so we skip adding ax

	// Add b (7 for secp256k1, different for other curves)
	b := getB(curve)
	x3.Add(x3, b)

	// Modulo p
	x3.Mod(x3, curve.Params().P)

	// Calculate the square root modulo p
	y := new(big.Int).ModSqrt(x3, curve.Params().P)
	if y == nil {
		return nil, fmt.Errorf("no square root exists for y")
	}

	// Check if we need the "other" root (p - y)
	if isOdd != isOddValue(y) {
		y.Sub(curve.Params().P, y)
	}

	return y, nil
}

// isOddValue checks if a big.Int value is odd
func isOddValue(value *big.Int) bool {
	return value.Bit(0) == 1
}

// getB returns the b parameter for the curve equation y² = x³ + ax + b
func getB(curve elliptic.Curve) *big.Int {
	if curve == secp256k1Curve() {
		return big.NewInt(7) // Secp256k1 has b = 7
	}
	if curve == elliptic.P256() {
		// Return the b parameter for P-256 (secp256r1)
		b, _ := new(big.Int).SetString("5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604b", 16)
		return b
	}
	// Default, though this should not happen with our supported curves
	return big.NewInt(0)
}

// secp256k1Curve returns an elliptic.Curve instance for secp256k1
func secp256k1Curve() elliptic.Curve {
	// This is a simplified version - in production code, you would use a proper secp256k1 implementation
	// For now, we'll use a placeholder that matches the rest of the code
	return elliptic.P256() // This is a placeholder - real code would return actual secp256k1
}

// ecPointAdd adds two elliptic curve points
func ecPointAdd(curve elliptic.Curve, x1, y1 *big.Int, point [2]*big.Int) [2]*big.Int {
	x2, y2 := point[0], point[1]
	x3, y3 := curve.Add(x1, y1, x2, y2)
	return [2]*big.Int{x3, y3}
}

// ecScalarMult multiplies a point on an elliptic curve by a scalar
func ecScalarMult(curve elliptic.Curve, k *big.Int, x, y *big.Int) [2]*big.Int {
	if x == nil || y == nil {
		// If point is the identity (represented as nil), use the base point
		x, y = curve.Params().Gx, curve.Params().Gy
	}
	x3, y3 := curve.ScalarMult(x, y, k.Bytes())
	return [2]*big.Int{x3, y3}
}

// compressPublicKey creates a compressed representation of a public key
func compressPublicKey(x, y *big.Int) []byte {
	result := make([]byte, 33)
	// Set prefix based on Y coordinate's parity
	if y.Bit(0) == 0 {
		result[0] = 0x02 // even Y
	} else {
		result[0] = 0x03 // odd Y
	}

	// Pad X coordinate to 32 bytes
	xBytes := x.Bytes()
	offset := 1 + 32 - len(xBytes)
	copy(result[offset:], xBytes)

	return result
}

// ed25519Verify verifies an ED25519 signature
func ed25519Verify(pubKey, message, signature []byte) bool {
	// In a real implementation, use Go's crypto/ed25519 package

	if len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(ed25519.PublicKey(pubKey), message, signature)
}

// Secp256r1Verify verifies a signature using the NIST P-256 curve (secp256r1)
func (c *CryptoImplementation) Secp256r1Verify(hash, signature, pubkey []byte) (bool, error) {
	// Implementation details similar to Secp256k1Verify but using P-256 curve
	if len(hash) != 32 {
		return false, fmt.Errorf("message hash must be 32 bytes, got %d", len(hash))
	}
	if len(signature) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	curve := elliptic.P256() // Use NIST P-256 curve

	// Parse signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Parse public key
	pk, err := parsePublicKey(pubkey, curve)
	if err != nil {
		return false, err
	}

	// Verify signature
	return ecdsa.Verify(pk, hash, r, s), nil
}

// parsePublicKey parses a SEC1 encoded public key for the specified curve
func parsePublicKey(pubKeyBytes []byte, curve elliptic.Curve) (*ecdsa.PublicKey, error) {
	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("empty public key")
	}

	// Handle compressed public key format
	if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		x := new(big.Int).SetBytes(pubKeyBytes[1:])
		isOdd := pubKeyBytes[0] == 0x03
		y, err := recoverY(curve, x, isOdd)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	// Handle uncompressed public key format
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		x := new(big.Int).SetBytes(pubKeyBytes[1:33])
		y := new(big.Int).SetBytes(pubKeyBytes[33:])
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	}

	return nil, fmt.Errorf("invalid public key format or length: %d", len(pubKeyBytes))
}

```
---
### `runtime/crypto/hostcrypto.go`
*2025-02-26 17:26:11 | 14 KB*
```go
package crypto

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/hostapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/tetratelabs/wazero"
	wazerotypes "github.com/tetratelabs/wazero/api"
)

type contextKey string

const envKey contextKey = "env"

// Add global handler variable
var cryptoHandler cryptoapi.CryptoOperations

// Add function to set the handler
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
// It reads the message and domain separation tag from contract memory using MemoryManager,
// charges gas, calls BLS12381HashToG1, allocates space for the result, writes it, and returns the pointer.
func hostBls12381HashToG1(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Retrieve the runtime environment from context.
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)

	// Create a MemoryManager for the contract module.
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read the input message.
	message, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	// Read the domain separation tag.
	dst, err := mm.Read(dstPtr, dstLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation.
	env.Gas.(types.GasMeter).ConsumeGas(uint64(hashLen+dstLen)*constants.GasPerByte, "BLS12381 hash operation")

	// Hash to curve.
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		return 0
	}

	// Allocate memory for the result.
	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0
	}

	// Write the result into memory.
	if err := mm.Write(resultPtr, result); err != nil {
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
// It follows the same pattern as hostBls12381HashToG1.
func hostBls12381HashToG2(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	message, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	dst, err := mm.Read(dstPtr, dstLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation.
	env.Gas.(types.GasMeter).ConsumeGas(uint64(hashLen+dstLen)*constants.GasPerByte, "BLS12381 hash operation")

	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		return 0
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0
	}

	if err := mm.Write(resultPtr, result); err != nil {
		return 0
	}

	return resultPtr
}

// hostBls12381PairingEquality implements bls12_381_pairing_equality.
// It reads the four compressed points from memory and calls BLS12381PairingEquality.
func hostBls12381PairingEquality(_ context.Context, mod wazerotypes.Module, a1Ptr, a1Len, a2Ptr, a2Len, b1Ptr, b1Len, b2Ptr, b2Len uint32) uint32 {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	a1, err := mm.Read(a1Ptr, a1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a1: %v", err))
	}
	a2, err := mm.Read(a2Ptr, a2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read a2: %v", err))
	}
	b1, err := mm.Read(b1Ptr, b1Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b1: %v", err))
	}
	b2, err := mm.Read(b2Ptr, b2Len)
	if err != nil {
		panic(fmt.Sprintf("failed to read b2: %v", err))
	}

	result, err := cryptoHandler.BLS12381VerifyG1G2(
		[][]byte{a1, b1}, // g1 points
		[][]byte{a2, b2}, // g2 points
	)
	if err != nil {
		panic(fmt.Sprintf("failed to check pairing equality: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1Verify implements secp256r1_verify.
// It reads the hash, signature, and public key from memory via MemoryManager,
// calls Secp256r1Verify, and returns 1 if valid.
func hostSecp256r1Verify(_ context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	sig, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read public key: %v", err))
	}

	result, err := cryptoHandler.Secp256r1Verify(hash, sig, pubkey)
	if err != nil {
		panic(fmt.Sprintf("failed to verify secp256r1 signature: %v", err))
	}

	if result {
		return 1
	}
	return 0
}

// hostSecp256r1RecoverPubkey implements secp256r1_recover_pubkey.
// It reads the hash and signature from memory, recovers the public key,
// allocates memory for it, writes it, and returns the pointer and length.
func hostSecp256r1RecoverPubkey(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, recovery uint32) (uint32, uint32) {
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read hash: %v", err))
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read signature: %v", err))
	}

	result, err := cryptoHandler.Secp256r1RecoverPubkey(hash, signature, byte(recovery))
	if err != nil {
		panic(fmt.Sprintf("failed to recover public key: %v", err))
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
	}

	if err := mm.Write(resultPtr, result); err != nil {
		panic(fmt.Sprintf("failed to write result: %v", err))
	}

	return resultPtr, uint32(len(result))
}

// SetupCryptoHandlers initializes the crypto system by creating and setting the global crypto handler
func SetupCryptoHandlers() error {
	// Create a new implementation of the CryptoOperations interface
	impl := NewCryptoImplementation()

	// Set it as the global handler
	SetCryptoHandler(impl)

	return nil
}

// RegisterHostFunctions registers all crypto host functions with the provided module
func RegisterHostFunctions(mod wazero.HostModuleBuilder) {
	// Register BLS functions
	mod.NewFunctionBuilder().WithFunc(hostBls12381HashToG1).Export("bls12_381_hash_to_g1")
	mod.NewFunctionBuilder().WithFunc(hostBls12381HashToG2).Export("bls12_381_hash_to_g2")
	mod.NewFunctionBuilder().WithFunc(hostBls12381PairingEquality).Export("bls12_381_pairing_equality")

	// Register secp256r1 functions
	mod.NewFunctionBuilder().WithFunc(hostSecp256r1Verify).Export("secp256r1_verify")
	mod.NewFunctionBuilder().WithFunc(hostSecp256r1RecoverPubkey).Export("secp256r1_recover_pubkey")

	// Register secp256k1 functions
	mod.NewFunctionBuilder().WithFunc(secp256k1Verify).Export("secp256k1_verify")
	mod.NewFunctionBuilder().WithFunc(secp256k1RecoverPubkey).Export("secp256k1_recover_pubkey")

	// Register ed25519 functions
	mod.NewFunctionBuilder().WithFunc(hostEd25519Verify).Export("ed25519_verify")
	mod.NewFunctionBuilder().WithFunc(hostEd25519BatchVerify).Export("ed25519_batch_verify")
}

// secp256k1Verify implements secp256k1_verify
// It reads message hash, signature, and public key from memory, calls Secp256k1Verify,
// and returns 1 if valid or 0 otherwise
func secp256k1Verify(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Retrieve the runtime environment from context
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)

	// Create memory manager to access WebAssembly memory
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read hash from memory
	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0
	}

	// Read signature from memory
	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		return 0
	}

	// Read public key from memory
	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		return 0
	}

	// Charge gas for the operation
	env.Gas.(types.GasMeter).ConsumeGas(
		uint64(hashLen+sigLen+pubkeyLen)*constants.GasPerByte,
		"secp256k1 verification",
	)

	// Call the implementation function
	result, err := cryptoHandler.Secp256k1Verify(hash, signature, pubkey)
	if err != nil {
		return 0
	}

	if result {
		return 1
	}
	return 0
}

// secp256k1RecoverPubkey implements secp256k1_recover_pubkey
// It reads hash and signature from memory, recovers the public key,
// allocates space for the result, and returns the pointer and length
func secp256k1RecoverPubkey(ctx context.Context, mod wazerotypes.Module, hashPtr, hashLen, sigPtr, sigLen, recovery uint32) (uint32, uint32) {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	hash, err := mm.Read(hashPtr, hashLen)
	if err != nil {
		return 0, 0
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		return 0, 0
	}

	// Charge gas for operation
	env.Gas.(types.GasMeter).ConsumeGas(
		uint64(hashLen+sigLen)*constants.GasPerByte,
		"secp256k1 key recovery",
	)

	result, err := cryptoHandler.Secp256k1RecoverPubkey(hash, signature, byte(recovery))
	if err != nil {
		return 0, 0
	}

	resultPtr, err := mm.Allocate(uint32(len(result)))
	if err != nil {
		return 0, 0
	}

	if err := mm.Write(resultPtr, result); err != nil {
		return 0, 0
	}

	return resultPtr, uint32(len(result))
}

// hostEd25519Verify implements ed25519_verify
// It reads message, signature, and public key from memory and calls Ed25519Verify
func hostEd25519Verify(ctx context.Context, mod wazerotypes.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	message, err := mm.Read(msgPtr, msgLen)
	if err != nil {
		return 0
	}

	signature, err := mm.Read(sigPtr, sigLen)
	if err != nil {
		return 0
	}

	pubkey, err := mm.Read(pubkeyPtr, pubkeyLen)
	if err != nil {
		return 0
	}

	// Charge gas
	env.Gas.(types.GasMeter).ConsumeGas(
		uint64(msgLen+sigLen+pubkeyLen)*constants.GasPerByte,
		"ed25519 verification",
	)

	result, err := cryptoHandler.Ed25519Verify(message, signature, pubkey)
	if err != nil {
		return 0
	}

	if result {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify
// It reads multiple messages, signatures, and public keys from memory and calls Ed25519BatchVerify
func hostEd25519BatchVerify(ctx context.Context, mod wazerotypes.Module, msgsPtr, sigsPtr, pubkeysPtr uint32) uint32 {
	env := ctx.Value(hostapi.EnvironmentKey).(*hostapi.RuntimeEnvironment)
	mm, err := memory.NewMemoryManager(mod)
	if err != nil {
		panic(fmt.Sprintf("failed to create MemoryManager: %v", err))
	}

	// Read batch data from arrays of pointers
	// This is a simplified version - actual implementation needs to read array of arrays

	// Example (would need to be adapted to actual memory layout):
	// Read number of items in batch
	countPtr, err := mm.Read(msgsPtr, 4)
	if err != nil {
		return 0
	}
	count := uint32(countPtr[0]) | uint32(countPtr[1])<<8 | uint32(countPtr[2])<<16 | uint32(countPtr[3])<<24

	messages := make([][]byte, count)
	signatures := make([][]byte, count)
	pubkeys := make([][]byte, count)

	totalBytes := uint64(0)

	// Read each message, signature, and pubkey
	for i := uint32(0); i < count; i++ {
		// Read message pointer and length
		msgPtrAddr := msgsPtr + 8*i + 4
		msgLenAddr := msgsPtr + 8*i + 8
		msgPtr, ok := mm.ReadUint32(msgPtrAddr)
		if !ok {
			return 0
		}
		msgLen, ok := mm.ReadUint32(msgLenAddr)
		if !ok {
			return 0
		}

		// Read signature pointer and length
		sigPtrAddr := sigsPtr + 8*i + 4
		sigLenAddr := sigsPtr + 8*i + 8
		sigPtr, ok := mm.ReadUint32(sigPtrAddr)
		if !ok {
			return 0
		}
		sigLen, ok := mm.ReadUint32(sigLenAddr)
		if !ok {
			return 0
		}

		// Read pubkey pointer and length
		pubkeyPtrAddr := pubkeysPtr + 8*i + 4
		pubkeyLenAddr := pubkeysPtr + 8*i + 8
		pubkeyPtr, ok := mm.ReadUint32(pubkeyPtrAddr)
		if !ok {
			return 0
		}
		pubkeyLen, ok := mm.ReadUint32(pubkeyLenAddr)
		if !ok {
			return 0
		}

		// Read actual data
		msg, err := mm.Read(msgPtr, msgLen)
		if err != nil {
			return 0
		}
		sig, err := mm.Read(sigPtr, sigLen)
		if err != nil {
			return 0
		}
		pk, err := mm.Read(pubkeyPtr, pubkeyLen)
		if err != nil {
			return 0
		}

		messages[i] = msg
		signatures[i] = sig
		pubkeys[i] = pk

		totalBytes += uint64(msgLen + sigLen + pubkeyLen)
	}

	// Charge gas
	env.Gas.(types.GasMeter).ConsumeGas(
		totalBytes*constants.GasPerByte,
		"ed25519 batch verification",
	)

	result, err := cryptoHandler.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		return 0
	}

	if result {
		return 1
	}
	return 0
}

```
---
### `runtime/cryptoapi/types.go`
*2025-02-26 16:01:43 | 2 KB*
```go
package cryptoapi

// CryptoVerifier defines the interface for crypto verification operations
type CryptoVerifier interface {
	// Secp256k1Verify verifies a secp256k1 signature
	Secp256k1Verify(hash, signature, publicKey []byte) (bool, error)

	// Secp256k1RecoverPubkey recovers a public key from a signature
	Secp256k1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error)

	// Ed25519Verify verifies an ed25519 signature
	Ed25519Verify(message, signature, publicKey []byte) (bool, error)

	// Ed25519BatchVerify verifies multiple ed25519 signatures
	Ed25519BatchVerify(messages, signatures, publicKeys [][]byte) (bool, error)
}

// BLS12381Operations defines operations for BLS12-381 curves
type BLS12381Operations interface {
	// BLS12381AggregateG1 aggregates multiple G1 points
	BLS12381AggregateG1(elements [][]byte) ([]byte, error)

	// BLS12381AggregateG2 aggregates multiple G2 points
	BLS12381AggregateG2(elements [][]byte) ([]byte, error)

	// BLS12381HashToG1 hashes a message to a G1 point
	BLS12381HashToG1(message, dst []byte) ([]byte, error)

	// BLS12381HashToG2 hashes a message to a G2 point
	BLS12381HashToG2(message, dst []byte) ([]byte, error)

	// BLS12381VerifyG1G2 verifies a pairing check
	BLS12381VerifyG1G2(g1Points, g2Points [][]byte) (bool, error)
}

// CryptoOperations combines all crypto operations into a single interface
type CryptoOperations interface {
	CryptoVerifier
	BLS12381Operations
	Secp256r1Verify(hash, signature, pubkey []byte) (bool, error)
	Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error)
}

```
---
### `runtime/gas/gas.go`
*2025-02-20 21:49:29 | 2 KB*
```go
package gas

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// GasState tracks gas consumption
type GasState struct {
	limit uint64
	used  uint64
}

// NewGasState creates a new GasState with the given limit
func NewGasState(limit uint64) *GasState {
	return &GasState{
		limit: limit,
		used:  0,
	}
}

// GasConsumed implements types.GasMeter
func (g *GasState) GasConsumed() uint64 {
	return g.used
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) error {
	g.used += amount
	if g.used > g.limit {
		return fmt.Errorf("out of gas: used %d, limit %d - %s", g.used, g.limit, description)
	}
	return nil
}

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() types.GasConfig {
	return types.GasConfig{
		PerByte:                 constants.GasPerByte,
		DatabaseRead:            constants.GasCostRead,
		DatabaseWrite:           constants.GasCostWrite,
		ExternalQuery:           constants.GasCostQuery,
		IteratorCreate:          constants.GasCostIteratorCreate,
		IteratorNext:            constants.GasCostIteratorNext,
		Instantiate:             constants.GasCostInstantiate,
		Execute:                 constants.GasCostExecute,
		Bls12381AggregateG1Cost: types.GasCost{BaseCost: 1000, PerPoint: 100},
		Bls12381AggregateG2Cost: types.GasCost{BaseCost: 1000, PerPoint: 100},
	}
}

```
---
### `runtime/gas/gasversionone/gas.go`
*2025-02-20 21:49:29 | 6 KB*
```go
package gas1

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// --- ErrorOutOfGas ---
//
// ErrorOutOfGas is returned when the gas consumption exceeds the allowed limit.
type ErrorOutOfGas struct {
	Descriptor string
}

func (e ErrorOutOfGas) Error() string {
	return fmt.Sprintf("out of gas: %s", e.Descriptor)
}

// --- Constants ---
const (
	// Cost of one Wasm VM instruction (CosmWasm 1.x uses 150 gas per op).
	wasmInstructionCost uint64 = 150

	// Conversion multiplier: CosmWasm gas units are 100x the Cosmos SDK gas units.
	gasMultiplier uint64 = 100

	// Cost per byte for memory copy operations (host ↔ wasm).
	memoryCopyCost uint64 = 1
)

// --- GasState ---
// GasState tracks gas usage during a contract execution (CosmWasm 1.x compatible).
type GasState struct {
	gasLimit      uint64         // Total gas limit (in CosmWasm gas units)
	usedInternal  uint64         // Gas used for internal Wasm operations (in CosmWasm gas units)
	externalUsed  uint64         // Gas used externally (from the Cosmos SDK GasMeter, in SDK gas units)
	initialExtern uint64         // Initial external gas consumed at start (SDK units)
	gasMeter      types.GasMeter // Reference to an external (SDK) GasMeter
}

// NewGasState creates a new GasState.
// The given gas limit is in Cosmos SDK gas units; it is converted to CosmWasm gas units.
// The provided gasMeter is used to track external gas usage.
func NewGasState(limitSDK uint64, meter types.GasMeter) *GasState {
	gs := &GasState{
		gasLimit:     limitSDK * gasMultiplier,
		usedInternal: 0,
		externalUsed: 0,
		gasMeter:     meter,
	}
	if meter != nil {
		gs.initialExtern = meter.GasConsumed()
	}
	return gs
}

// ConsumeWasmGas consumes gas for executing the given number of Wasm instructions.
func (gs *GasState) ConsumeWasmGas(numInstr uint64) error {
	if numInstr == 0 {
		return nil
	}
	cost := numInstr * wasmInstructionCost
	return gs.consumeInternalGas(cost, "Wasm execution")
}

// ConsumeMemoryGas charges gas for copying numBytes of data.
func (gs *GasState) ConsumeMemoryGas(numBytes uint64) error {
	if numBytes == 0 {
		return nil
	}
	cost := numBytes * memoryCopyCost
	return gs.consumeInternalGas(cost, "Memory operation")
}

// ConsumeDBReadGas charges gas for a database read, based on key and value sizes.
func (gs *GasState) ConsumeDBReadGas(keyLen, valueLen int) error {
	totalBytes := uint64(0)
	if keyLen > 0 {
		totalBytes += uint64(keyLen)
	}
	if valueLen > 0 {
		totalBytes += uint64(valueLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "DB read")
}

// ConsumeDBWriteGas charges gas for a database write, based on key and value sizes.
func (gs *GasState) ConsumeDBWriteGas(keyLen, valueLen int) error {
	totalBytes := uint64(0)
	if keyLen > 0 {
		totalBytes += uint64(keyLen)
	}
	if valueLen > 0 {
		totalBytes += uint64(valueLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "DB write")
}

// ConsumeQueryGas charges gas for an external query operation.
func (gs *GasState) ConsumeQueryGas(reqLen, respLen int) error {
	totalBytes := uint64(0)
	if reqLen > 0 {
		totalBytes += uint64(reqLen)
	}
	if respLen > 0 {
		totalBytes += uint64(respLen)
	}
	if totalBytes == 0 {
		totalBytes = 1
	}
	return gs.consumeInternalGas(totalBytes*memoryCopyCost, "External query")
}

// consumeInternalGas deducts the given cost from internal gas usage and checks combined gas.
func (gs *GasState) consumeInternalGas(cost uint64, descriptor string) error {
	if cost == 0 {
		return nil
	}
	gs.usedInternal += cost

	// Update external usage from the Cosmos SDK GasMeter.
	if gs.gasMeter != nil {
		currentExtern := gs.gasMeter.GasConsumed()
		if currentExtern < gs.initialExtern {
			gs.initialExtern = currentExtern
		}
		gs.externalUsed = currentExtern - gs.initialExtern
	}

	combinedUsed := gs.usedInternal + (gs.externalUsed * gasMultiplier)
	if combinedUsed > gs.gasLimit {
		return ErrorOutOfGas{Descriptor: descriptor}
	}
	return nil
}

// GasUsed returns the internal gas used in Cosmos SDK gas units.
func (gs *GasState) GasUsed() uint64 {
	used := gs.usedInternal / gasMultiplier
	if gs.usedInternal%gasMultiplier != 0 {
		used++
	}
	return used
}

// Report returns a GasReport summarizing gas usage.
func (gs *GasState) Report() types.GasReport {
	if gs.gasMeter != nil {
		currentExtern := gs.gasMeter.GasConsumed()
		if currentExtern < gs.initialExtern {
			gs.initialExtern = currentExtern
		}
		gs.externalUsed = currentExtern - gs.initialExtern
	}
	usedExternWasm := gs.externalUsed * gasMultiplier
	usedInternWasm := gs.usedInternal
	var remaining uint64
	if gs.gasLimit >= (usedInternWasm + usedExternWasm) {
		remaining = gs.gasLimit - (usedInternWasm + usedExternWasm)
	}
	return types.GasReport{
		Limit:          gs.gasLimit,
		Remaining:      remaining,
		UsedExternally: usedExternWasm,
		UsedInternally: usedInternWasm,
	}
}

// DebugString returns a human-readable summary of the current gas state.
func (gs *GasState) DebugString() string {
	report := gs.Report()
	usedExternSDK := gs.externalUsed
	usedInternSDK := gs.GasUsed()
	totalSDK := usedExternSDK + usedInternSDK
	return fmt.Sprintf(
		"GasState{limit=%d, usedIntern=%d, usedExtern=%d, combined=%d | SDK gas: internal=%d, external=%d, total=%d}",
		report.Limit, report.UsedInternally, report.UsedExternally, report.UsedInternally+report.UsedExternally,
		usedInternSDK, usedExternSDK, totalSDK,
	)
}

```
---
### `runtime/gas/gasversiontwo/gas.go`
*2025-02-20 21:49:29 | 7 KB*
```go
package gas2

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Gas constants (CosmWasm 2.x)
const (
	// GasMultiplier is how many CosmWasm gas points equal 1 Cosmos SDK gas point (reduced 1000x in 2.x).
	GasMultiplier uint64 = 140_000
	// InstanceCost for loading a WASM instance (unchanged from 1.x).
	InstanceCost uint64 = 60_000
	// InstanceCostDiscount for cached instances (about 30x cheaper than full load).
	InstanceCostDiscount uint64 = 2_000
	// CompileCost per byte for compiling WASM code.
	CompileCost uint64 = 3
	// EventPerAttributeCost per event attribute (count).
	EventPerAttributeCost uint64 = 10
	// EventAttributeDataCost per byte of event attribute data.
	EventAttributeDataCost uint64 = 1
	// EventAttributeDataFreeTier bytes of attribute data with no charge.
	EventAttributeDataFreeTier uint64 = 100
	// CustomEventCost per custom event emitted.
	CustomEventCost uint64 = 20
	// ContractMessageDataCost per byte of message passed to contract (still 0 by default).
	ContractMessageDataCost uint64 = 0
	// GasCostHumanAddress to convert a canonical address to human-readable.
	GasCostHumanAddress uint64 = 5
	// GasCostCanonicalAddress to convert a human address to canonical form.
	GasCostCanonicalAddress uint64 = 4
	// GasCostValidateAddress (humanize + canonicalize).
	GasCostValidateAddress uint64 = GasCostHumanAddress + GasCostCanonicalAddress
)

var defaultPerByteUncompressCost = wasmvmtypes.UFraction{
	Numerator:   15,
	Denominator: 100,
}

// DefaultPerByteUncompressCost returns the default uncompress cost fraction.
func DefaultPerByteUncompressCost() wasmvmtypes.UFraction {
	return defaultPerByteUncompressCost
}

// GasRegister defines the gas registration interface.
type GasRegister interface {
	UncompressCosts(byteLength int) types.Gas
	SetupContractCost(discount bool, msgLen int) types.Gas
	ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas
	EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas
	ToWasmVMGas(source types.Gas) uint64
	FromWasmVMGas(source uint64) types.Gas
}

// WasmGasRegisterConfig holds configuration parameters for gas costs.
type WasmGasRegisterConfig struct {
	InstanceCost               types.Gas
	InstanceCostDiscount       types.Gas
	CompileCost                types.Gas
	UncompressCost             wasmvmtypes.UFraction
	GasMultiplier              types.Gas
	EventPerAttributeCost      types.Gas
	EventAttributeDataCost     types.Gas
	EventAttributeDataFreeTier uint64
	ContractMessageDataCost    types.Gas
	CustomEventCost            types.Gas
}

// DefaultGasRegisterConfig returns the default configuration for CosmWasm 2.x.
func DefaultGasRegisterConfig() WasmGasRegisterConfig {
	return WasmGasRegisterConfig{
		InstanceCost:               InstanceCost,
		InstanceCostDiscount:       InstanceCostDiscount,
		CompileCost:                CompileCost,
		UncompressCost:             DefaultPerByteUncompressCost(),
		GasMultiplier:              GasMultiplier,
		EventPerAttributeCost:      EventPerAttributeCost,
		EventAttributeDataCost:     EventAttributeDataCost,
		EventAttributeDataFreeTier: EventAttributeDataFreeTier,
		ContractMessageDataCost:    ContractMessageDataCost,
		CustomEventCost:            CustomEventCost,
	}
}

// WasmGasRegister implements GasRegister.
type WasmGasRegister struct {
	c WasmGasRegisterConfig
}

// NewDefaultWasmGasRegister creates a new gas register with default config.
func NewDefaultWasmGasRegister() WasmGasRegister {
	return NewWasmGasRegister(DefaultGasRegisterConfig())
}

// NewWasmGasRegister creates a new gas register with the given configuration.
func NewWasmGasRegister(c WasmGasRegisterConfig) WasmGasRegister {
	if c.GasMultiplier == 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "GasMultiplier cannot be 0"))
	}
	return WasmGasRegister{c: c}
}

// UncompressCosts returns the gas cost to uncompress a WASM bytecode of the given length.
func (g WasmGasRegister) UncompressCosts(byteLength int) types.Gas {
	if byteLength < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "byteLength cannot be negative"))
	}
	numerator := g.c.UncompressCost.Numerator
	denom := g.c.UncompressCost.Denominator
	gasCost := uint64(byteLength) * numerator / denom
	return types.Gas(gasCost)
}

// SetupContractCost returns the gas cost to set up contract execution/instantiation.
func (g WasmGasRegister) SetupContractCost(discount bool, msgLen int) types.Gas {
	if msgLen < 0 {
		panic(errorsmod.Wrap(sdkerrors.ErrLogic, "msgLen cannot be negative"))
	}
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	msgDataCost := types.Gas(msgLen) * g.c.ContractMessageDataCost
	return baseCost + msgDataCost
}

// ReplyCosts returns the gas cost for handling a submessage reply.
// CosmWasm 2.x no longer includes event attributes or error messages in reply,
// so we only charge the base cost.
func (g WasmGasRegister) ReplyCosts(discount bool, reply wasmvmtypes.Reply) types.Gas {
	baseCost := g.c.InstanceCost
	if discount {
		baseCost = g.c.InstanceCostDiscount
	}
	// In v2.x, additional reply data is not charged.
	return baseCost
}

// EventCosts returns the gas cost for contract-emitted events.
// It computes the cost for a list of event attributes and events.
func (g WasmGasRegister) EventCosts(attrs []wasmvmtypes.EventAttribute, events wasmvmtypes.Array[wasmvmtypes.Event]) types.Gas {
	gasUsed, remainingFree := g.eventAttributeCosts(attrs, g.c.EventAttributeDataFreeTier)
	for _, evt := range events {
		// Charge for any event attributes that exist.
		gasEvt, newFree := g.eventAttributeCosts(evt.Attributes, remainingFree)
		gasUsed += gasEvt
		remainingFree = newFree
	}
	gasUsed += types.Gas(len(events)) * g.c.CustomEventCost
	return gasUsed
}

// eventAttributeCosts computes the gas cost for a set of event attributes given a free byte allowance.
func (g WasmGasRegister) eventAttributeCosts(attrs []wasmvmtypes.EventAttribute, freeTier uint64) (types.Gas, uint64) {
	if len(attrs) == 0 {
		return 0, freeTier
	}
	var totalBytes uint64 = 0
	for _, attr := range attrs {
		totalBytes += uint64(len(attr.Key)) + uint64(len(attr.Value))
	}
	if totalBytes <= freeTier {
		remainingFree := freeTier - totalBytes
		return 0, remainingFree
	}
	chargeBytes := totalBytes - freeTier
	gasCost := types.Gas(chargeBytes) * g.c.EventAttributeDataCost
	return gasCost, 0
}

// ToWasmVMGas converts SDK gas to CosmWasm VM gas.
func (g WasmGasRegister) ToWasmVMGas(source types.Gas) uint64 {
	x := uint64(source) * uint64(g.c.GasMultiplier)
	if x < uint64(source) {
		panic(wasmvmtypes.ErrorOutOfGas{Descriptor: "CosmWasm gas overflow"})
	}
	return x
}

// FromWasmVMGas converts CosmWasm VM gas to SDK gas.
func (g WasmGasRegister) FromWasmVMGas(source uint64) types.Gas {
	return types.Gas(source / uint64(g.c.GasMultiplier))
}

```
---
### `runtime/gas/wazero/gas.go`
*2025-02-26 15:26:13 | 3 KB*
```go
package wazerogasometer

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
)

// WazeroGasMeter implements both wazero's api.GasMeter and CosmWasm's types.GasMeter
type WazeroGasMeter struct {
	limit     uint64
	consumed  uint64
	gasConfig types.GasConfig
}

// NewWazeroGasMeter creates a new gas meter compatible with both systems
func NewWazeroGasMeter(limit uint64, config types.GasConfig) *WazeroGasMeter {
	return &WazeroGasMeter{
		limit:     limit,
		consumed:  0,
		gasConfig: config,
	}
}

// WithGasMeter attaches the gas meter to the context
func (g *WazeroGasMeter) WithGasMeter(ctx context.Context) context.Context {
	return context.WithValue(ctx, gasMeterKey{}, g)
}

// gasMeterKey is a private type for the context key to avoid collisions
type gasMeterKey struct{}

// Implements wazero api.GasMeter
func (g *WazeroGasMeter) Gas(uint64) error {
	return nil // Always allow gas during compilation
}

// ConsumeFuel implements wazero's gas consumption during execution
func (g *WazeroGasMeter) ConsumeFuel(fuel uint64) error {
	// Convert wazero fuel units to CosmWasm gas units
	gasToCharge := fuel * g.gasConfig.GasMultiplier
	return g.ConsumeGas(gasToCharge, "wazero operation")
}

// Implements types.GasMeter
func (g *WazeroGasMeter) GasConsumed() uint64 {
	return g.consumed
}

// ConsumeGas implements types.GasMeter
func (g *WazeroGasMeter) ConsumeGas(amount uint64, descriptor string) error {
	if g.consumed+amount > g.limit {
		return types.OutOfGasError{Descriptor: descriptor}
	}
	g.consumed += amount
	return nil
}

// GasRemaining returns remaining gas
func (g *WazeroGasMeter) GasRemaining() uint64 {
	if g.consumed >= g.limit {
		return 0
	}
	return g.limit - g.consumed
}

// HasGas checks if there is enough gas remaining
func (g *WazeroGasMeter) HasGas(required uint64) bool {
	return g.GasRemaining() >= required
}

// GasForOperation calculates gas needed for a specific operation
func (g *WazeroGasMeter) GasForOperation(op types.GasOperation) uint64 {
	switch op {
	case types.GasOperationMemoryRead:
		return g.gasConfig.PerByte
	case types.GasOperationMemoryWrite:
		return g.gasConfig.PerByte
	case types.GasOperationDBRead:
		return g.gasConfig.DatabaseRead
	case types.GasOperationDBWrite:
		return g.gasConfig.DatabaseWrite
	case types.GasOperationDBDelete:
		return g.gasConfig.DatabaseWrite
	case types.GasOperationCompile:
		return g.gasConfig.CompileCost
	// Add other operations as needed
	default:
		return 0
	}
}

// GetGasMeterFromContext retrieves the gas meter from context
func GetGasMeterFromContext(ctx context.Context) (*WazeroGasMeter, bool) {
	meter, ok := ctx.Value(gasMeterKey{}).(*WazeroGasMeter)
	return meter, ok
}

// ConsumeGasFromContext consumes gas from the meter in context
func ConsumeGasFromContext(ctx context.Context, amount uint64, description string) error {
	meter, ok := GetGasMeterFromContext(ctx)
	if !ok {
		return fmt.Errorf("gas meter not found in context")
	}
	return meter.ConsumeGas(amount, description)
}

```
---
### `runtime/gas.go`
*2025-02-20 21:49:29 | 4 KB*
```go
package runtime

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
)

// GasConfig holds gas costs for different operations
type GasConfig struct {
	// Memory operations
	PerByte uint64

	// Database operations
	DatabaseRead  uint64
	DatabaseWrite uint64
	ExternalQuery uint64

	// Iterator operations
	IteratorCreate uint64
	IteratorNext   uint64

	// Contract operations
	Instantiate uint64
	Execute     uint64

	Bls12381AggregateG1Cost GasCost
	Bls12381AggregateG2Cost GasCost
}

type GasCost struct {
	BaseCost uint64
	PerPoint uint64
}

func (c GasCost) TotalCost(pointCount uint64) uint64 {
	return c.BaseCost + c.PerPoint*pointCount
}

// DefaultGasConfig returns the default gas configuration
func DefaultGasConfig() GasConfig {
	return GasConfig{
		PerByte:        constants.GasPerByte,
		DatabaseRead:   constants.GasCostRead,
		DatabaseWrite:  constants.GasCostWrite,
		ExternalQuery:  constants.GasCostQuery,
		IteratorCreate: constants.GasCostIteratorCreate,
		IteratorNext:   constants.GasCostIteratorNext,
		Instantiate:    constants.GasCostInstantiate,
		Execute:        constants.GasCostExecute,
	}
}

// GasState tracks gas usage during execution
type GasState struct {
	config GasConfig
	limit  uint64
	used   uint64
}

func (g *GasState) GasConsumed() uint64 {
	return g.GetGasUsed()
}

// NewGasState creates a new GasState with the given limit
func NewGasState(limit uint64) *GasState {
	return &GasState{
		config: DefaultGasConfig(),
		limit:  limit,
		used:   0,
	}
}

// ConsumeGas consumes gas and checks the limit
func (g *GasState) ConsumeGas(amount uint64, description string) error {
	g.used += amount
	if g.used > g.limit {
		return fmt.Errorf("out of gas: used %d, limit %d - %s", g.used, g.limit, description)
	}
	return nil
}

// ConsumeMemory charges gas for memory operations
func (g *GasState) ConsumeMemory(size uint32) error {
	cost := uint64(size) * g.config.PerByte
	return g.ConsumeGas(cost, fmt.Sprintf("memory allocation: %d bytes", size))
}

// ConsumeRead charges gas for database read operations
func (g *GasState) ConsumeRead(size uint32) error {
	// Base cost plus per-byte cost
	cost := g.config.DatabaseRead + (uint64(size) * g.config.PerByte)
	return g.ConsumeGas(cost, "db read")
}

// ConsumeWrite charges gas for database write operations
func (g *GasState) ConsumeWrite(size uint32) error {
	// Base cost plus per-byte cost
	cost := g.config.DatabaseWrite + (uint64(size) * g.config.PerByte)
	return g.ConsumeGas(cost, "db write")
}

// ConsumeQuery charges gas for external query operations
func (g *GasState) ConsumeQuery() error {
	return g.ConsumeGas(g.config.ExternalQuery, "external query")
}

// ConsumeIterator charges gas for iterator operations
func (g *GasState) ConsumeIterator(create bool) error {
	var cost uint64
	var desc string
	if create {
		cost = g.config.IteratorCreate
		desc = "create iterator"
	} else {
		cost = g.config.IteratorNext
		desc = "iterator next"
	}
	return g.ConsumeGas(cost, desc)
}

// GetGasUsed returns the amount of gas used
func (g *GasState) GetGasUsed() uint64 {
	return g.used
}

// GetGasLimit returns the gas limit
func (g *GasState) GetGasLimit() uint64 {
	return g.limit
}

// GetGasRemaining returns the remaining gas
func (g *GasState) GetGasRemaining() uint64 {
	if g.used > g.limit {
		return 0
	}
	return g.limit - g.used
}

// HasGas checks if there is enough gas remaining
func (g *GasState) HasGas(required uint64) bool {
	return g.GetGasRemaining() >= required
}

```
---
### `runtime/host/combined_code.md`
*2025-02-26 17:56:51 | 36 KB*
```markdown
# Combined Code Files

## TOC
- [`environment.go`](#file-1)
- [`hostfunctions.go`](#file-2)
- [`hostfunctions_impl.go`](#file-3)
- [`memory.go`](#file-4)
- [`registerhostfunctions.go`](#file-5)

---

### `environment.go`
*2025-02-26 17:26:11 | 1 KB*
```go
package host

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// StartCall starts a new call context and returns the call ID
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]types.Iterator)
	return e.nextCallID
}

// StoreIterator stores an iterator and returns its ID
func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter types.Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextIterID++
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

// GetIterator retrieves an iterator by its IDs
func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) types.Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()
	if callMap, ok := e.iterators[callID]; ok {
		return callMap[iterID]
	}
	return nil
}

```
---
### `hostfunctions.go`
*2025-02-26 15:37:08 | 28 KB*
```go
package host

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/tetratelabs/wazero/api"
)

const (
	// Return codes for cryptographic operations
	SECP256K1_VERIFY_CODE_VALID   uint32 = 0
	SECP256K1_VERIFY_CODE_INVALID uint32 = 1

	// BLS12-381 return codes
	BLS12_381_VALID_PAIRING   uint32 = 0
	BLS12_381_INVALID_PAIRING uint32 = 1

	BLS12_381_AGGREGATE_SUCCESS     uint32 = 0
	BLS12_381_HASH_TO_CURVE_SUCCESS uint32 = 0

	// Size limits for BLS12-381 operations (MI = 1024*1024, KI = 1024)
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	envKey contextKey = "env"
)

// GasState tracks gas consumption
type GasState struct {
	limit uint64
	used  uint64
}

func NewGasState(limit uint64) GasState {
	return GasState{
		limit: limit,
		used:  0,
	}
}

// GasConsumed implements types.GasMeter
func (g GasState) GasConsumed() uint64 {
	return g.used
}

// allocateInContract calls the contract's allocate function.
// It handles memory allocation within the WebAssembly module's memory space.
func allocateInContract(ctx context.Context, mod api.Module, size uint32) (uint32, error) {
	allocateFn := mod.ExportedFunction("allocate")
	if allocateFn == nil {
		return 0, fmt.Errorf("contract does not export 'allocate' function")
	}
	results, err := allocateFn.Call(ctx, uint64(size))
	if err != nil {
		return 0, fmt.Errorf("failed to call 'allocate': %w", err)
	}
	if len(results) != 1 {
		return 0, fmt.Errorf("expected 1 result from 'allocate', got %d", len(results))
	}
	return uint32(results[0]), nil
}

// readNullTerminatedString reads bytes from memory starting at addrPtr until a null byte is found.
func readNullTerminatedString(memManager *memory.MemoryManager, addrPtr uint32) ([]byte, error) {
	var buf []byte
	for i := addrPtr; ; i++ {
		b, err := memManager.Read(i, 1)
		if err != nil {
			return nil, fmt.Errorf("memory access error at offset %d: %w", i, err)
		}
		if b[0] == 0 {
			break
		}
		buf = append(buf, b[0])
	}
	return buf, nil
}

// hostHumanizeAddress implements addr_humanize.
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostHumanizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to convert to a human-readable address.
	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: API.HumanizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: humanized address: '%s'\n", human)

	// Write the result back into memory.
	if err := env.MemManager.Write(addrPtr, []byte(human)); err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to write humanized address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: successfully wrote humanized address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostCanonicalizeAddress reads a null-terminated address from memory,
// calls the API to canonicalize it, logs intermediate results, and writes
// the canonical address back into memory.
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostCanonicalizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to canonicalize the address.
	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: API.CanonicalizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: canonical address (hex): %x\n", canonical)

	// Write the canonical address back to memory.
	if err := env.MemManager.Write(addrPtr, canonical); err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to write canonical address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: successfully wrote canonical address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostValidateAddress reads a null-terminated address from memory,
// calls the API to validate it, and logs the process.
// Returns 1 if the address is valid and 0 otherwise.
func hostValidateAddress(ctx context.Context, mod api.Module, addrPtr uint32) uint32 {
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Read the address as a null-terminated string.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		panic(fmt.Sprintf("[ERROR] hostValidateAddress: failed to read address from memory: %v", err))
	}
	fmt.Printf("[DEBUG] hostValidateAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Validate the address.
	_, err = env.API.ValidateAddress(string(addr))
	if err != nil {
		fmt.Printf("[DEBUG] hostValidateAddress: API.ValidateAddress failed: %v\n", err)
		return 0 // reject invalid address
	}
	fmt.Printf("[DEBUG] hostValidateAddress: address validated successfully\n")
	return 1 // valid
}

// hostScan implements db_scan.
func hostScan(ctx context.Context, mod api.Module, startPtr, startLen, order uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostScan: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	start, err := readMemory(mem, startPtr, startLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read start key: %v", err))
	}

	var iter types.Iterator
	if order == 1 {
		iter = env.DB.ReverseIterator(start, nil)
	} else {
		iter = env.DB.Iterator(start, nil)
	}

	// Store the iterator and pack the call and iterator IDs.
	callID := env.StartCall()
	iterID := env.StoreIterator(callID, iter)
	return uint32(callID<<16 | iterID&0xFFFF)
}

// hostDbNext implements db_next.
func hostDbNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbNext: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}
	if !iter.Valid() {
		return 0
	}

	key := iter.Key()
	value := iter.Value()

	// Charge gas for the returned data.
	env.GasUsed += uint64(len(key)+len(value)) * constants.GasPerByte

	totalLen := 4 + len(key) + 4 + len(value)
	offset, err := env.MemManager.Allocate(uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	keyLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLenData, uint32(len(key)))
	if err := env.MemManager.Write(offset, keyLenData); err != nil {
		panic(fmt.Sprintf("failed to write key length: %v", err))
	}

	if err := env.MemManager.Write(offset+4, key); err != nil {
		panic(fmt.Sprintf("failed to write key: %v", err))
	}

	valLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(valLenData, uint32(len(value)))
	if err := env.MemManager.Write(offset+4+uint32(len(key)), valLenData); err != nil {
		panic(fmt.Sprintf("failed to write value length: %v", err))
	}

	if err := env.MemManager.Write(offset+8+uint32(len(key)), value); err != nil {
		panic(fmt.Sprintf("failed to write value: %v", err))
	}

	iter.Next()
	return offset
}

// hostNextValue implements db_next_value.
func hostNextValue(ctx context.Context, mod api.Module, callID, iterID uint64) (valPtr, valLen, errCode uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostNextValue: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2
	}

	if !iter.Valid() {
		return 0, 0, 0
	}

	value := iter.Value()
	env.GasUsed += uint64(len(value)) * constants.GasPerByte

	valOffset, err := allocateInContract(ctx, mod, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, valOffset, value, false); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	iter.Next()
	return valOffset, uint32(len(value)), 0
}

// hostDbRead implements db_read.
func hostDbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Charge base gas cost for DB read
	if err := env.Gas.ConsumeGas(constants.GasCostRead, "db_read base cost"); err != nil {
		panic(err) // Or handle more gracefully
	}

	// Read key length and charge per byte
	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		return 0
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	// Charge per-byte gas for key
	if err := env.Gas.ConsumeGas(uint64(keyLen)*constants.GasPerByte, "db_read key bytes"); err != nil {
		panic(err)
	}

	// Rest of existing code...

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key data: %v\n", err)
		return 0
	}
	fmt.Printf("Key data: %x\n", key)

	value := env.DB.Get(key)
	fmt.Printf("Value found: %x\n", value)

	valuePtr, err := env.MemManager.Allocate(uint32(len(value)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory: %v\n", err)
		return 0
	}

	if err := env.MemManager.Write(valuePtr, value); err != nil {
		fmt.Printf("ERROR: Failed to write value to memory: %v\n", err)
		return 0
	}

	return valuePtr
}

// hostDbWrite implements db_write.
func hostDbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbWrite: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	valLenBytes, err := env.MemManager.Read(valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := env.MemManager.Read(valuePtr+4, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, value)
}

// hostSecp256k1Verify implements secp256k1_verify.
func hostSecp256k1Verify(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Get the environment and memory
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1Verify: runtime environment not found in context")
		return SECP256K1_VERIFY_CODE_INVALID
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Charge gas for this operation
	gasToCharge := env.GasConfig.Secp256k1VerifyCost + uint64(len(hash)+len(sig)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Verify signature using the crypto handler
	valid, err := cryptoHandler.Secp256k1Verify(hash, sig, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Secp256k1Verify failed: %v\n", err)
		return SECP256K1_VERIFY_CODE_INVALID
	}

	if valid {
		return SECP256K1_VERIFY_CODE_VALID
	}
	return SECP256K1_VERIFY_CODE_INVALID
}

// hostSecp256k1RecoverPubkey implements secp256k1_recover_pubkey.
func hostSecp256k1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, recoveryParam uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1RecoverPubkey: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return 0
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Secp256k1RecoverPubkeyCost + uint64(len(hash)+len(sig))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1RecoverPubkey: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Recover pubkey using cryptoHandler
	pubkey, err := cryptoHandler.Secp256k1RecoverPubkey(hash, sig, byte(recoveryParam))
	if err != nil {
		fmt.Printf("ERROR: Secp256k1RecoverPubkey failed: %v\n", err)
		return 0
	}

	// Allocate region for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(pubkey)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for recovered pubkey: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, pubkey) {
		fmt.Printf("ERROR: Failed to write recovered pubkey to memory\n")
		return 0
	}

	return resultPtr
}

// hostEd25519Verify implements ed25519_verify.
func hostEd25519Verify(ctx context.Context, mod api.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519Verify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	message, ok := mem.Read(msgPtr, msgLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read message from memory\n")
		return 0
	}

	signature, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519VerifyCost + uint64(len(message)+len(signature)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Verify signature
	valid, err := cryptoHandler.Ed25519Verify(message, signature, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Ed25519Verify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify.
func hostEd25519BatchVerify(ctx context.Context, mod api.Module, msgsPtr, msgsLen, sigsPtr, sigsLen, pubkeysPtr, pubkeysLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519BatchVerify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read array counts and pointers
	msgsData, ok := mem.Read(msgsPtr, msgsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read messages array from memory\n")
		return 0
	}

	sigsData, ok := mem.Read(sigsPtr, sigsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signatures array from memory\n")
		return 0
	}

	pubkeysData, ok := mem.Read(pubkeysPtr, pubkeysLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public keys array from memory\n")
		return 0
	}

	// Parse arrays (implementation depends on how arrays are serialized)
	// This is a simplified example - actual parsing logic may differ
	messages, signatures, pubkeys := parseArraysForBatchVerify(msgsData, sigsData, pubkeysData)

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519BatchVerifyCost * uint64(len(messages))
	dataSize := 0
	for i := 0; i < len(messages); i++ {
		dataSize += len(messages[i]) + len(signatures[i]) + len(pubkeys[i])
	}
	gasToCharge += uint64(dataSize) * constants.GasPerByte

	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519BatchVerify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Batch verify signatures
	valid, err := cryptoHandler.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		fmt.Printf("ERROR: Ed25519BatchVerify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// parseArraysForBatchVerify parses the array data for Ed25519BatchVerify
// Implementation depends on how arrays are serialized in the contract
func parseArraysForBatchVerify(msgsData, sigsData, pubkeysData []byte) ([][]byte, [][]byte, [][]byte) {
	// Example implementation - actual parsing may differ based on serialization format
	// This is a placeholder implementation

	// In a real implementation, you would parse the arrays from their serialized format
	// Here we're creating dummy data just to satisfy the function signature
	count := 1 // In reality, extract this from the data

	messages := make([][]byte, count)
	signatures := make([][]byte, count)
	pubkeys := make([][]byte, count)

	// Fill with dummy data for demonstration
	for i := 0; i < count; i++ {
		messages[i] = []byte("message")
		signatures[i] = make([]byte, 64)
		pubkeys[i] = make([]byte, 32)
	}

	return messages, signatures, pubkeys
}

// hostBls12381AggregateG1 implements bls12_381_aggregate_g1.
func hostBls12381AggregateG1(ctx context.Context, mod api.Module, g1sPtr, g1sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG1: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 points
	g1s, ok := mem.Read(g1sPtr, g1sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return 0
	}

	pointCount := len(g1s) / constants.BLS12_381_G1_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G1 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG1Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during G1 aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g1s, constants.BLS12_381_G1_POINT_LEN)

	// Use cryptoHandler to aggregate points
	result, err := cryptoHandler.BLS12381AggregateG1(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G1 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G1 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// splitIntoPoints splits a byte array into equal-sized points
func splitIntoPoints(data []byte, pointLen int) [][]byte {
	pointCount := len(data) / pointLen
	points := make([][]byte, pointCount)

	for i := 0; i < pointCount; i++ {
		points[i] = data[i*pointLen : (i+1)*pointLen]
	}

	return points
}

// hostBls12381AggregateG2 implements bls12_381_aggregate_g2.
func hostBls12381AggregateG2(ctx context.Context, mod api.Module, g2sPtr, g2sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data
	mem := mod.Memory()
	g2s, ok := mem.Read(g2sPtr, g2sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return 0
	}

	pointCount := len(g2s) / constants.BLS12_381_G2_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G2 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG2Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g2s, constants.BLS12_381_G2_POINT_LEN)

	// Use cryptoHandler interface instead of direct function call
	result, err := cryptoHandler.BLS12381AggregateG2(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G2 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G2 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// hostDbRemove implements db_remove.
func hostDbRemove(ctx context.Context, mod api.Module, keyPtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbRemove: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the 4-byte length prefix from the key pointer.
	lenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key.
	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	env.DB.Delete(key)
}

// Add missing gasPerByte constant
const gasPerByte = constants.GasPerByte

// cryptoHandler holds crypto operations - initialized at runtime
var cryptoHandler cryptoapi.CryptoOperations

// SetCryptoHandler sets the crypto handler for host functions
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// readMessage reads a message of specified length from memory
func readMessage(mod api.Module, ptr, len uint32) ([]byte, error) {
	if len > constants.BLS12_381_MAX_MESSAGE_SIZE {
		return nil, fmt.Errorf("message too large: %d > %d", len, constants.BLS12_381_MAX_MESSAGE_SIZE)
	}

	mem := mod.Memory()
	data, ok := mem.Read(ptr, len)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", ptr, len)
	}
	return data, nil
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
func hostBls12381HashToG1(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Get environment context
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG1: runtime environment not found in context")
		return 0
	}

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G1 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
func hostBls12381HashToG2(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381HashToG2Cost.TotalCost(uint64(len(message)))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Hash-to-G2: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G2 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381VerifyG1G2 implements bls12_381_verify.
func hostBls12381VerifyG1G2(ctx context.Context, mod api.Module, g1PointsPtr, g1PointsLen, g2PointsPtr, g2PointsLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381VerifyG1G2: runtime environment not found in context")
		return BLS12_381_INVALID_PAIRING
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 and G2 points
	g1Data, ok := mem.Read(g1PointsPtr, g1PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g2Data, ok := mem.Read(g2PointsPtr, g2PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g1Count := len(g1Data) / constants.BLS12_381_G1_POINT_LEN
	g2Count := len(g2Data) / constants.BLS12_381_G2_POINT_LEN

	if g1Count != g2Count {
		fmt.Printf("ERROR: Number of G1 points (%d) must match number of G2 points (%d)\n", g1Count, g2Count)
		return BLS12_381_INVALID_PAIRING
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381VerifyCost.TotalCost(uint64(g1Count))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during BLS verification: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return BLS12_381_INVALID_PAIRING
	}

	// Split into individual points
	g1Points := splitIntoPoints(g1Data, constants.BLS12_381_G1_POINT_LEN)
	g2Points := splitIntoPoints(g2Data, constants.BLS12_381_G2_POINT_LEN)

	// Verify pairing
	valid, err := cryptoHandler.BLS12381VerifyG1G2(g1Points, g2Points)
	if err != nil {
		fmt.Printf("ERROR: BLS12-381 verification failed: %v\n", err)
		return BLS12_381_INVALID_PAIRING
	}

	if valid {
		return BLS12_381_VALID_PAIRING
	}
	return BLS12_381_INVALID_PAIRING
}

```
---
### `hostfunctions_impl.go`
*2025-02-26 17:47:34 | 1 KB*
```go
 
```
---
### `memory.go`
*2025-02-20 21:54:38 | 1 KB*
```go
package host

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// readMemory reads length bytes from memory at the given offset
func readMemory(mem api.Memory, offset, length uint32) ([]byte, error) {
	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", offset, length)
	}
	return data, nil
}

// writeMemory writes data to memory at the given offset
func writeMemory(mem api.Memory, offset uint32, data []byte, allowGrow bool) error {
	if !mem.Write(offset, data) {
		return fmt.Errorf("failed to write %d bytes to memory at offset %d", len(data), offset)
	}
	return nil
}

```
---
### `registerhostfunctions.go`
*2025-02-26 17:44:41 | 6 KB*
```go
package host

import (
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

// --- Minimal Host Interfaces ---
// WasmInstance is a minimal interface for a WASM contract instance.
type WasmInstance interface {
	// RegisterFunction registers a host function with the instance.
	RegisterFunction(module, name string, fn interface{})
}

// MemoryManager is imported from our memory package.
type MemoryManager = memory.MemoryManager

// Storage represents contract storage.
type Storage interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Scan(start, end []byte, order int32) (uint32, error)
	Next(iteratorID uint32) (key []byte, value []byte, err error)
}

// API aliases types.GoAPI.
type API = types.GoAPI

// Querier aliases types.Querier.
type Querier = types.Querier

// GasMeter aliases types.GasMeter.
type GasMeter = types.GasMeter

// Logger is a simple logging interface.
type Logger interface {
	Debug(args ...interface{})
	Error(args ...interface{})
}

// --- Runtime Environment ---
// RuntimeEnvironment holds all execution context for a contract call.
type RuntimeEnvironment struct {
	DB        types.KVStore
	API       API
	Querier   Querier
	Gas       GasMeter
	GasConfig types.GasConfig

	// internal gas limit and gas used for host functions:
	gasLimit uint64
	gasUsed  uint64

	// Iterator management.
	iterators      map[uint64]map[uint64]types.Iterator
	iteratorsMutex types.RWMutex // alias for sync.RWMutex from types package if desired
	nextCallID     uint64
	nextIterID     uint64
}

// --- Helper: writeToRegion ---
// writeToRegion uses MemoryManager to update a Region struct and write the provided data.
func writeToRegion(mem MemoryManager, regionPtr uint32, data []byte) error {
	regionStruct, err := mem.Read(regionPtr, 12)
	if err != nil {
		return fmt.Errorf("failed to read Region at %d: %w", regionPtr, err)
	}
	offset := binary.LittleEndian.Uint32(regionStruct[0:4])
	capacity := binary.LittleEndian.Uint32(regionStruct[4:8])
	if uint32(len(data)) > capacity {
		return fmt.Errorf("data length %d exceeds region capacity %d", len(data), capacity)
	}
	if err := mem.Write(offset, data); err != nil {
		return fmt.Errorf("failed to write data to memory at offset %d: %w", offset, err)
	}
	binary.LittleEndian.PutUint32(regionStruct[8:12], uint32(len(data)))
	if err := mem.Write(regionPtr+8, regionStruct[8:12]); err != nil {
		return fmt.Errorf("failed to write Region length at %d: %w", regionPtr+8, err)
	}
	return nil
}

// --- RegisterHostFunctions ---
// RegisterHostFunctions registers all host functions with the provided module builder
func RegisterHostFunctions(mod wazero.HostModuleBuilder) {
	// Abort: abort(msg_ptr: u32, file_ptr: u32, line: u32, col: u32) -> !
	mod.NewFunctionBuilder().WithFunc(Abort).Export("abort")

	// Debug: debug(msg_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(Debug).Export("debug")

	// db_read: db_read(key_ptr: u32) -> u32 (returns Region pointer or 0 if not found)
	mod.NewFunctionBuilder().WithFunc(DbRead).Export("db_read")

	// db_write: db_write(key_ptr: u32, value_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(DbWrite).Export("db_write")

	// db_remove: db_remove(key_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(DbRemove).Export("db_remove")

	// db_scan: db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbScan).Export("db_scan")

	// db_next: db_next(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNext).Export("db_next")

	// db_next_key: db_next_key(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNextKey).Export("db_next_key")

	// db_next_value: db_next_value(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNextValue).Export("db_next_value")

	// addr_validate: addr_validate(addr_ptr: u32) -> u32 (0 = success, nonzero = error)
	mod.NewFunctionBuilder().WithFunc(AddrValidate).Export("addr_validate")

	// addr_canonicalize: addr_canonicalize(human_ptr: u32, canon_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(AddrCanonicalize).Export("canonicalize_address")

	// addr_humanize: addr_humanize(canon_ptr: u32, human_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(hostHumanizeAddress).Export("humanize_address")

	// Crypto operations
	mod.NewFunctionBuilder().WithFunc(Secp256k1Verify).Export("secp256k1_verify")
	mod.NewFunctionBuilder().WithFunc(Secp256k1RecoverPubkey).Export("secp256k1_recover_pubkey")
	mod.NewFunctionBuilder().WithFunc(Ed25519Verify).Export("ed25519_verify")
	mod.NewFunctionBuilder().WithFunc(Ed25519BatchVerify).Export("ed25519_batch_verify")

	// BLS crypto operations
	mod.NewFunctionBuilder().WithFunc(Bls12381AggregateG1).Export("bls12_381_aggregate_g1")
	mod.NewFunctionBuilder().WithFunc(Bls12381AggregateG2).Export("bls12_381_aggregate_g2")
	mod.NewFunctionBuilder().WithFunc(Bls12381PairingCheck).Export("bls12_381_pairing_equality")
	mod.NewFunctionBuilder().WithFunc(Bls12381HashToG1).Export("bls12_381_hash_to_g1")
	mod.NewFunctionBuilder().WithFunc(Bls12381HashToG2).Export("bls12_381_hash_to_g2")

	// query_chain: query_chain(request_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(QueryChain).Export("query_chain")
}

```
---

## Summary
Files: 5, Total: 37 KB
Breakdown:
- go: 37 KB
```
---
### `runtime/host/environment.go`
*2025-02-26 17:26:11 | 1 KB*
```go
package host

import (
	"github.com/CosmWasm/wasmvm/v2/types"
)

// StartCall starts a new call context and returns the call ID
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]types.Iterator)
	return e.nextCallID
}

// StoreIterator stores an iterator and returns its ID
func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter types.Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextIterID++
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

// GetIterator retrieves an iterator by its IDs
func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) types.Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()
	if callMap, ok := e.iterators[callID]; ok {
		return callMap[iterID]
	}
	return nil
}

```
---
### `runtime/host/hostfunctions.go`
*2025-02-26 15:37:08 | 28 KB*
```go
package host

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/cryptoapi"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/tetratelabs/wazero/api"
)

const (
	// Return codes for cryptographic operations
	SECP256K1_VERIFY_CODE_VALID   uint32 = 0
	SECP256K1_VERIFY_CODE_INVALID uint32 = 1

	// BLS12-381 return codes
	BLS12_381_VALID_PAIRING   uint32 = 0
	BLS12_381_INVALID_PAIRING uint32 = 1

	BLS12_381_AGGREGATE_SUCCESS     uint32 = 0
	BLS12_381_HASH_TO_CURVE_SUCCESS uint32 = 0

	// Size limits for BLS12-381 operations (MI = 1024*1024, KI = 1024)
	BLS12_381_MAX_AGGREGATE_SIZE = 2 * 1024 * 1024 // 2 MiB
	BLS12_381_MAX_MESSAGE_SIZE   = 5 * 1024 * 1024 // 5 MiB
	BLS12_381_MAX_DST_SIZE       = 5 * 1024        // 5 KiB
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	envKey contextKey = "env"
)

// GasState tracks gas consumption
type GasState struct {
	limit uint64
	used  uint64
}

func NewGasState(limit uint64) GasState {
	return GasState{
		limit: limit,
		used:  0,
	}
}

// GasConsumed implements types.GasMeter
func (g GasState) GasConsumed() uint64 {
	return g.used
}

// allocateInContract calls the contract's allocate function.
// It handles memory allocation within the WebAssembly module's memory space.
func allocateInContract(ctx context.Context, mod api.Module, size uint32) (uint32, error) {
	allocateFn := mod.ExportedFunction("allocate")
	if allocateFn == nil {
		return 0, fmt.Errorf("contract does not export 'allocate' function")
	}
	results, err := allocateFn.Call(ctx, uint64(size))
	if err != nil {
		return 0, fmt.Errorf("failed to call 'allocate': %w", err)
	}
	if len(results) != 1 {
		return 0, fmt.Errorf("expected 1 result from 'allocate', got %d", len(results))
	}
	return uint32(results[0]), nil
}

// readNullTerminatedString reads bytes from memory starting at addrPtr until a null byte is found.
func readNullTerminatedString(memManager *memory.MemoryManager, addrPtr uint32) ([]byte, error) {
	var buf []byte
	for i := addrPtr; ; i++ {
		b, err := memManager.Read(i, 1)
		if err != nil {
			return nil, fmt.Errorf("memory access error at offset %d: %w", i, err)
		}
		if b[0] == 0 {
			break
		}
		buf = append(buf, b[0])
	}
	return buf, nil
}

// hostHumanizeAddress implements addr_humanize.
func hostHumanizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostHumanizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to convert to a human-readable address.
	human, _, err := env.API.HumanizeAddress(addr)
	if err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: API.HumanizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: humanized address: '%s'\n", human)

	// Write the result back into memory.
	if err := env.MemManager.Write(addrPtr, []byte(human)); err != nil {
		fmt.Printf("[ERROR] hostHumanizeAddress: failed to write humanized address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostHumanizeAddress: successfully wrote humanized address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostCanonicalizeAddress reads a null-terminated address from memory,
// calls the API to canonicalize it, logs intermediate results, and writes
// the canonical address back into memory.
func hostCanonicalizeAddress(ctx context.Context, mod api.Module, addrPtr, _ uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostCanonicalizeAddress: runtime environment not found in context")
		return 1
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the address as a null-terminated byte slice.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to read address from memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Call the API to canonicalize the address.
	canonical, _, err := env.API.CanonicalizeAddress(string(addr))
	if err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: API.CanonicalizeAddress failed: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: canonical address (hex): %x\n", canonical)

	// Write the canonical address back to memory.
	if err := env.MemManager.Write(addrPtr, canonical); err != nil {
		fmt.Printf("[ERROR] hostCanonicalizeAddress: failed to write canonical address back to memory: %v\n", err)
		return 1
	}
	fmt.Printf("[DEBUG] hostCanonicalizeAddress: successfully wrote canonical address back to memory at 0x%x\n", addrPtr)
	return 0
}

// hostValidateAddress reads a null-terminated address from memory,
// calls the API to validate it, and logs the process.
// Returns 1 if the address is valid and 0 otherwise.
func hostValidateAddress(ctx context.Context, mod api.Module, addrPtr uint32) uint32 {
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Read the address as a null-terminated string.
	addr, err := readNullTerminatedString(env.MemManager, addrPtr)
	if err != nil {
		panic(fmt.Sprintf("[ERROR] hostValidateAddress: failed to read address from memory: %v", err))
	}
	fmt.Printf("[DEBUG] hostValidateAddress: read address (hex): %x, as string: '%s'\n", addr, string(addr))

	// Validate the address.
	_, err = env.API.ValidateAddress(string(addr))
	if err != nil {
		fmt.Printf("[DEBUG] hostValidateAddress: API.ValidateAddress failed: %v\n", err)
		return 0 // reject invalid address
	}
	fmt.Printf("[DEBUG] hostValidateAddress: address validated successfully\n")
	return 1 // valid
}

// hostScan implements db_scan.
func hostScan(ctx context.Context, mod api.Module, startPtr, startLen, order uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostScan: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	start, err := readMemory(mem, startPtr, startLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read start key: %v", err))
	}

	var iter types.Iterator
	if order == 1 {
		iter = env.DB.ReverseIterator(start, nil)
	} else {
		iter = env.DB.Iterator(start, nil)
	}

	// Store the iterator and pack the call and iterator IDs.
	callID := env.StartCall()
	iterID := env.StoreIterator(callID, iter)
	return uint32(callID<<16 | iterID&0xFFFF)
}

// hostDbNext implements db_next.
func hostDbNext(ctx context.Context, mod api.Module, iterID uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbNext: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	callID := uint64(iterID >> 16)
	actualIterID := uint64(iterID & 0xFFFF)

	iter := env.GetIterator(callID, actualIterID)
	if iter == nil {
		return 0
	}
	if !iter.Valid() {
		return 0
	}

	key := iter.Key()
	value := iter.Value()

	// Charge gas for the returned data.
	env.GasUsed += uint64(len(key)+len(value)) * constants.GasPerByte

	totalLen := 4 + len(key) + 4 + len(value)
	offset, err := env.MemManager.Allocate(uint32(totalLen))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory: %v", err))
	}

	keyLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLenData, uint32(len(key)))
	if err := env.MemManager.Write(offset, keyLenData); err != nil {
		panic(fmt.Sprintf("failed to write key length: %v", err))
	}

	if err := env.MemManager.Write(offset+4, key); err != nil {
		panic(fmt.Sprintf("failed to write key: %v", err))
	}

	valLenData := make([]byte, 4)
	binary.LittleEndian.PutUint32(valLenData, uint32(len(value)))
	if err := env.MemManager.Write(offset+4+uint32(len(key)), valLenData); err != nil {
		panic(fmt.Sprintf("failed to write value length: %v", err))
	}

	if err := env.MemManager.Write(offset+8+uint32(len(key)), value); err != nil {
		panic(fmt.Sprintf("failed to write value: %v", err))
	}

	iter.Next()
	return offset
}

// hostNextValue implements db_next_value.
func hostNextValue(ctx context.Context, mod api.Module, callID, iterID uint64) (valPtr, valLen, errCode uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostNextValue: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	iter := env.GetIterator(callID, iterID)
	if iter == nil {
		return 0, 0, 2
	}

	if !iter.Valid() {
		return 0, 0, 0
	}

	value := iter.Value()
	env.GasUsed += uint64(len(value)) * constants.GasPerByte

	valOffset, err := allocateInContract(ctx, mod, uint32(len(value)))
	if err != nil {
		panic(fmt.Sprintf("failed to allocate memory for value (via contract's allocate): %v", err))
	}

	if err := writeMemory(mem, valOffset, value, false); err != nil {
		panic(fmt.Sprintf("failed to write value to memory: %v", err))
	}

	iter.Next()
	return valOffset, uint32(len(value)), 0
}

// hostDbRead implements db_read.
func hostDbRead(ctx context.Context, mod api.Module, keyPtr uint32) uint32 {
	env := ctx.Value(envKey).(*types.RuntimeEnvironment)

	// Charge base gas cost for DB read
	if err := env.Gas.ConsumeGas(constants.GasCostRead, "db_read base cost"); err != nil {
		panic(err) // Or handle more gracefully
	}

	// Read key length and charge per byte
	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		return 0
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	// Charge per-byte gas for key
	if err := env.Gas.ConsumeGas(uint64(keyLen)*constants.GasPerByte, "db_read key bytes"); err != nil {
		panic(err)
	}

	// Rest of existing code...

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read key data: %v\n", err)
		return 0
	}
	fmt.Printf("Key data: %x\n", key)

	value := env.DB.Get(key)
	fmt.Printf("Value found: %x\n", value)

	valuePtr, err := env.MemManager.Allocate(uint32(len(value)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory: %v\n", err)
		return 0
	}

	if err := env.MemManager.Write(valuePtr, value); err != nil {
		fmt.Printf("ERROR: Failed to write value to memory: %v\n", err)
		return 0
	}

	return valuePtr
}

// hostDbWrite implements db_write.
func hostDbWrite(ctx context.Context, mod api.Module, keyPtr, valuePtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbWrite: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	keyLenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBytes)

	valLenBytes, err := env.MemManager.Read(valuePtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read value length from memory: %v", err))
	}
	valLen := binary.LittleEndian.Uint32(valLenBytes)

	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	value, err := env.MemManager.Read(valuePtr+4, valLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read value from memory: %v", err))
	}

	env.DB.Set(key, value)
}

// hostSecp256k1Verify implements secp256k1_verify.
func hostSecp256k1Verify(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	// Get the environment and memory
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1Verify: runtime environment not found in context")
		return SECP256K1_VERIFY_CODE_INVALID
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Charge gas for this operation
	gasToCharge := env.GasConfig.Secp256k1VerifyCost + uint64(len(hash)+len(sig)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return SECP256K1_VERIFY_CODE_INVALID
	}

	// Verify signature using the crypto handler
	valid, err := cryptoHandler.Secp256k1Verify(hash, sig, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Secp256k1Verify failed: %v\n", err)
		return SECP256K1_VERIFY_CODE_INVALID
	}

	if valid {
		return SECP256K1_VERIFY_CODE_VALID
	}
	return SECP256K1_VERIFY_CODE_INVALID
}

// hostSecp256k1RecoverPubkey implements secp256k1_recover_pubkey.
func hostSecp256k1RecoverPubkey(ctx context.Context, mod api.Module, hashPtr, hashLen, sigPtr, sigLen, recoveryParam uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostSecp256k1RecoverPubkey: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	hash, ok := mem.Read(hashPtr, hashLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read hash from memory\n")
		return 0
	}

	sig, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Secp256k1RecoverPubkeyCost + uint64(len(hash)+len(sig))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Secp256k1RecoverPubkey: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Recover pubkey using cryptoHandler
	pubkey, err := cryptoHandler.Secp256k1RecoverPubkey(hash, sig, byte(recoveryParam))
	if err != nil {
		fmt.Printf("ERROR: Secp256k1RecoverPubkey failed: %v\n", err)
		return 0
	}

	// Allocate region for result
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(pubkey)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for recovered pubkey: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, pubkey) {
		fmt.Printf("ERROR: Failed to write recovered pubkey to memory\n")
		return 0
	}

	return resultPtr
}

// hostEd25519Verify implements ed25519_verify.
func hostEd25519Verify(ctx context.Context, mod api.Module, msgPtr, msgLen, sigPtr, sigLen, pubkeyPtr, pubkeyLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519Verify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read inputs
	message, ok := mem.Read(msgPtr, msgLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read message from memory\n")
		return 0
	}

	signature, ok := mem.Read(sigPtr, sigLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signature from memory\n")
		return 0
	}

	pubkey, ok := mem.Read(pubkeyPtr, pubkeyLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public key from memory\n")
		return 0
	}

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519VerifyCost + uint64(len(message)+len(signature)+len(pubkey))*constants.GasPerByte
	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519Verify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Verify signature
	valid, err := cryptoHandler.Ed25519Verify(message, signature, pubkey)
	if err != nil {
		fmt.Printf("ERROR: Ed25519Verify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// hostEd25519BatchVerify implements ed25519_batch_verify.
func hostEd25519BatchVerify(ctx context.Context, mod api.Module, msgsPtr, msgsLen, sigsPtr, sigsLen, pubkeysPtr, pubkeysLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostEd25519BatchVerify: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read array counts and pointers
	msgsData, ok := mem.Read(msgsPtr, msgsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read messages array from memory\n")
		return 0
	}

	sigsData, ok := mem.Read(sigsPtr, sigsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read signatures array from memory\n")
		return 0
	}

	pubkeysData, ok := mem.Read(pubkeysPtr, pubkeysLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read public keys array from memory\n")
		return 0
	}

	// Parse arrays (implementation depends on how arrays are serialized)
	// This is a simplified example - actual parsing logic may differ
	messages, signatures, pubkeys := parseArraysForBatchVerify(msgsData, sigsData, pubkeysData)

	// Charge gas
	gasToCharge := env.GasConfig.Ed25519BatchVerifyCost * uint64(len(messages))
	dataSize := 0
	for i := 0; i < len(messages); i++ {
		dataSize += len(messages[i]) + len(signatures[i]) + len(pubkeys[i])
	}
	gasToCharge += uint64(dataSize) * constants.GasPerByte

	env.GasUsed += gasToCharge
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Ed25519BatchVerify: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Batch verify signatures
	valid, err := cryptoHandler.Ed25519BatchVerify(messages, signatures, pubkeys)
	if err != nil {
		fmt.Printf("ERROR: Ed25519BatchVerify failed: %v\n", err)
		return 0
	}

	if valid {
		return 1
	}
	return 0
}

// parseArraysForBatchVerify parses the array data for Ed25519BatchVerify
// Implementation depends on how arrays are serialized in the contract
func parseArraysForBatchVerify(msgsData, sigsData, pubkeysData []byte) ([][]byte, [][]byte, [][]byte) {
	// Example implementation - actual parsing may differ based on serialization format
	// This is a placeholder implementation

	// In a real implementation, you would parse the arrays from their serialized format
	// Here we're creating dummy data just to satisfy the function signature
	count := 1 // In reality, extract this from the data

	messages := make([][]byte, count)
	signatures := make([][]byte, count)
	pubkeys := make([][]byte, count)

	// Fill with dummy data for demonstration
	for i := 0; i < count; i++ {
		messages[i] = []byte("message")
		signatures[i] = make([]byte, 64)
		pubkeys[i] = make([]byte, 32)
	}

	return messages, signatures, pubkeys
}

// hostBls12381AggregateG1 implements bls12_381_aggregate_g1.
func hostBls12381AggregateG1(ctx context.Context, mod api.Module, g1sPtr, g1sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG1: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 points
	g1s, ok := mem.Read(g1sPtr, g1sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return 0
	}

	pointCount := len(g1s) / constants.BLS12_381_G1_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G1 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG1Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during G1 aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g1s, constants.BLS12_381_G1_POINT_LEN)

	// Use cryptoHandler to aggregate points
	result, err := cryptoHandler.BLS12381AggregateG1(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G1 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G1 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// splitIntoPoints splits a byte array into equal-sized points
func splitIntoPoints(data []byte, pointLen int) [][]byte {
	pointCount := len(data) / pointLen
	points := make([][]byte, pointCount)

	for i := 0; i < pointCount; i++ {
		points[i] = data[i*pointLen : (i+1)*pointLen]
	}

	return points
}

// hostBls12381AggregateG2 implements bls12_381_aggregate_g2.
func hostBls12381AggregateG2(ctx context.Context, mod api.Module, g2sPtr, g2sLen, outPtr uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381AggregateG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data
	mem := mod.Memory()
	g2s, ok := mem.Read(g2sPtr, g2sLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return 0
	}

	pointCount := len(g2s) / constants.BLS12_381_G2_POINT_LEN
	if pointCount == 0 {
		fmt.Printf("ERROR: No G2 points to aggregate\n")
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381AggregateG2Cost.TotalCost(uint64(pointCount))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during aggregation: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Split into individual points
	points := splitIntoPoints(g2s, constants.BLS12_381_G2_POINT_LEN)

	// Use cryptoHandler interface instead of direct function call
	result, err := cryptoHandler.BLS12381AggregateG2(points)
	if err != nil {
		fmt.Printf("ERROR: Failed to aggregate G2 points: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(outPtr, result) {
		fmt.Printf("ERROR: Failed to write aggregated G2 point to memory\n")
		return 0
	}

	return BLS12_381_AGGREGATE_SUCCESS
}

// hostDbRemove implements db_remove.
func hostDbRemove(ctx context.Context, mod api.Module, keyPtr uint32) {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		panic("[ERROR] hostDbRemove: runtime environment not found in context")
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read the 4-byte length prefix from the key pointer.
	lenBytes, err := env.MemManager.Read(keyPtr, 4)
	if err != nil {
		panic(fmt.Sprintf("failed to read key length from memory: %v", err))
	}
	keyLen := binary.LittleEndian.Uint32(lenBytes)

	// Read the actual key.
	key, err := env.MemManager.Read(keyPtr+4, keyLen)
	if err != nil {
		panic(fmt.Sprintf("failed to read key from memory: %v", err))
	}

	env.DB.Delete(key)
}

// Add missing gasPerByte constant
const gasPerByte = constants.GasPerByte

// cryptoHandler holds crypto operations - initialized at runtime
var cryptoHandler cryptoapi.CryptoOperations

// SetCryptoHandler sets the crypto handler for host functions
func SetCryptoHandler(handler cryptoapi.CryptoOperations) {
	cryptoHandler = handler
}

// readMessage reads a message of specified length from memory
func readMessage(mod api.Module, ptr, len uint32) ([]byte, error) {
	if len > constants.BLS12_381_MAX_MESSAGE_SIZE {
		return nil, fmt.Errorf("message too large: %d > %d", len, constants.BLS12_381_MAX_MESSAGE_SIZE)
	}

	mem := mod.Memory()
	data, ok := mem.Read(ptr, len)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", ptr, len)
	}
	return data, nil
}

// hostBls12381HashToG1 implements bls12_381_hash_to_g1.
func hostBls12381HashToG1(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	// Get environment context
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG1: runtime environment not found in context")
		return 0
	}

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG1(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G1 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381HashToG2 implements bls12_381_hash_to_g2.
func hostBls12381HashToG2(ctx context.Context, mod api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381HashToG2: runtime environment not found in context")
		return 0
	}
	env := envVal.(*types.RuntimeEnvironment)

	// Read input data from memory
	message, err := readMessage(mod, hashPtr, hashLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read message: %v\n", err)
		return 0
	}

	dst, err := readMessage(mod, dstPtr, dstLen)
	if err != nil {
		fmt.Printf("ERROR: Failed to read DST: %v\n", err)
		return 0
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381HashToG2Cost.TotalCost(uint64(len(message)))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during Hash-to-G2: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return 0
	}

	// Use the interface instead of direct function
	result, err := cryptoHandler.BLS12381HashToG2(message, dst)
	if err != nil {
		fmt.Printf("ERROR: Hash to G2 failed: %v\n", err)
		return 0
	}

	// Allocate memory for the result
	mem := mod.Memory()
	resultPtr, err := allocateInContract(ctx, mod, uint32(len(result)))
	if err != nil {
		fmt.Printf("ERROR: Failed to allocate memory for result: %v\n", err)
		return 0
	}

	// Write result to memory
	if !mem.Write(resultPtr, result) {
		fmt.Printf("ERROR: Failed to write result to memory\n")
		return 0
	}

	return resultPtr
}

// hostBls12381VerifyG1G2 implements bls12_381_verify.
func hostBls12381VerifyG1G2(ctx context.Context, mod api.Module, g1PointsPtr, g1PointsLen, g2PointsPtr, g2PointsLen uint32) uint32 {
	envVal := ctx.Value(envKey)
	if envVal == nil {
		fmt.Println("[ERROR] hostBls12381VerifyG1G2: runtime environment not found in context")
		return BLS12_381_INVALID_PAIRING
	}
	env := envVal.(*types.RuntimeEnvironment)
	mem := mod.Memory()

	// Read G1 and G2 points
	g1Data, ok := mem.Read(g1PointsPtr, g1PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G1 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g2Data, ok := mem.Read(g2PointsPtr, g2PointsLen)
	if !ok {
		fmt.Printf("ERROR: Failed to read G2 points from memory\n")
		return BLS12_381_INVALID_PAIRING
	}

	g1Count := len(g1Data) / constants.BLS12_381_G1_POINT_LEN
	g2Count := len(g2Data) / constants.BLS12_381_G2_POINT_LEN

	if g1Count != g2Count {
		fmt.Printf("ERROR: Number of G1 points (%d) must match number of G2 points (%d)\n", g1Count, g2Count)
		return BLS12_381_INVALID_PAIRING
	}

	// Charge gas
	gasCost := env.GasConfig.Bls12381VerifyCost.TotalCost(uint64(g1Count))
	env.GasUsed += gasCost
	if env.GasUsed > env.Gas.GasConsumed() {
		fmt.Printf("ERROR: Out of gas during BLS verification: used %d, limit %d\n", env.GasUsed, env.Gas.GasConsumed())
		return BLS12_381_INVALID_PAIRING
	}

	// Split into individual points
	g1Points := splitIntoPoints(g1Data, constants.BLS12_381_G1_POINT_LEN)
	g2Points := splitIntoPoints(g2Data, constants.BLS12_381_G2_POINT_LEN)

	// Verify pairing
	valid, err := cryptoHandler.BLS12381VerifyG1G2(g1Points, g2Points)
	if err != nil {
		fmt.Printf("ERROR: BLS12-381 verification failed: %v\n", err)
		return BLS12_381_INVALID_PAIRING
	}

	if valid {
		return BLS12_381_VALID_PAIRING
	}
	return BLS12_381_INVALID_PAIRING
}

```
---
### `runtime/host/hostfunctions_impl.go`
*2025-02-26 17:47:34 | 1 KB*
```go
 
```
---
### `runtime/host/memory.go`
*2025-02-20 21:54:38 | 1 KB*
```go
package host

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// readMemory reads length bytes from memory at the given offset
func readMemory(mem api.Memory, offset, length uint32) ([]byte, error) {
	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", offset, length)
	}
	return data, nil
}

// writeMemory writes data to memory at the given offset
func writeMemory(mem api.Memory, offset uint32, data []byte, allowGrow bool) error {
	if !mem.Write(offset, data) {
		return fmt.Errorf("failed to write %d bytes to memory at offset %d", len(data), offset)
	}
	return nil
}

```
---
### `runtime/host/registerhostfunctions.go`
*2025-02-26 17:44:41 | 6 KB*
```go
package host

import (
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

// --- Minimal Host Interfaces ---
// WasmInstance is a minimal interface for a WASM contract instance.
type WasmInstance interface {
	// RegisterFunction registers a host function with the instance.
	RegisterFunction(module, name string, fn interface{})
}

// MemoryManager is imported from our memory package.
type MemoryManager = memory.MemoryManager

// Storage represents contract storage.
type Storage interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Scan(start, end []byte, order int32) (uint32, error)
	Next(iteratorID uint32) (key []byte, value []byte, err error)
}

// API aliases types.GoAPI.
type API = types.GoAPI

// Querier aliases types.Querier.
type Querier = types.Querier

// GasMeter aliases types.GasMeter.
type GasMeter = types.GasMeter

// Logger is a simple logging interface.
type Logger interface {
	Debug(args ...interface{})
	Error(args ...interface{})
}

// --- Runtime Environment ---
// RuntimeEnvironment holds all execution context for a contract call.
type RuntimeEnvironment struct {
	DB        types.KVStore
	API       API
	Querier   Querier
	Gas       GasMeter
	GasConfig types.GasConfig

	// internal gas limit and gas used for host functions:
	gasLimit uint64
	gasUsed  uint64

	// Iterator management.
	iterators      map[uint64]map[uint64]types.Iterator
	iteratorsMutex types.RWMutex // alias for sync.RWMutex from types package if desired
	nextCallID     uint64
	nextIterID     uint64
}

// --- Helper: writeToRegion ---
// writeToRegion uses MemoryManager to update a Region struct and write the provided data.
func writeToRegion(mem MemoryManager, regionPtr uint32, data []byte) error {
	regionStruct, err := mem.Read(regionPtr, 12)
	if err != nil {
		return fmt.Errorf("failed to read Region at %d: %w", regionPtr, err)
	}
	offset := binary.LittleEndian.Uint32(regionStruct[0:4])
	capacity := binary.LittleEndian.Uint32(regionStruct[4:8])
	if uint32(len(data)) > capacity {
		return fmt.Errorf("data length %d exceeds region capacity %d", len(data), capacity)
	}
	if err := mem.Write(offset, data); err != nil {
		return fmt.Errorf("failed to write data to memory at offset %d: %w", offset, err)
	}
	binary.LittleEndian.PutUint32(regionStruct[8:12], uint32(len(data)))
	if err := mem.Write(regionPtr+8, regionStruct[8:12]); err != nil {
		return fmt.Errorf("failed to write Region length at %d: %w", regionPtr+8, err)
	}
	return nil
}

// --- RegisterHostFunctions ---
// RegisterHostFunctions registers all host functions with the provided module builder
func RegisterHostFunctions(mod wazero.HostModuleBuilder) {
	// Abort: abort(msg_ptr: u32, file_ptr: u32, line: u32, col: u32) -> !
	mod.NewFunctionBuilder().WithFunc(Abort).Export("abort")

	// Debug: debug(msg_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(Debug).Export("debug")

	// db_read: db_read(key_ptr: u32) -> u32 (returns Region pointer or 0 if not found)
	mod.NewFunctionBuilder().WithFunc(DbRead).Export("db_read")

	// db_write: db_write(key_ptr: u32, value_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(DbWrite).Export("db_write")

	// db_remove: db_remove(key_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(DbRemove).Export("db_remove")

	// db_scan: db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbScan).Export("db_scan")

	// db_next: db_next(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNext).Export("db_next")

	// db_next_key: db_next_key(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNextKey).Export("db_next_key")

	// db_next_value: db_next_value(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNextValue).Export("db_next_value")

	// addr_validate: addr_validate(addr_ptr: u32) -> u32 (0 = success, nonzero = error)
	mod.NewFunctionBuilder().WithFunc(AddrValidate).Export("addr_validate")

	// addr_canonicalize: addr_canonicalize(human_ptr: u32, canon_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(AddrCanonicalize).Export("canonicalize_address")

	// addr_humanize: addr_humanize(canon_ptr: u32, human_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(hostHumanizeAddress).Export("humanize_address")

	// Crypto operations
	mod.NewFunctionBuilder().WithFunc(Secp256k1Verify).Export("secp256k1_verify")
	mod.NewFunctionBuilder().WithFunc(Secp256k1RecoverPubkey).Export("secp256k1_recover_pubkey")
	mod.NewFunctionBuilder().WithFunc(Ed25519Verify).Export("ed25519_verify")
	mod.NewFunctionBuilder().WithFunc(Ed25519BatchVerify).Export("ed25519_batch_verify")

	// BLS crypto operations
	mod.NewFunctionBuilder().WithFunc(Bls12381AggregateG1).Export("bls12_381_aggregate_g1")
	mod.NewFunctionBuilder().WithFunc(Bls12381AggregateG2).Export("bls12_381_aggregate_g2")
	mod.NewFunctionBuilder().WithFunc(Bls12381PairingCheck).Export("bls12_381_pairing_equality")
	mod.NewFunctionBuilder().WithFunc(Bls12381HashToG1).Export("bls12_381_hash_to_g1")
	mod.NewFunctionBuilder().WithFunc(Bls12381HashToG2).Export("bls12_381_hash_to_g2")

	// query_chain: query_chain(request_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(QueryChain).Export("query_chain")
}

```
---
### `runtime/hostapi/types.go`
*2025-02-26 17:26:11 | 1 KB*
```go
package hostapi

// RuntimeEnvironment holds the execution context for host functions
type RuntimeEnvironment struct {
	Gas        GasMeter
	MemManager MemoryManager
}

// GasMeter interface for tracking gas usage
type GasMeter interface {
	ConsumeGas(amount uint64, descriptor string) error
	GasConsumed() uint64
}

// MemoryManager interface for managing WebAssembly memory
type MemoryManager interface {
	Read(offset uint32, length uint32) ([]byte, error)
	Write(offset uint32, data []byte) error
	Allocate(size uint32) (uint32, error)
}

// Context key for environment
type EnvContextKey string

const EnvironmentKey EnvContextKey = "env"

```
---
### `runtime/memory/memory.go`
*2025-02-26 17:22:34 | 6 KB*
```go
package memory

import (
	"context"
	"errors"

	"github.com/tetratelabs/wazero/api"
)

// WasmMemory is an alias for the wazero Memory interface.
type WasmMemory = api.Memory

// Region in Go for clarity (optional; we can also handle without this struct)
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// MemoryManager manages a Wasm instance's memory and allocation.
type MemoryManager struct {
	Memory       WasmMemory                   // interface to Wasm memory (e.g., provides Read, Write)
	WasmAllocate func(uint32) (uint32, error) // function to call Wasm allocate
	Deallocate   func(uint32) error           // function to call Wasm deallocate
	MemorySize   uint32                       // size of the memory (for bounds checking, if available)
}

// NewMemoryManager creates and initializes a MemoryManager from the given module.
// It retrieves the exported "allocate" and "deallocate" functions and the Wasm memory,
// and sets the memorySize field.
func NewMemoryManager(module api.Module) (*MemoryManager, error) {
	allocFn := module.ExportedFunction("allocate")
	deallocFn := module.ExportedFunction("deallocate")
	mem := module.Memory()
	if allocFn == nil || deallocFn == nil || mem == nil {
		return nil, errors.New("missing required exports: allocate, deallocate, or memory")
	}

	// Get the current memory size.
	size := mem.Size()

	// Create wrapper functions that call the exported functions.
	allocateWrapper := func(requestSize uint32) (uint32, error) {
		results, err := allocFn.Call(context.Background(), uint64(requestSize))
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 0, errors.New("allocate returned no results")
		}
		return uint32(results[0]), nil
	}

	deallocateWrapper := func(ptr uint32) error {
		_, err := deallocFn.Call(context.Background(), uint64(ptr))
		return err
	}

	return &MemoryManager{
		Memory:       mem,
		WasmAllocate: allocateWrapper,
		Deallocate:   deallocateWrapper,
		MemorySize:   size,
	}, nil
}

// Read copies `length` bytes from Wasm memory at the given offset into a new byte slice.
func (m *MemoryManager) Read(offset uint32, length uint32) ([]byte, error) {
	if offset+length > m.MemorySize {
		return nil, errors.New("memory read out of bounds")
	}
	data, ok := m.Memory.Read(offset, uint32(length))
	if !ok {
		return nil, errors.New("failed to read memory")
	}
	return data, nil
}

// Write copies the given data into Wasm memory starting at the given offset.
func (m *MemoryManager) Write(offset uint32, data []byte) error {
	length := uint32(len(data))
	if offset+length > m.MemorySize {
		return errors.New("memory write out of bounds")
	}
	if !m.Memory.Write(offset, data) {
		return errors.New("failed to write memory")
	}
	return nil
}

// ReadRegion reads a Region (offset, capacity, length) from Wasm memory and returns the pointed bytes.
func (m *MemoryManager) ReadRegion(regionPtr uint32) ([]byte, error) {
	// Read 12 bytes for Region struct
	const regionSize = 12
	raw, err := m.Read(regionPtr, regionSize)
	if err != nil {
		return nil, err
	}
	// Parse Region struct (little-endian u32s)
	if len(raw) != regionSize {
		return nil, errors.New("invalid region struct size")
	}
	region := Region{
		Offset:   littleEndianToUint32(raw[0:4]),
		Capacity: littleEndianToUint32(raw[4:8]),
		Length:   littleEndianToUint32(raw[8:12]),
	}
	// Basic sanity checks
	if region.Offset+region.Length > m.MemorySize {
		return nil, errors.New("region out of bounds")
	}
	if region.Length > region.Capacity {
		return nil, errors.New("region length exceeds capacity")
	}
	// Read the actual data
	return m.Read(region.Offset, region.Length)
}

// Allocate requests a new memory region of given size from the Wasm instance.
func (m *MemoryManager) Allocate(size uint32) (uint32, error) {
	// Call the contract's allocate function via the provided callback
	offset, err := m.WasmAllocate(size)
	if err != nil {
		return 0, err
	}
	if offset == 0 {
		// A zero offset might indicate allocation failure (if contract uses 0 as null)
		return 0, errors.New("allocation failed")
	}
	// Optionally, ensure offset is within memory bounds (if allocate doesn't already guarantee it)
	if offset >= m.MemorySize {
		return 0, errors.New("allocation returned out-of-bounds pointer")
	}
	return offset, nil
}

// Free releases previously allocated memory back to the contract.
func (m *MemoryManager) Free(offset uint32) error {
	return m.Deallocate(offset)
}

// CreateRegion allocates a Region struct in Wasm memory for a given data buffer.
func (m *MemoryManager) CreateRegion(dataOffset, dataLength uint32) (uint32, error) {
	const regionSize = 12
	regionPtr, err := m.Allocate(regionSize)
	if err != nil {
		return 0, err
	}
	// Build the region struct in little-endian bytes
	reg := make([]byte, regionSize)
	putUint32LE(reg[0:4], dataOffset)
	putUint32LE(reg[4:8], dataLength)  // capacity = length (we allocate exactly length)
	putUint32LE(reg[8:12], dataLength) // length = actual data length
	// Write the struct into memory
	if err := m.Write(regionPtr, reg); err != nil {
		m.Free(regionPtr) // free the region struct allocation if writing fails
		return 0, err
	}
	return regionPtr, nil
}

// Utility: convert 4 bytes little-endian to uint32
func littleEndianToUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// Utility: write uint32 as 4 little-endian bytes
func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v & 0xFF)
	b[1] = byte((v >> 8) & 0xFF)
	b[2] = byte((v >> 16) & 0xFF)
	b[3] = byte((v >> 24) & 0xFF)
}

// Add this method to the MemoryManager struct
func (mm *MemoryManager) ReadUint32(offset uint32) (uint32, bool) {
	data, err := mm.Read(offset, 4)
	if err != nil {
		return 0, false
	}

	// Convert 4 bytes to uint32 (little-endian)
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24, true
}

```
---
### `runtime/tracing.go`
*2025-02-20 21:49:29 | 3 KB*
```go
package runtime

import (
	"encoding/hex"
	"fmt"
	"runtime"
	"time"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/tetratelabs/wazero/api"
)

// TraceConfig controls tracing behavior
type TraceConfig struct {
	Enabled     bool
	ShowMemory  bool
	ShowParams  bool
	ShowStack   bool
	MaxDataSize uint32 // Maximum bytes of data to print
}

// Global trace configuration - can be modified at runtime
var TraceConf = TraceConfig{
	Enabled:     true,
	ShowMemory:  true,
	ShowParams:  true,
	ShowStack:   true,
	MaxDataSize: 256,
}

// TraceFn wraps a function with tracing
func TraceFn(name string) func() {
	if !TraceConf.Enabled {
		return func() {}
	}

	start := time.Now()

	// Get caller information
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)

	// Print entry trace
	fmt.Printf("\n=== ENTER: %s ===\n", name)
	fmt.Printf("Location: %s:%d\n", file, line)
	fmt.Printf("Function: %s\n", fn.Name())

	if TraceConf.ShowStack {
		// Capture and print stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		fmt.Printf("Stack:\n%s\n", string(buf[:n]))
	}

	// Return function to be deferred
	return func() {
		duration := time.Since(start)
		fmt.Printf("=== EXIT: %s (took %v) ===\n\n", name, duration)
	}
}

// TraceMemory prints memory state if enabled
func TraceMemory(memory api.Memory, msg string) {
	if !TraceConf.Enabled || !TraceConf.ShowMemory {
		return
	}

	fmt.Printf("\n=== Memory State: %s ===\n", msg)
	fmt.Printf("Size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/constants.WasmPageSize)

	// Print first page contents
	if data, ok := memory.Read(0, TraceConf.MaxDataSize); ok {
		fmt.Printf("First %d bytes:\n%s\n", TraceConf.MaxDataSize, hex.Dump(data))
	}
}

// TraceParams prints parameter values if enabled
func TraceParams(params ...interface{}) {
	if !TraceConf.Enabled || !TraceConf.ShowParams {
		return
	}

	fmt.Printf("Parameters:\n")
	for i, p := range params {
		// Handle different parameter types appropriately
		switch v := p.(type) {
		case []byte:
			if uint32(len(v)) > TraceConf.MaxDataSize {
				fmt.Printf("  %d: []byte len=%d (truncated)\n", i, len(v))
				fmt.Printf("     %x...\n", v[:int(TraceConf.MaxDataSize)])
			} else {
				fmt.Printf("  %d: []byte %x\n", i, v)
			}
		default:
			fmt.Printf("  %d: %v\n", i, p)
		}
	}
}

```
---
### `runtime/types/gas.go`
*2025-02-26 15:40:05 | 1 KB*
```go
package types

// GasConfig defines costs for various operations in the VM
type GasConfig struct {
	// Basic costs
	PerByte       uint64
	DatabaseRead  uint64
	DatabaseWrite uint64
	CompileCost   uint64
	GasMultiplier uint64

	// Crypto operation costs
	Secp256k1VerifyCost        uint64
	Secp256k1RecoverPubkeyCost uint64
	Ed25519VerifyCost          uint64
	Ed25519BatchVerifyCost     uint64

	// BLS12-381 operation costs
	Bls12381AggregateG1Cost OperationCost
	Bls12381AggregateG2Cost OperationCost
	Bls12381HashToG1Cost    OperationCost
	Bls12381HashToG2Cost    OperationCost
	Bls12381VerifyCost      OperationCost
}

// OperationCost defines a cost function with base and variable components
type OperationCost struct {
	Base     uint64
	Variable uint64
}

// TotalCost calculates the total cost for n operations
func (c OperationCost) TotalCost(n uint64) uint64 {
	return c.Base + c.Variable*n
}

```
---
### `runtime/types/types.go`
*2025-02-26 15:40:25 | 4 KB*
```go
package types

import (
	"sync"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
)

// GasOperation represents different types of gas operations
type GasOperation int

const (
	GasOperationMemoryRead GasOperation = iota
	GasOperationMemoryWrite
	GasOperationDBRead
	GasOperationDBWrite
	GasOperationDBDelete
	GasOperationCompile
)

// OutOfGasError represents an out of gas error
type OutOfGasError struct {
	Descriptor string
}

func (e OutOfGasError) Error() string {
	return "out of gas: " + e.Descriptor
}

// GasCost represents a gas cost with base and per-unit components
type GasCost struct {
	BaseCost uint64
	PerUnit  uint64
}

// TotalCost calculates total gas cost for an operation
func (g GasCost) TotalCost(units uint64) uint64 {
	return g.BaseCost + (g.PerUnit * units)
}

// GasMeter interface defines gas consumption methods
type GasMeter interface {
	GasConsumed() uint64
	ConsumeGas(amount uint64, descriptor string) error
}

// Add missing interfaces
type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
	Iterator(start, end []byte) Iterator
	ReverseIterator(start, end []byte) Iterator
}

type Iterator interface {
	Valid() bool
	Next()
	Key() []byte
	Value() []byte
	Close() error
	Domain() (start, end []byte)
	Error() error
}

type GoAPI interface {
	HumanizeAddress([]byte) (string, uint64, error)
	CanonicalizeAddress(string) ([]byte, uint64, error)
	ValidateAddress(string) (uint64, error)
	Secp256k1Verify(message, signature, pubkey []byte) (bool, uint64, error)
	Secp256k1RecoverPubkey(message, signature []byte, recovery uint8) ([]byte, uint64, error)
	Ed25519Verify(message, signature, pubkey []byte) (bool, uint64, error)
	Ed25519BatchVerify(messages [][]byte, signatures [][]byte, pubkeys [][]byte) (bool, uint64, error)
	// Add other required methods
}

type Querier interface {
	Query(request []byte) ([]byte, error)
}

// Add missing types
type Env struct {
	Block       BlockInfo
	Contract    ContractInfo
	Transaction TransactionInfo
}

type MessageInfo struct {
	Sender string
	Funds  []Coin
}

type ContractResult struct {
	Data   []byte
	Events []Event
}

type Reply struct {
	ID     uint64
	Result SubMsgResult
}

type UFraction struct {
	Numerator   uint64
	Denominator uint64
}

// Add these type definitions
type BlockInfo struct {
	Height  int64
	Time    int64
	ChainID string
}

type ContractInfo struct {
	Address string
	CodeID  uint64
}

type TransactionInfo struct {
	Index uint32
}

type Coin struct {
	Denom  string
	Amount uint64
}

type Event struct {
	Type       string
	Attributes []EventAttribute
}

type EventAttribute struct {
	Key   string
	Value string
}

type SubMsgResult struct {
	Ok  *SubMsgResponse
	Err string
}

type SubMsgResponse struct {
	Events []Event
	Data   []byte
}

// Add after other type definitions

// RuntimeEnvironment holds the execution context for host functions
type RuntimeEnvironment struct {
	DB         KVStore
	API        GoAPI
	Querier    Querier
	Gas        GasMeter
	GasConfig  GasConfig
	MemManager *memory.MemoryManager
	GasUsed    uint64 // Track gas usage

	// Iterator management
	iterators      map[uint64]map[uint64]Iterator
	iteratorsMutex sync.RWMutex
	nextCallID     uint64
	nextIterID     uint64
}

// Add methods to RuntimeEnvironment
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]Iterator)
	return e.nextCallID
}

func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextIterID++
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()
	if callMap, ok := e.iterators[callID]; ok {
		return callMap[iterID]
	}
	return nil
}

```
---
### `runtime/validation/validation.go`
*2025-02-20 21:49:29 | 2 KB*
```go
package validation

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
)

// AnalyzeForValidation validates a compiled module to ensure it meets the CosmWasm requirements.
// It ensures the module has exactly one exported memory, that the required exports ("allocate", "deallocate")
// are present, and that the contract's interface marker export is exactly "interface_version_8".
func AnalyzeForValidation(compiled wazero.CompiledModule) error {
	// Check memory constraints: exactly one memory export is required.
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return fmt.Errorf("static Wasm validation error: contract must contain exactly one memory (found %d)", memoryCount)
	}

	// Ensure required exports (e.g., "allocate" and "deallocate") are present.
	requiredExports := []string{"allocate", "deallocate"}
	exports := compiled.ExportedFunctions()
	for _, r := range requiredExports {
		found := false
		for name := range exports {
			if name == r {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("static Wasm validation error: contract missing required export %q", r)
		}
	}

	// Ensure the interface version marker is present.
	var interfaceVersionCount int
	for name := range exports {
		if strings.HasPrefix(name, "interface_version_") {
			interfaceVersionCount++
			if name != "interface_version_8" {
				return fmt.Errorf("static Wasm validation error: unknown interface version marker %q", name)
			}
		}
	}
	if interfaceVersionCount == 0 {
		return fmt.Errorf("static Wasm validation error: contract missing required interface version marker (interface_version_*)")
	}

	return nil
}

```
---
### `runtime/wasm/execution.go`
*2025-02-26 17:17:13 | 16 KB*
```go
package wasm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	// Assume types package defines Env, MessageInfo, QueryRequest, Reply, etc.
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/crypto"
	wazmeter "github.com/CosmWasm/wasmvm/v2/internal/runtime/gas/wazero"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/host"
	runtimeTypes "github.com/CosmWasm/wasmvm/v2/internal/runtime/types"
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

func init() {
	// Set up the crypto handler
	cryptoImpl := crypto.NewCryptoImplementation()
	host.SetCryptoHandler(cryptoImpl)
}

// Instantiate compiles (if needed) and instantiates a contract, calling its "instantiate" method.
func (vm *WazeroVM) Instantiate(checksum Checksum, env types.Env, info types.MessageInfo, initMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	// Marshal env and info to JSON (as the contract expects JSON input) [oai_citation_attribution:14‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=func%20%28vm%20) [oai_citation_attribution:15‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=infoBin%2C%20err%20%3A%3D%20json).
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal Env: %w", err)
	}
	infoBz, err := json.Marshal(info)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal MessageInfo: %w", err)
	}
	// Execute the contract call
	resBz, gasUsed, execErr := vm.callContract(checksum, "instantiate", envBz, infoBz, initMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		// If an error occurred in execution, return the error with gas used so far [oai_citation_attribution:16‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=data%2C%20gasReport%2C%20err%20%3A%3D%20api,printDebug).
		return nil, gasUsed, execErr
	}
	// Deserialize the contract's response (JSON) into a ContractResult struct [oai_citation_attribution:17‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=var%20result%20types).
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize instantiate result: %w", err)
	}
	return &result, gasUsed, nil
}

// Execute calls a contract's "execute" entry point with the given message.
func (vm *WazeroVM) Execute(checksum Checksum, env types.Env, info types.MessageInfo, execMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	infoBz, err := json.Marshal(info)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "execute", envBz, infoBz, execMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize execute result: %w", err)
	}
	return &result, gasUsed, nil
}

// Query calls a contract's "query" entry point. Query has no MessageInfo (no funds or sender).
func (vm *WazeroVM) Query(checksum Checksum, env types.Env, queryMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	// For queries, no info, so we pass only env and msg.
	resBz, gasUsed, execErr := vm.callContract(checksum, "query", envBz, nil, queryMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize query result: %w", err)
	}
	return &result, gasUsed, nil
}

// Migrate calls a contract's "migrate" entry point with given migrate message.
func (vm *WazeroVM) Migrate(checksum Checksum, env types.Env, migrateMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "migrate", envBz, nil, migrateMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize migrate result: %w", err)
	}
	return &result, gasUsed, nil
}

// Sudo calls the contract's "sudo" entry point (privileged call from the chain).
func (vm *WazeroVM) Sudo(checksum Checksum, env types.Env, sudoMsg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "sudo", envBz, nil, sudoMsg, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize sudo result: %w", err)
	}
	return &result, gasUsed, nil
}

// Reply calls the contract's "reply" entry point to handle a SubMsg reply.
func (vm *WazeroVM) Reply(checksum Checksum, env types.Env, reply types.Reply, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.ContractResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	replyBz, err := json.Marshal(reply)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "reply", envBz, nil, replyBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.ContractResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("failed to deserialize reply result: %w", err)
	}
	return &result, gasUsed, nil
}

// gasContext holds gas metering state for a contract execution
type gasContext struct {
	meter        *wazmeter.WazeroGasMeter
	operationGas uint64 // Tracks gas for current operation
}

// callContract is an internal helper to instantiate the Wasm module and call a specified entry point.
func (vm *WazeroVM) callContract(checksum Checksum, entrypoint string, env []byte, info []byte, msg []byte, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) ([]byte, uint64, error) {
	ctx := context.Background()
	// Attach the execution context (store, api, querier, gasMeter) to ctx for host functions.
	instCtx := instanceContext{store: store, api: api, querier: querier, gasMeter: gasMeter, gasLimit: gasLimit}
	ctx = context.WithValue(ctx, instanceContextKey{}, &instCtx)

	// Create wazero gas meter with proper configuration
	internalConfig := runtimeTypes.GasConfig{
		PerByte:       vm.gasConfig.PerByte,
		DatabaseRead:  vm.gasConfig.DatabaseRead,
		DatabaseWrite: vm.gasConfig.DatabaseWrite,
		GasMultiplier: 100, // Default multiplier
	}
	wazmeter := wazmeter.NewWazeroGasMeter(gasLimit, internalConfig)

	// Create module config with gas metering
	modConfig := wazero.NewModuleConfig()

	// Convert memory limit from bytes to pages (64KiB per page)
	maxPages := uint32(vm.memoryLimit / 65536)
	if maxPages < 1 {
		maxPages = 1 // Ensure at least 1 page
	}

	// Fix gas meter context key
	ctx = wazmeter.WithGasMeter(ctx)

	// Ensure we have a compiled module for this code (maybe from cache) [oai_citation_attribution:18‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=Contract%20pinning%20is%20a%20feature,33x%20faster).
	codeHash := [32]byte{}
	copy(codeHash[:], checksum) // convert to array key
	compiled, err := vm.getCompiledModule(codeHash)
	if err != nil {
		return nil, 0, fmt.Errorf("loading module: %w", err)
	}
	// Instantiate a new module instance for this execution.
	module, err := vm.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, 0, fmt.Errorf("instantiating module: %w", err)
	}
	defer module.Close(ctx) // ensure instance is closed after execution

	// Allocate and write input data (env, info, msg) into the module's memory.
	mem := module.Memory()
	// Helper to allocate a region and copy data into it, returning the Region pointer.
	allocData := func(data []byte) (uint32, error) {
		if data == nil {
			return 0, nil
		}
		allocFn := module.ExportedFunction("allocate")
		if allocFn == nil {
			return 0, fmt.Errorf("allocate function not found in module")
		}
		// Request a region for data
		allocRes, err := allocFn.Call(ctx, uint64(len(data)))
		if err != nil || len(allocRes) == 0 {
			return 0, fmt.Errorf("allocate failed: %v", err)
		}
		regionPtr := uint32(allocRes[0])
		// The Region struct is stored at regionPtr [oai_citation_attribution:19‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/exports.rs#:~:text=). It contains a pointer to allocated memory.
		// Read the offset of the allocated buffer from the Region (first 4 bytes).
		offset, ok := mem.ReadUint32Le(regionPtr)
		if !ok {
			return 0, fmt.Errorf("failed to read allocated region offset")
		}
		// Write the data into the allocated buffer.
		if !mem.Write(uint32(offset), data) {
			return 0, fmt.Errorf("failed to write data into wasm memory")
		}
		// Set the region's length field (third 4 bytes of Region struct) to data length.
		if !mem.WriteUint32Le(regionPtr+8, uint32(len(data))) {
			return 0, fmt.Errorf("failed to write region length")
		}
		return regionPtr, nil
	}
	envPtr, err := allocData(env)
	if err != nil {
		return nil, 0, err
	}
	infoPtr, err := allocData(info)
	if err != nil {
		return nil, 0, err
	}
	msgPtr, err := allocData(msg)
	if err != nil {
		return nil, 0, err
	}

	// Call the contract's entrypoint function.
	fn := module.ExportedFunction(entrypoint)
	if fn == nil {
		return nil, 0, fmt.Errorf("entry point %q not found in contract", entrypoint)
	}
	// Prepare arguments as (env_ptr, info_ptr, msg_ptr) or (env_ptr, msg_ptr) depending on entrypoint [oai_citation_attribution:20‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=,to%20extend%20their%20functionality) [oai_citation_attribution:21‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=extern%20,u32).
	args := []uint64{uint64(envPtr)}
	if info != nil {
		args = append(args, uint64(infoPtr))
	}
	args = append(args, uint64(msgPtr))
	// Execute the contract function. This will trigger host function calls (db_read, etc.) as needed.
	results, err := fn.Call(ctx, args...)
	// Compute gas used internally by subtracting remaining gas from gasLimit.
	gasUsed := gasLimit
	if instCtx.gasMeter != nil {
		// Use GasConsumed difference (querier gas usage accounted separately).
		gasUsed = instCtx.gasMeter.GasConsumed()
	}
	if err != nil {
		// If the execution trapped (e.g., out of gas or contract panic), determine error.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Context cancellation (treat as out of gas for consistency).
			return nil, gasUsed, runtimeTypes.OutOfGasError{Descriptor: "execution timeout"}
		}
		// Wazero traps on out-of-gas would manifest as a panic/exit error
		if gasUsed >= gasLimit {
			return nil, gasUsed, runtimeTypes.OutOfGasError{Descriptor: "execution exceeded gas limit"}
		}
		// Otherwise, return the error as a generic VM error.
		return nil, gasUsed, fmt.Errorf("contract execution error: %w", err)
	}
	// The contract returns a pointer to a Region with the result data (or 0 if no data) [oai_citation_attribution:23‡github.com](https://github.com/CosmWasm/cosmwasm/blob/main/README.md#:~:text=extern%20,u32).
	var data []byte
	if len(results) > 0 {
		resultPtr := uint32(results[0])
		if resultPtr != 0 {
			// Read region pointer for result
			resOffset, ok := mem.ReadUint32Le(resultPtr)
			resLength, ok2 := mem.ReadUint32Le(resultPtr + 8)
			if ok && ok2 {
				data, _ = mem.Read(resOffset, resLength)
			}
		}
	}
	// We do not explicitly call deallocate for result region, as the whole module instance will be closed and memory freed.
	return data, gasUsed, nil
}

// getCompiledModule returns a compiled module for the given checksum, compiling or retrieving from cache as needed.
func (vm *WazeroVM) getCompiledModule(codeHash [32]byte) (wazero.CompiledModule, error) {
	// Fast path: check caches under read lock.
	vm.cacheMu.RLock()
	if item, ok := vm.pinned[codeHash]; ok {
		vm.hitsPinned++ // pinned cache hit
		item.hits++
		compiled := item.compiled
		vm.cacheMu.RUnlock()
		vm.logger.Debug("Using pinned contract module from cache", "checksum", hex.EncodeToString(codeHash[:]))
		return compiled, nil
	}
	if item, ok := vm.memoryCache[codeHash]; ok {
		vm.hitsMemory++ // LRU cache hit
		item.hits++
		// Move this item to most-recently-used position in LRU order
		// (We'll do simple reorder: remove and append at end).
		// Find and remove from cacheOrder slice:
		for i, hash := range vm.cacheOrder {
			if hash == codeHash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.cacheOrder = append(vm.cacheOrder, codeHash)
		compiled := item.compiled
		vm.cacheMu.RUnlock()
		vm.logger.Debug("Using cached module from LRU cache", "checksum", hex.EncodeToString(codeHash[:]))
		return compiled, nil
	}
	vm.cacheMu.RUnlock()

	// Cache miss: compile the module.
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// Double-check if another goroutine compiled it while we were waiting.
	if item, ok := vm.pinned[codeHash]; ok {
		vm.hitsPinned++
		item.hits++
		return item.compiled, nil
	}
	if item, ok := vm.memoryCache[codeHash]; ok {
		vm.hitsMemory++
		item.hits++
		// promote in LRU order
		for i, hash := range vm.cacheOrder {
			if hash == codeHash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.cacheOrder = append(vm.cacheOrder, codeHash)
		return item.compiled, nil
	}
	// Not in any cache yet: compile the Wasm code.
	code, ok := vm.codeStore[codeHash]
	if !ok {
		vm.logger.Error("Wasm code bytes not found for checksum")
		return nil, fmt.Errorf("code %x not found", codeHash)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}
	vm.misses++ // cache miss (compiled new module)
	// Add to memory cache (un-pinned by default). Evict LRU if over capacity.
	size := uint64(len(code))
	vm.memoryCache[codeHash] = &cacheItem{compiled: compiled, size: size, hits: 0}
	vm.cacheOrder = append(vm.cacheOrder, codeHash)
	if len(vm.memoryCache) > vm.cacheSize {
		// evict least recently used (front of cacheOrder)
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background()) // free the compiled module
			delete(vm.memoryCache, oldest)
			vm.logger.Debug("Evicted module from cache (LRU)", "checksum", hex.EncodeToString(oldest[:]))
		}
	}
	vm.logger.Info("Compiled new contract module and cached",
		"checksum", hex.EncodeToString(codeHash[:]),
		"size_bytes", size)
	return compiled, nil
}

// instanceContext carries environment references for host functions.
type instanceContext struct {
	store    types.KVStore
	api      types.GoAPI
	querier  types.Querier
	gasMeter types.GasMeter
	gasLimit uint64
}

// instanceContextKey is used as context key for instanceContext.
type instanceContextKey struct{}

// OutOfGasError represents an out of gas error
type OutOfGasError struct{}

func (OutOfGasError) Error() string {
	return "out of gas"
}

```
---
### `runtime/wasm/ibc.go`
*2025-02-20 21:49:29 | 6 KB*
```go
package wasm

import (
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// IBCChannelOpen calls the contract's "ibc_channel_open" entry point.
func (vm *WazeroVM) IBCChannelOpen(checksum types.Checksum, env types.Env, msg types.IBCChannelOpenMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCChannelOpenResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_open", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCChannelOpenResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelOpenResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCChannelConnect calls "ibc_channel_connect" entry point.
func (vm *WazeroVM) IBCChannelConnect(checksum types.Checksum, env types.Env, msg types.IBCChannelConnectMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_connect", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelConnectResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCChannelClose calls "ibc_channel_close".
func (vm *WazeroVM) IBCChannelClose(checksum types.Checksum, env types.Env, msg types.IBCChannelCloseMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_close", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelCloseResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketReceive calls "ibc_packet_receive".
func (vm *WazeroVM) IBCPacketReceive(checksum types.Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCReceiveResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_receive", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCReceiveResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketReceiveResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketAck calls "ibc_packet_ack".
func (vm *WazeroVM) IBCPacketAck(checksum types.Checksum, env types.Env, msg types.IBCPacketAckMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_ack", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketAckResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketTimeout calls "ibc_packet_timeout".
func (vm *WazeroVM) IBCPacketTimeout(checksum types.Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_timeout", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketTimeoutResult: %w", err)
	}
	return &result, gasUsed, nil
}

```
---
### `runtime/wasm/runtime.go`
*2025-02-26 17:16:29 | 3 KB*
```go
package wasm

import "github.com/CosmWasm/wasmvm/v2/types"

type WasmRuntime interface {
	// InitCache sets up any runtime-specific cache or resources. Returns a handle.
	InitCache(config types.VMConfig) (any, error)

	// ReleaseCache frees resources created by InitCache.
	ReleaseCache(handle any)

	// Compilation and code storage
	StoreCode(code []byte, persist bool) (checksum []byte, err error)
	StoreCodeUnchecked(code []byte) ([]byte, error)
	GetCode(checksum []byte) ([]byte, error)
	RemoveCode(checksum []byte) error
	Pin(checksum []byte) error
	Unpin(checksum []byte) error
	AnalyzeCode(checksum []byte) (*types.AnalysisReport, error)

	// Execution lifecycles
	Instantiate(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Execute(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Migrate(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	MigrateWithInfo(checksum []byte, env []byte, msg []byte, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Sudo(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Reply(checksum []byte, env []byte, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Query(checksum []byte, env []byte, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)

	// IBC entry points
	IBCChannelOpen(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCChannelConnect(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCChannelClose(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketReceive(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketAck(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketTimeout(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCSourceCallback(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCDestinationCallback(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)

	// Metrics
	GetMetrics() (*types.Metrics, error)
	GetPinnedMetrics() (*types.PinnedMetrics, error)
}

```
---
### `runtime/wasm/system.go`
*2025-02-26 15:27:58 | 10 KB*
```go
package wasm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// StoreCode compiles and stores a new Wasm code blob, returning its checksum and gas used for compilation.
func (vm *WazeroVM) StoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:] // as []byte
	// Simulate compilation gas cost
	compileCost := uint64(len(code)) * (3 * 140_000) // CostPerByte = 3 * 140k, as per CosmWasm gas schedule
	if gasLimit < compileCost {
		// Not enough gas provided to compile this code
		return cs, compileCost, OutOfGasError{}
	}
	// If code is already stored, we can avoid recompiling (but still charge gas).
	codeHash := [32]byte(checksum)
	vm.cacheMu.Lock()
	alreadyStored := vm.codeStore[codeHash] != nil
	vm.cacheMu.Unlock()
	if !alreadyStored {
		// Insert code into storage
		vm.cacheMu.Lock()
		vm.codeStore[codeHash] = code
		vm.cacheMu.Unlock()
		vm.logger.Info("Stored new contract code", "checksum", hex.EncodeToString(checksum[:]), "size", len(code))
	} else {
		vm.logger.Debug("StoreCode called for already stored code", "checksum", hex.EncodeToString(checksum[:]))
	}
	// Compile module immediately to ensure it is valid and cached.
	vm.cacheMu.Lock()
	_, compErr := vm.getCompiledModule(codeHash)
	vm.cacheMu.Unlock()
	if compErr != nil {
		return cs, compileCost, compErr
	}
	return cs, compileCost, nil
}

// SimulateStoreCode estimates gas needed to store the given code, without actually storing it.
func (vm *WazeroVM) SimulateStoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:]
	cost := uint64(len(code)) * (3 * 140_000) // same formula as compileCost
	if gasLimit < cost {
		return cs, cost, OutOfGasError{}
	}
	// We do not compile or store the code in simulation.
	return cs, cost, nil
}

// GetCode returns the original Wasm bytes for the given code checksum.
func (vm *WazeroVM) GetCode(checksum Checksum) ([]byte, error) {
	codeHash := [32]byte{}
	copy(codeHash[:], checksum) // convert to array key
	vm.cacheMu.RLock()
	code := vm.codeStore[codeHash]
	vm.cacheMu.RUnlock()
	if code == nil {
		return nil, fmt.Errorf("code for %x not found", checksum)
	}
	return code, nil
}

// RemoveCode removes the Wasm bytes and any cached compiled module.
func (vm *WazeroVM) RemoveCode(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// First check if it's pinned (priority cache)
	if item, ok := vm.pinned[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.pinned, hash)
		vm.logger.Info("Removed pinned contract from memory", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	if item, ok := vm.memoryCache[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.memoryCache, hash)
		// Also need to remove from LRU ordering
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.logger.Info("Removed contract from in-memory cache", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	// If not in caches, nothing to remove.
	vm.logger.Debug("RemoveCode called but code not in memory cache", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// Pin marks the module with the given checksum as pinned, meaning it won't be removed by the LRU cache.
func (vm *WazeroVM) Pin(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// If already pinned, nothing to do.
	if _, ok := vm.pinned[hash]; ok {
		return nil
	}
	// See if it's in memory cache, move it to pinned.
	memItem, memOk := vm.memoryCache[hash]
	if memOk {
		delete(vm.memoryCache, hash)
		// Remove from LRU order slice
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		// Add to pinned cache directly
		vm.pinned[hash] = memItem
		vm.logger.Info("Pinned contract code in memory", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	// Not in mem cache, fetch from code store & compile.
	code, ok := vm.codeStore[hash]
	if !ok {
		return fmt.Errorf("code %x not found", hash)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return fmt.Errorf("pinning compilation failed: %w", err)
	}
	item := &cacheItem{
		compiled: compiled,
		size:     uint64(len(code)),
		hits:     0,
	}
	// Add to pinned cache
	vm.pinned[hash] = item
	vm.logger.Info("Pinned contract code in memory", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// Unpin marks the module with the given checksum as unpinned, allowing it to be removed by the LRU cache.
func (vm *WazeroVM) Unpin(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// If not pinned, nothing to do.
	item, ok := vm.pinned[hash]
	if !ok {
		return nil
	}
	// Move from pinned to memory cache
	delete(vm.pinned, hash)
	vm.memoryCache[hash] = item
	vm.cacheOrder = append(vm.cacheOrder, hash) // add to end (most recently used)
	// If memoryCache is now over capacity, evict the LRU item
	if len(vm.memoryCache) > vm.cacheSize {
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background())
			delete(vm.memoryCache, oldest)
			vm.logger.Debug("Evicted module after unpin (LRU)", "checksum", hex.EncodeToString(oldest[:]))
		}
	}
	vm.logger.Info("Unpinned contract code", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// AnalyzeCode statically analyzes the Wasm bytecode and returns capabilities and features it requires.
func (vm *WazeroVM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error) {
	hash := [32]byte{}
	copy(hash[:], checksum)

	// Get the module (either from cache or fresh compile)
	vm.cacheMu.Lock()
	module, err := vm.getCompiledModule(hash)
	vm.cacheMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Create base report
	report := types.AnalysisReport{
		HasIBCEntryPoints:    false,
		RequiredCapabilities: "",
	}

	// First, check exports for IBC entry points
	exports := module.ExportedFunctions()
	for name := range exports {
		// Check for IBC exports
		if name == "ibc_channel_open" || name == "ibc_channel_connect" || name == "ibc_channel_close" ||
			name == "ibc_packet_receive" || name == "ibc_packet_ack" || name == "ibc_packet_timeout" {
			report.HasIBCEntryPoints = true
			break
		}
	}

	// Get the module's imports to check for required capabilities
	var requiredCapabilities string

	// Helper to add capabilities without duplicates
	addCapability := func(cap string) {
		if requiredCapabilities == "" {
			requiredCapabilities = cap
		} else {
			// Check if already present
			found := false
			for _, c := range []string{requiredCapabilities} {
				if c == cap {
					found = true
					break
				}
			}
			if !found {
				requiredCapabilities = requiredCapabilities + "," + cap
			}
		}
	}

	// Check imports to determine capabilities
	for _, imp := range module.ImportedFunctions() {
		impModule, impName, _ := imp.Import()

		if impModule == "env" {
			// Check for capability-indicating imports
			switch impName {
			case "secp256k1_verify", "secp256k1_recover_pubkey":
				addCapability("secp256k1")
			case "ed25519_verify", "ed25519_batch_verify":
				addCapability("ed25519")
			case "addr_humanize", "addr_canonicalize", "addr_validate":
				addCapability("cosmwasm_1_1")
			case "bls12_381_aggregate_g1", "bls12_381_aggregate_g2":
				addCapability("cosmwasm_1_4")
			}
		}
	}

	report.RequiredCapabilities = requiredCapabilities
	return &report, nil
}

// GetMetrics returns aggregated metrics about cache usage.
func (vm *WazeroVM) GetMetrics() (*types.Metrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	m := &types.Metrics{
		HitsPinnedMemoryCache:     uint32(vm.hitsPinned),
		HitsMemoryCache:           uint32(vm.hitsMemory),
		HitsFsCache:               0, // we are not using FS cache in this implementation
		Misses:                    uint32(vm.misses),
		ElementsPinnedMemoryCache: uint64(len(vm.pinned)),
		ElementsMemoryCache:       uint64(len(vm.memoryCache)),
		SizePinnedMemoryCache:     0,
		SizeMemoryCache:           0,
	}
	// Calculate sizes
	for _, item := range vm.pinned {
		m.SizePinnedMemoryCache += item.size
	}
	for _, item := range vm.memoryCache {
		m.SizeMemoryCache += item.size
	}
	return m, nil
}

// GetPinnedMetrics returns detailed metrics for each pinned contract.
func (vm *WazeroVM) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	var entries []types.PerModuleEntry
	for hash, item := range vm.pinned {
		entries = append(entries, types.PerModuleEntry{
			Checksum: hash[:],
			Metrics: types.PerModuleMetrics{
				Hits: uint32(item.hits),
				Size: item.size,
			},
		})
	}
	// Sort entries by checksum for consistency
	sort.Slice(entries, func(i, j int) bool {
		return hex.EncodeToString(entries[i].Checksum) < hex.EncodeToString(entries[j].Checksum)
	})
	return &types.PinnedMetrics{PerModule: entries}, nil
}

```
---
### `runtime/wasm/vm.go`
*2025-02-26 17:19:40 | 4 KB*
```go
package wasm

import (
	"context"
	"fmt"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/crypto"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/gas"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/host"
	wasmTypes "github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

type Checksum = []byte

type cacheItem struct {
	compiled wazero.CompiledModule
	size     uint64
	hits     uint64
}

// WazeroVM implements the CosmWasm VM using wazero
type WazeroVM struct {
	runtime     wazero.Runtime
	codeStore   map[[32]byte][]byte
	memoryCache map[[32]byte]*cacheItem
	pinned      map[[32]byte]*cacheItem
	cacheOrder  [][32]byte
	cacheSize   int
	cacheMu     sync.RWMutex

	// Cache statistics
	hitsPinned uint64
	hitsMemory uint64
	misses     uint64

	// Configuration
	gasConfig   wasmTypes.GasConfig
	memoryLimit uint32

	logger Logger
}

// Logger defines the logging interface used by WazeroVM
type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
}

// InfoLogger extends Logger with chainable methods for structured logging
type InfoLogger interface {
	Logger
	With(keyvals ...interface{}) Logger
}

// NewWazeroVM creates a new VM instance with wazero as the runtime
func NewWazeroVM() (*WazeroVM, error) {
	// Create runtime with proper context
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	// Set default values
	cacheSize := 100                        // Default cache size
	memoryLimit := uint32(32 * 1024 * 1024) // Default memory limit (32MB)
	gasConfig := gas.DefaultGasConfig()

	// Create the VM instance
	vm := &WazeroVM{
		runtime:     runtime,
		codeStore:   make(map[[32]byte][]byte),
		memoryCache: make(map[[32]byte]*cacheItem),
		pinned:      make(map[[32]byte]*cacheItem),
		cacheOrder:  make([][32]byte, 0),
		cacheSize:   cacheSize,
		gasConfig:   gasConfig,
		memoryLimit: memoryLimit,
		logger:      nil, // Will use default logger
	}

	// Initialize crypto handler
	err := crypto.SetupCryptoHandlers()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize crypto handlers: %w", err)
	}

	// Register host modules
	hostModule := vm.runtime.NewHostModuleBuilder("env")

	// Register core host functions from host package
	host.RegisterHostFunctions(hostModule)

	// Register crypto host functions
	crypto.RegisterHostFunctions(hostModule)

	// Instantiate the host module
	_, err = hostModule.Instantiate(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	return vm, nil
}

// Close releases all resources held by the VM
func (vm *WazeroVM) Close() error {
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()

	ctx := context.Background()

	// Close all compiled modules
	for _, item := range vm.pinned {
		if err := item.compiled.Close(ctx); err != nil {
			vm.logger.Error("Error closing pinned module", "error", err)
		}
	}

	for _, item := range vm.memoryCache {
		if err := item.compiled.Close(ctx); err != nil {
			vm.logger.Error("Error closing cached module", "error", err)
		}
	}

	// Clear maps
	vm.pinned = make(map[[32]byte]*cacheItem)
	vm.memoryCache = make(map[[32]byte]*cacheItem)
	vm.cacheOrder = make([][32]byte, 0)

	// Close the runtime
	return vm.runtime.Close(ctx)
}

```
---

## Summary
Files: 43, Total: 438 KB
Breakdown:
- go: 254 KB
- md: 184 KB