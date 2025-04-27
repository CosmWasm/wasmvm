// queue_iterator_test.go

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

// queueData wraps contract info to make test usage easier
type queueData struct {
	checksum []byte
	store    *Lookup
	api      *types.GoAPI
	querier  types.Querier
}

// Store provides a KVStore with an updated gas meter
func (q queueData) Store(meter MockGasMeter) types.KVStore {
	return q.store.WithGasMeter(meter)
}

// setupQueueContractWithData uploads/instantiates a queue contract, optionally enqueuing data
func setupQueueContractWithData(t *testing.T, cache Cache, values ...int) queueData {
	t.Helper()
	checksum := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})

	// Initialize with empty msg (`{}`)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	igasMeter1 := types.GasMeter(gasMeter1)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err, "Instantiation must succeed")
	requireOkResponse(t, res, 0)

	// Optionally enqueue some integer values
	for _, value := range values {
		var gasMeter2 types.GasMeter = NewMockGasMeter(TESTING_GAS_LIMIT)
		push := fmt.Appendf(nil, `{"enqueue":{"value":%d}}`, value)
		res, _, err = Execute(cache, checksum, env, info, push, &gasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err, "Enqueue must succeed for value %d", value)
		requireOkResponse(t, res, 0)
	}

	return queueData{
		checksum: checksum,
		store:    store,
		api:      api,
		querier:  querier,
	}
}

// setupQueueContract is a convenience that uses default enqueued values
func setupQueueContract(t *testing.T, cache Cache) queueData {
	t.Helper()
	return setupQueueContractWithData(t, cache, 17, 22)
}

//---------------------
// Table-based tests
//---------------------

func TestStoreIterator_TableDriven(t *testing.T) {
	type testCase struct {
		name    string
		actions []func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error)
		expect  []uint64 // expected return values from storeIterator
	}

	store := testdb.NewMemDB()
	const limit = 2000

	// We’ll define 2 callIDs, each storing a few iterators
	callID1 := startCall()
	callID2 := startCall()

	// Action helper: open a new iterator, then call storeIterator
	createIter := func(t *testing.T, store *testdb.MemDB) types.Iterator {
		t.Helper()
		iter, _ := store.Iterator(nil, nil)
		require.NotNil(t, iter, "iter creation must not fail")
		return iter
	}

	// We define test steps where each function returns a (uint64, error).
	// Then we compare with the expected result (uint64) if error is nil.
	tests := []testCase{
		{
			name: "CallID1: two iterators in sequence",
			actions: []func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error){
				func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error) {
					t.Helper()
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
				func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error) {
					t.Helper()
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
			},
			expect: []uint64{1, 2}, // first call ->1, second call ->2
		},
		{
			name: "CallID2: three iterators in sequence",
			actions: []func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error){
				func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error) {
					t.Helper()
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
				func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error) {
					t.Helper()
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
				func(t *testing.T, store *testdb.MemDB, callID uint64, limit int) (uint64, error) {
					t.Helper()
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
			},
			expect: []uint64{1, 2, 3},
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			var results []uint64
			// Decide which callID to use by name
			// We'll do a simple check:
			var activeCallID uint64
			if tc.name == "CallID1: two iterators in sequence" {
				activeCallID = callID1
			} else {
				activeCallID = callID2
			}

			for i, step := range tc.actions {
				got, err := step(t, store, activeCallID, limit)
				require.NoError(t, err, "storeIterator must not fail in step[%d]", i)
				results = append(results, got)
			}
			require.Equal(t, tc.expect, results, "Mismatch in expected results for test '%s'", tc.name)
		})
	}

	// Cleanup
	endCall(callID1)
	endCall(callID2)
}

func TestStoreIteratorHitsLimit_TableDriven(t *testing.T) {
	const limit = 2
	callID := startCall()
	store := testdb.NewMemDB()

	// We want to store iterators up to limit and then exceed
	tests := []struct {
		name       string
		numIters   int
		shouldFail bool
	}{
		{
			name:       "Store 1st iter (success)",
			numIters:   1,
			shouldFail: false,
		},
		{
			name:       "Store 2nd iter (success)",
			numIters:   2,
			shouldFail: false,
		},
		{
			name:       "Store 3rd iter (exceeds limit =2)",
			numIters:   3,
			shouldFail: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			iter, _ := store.Iterator(nil, nil)
			_, err := storeIterator(callID, iter, limit)
			if tc.shouldFail {
				require.ErrorContains(t, err, "Reached iterator limit (2)")
			} else {
				require.NoError(t, err, "should not exceed limit for test '%s'", tc.name)
			}
		})
	}

	endCall(callID)
}

func TestRetrieveIterator_TableDriven(t *testing.T) {
	const limit = 2000
	callID1 := startCall()
	callID2 := startCall()

	store := testdb.NewMemDB()

	// Setup initial iterators
	iterA, _ := store.Iterator(nil, nil)
	idA, err := storeIterator(callID1, iterA, limit)
	require.NoError(t, err)
	iterB, _ := store.Iterator(nil, nil)
	_, err = storeIterator(callID1, iterB, limit)
	require.NoError(t, err)

	iterC, _ := store.Iterator(nil, nil)
	_, err = storeIterator(callID2, iterC, limit)
	require.NoError(t, err)
	iterD, _ := store.Iterator(nil, nil)
	idD, err := storeIterator(callID2, iterD, limit)
	require.NoError(t, err)
	iterE, _ := store.Iterator(nil, nil)
	idE, err := storeIterator(callID2, iterE, limit)
	require.NoError(t, err)

	tests := []struct {
		name      string
		callID    uint64
		iterID    uint64
		expectNil bool
	}{
		{
			name:      "Retrieve existing iter idA on callID1",
			callID:    callID1,
			iterID:    idA,
			expectNil: false,
		},
		{
			name:      "Retrieve existing iter idD on callID2",
			callID:    callID2,
			iterID:    idD,
			expectNil: false,
		},
		{
			name:      "Retrieve ID from different callID => nil",
			callID:    callID1,
			iterID:    idE, // e belongs to callID2
			expectNil: true,
		},
		{
			name:      "Retrieve zero => nil",
			callID:    callID1,
			iterID:    0,
			expectNil: true,
		},
		{
			name:      "Retrieve large => nil",
			callID:    callID1,
			iterID:    18446744073709551615,
			expectNil: true,
		},
		{
			name:      "Non-existent callID => nil",
			callID:    callID1 + 1234567,
			iterID:    idE,
			expectNil: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			iter := retrieveIterator(tc.callID, tc.iterID)
			if tc.expectNil {
				require.Nil(t, iter, "expected nil for test: %s", tc.name)
			} else {
				require.NotNil(t, iter, "expected a valid iterator for test: %s", tc.name)
			}
		})
	}

	endCall(callID1)
	endCall(callID2)
}

