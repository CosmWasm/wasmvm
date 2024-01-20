package api

import (
	"fmt"
	"sync"

	"github.com/CosmWasm/wasmvm/types"
)

// frame stores all Iterators for one contract call
type frame []types.Iterator

// iteratorFrames contains one frame for each contract call, indexed by contract call ID.
var iteratorFrames sync.Map

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
	return latestCallID
}

// endCall is called at the end of a contract call to remove one item the iteratorFrames
func endCall(callID uint64) {
	// The remove can be nil when the frame is not initialized,
	// i.e. when startCall() is called but no iterator is stored.
	removedFrame, didExist := iteratorFrames.LoadAndDelete(callID)

	// free all iterators in the frame when we release it
	if didExist {
		for _, iter := range removedFrame.(frame) {
			iter.Close()
		}
	}
}

// storeIterator will add this to the end of the frame for the given call ID and return
// an interator ID to reference it.
//
// We assign iterator IDs starting with 1 for historic reasons. This could be changed to 0
// I guess.
func storeIterator(callID uint64, it types.Iterator, frameLenLimit int) (uint64, error) {
	// We load, change, store the frame for a given call ID here. If storeIterator
	// is called twice for the same call ID, this is incorrect. But everything in a single
	// call is serial.

	loadedFrame, found := iteratorFrames.Load(callID)
	var newFrame frame
	if found {
		newFrame = loadedFrame.(frame) // panics if wrong type was found
	} else {
		newFrame = make(frame, 0, 8)
	}

	new_index := len(newFrame)
	if new_index >= frameLenLimit {
		return 0, fmt.Errorf("Reached iterator limit (%d)", frameLenLimit)
	}

	// store at array position `new_index`
	newFrame = append(newFrame, it)
	iteratorFrames.Store(callID, newFrame)

	iterator_id, ok := indexToIteratorID(new_index)
	if !ok {
		// This error case is not expected to happen since the above code ensures the
		// index is in the range [0, frameLenLimit-1]
		return 0, fmt.Errorf("Could not convert index to iterator ID")
	}
	return iterator_id, nil
}

// retrieveIterator will recover an iterator based on its ID.
func retrieveIterator(callID uint64, iteratorID uint64) types.Iterator {
	indexInFrame, ok := iteratorIdToIndex(iteratorID)
	if !ok {
		return nil
	}

	loadedFrame, found := iteratorFrames.Load(callID)
	if found {
		loadedFrmaeAsFrame := loadedFrame.(frame) // panics if wrong type was found
		if indexInFrame >= len(loadedFrmaeAsFrame) {
			// index out of range
			return nil
		}
		return loadedFrmaeAsFrame[indexInFrame]
	} else {
		return nil
	}
}

const (
	INT32_MAX_AS_UINT64 uint64 = 2147483647
	INT32_MAX_AS_INT    int    = 2147483647
)

// iteratorIdToIndex converts an iterator ID to an index in the frame.
// The second value marks of the conversion was succeeded.
func iteratorIdToIndex(id uint64) (int, bool) {
	if id < 1 || id > INT32_MAX_AS_UINT64 {
		return 777777777, false
	}

	// Int conversion safe because value is in signed 32bit integer range
	return int(id) - 1, true
}

// indexToIteratorID converts an index in the frame to an iterator ID.
// The second value marks of the conversion was succeeded.
func indexToIteratorID(index int) (uint64, bool) {
	if index < 0 || index > INT32_MAX_AS_INT {
		return 888888888, false
	}

	return uint64(index) + 1, true
}
