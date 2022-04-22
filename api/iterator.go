package api

import (
	dbm "github.com/tendermint/tm-db"
	"sync"
)

// frame stores all Iterators for one contract call
type frame []dbm.Iterator

// iteratorFrames contains one frame for each contract call, indexed by contract call ID.
var iteratorFrames sync.Map

// latestCallID is a global counter for creating call IDs.
// Instead of using a mutex, https://pkg.go.dev/sync/atomic#AddUint64 could be used. But
// at least on ARM this did not create any measurable improvement.
var latestCallID uint64
var latestCallIDMutex sync.Mutex

// startCall is called at the beginning of a contract call to create a new frame in iteratorFrames.
// It updates latestCallID for generating a new call ID.
func startCall() uint64 {
	latestCallIDMutex.Lock()
	defer latestCallIDMutex.Unlock()
	latestCallID += 1
	return latestCallID
}

// endCall is called at the end of a contract call to remove one item from the iteratorFrames
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

// storeIterator will add this to the end of the frame for the given ID and return a reference to it.
// We start counting with 1, so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func storeIterator(callID uint64, it dbm.Iterator) uint64 {
	loadedFrame, found := iteratorFrames.Load(callID)
	var newFrame frame
	if found {
		newFrame = loadedFrame.(frame)
	} else {
		newFrame = make(frame, 0, 8)
	}
	newFrame = append(newFrame, it)
	iteratorIndex := uint64(len(newFrame))
	iteratorFrames.Store(callID, newFrame)
	return iteratorIndex
}

// retrieveIterator will recover an iterator based on index. This ensures it will not be garbage collected.
// We start counting with 1, in storeIterator so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func retrieveIterator(callID uint64, index uint64) dbm.Iterator {
	loadedFrame, found := iteratorFrames.Load(callID)
	if found {
		return (loadedFrame.(frame))[index-1]
	} else {
		return nil
	}
}