func TestQueueIteratorSimple_TableDriven(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	tests := []struct {
		name    string
		query   string
		expErr  string
		expResp string
	}{
		{
			name:    "sum query => 39",
			query:   `{"sum":{}}`,
			expErr:  "",
			expResp: `{"sum":39}`,
		},
		{
			name:    "reducer query => counters",
			query:   `{"reducer":{}}`,
			expErr:  "",
			expResp: `{"counters":[[17,22],[22,0]]}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			igasMeter := types.GasMeter(gasMeter)
			store := setup.Store(gasMeter)
			env := MockEnvBin(t)

			data, _, err := Query(
				cache,
				checksum,
				env,
				[]byte(tc.query),
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)
			require.NoError(t, err, "Query must not fail in scenario: %s", tc.name)

			var result types.QueryResult
			err = json.Unmarshal(data, &result)
			require.NoError(t, err,
				"JSON decode of QueryResult must succeed in scenario: %s", tc.name)
			require.Equal(t, tc.expErr, result.Err,
				"Mismatch in 'Err' for scenario %s", tc.name)
			require.Equal(t, tc.expResp, string(result.Ok),
				"Mismatch in 'Ok' response for scenario %s", tc.name)
		})
	}
}

func TestQueueIteratorRaces_TableDriven(t *testing.T) {
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

		query := []byte(`{"reducer":{}}`)
		data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err)
		var r types.QueryResult
		err = json.Unmarshal(data, &r)
		require.NoError(t, err)
		require.Equal(t, "", r.Err)
		require.Equal(t, fmt.Sprintf(`{"counters":%s}`, expected), string(r.Ok))
	}

	// We define a table for the concurrent contract calls
	tests := []struct {
		name           string
		contract       queueData
		expectedResult string
	}{
		{"contract1", contract1, "[[17,22],[22,0]]"},
		{"contract2", contract2, "[[1,68],[19,35],[6,62],[35,0],[8,54]]"},
		{"contract3", contract3, "[[11,0],[6,11],[2,17]]"},
	}

	const numBatches = 30
	var wg sync.WaitGroup
	wg.Add(numBatches * len(tests))

	// The same concurrency approach, but now in a loop
	for i := 0; i < numBatches; i++ {
		for _, tc := range tests {
			tc := tc
			go func() {
				reduceQuery(t, tc.contract, tc.expectedResult)
				wg.Done()
			}()
		}
	}
	wg.Wait()

	// when they finish, we should have removed all frames
	require.Empty(t, iteratorFrames)
}

func TestQueueIteratorLimit_TableDriven(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	tests := []struct {
		name        string
		count       int
		multiplier  int
		expectError bool
		errContains string
	}{
		{
			name:        "Open 5000 iterators, no error",
			count:       5000,
			multiplier:  1,
			expectError: false,
		},
		{
			name:        "Open 35000 iterators => exceed limit(32768)",
			count:       35000,
			multiplier:  4,
			expectError: true,
			errContains: "Reached iterator limit (32768)",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gasLimit := TESTING_GAS_LIMIT * uint64(tc.multiplier)
			gasMeter := NewMockGasMeter(gasLimit)
			igasMeter := types.GasMeter(gasMeter)
			store := setup.Store(gasMeter)
			env := MockEnvBin(t)

			msg := fmt.Sprintf(`{"open_iterators":{"count":%d}}`, tc.count)
			data, _, err := Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, gasLimit, TESTING_PRINT_DEBUG)
			if tc.expectError {
				require.Error(t, err, "Expected an error in test '%s'", tc.name)
				require.Contains(t, err.Error(), tc.errContains, "Error mismatch in test '%s'", tc.name)
				return
			}
			require.NoError(t, err, "No error expected in test '%s'", tc.name)

			// decode the success
			var qResult types.QueryResult
			err = json.Unmarshal(data, &qResult)
			require.NoError(t, err, "JSON decode must succeed in test '%s'", tc.name)
			require.Equal(t, "", qResult.Err, "Expected no error in QueryResult for test '%s'", tc.name)
			require.Equal(t, `{}`, string(qResult.Ok),
				"Expected an empty obj response for test '%s'", tc.name)
		})
	}
}

//--------------------
// Suggestions
//--------------------
//
// 1. We added more debug logs (e.g., inline string formatting, ensuring we mention scenario names).
// 2. For concurrency tests (like "races"), we used table-driven expansions for concurrency loops.
// 3. We introduced partial success/failure checks for error messages using `require.Contains` or `require.Equal`.
// 4. You can expand your negative test cases to verify what happens if the KVStore fails or the env is invalid.
// 5. For even more thorough coverage, you might add invalid parameters or zero-limit scenarios to the tables.
