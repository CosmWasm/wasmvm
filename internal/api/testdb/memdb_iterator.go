package testdb

import (
	"bytes"
	"context"

	"github.com/google/btree"
)

const (
	// Size of the channel buffer between traversal goroutine and iterator. Using an unbuffered
	// channel causes two context switches per item sent, while buffering allows more work per
	// context switch. Tuned with benchmarks.
	chBufferSize = 64
)

// memDBIterator is a memDB iterator.
type memDBIterator struct {
	ch     <-chan *item
	cancel context.CancelFunc
	item   *item
	start  []byte
	end    []byte
	useMtx bool
}

var _ Iterator = (*memDBIterator)(nil)

// newMemDBIteratorAscending creates a new memDBIterator that iterates in ascending order.
func newMemDBIteratorAscending(db *MemDB, start, end []byte) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
		useMtx: true,
	}

	db.mtx.RLock()
	go func() {
		defer db.mtx.RUnlock()
		vs := newVisitorState(ctx, ch)
		db.traverseAscending(start, end, vs)
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}
	return iter
}

// newMemDBIteratorDescending creates a new memDBIterator that iterates in descending order.
func newMemDBIteratorDescending(db *MemDB, start, end []byte) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
		useMtx: true,
	}

	db.mtx.RLock()
	go func() {
		defer db.mtx.RUnlock()
		vs := newVisitorState(ctx, ch)
		db.traverseDescending(start, end, vs)
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}
	return iter
}

// visitorState holds the state needed for the visitor function
type visitorState struct {
	ctx           context.Context
	ch            chan<- *item
	skipEqual     []byte
	abortLessThan []byte
}

// newVisitorState creates a new visitorState
func newVisitorState(ctx context.Context, ch chan<- *item) *visitorState {
	return &visitorState{
		ctx: ctx,
		ch:  ch,
	}
}

// visitor is the function that processes each item in the btree
func (vs *visitorState) visitor(i btree.Item) bool {
	item, ok := i.(*item)
	if !ok {
		panic("btree item is not of type *item") // Should ideally not happen
	}
	if vs.skipEqual != nil && bytes.Equal(item.key, vs.skipEqual) {
		vs.skipEqual = nil
		return true
	}
	if vs.abortLessThan != nil && bytes.Compare(item.key, vs.abortLessThan) == -1 {
		return false
	}
	select {
	case <-vs.ctx.Done():
		return false
	case vs.ch <- item:
		return true
	}
}

// traverseAscending handles ascending traversal cases
func (db *MemDB) traverseAscending(start, end []byte, vs *visitorState) {
	//nolint:gocritic // The switch {} is clearer than other switch forms here
	switch {
	case start == nil && end == nil:
		db.btree.Ascend(vs.visitor)
	case end == nil:
		db.btree.AscendGreaterOrEqual(newKey(start), vs.visitor)
	default:
		db.btree.AscendRange(newKey(start), newKey(end), vs.visitor)
	}
}

// traverseDescending handles descending traversal cases
func (db *MemDB) traverseDescending(start, end []byte, vs *visitorState) {
	if end == nil {
		// abort after start, since we use [start, end) while btree uses (start, end]
		vs.abortLessThan = start
		db.btree.Descend(vs.visitor)
	} else {
		// skip end and abort after start, since we use [start, end) while btree uses (start, end]
		vs.skipEqual = end
		vs.abortLessThan = start
		db.btree.DescendLessOrEqual(newKey(end), vs.visitor)
	}
}

// newMemDBIteratorNoMtxAscending creates a new memDBIterator that iterates in ascending order without locking.
func newMemDBIteratorNoMtxAscending(db *MemDB, start, end []byte) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
		useMtx: false,
	}

	go func() {
		vs := newVisitorState(ctx, ch)
		db.traverseAscending(start, end, vs)
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}
	return iter
}

// newMemDBIteratorNoMtxDescending creates a new memDBIterator that iterates in descending order without locking.
func newMemDBIteratorNoMtxDescending(db *MemDB, start, end []byte) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
		useMtx: false,
	}

	go func() {
		vs := newVisitorState(ctx, ch)
		db.traverseDescending(start, end, vs)
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}
	return iter
}

// Close implements Iterator.
func (i *memDBIterator) Close() error {
	i.cancel()
	// Drain the channel synchronously to ensure the traversal goroutine completes
	for item := range i.ch {
		_ = item // explicitly discard the item
	}
	i.item = nil
	return nil
}

// Domain implements Iterator.
func (i *memDBIterator) Domain() (start []byte, end []byte) {
	return i.start, i.end
}

// Valid implements Iterator.
func (i *memDBIterator) Valid() bool {
	return i.item != nil
}

// Next implements Iterator.
func (i *memDBIterator) Next() {
	i.assertIsValid()
	item, ok := <-i.ch
	switch {
	case ok:
		i.item = item
	default:
		i.item = nil
	}
}

// Error implements the types.Iterator interface.
func (*memDBIterator) Error() error {
	return nil // famous last words
}

// Key implements Iterator.
func (i *memDBIterator) Key() []byte {
	i.assertIsValid()
	return i.item.key
}

// Value implements Iterator.
func (i *memDBIterator) Value() []byte {
	i.assertIsValid()
	return i.item.value
}

func (i *memDBIterator) assertIsValid() {
	if !i.Valid() {
		panic("iterator is invalid")
	}
}
