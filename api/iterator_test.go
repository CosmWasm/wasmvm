package api

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/go-cosmwasm/types"
)

type queueData struct {
	id      []byte
	store   KVStore
	api     *GoAPI
	querier types.Querier
}

func setupQueueContract(t *testing.T, cache Cache) queueData {
	id := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, id, params, msg, &gasMeter1, store, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// push 17
	gasMeter2 := NewMockGasMeter(100000000)
	push := []byte(`{"enqueue":{"value":17}}`)
	res, _, err = Handle(cache, id, params, push, &gasMeter2, store, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	// push 22
	gasMeter3 := NewMockGasMeter(100000000)
	push = []byte(`{"enqueue":{"value":22}}`)
	res, _, err = Handle(cache, id, params, push, &gasMeter3, store, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	return queueData{
		id:      id,
		store:   store,
		api:     api,
		querier: querier,
	}

}

func TestQueueIterator(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	id, store, querier, api := setup.id, setup.store, setup.querier, setup.api

	// query the sum
	gasMeter := NewMockGasMeter(100000000)
	query := []byte(`{"sum":{}}`)
	data, _, err := Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Nil(t, qres.Err, "%v", qres.Err)
	require.Equal(t, string(qres.Ok), `{"sum":39}`)

	// query reduce (multiple iterators at once)
	query = []byte(`{"reducer":{}}`)
	data, _, err = Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var reduced types.QueryResponse
	err = json.Unmarshal(data, &reduced)
	require.NoError(t, err)
	require.Nil(t, reduced.Err, "%v", reduced.Err)
	require.Equal(t, string(reduced.Ok), `{"counters":[[17,22],[22,0]]}`)
}

func TestQueueIteratorRaces(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	assert.Equal(t, len(iteratorStack), 0)

	setup := setupQueueContract(t, cache)

	reduceQuery := func(t *testing.T, setup queueData) {
		id, store, querier, api := setup.id, setup.store, setup.querier, setup.api

		// query reduce (multiple iterators at once)
		query := []byte(`{"reducer":{}}`)
		gasMeter := NewMockGasMeter(100000000)
		data, _, err := Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
		require.NoError(t, err)
		var reduced types.QueryResponse
		err = json.Unmarshal(data, &reduced)
		require.NoError(t, err)
		require.Nil(t, reduced.Err, "%v", reduced.Err)
		require.Equal(t, string(reduced.Ok), `{"counters":[[17,22],[22,0]]}`)
	}

	numRoutines := 10

	var wg sync.WaitGroup
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			reduceQuery(t, setup)
			wg.Done()
		}()
	}
	wg.Wait()

	// when they finish, we should have popped everything off the stack
	assert.Equal(t, len(iteratorStack), 0)
}
