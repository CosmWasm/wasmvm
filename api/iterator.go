package api

import (
	"fmt"
	dbm "github.com/tendermint/tm-db"
)

// frame stores all Iterators for one contract
type frame []dbm.Iterator

// iteratorStack contains one frame for each contract, indexed by a counter
// TODO: protect access (in buildDBState) via mutex
var iteratorStack = make(map[uint64]frame, 60)

// this is a global counter when we create DBs
// TODO: protect access (in buildDBState) via mutex
var dbCounter uint64

func nextCounter() uint64 {
	// TODO: add mutex
	dbCounter += 1
	return dbCounter
}

// startContract is called at the beginning of a contract runtime to create a new frame on the iteratorStack
// updates dbCounter for an index
func startContract() uint64 {
	counter := nextCounter()
	// TODO: remove debug
	fmt.Printf("startContract: new frame: %d\n", counter)
	return counter
}

// endContract is called at the end of a contract runtime to remove one item from the IteratorStack
func endContract(counter uint64) {
	// TODO: remove debug
	fmt.Printf("endContract: remove frame: %d\n", counter)

	// get the item from the stack
	remove := iteratorStack[counter]
	delete(iteratorStack, counter)

	// free all iterators in the frame when we release it
	for _, iter := range remove {
		fmt.Printf("endContract: close iterator\n")
		iter.Close()
	}
}

// storeIterator will add this to the end of the latest stack and return a reference to it.
// We start counting with 1, so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func storeIterator(dbCounter uint64, it dbm.Iterator) uint64 {
	frame := append(iteratorStack[dbCounter], it)
	iteratorStack[dbCounter] = frame
	index := len(frame)
	// TODO: remove debug
	fmt.Printf("store iterator: counter (idx): %d (%d)\n", dbCounter, index)
	return uint64(index)
}

// retrieveIterator will recover an iterator based on index. This ensures it will not be garbage collected.
// We start counting with 1, in storeIterator so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func retrieveIterator(dbCounter uint64, index uint64) dbm.Iterator {
	// TODO: remove debug
	fmt.Printf("retrieveIterator: height (idx/size): %d (%d/%d)\n", dbCounter, index, len(iteratorStack[dbCounter]))
	return iteratorStack[dbCounter][index-1]
}
