package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
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

// Store returns a KVStore with a given gas meter attached.
func (q queueData) Store(meter MockGasMeter) types.KVStore {
	return q.store.WithGasMeter(meter)
}

// setupQueueContractWithData instantiates the queue contract with initial data
// and optionally enqueues the provided values.
func setupQueueContractWithData(t *testing.T, cache Cache, values ...int) queueData {
	checksum := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(testingGasLimit)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	igasMeter1 := types.GasMeter(gasMeter1)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	for _, value := range values {
		gasMeter2 := NewMockGasMeter(testingGasLimit)
		igasMeter2 := types.GasMeter(gasMeter2)
		push := []byte(fmt.Sprintf(`{"enqueue":{"value":%d}}`, value))
		res, _, err = Execute(cache, checksum, env, info, push, &igasMeter2, store, api, &querier, testingGasLimit, testingPrintDebug)
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

// setupQueueContract instantiates a queue contract and enqueues two values: 17 and 22.
func setupQueueContract(t *testing.T, cache Cache) queueData {
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

	// Retrieve with non-existent iterator IDs
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
	cache, _ := withCache(t) // No need for t.Cleanup(cleanup)

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	gasMeter := NewMockGasMeter(testingGasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := setup.Store(gasMeter)

	// query the sum
	query := []byte(`{"sum":{}}`)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.Equal(t, `{"sum":39}`, string(qResult.Ok))

	// query reduce (multiple iterators at once)
	query = []byte(`{"reducer":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
	require.NoError(t, err)
	var reduced types.QueryResult
	err = json.Unmarshal(data, &reduced)
	require.NoError(t, err)
	require.Equal(t, "", reduced.Err)
	require.JSONEq(t, `{"counters":[[17,22],[22,0]]}`, string(reduced.Ok))
}

func TestQueueIteratorRaces(t *testing.T) {
	cache, _ := withCache(t) // No need for t.Cleanup(cleanup)

	assert.Empty(t, iteratorFrames)

	contract1 := setupQueueContractWithData(t, cache, 17, 22)
	contract2 := setupQueueContractWithData(t, cache, 1, 19, 6, 35, 8)
	contract3 := setupQueueContractWithData(t, cache, 11, 6, 2)
	env := MockEnvBin(t)

	// reduceQuery queries the "reducer" endpoint and compares the result to expected.
	reduceQuery := func(t *testing.T, setup queueData, expected string) {
		checksum, querier, api := setup.checksum, setup.querier, setup.api
		gasMeter := NewMockGasMeter(testingGasLimit)
		igasMeter := types.GasMeter(gasMeter)
		store := setup.Store(gasMeter)

		query := []byte(`{"reducer":{}}`)
		data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, testingGasLimit, testingPrintDebug)
		assert.NoError(t, err)
		var reduced types.QueryResult
		err = json.Unmarshal(data, &reduced)
		assert.NoError(t, err)
		assert.Equal(t, "", reduced.Err)
		assert.Equal(t, fmt.Sprintf(`{"counters":%s}`, expected), string(reduced.Ok))
	}

	// 30 concurrent batches to trigger race conditions if any
	numBatches := 30
	var wg sync.WaitGroup
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

	// after all done, no frames should remain
	assert.Empty(t, iteratorFrames)
}

func TestQueueIteratorLimit(t *testing.T) {
	cache, _ := withCache(t) // No need for t.Cleanup(cleanup)

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	// Open 5000 iterators
	gasLimit := testingGasLimit
	gasMeter := NewMockGasMeter(gasLimit)
	igasMeter := types.GasMeter(gasMeter)
	store := setup.Store(gasMeter)
	query := []byte(`{"open_iterators":{"count":5000}}`)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, gasLimit, testingPrintDebug)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.Equal(t, `{}`, string(qResult.Ok))

	// Open 35000 iterators, expecting limit error
	gasLimit = testingGasLimit * 4
	gasMeter = NewMockGasMeter(gasLimit)
	igasMeter = types.GasMeter(gasMeter)
	store = setup.Store(gasMeter)
	query = []byte(`{"open_iterators":{"count":35000}}`)
	env = MockEnvBin(t)
	_, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, gasLimit, testingPrintDebug)
	require.ErrorContains(t, err, "Reached iterator limit (32768)")
}
