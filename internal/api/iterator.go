package api

import (
	"fmt"
	"math"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// frame stores all Iterators for one contract call
type frame struct {
	iterators []types.Iterator
	mutex     sync.Mutex
}

// iteratorFrames contains one frame for each contract call, indexed by contract call ID.
var (
	iteratorFrames      = make(map[uint64]*frame)
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
	latestCallID += 1
	callID := latestCallID

	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()
	iteratorFrames[callID] = &frame{iterators: make([]types.Iterator, 0)}
	return callID
}

// removeFrame removes the frame for the given call ID.
// The result can be nil when the frame is not initialized.
func removeFrame(callID uint64) *frame {
	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()

	f := iteratorFrames[callID]
	delete(iteratorFrames, callID)
	return f
}

// endCall is called at the end of a contract call to remove one item from iteratorFrames
func endCall(callID uint64) {
	// we pull removeFrame in another function so we don't hold the mutex while cleaning up
	f := removeFrame(callID)
	if f == nil {
		return
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	// free all iterators in the frame when we release it
	for _, iter := range f.iterators {
		iter.Close()
	}
}

// storeIterator will add this to the end of the frame for the given call ID and return
// an iterator ID to reference it.
func storeIterator(callID uint64, it types.Iterator, frameLenLimit int) (uint64, error) {
	iteratorFramesMutex.Lock()
	f, exists := iteratorFrames[callID]
	if !exists {
		iteratorFramesMutex.Unlock()
		return 0, fmt.Errorf("no frame for call ID %d", callID)
	}
	iteratorFramesMutex.Unlock()

	f.mutex.Lock()
	defer f.mutex.Unlock()

	newIndex := len(f.iterators)
	if newIndex >= frameLenLimit {
		return 0, fmt.Errorf("reached iterator limit (%d)", frameLenLimit)
	}

	// store at array position `newIndex`
	f.iterators = append(f.iterators, it)

	iteratorID, ok := indexToIteratorID(newIndex)
	if !ok {
		return 0, fmt.Errorf("could not convert index to iterator ID")
	}
	return iteratorID, nil
}

// retrieveIterator will recover an iterator based on its ID.
func retrieveIterator(callID uint64, iteratorID uint64) types.Iterator {
	indexInFrame, ok := iteratorIdToIndex(iteratorID)
	if !ok {
		return nil
	}

	iteratorFramesMutex.Lock()
	f, exists := iteratorFrames[callID]
	iteratorFramesMutex.Unlock()
	if !exists {
		return nil
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()
	if indexInFrame >= len(f.iterators) {
		return nil
	}
	return f.iterators[indexInFrame]
}

// iteratorIdToIndex converts an iterator ID to an index in the frame.
// The second value marks if the conversion succeeded.
func iteratorIdToIndex(id uint64) (int, bool) {
	if id < 1 || id > math.MaxInt32 {
		return 777777777, false
	}
	return int(id) - 1, true
}

// indexToIteratorID converts an index in the frame to an iterator ID.
// The second value marks if the conversion succeeded.
func indexToIteratorID(index int) (uint64, bool) {
	if index < 0 || index > math.MaxInt32 {
		return 888888888, false
	}
	return uint64(index) + 1, true
}
