package api

import (
	"fmt"
	dbm "github.com/tendermint/tm-db"
)

// This stores all Iterators for one contract
type Iterators []dbm.Iterator

// IteratorStack contains one entry for each contract, LIFO style
var IteratorStack []Iterators

// startContract is called at the beginning of a contract runtime to create a new item on the IteratorStack
func startContract() {
	IteratorStack = append(IteratorStack, nil)
	// TODO: remove debug
	fmt.Printf("startContract: Stack height: %d\n", len(IteratorStack))
}

// endContract is called at the end of a contract runtime to remove one item from the IteratorStack
func endContract() {
	l := len(IteratorStack)
	var remove Iterators
	IteratorStack, remove = IteratorStack[:l-1], IteratorStack[l-1]
	// free all iterators we pop off the stack
	for _, iter := range remove {
		fmt.Printf("endContract: close iterator\n")
		iter.Close()
	}
	// TODO: remove debug
	fmt.Printf("endContract: Stack height: %d\n", len(IteratorStack))
}

// storeIterator will add this to the end of the latest stack and return a reference to it.
// We start counting with 1, so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func storeIterator(it dbm.Iterator) int {
	l := len(IteratorStack)
	if l == 0 {
		panic("cannot storeIterator with empty iterator stack")
	}
	IteratorStack[l-1] = append(IteratorStack[l-1], it)
	// TODO: remove debug
	fmt.Printf("store iterator: height (idx): %d (%d)\n", len(IteratorStack), len(IteratorStack[l-1]))
	return len(IteratorStack[l-1])
}

// retrieveIterator will recover an iterator based on index. This ensures it will not be garbage collected.
// We start counting with 1, in storeIterator so the 0 value is flagged as an error. This means we must
// remember to do idx-1 when retrieving
func retrieveIterator(idx int) dbm.Iterator {
	l := len(IteratorStack)
	if l == 0 {
		panic("cannot retrieveIterator with empty iterator stack")
	}
	// TODO: remove debug
	fmt.Printf("retrieveIterator: height (idx/size): %d (%d/%d)\n", len(IteratorStack), idx, len(IteratorStack[l-1]))
	return IteratorStack[l-1][idx-1]
}
