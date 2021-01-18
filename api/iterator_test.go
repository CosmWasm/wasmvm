package api

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/types"
)

type queueData struct {
	id      []byte
	store   *Lookup
	api     *GoAPI
	querier types.Querier
}

func (q queueData) Store(meter MockGasMeter) KVStore {
	return q.store.WithGasMeter(meter)
}

func setupQueueContractWithData(t *testing.T, cache Cache, values ...int) queueData {
	id := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	igasMeter1 := GasMeter(gasMeter1)
	res, flags, _, err := Instantiate(cache, id, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.False(t, flags.IBCEnabled)
	assert.False(t, flags.Stargate)

	for _, value := range values {
		// push 17
		var gasMeter2 GasMeter = NewMockGasMeter(TESTING_GAS_LIMIT)
		push := []byte(fmt.Sprintf(`{"enqueue":{"value":%d}}`, value))
		res, _, err = Handle(cache, id, env, info, push, &gasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err)
		requireOkResponse(t, res, 0)
	}

	return queueData{
		id:      id,
		store:   store,
		api:     api,
		querier: querier,
	}
}

func setupQueueContract(t *testing.T, cache Cache) queueData {
	return setupQueueContractWithData(t, cache, 17, 22)
}

func TestQueueIterator(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	id, querier, api := setup.id, setup.querier, setup.api

	// query the sum
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	store := setup.Store(gasMeter)
	query := []byte(`{"sum":{}}`)
	env := MockEnvBin(t)
	data, _, err := Query(cache, id, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Equal(t, "", qres.Err)
	require.Equal(t, `{"sum":39}`, string(qres.Ok))

	// query reduce (multiple iterators at once)
	query = []byte(`{"reducer":{}}`)
	data, _, err = Query(cache, id, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var reduced types.QueryResponse
	err = json.Unmarshal(data, &reduced)
	require.NoError(t, err)
	require.Equal(t, "", reduced.Err)
	require.Equal(t, `{"counters":[[17,22],[22,0]]}`, string(reduced.Ok))
}

func TestQueueIteratorRaces(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	assert.Equal(t, len(iteratorStack), 0)

	contract1 := setupQueueContractWithData(t, cache, 17, 22)
	contract2 := setupQueueContractWithData(t, cache, 1, 19, 6, 35, 8)
	contract3 := setupQueueContractWithData(t, cache, 11, 6, 2)
	env := MockEnvBin(t)

	reduceQuery := func(t *testing.T, setup queueData, expected string) {
		id, querier, api := setup.id, setup.querier, setup.api
		gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
		igasMeter := GasMeter(gasMeter)
		store := setup.Store(gasMeter)

		// query reduce (multiple iterators at once)
		query := []byte(`{"reducer":{}}`)
		data, _, err := Query(cache, id, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err)
		var reduced types.QueryResponse
		err = json.Unmarshal(data, &reduced)
		require.NoError(t, err)
		require.Equal(t, "", reduced.Err)
		require.Equal(t, fmt.Sprintf(`{"counters":%s}`, expected), string(reduced.Ok))
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

	// when they finish, we should have popped everything off the stack
	assert.Equal(t, len(iteratorStack), 0)
}
