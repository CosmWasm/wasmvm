package api

import (
	dbm "github.com/tendermint/tm-db"
	"sync"
)

// frame stores all Iterators for one contract
type frame []dbm.Iterator

// iteratorStack contains one frame for each contract call, indexed by contract call ID.
var iteratorStack = make(map[uint64]frame)
var iteratorStackMutex sync.Mutex

// this is a global counter for creating call IDs
var latestCallID uint64
var latestCallIDMutex sync.Mutex

// startCall is called at the beginning of a contract call to create a new frame on the iteratorStack.
// It updates latestCallID for generating a new call ID.
func startCall() uint64 {
	latestCallIDMutex.Lock()
	defer latestCallIDMutex.Unlock()
	latestCallID += 1
	return latestCallID
}

// removeFrame removes the frame with for the given call ID.
// The result can be nil when the frame is not initialized,
// i.e. when startCall() is called but no iterator is stored.
func removeFrame(callID uint64) frame {
	iteratorStackMutex.Lock()
	defer iteratorStackMutex.Unlock()

	remove := iteratorStack[callID]
	delete(iteratorStack, callID)
	return remove
}

// endCall is called at the end of a contract call to remove one item from the IteratorStack
func endCall(callID uint64) {
	// we pull removeFrame in another function so we don't hold the mutex while cleaning up the removed frame
	remove := removeFrame(callID)
	// free all iterators in the frame when we release it
	for _, iter := range remove {
		iter.Close()
	}
}

// storeIterator will add this to the end of the latest stack and return a reference to it.
// We start counting with 1, so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func storeIterator(callID uint64, it dbm.Iterator) uint64 {
	iteratorStackMutex.Lock()
	defer iteratorStackMutex.Unlock()

	frame := append(iteratorStack[callID], it)
	iteratorStack[callID] = frame
	return uint64(len(frame))
}

// retrieveIterator will recover an iterator based on index. This ensures it will not be garbage collected.
// We start counting with 1, in storeIterator so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func retrieveIterator(callID uint64, index uint64) dbm.Iterator {
	iteratorStackMutex.Lock()
	defer iteratorStackMutex.Unlock()
	return iteratorStack[callID][index-1]
}
