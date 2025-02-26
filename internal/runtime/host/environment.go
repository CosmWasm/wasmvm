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
