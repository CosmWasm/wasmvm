package db

import (
	"bytes"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// Store implements a thread-safe key-value store
type Store struct {
	mu    sync.RWMutex
	store types.KVStore
}

// New creates a new store instance
func New(store types.KVStore) *Store {
	return &Store{
		store: store,
	}
}

// Get retrieves a value by key
func (s *Store) Get(key []byte) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.Get(key)
}

// Set stores a key-value pair
func (s *Store) Set(key, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store.Set(key, value)
}

// Delete removes a key-value pair
func (s *Store) Delete(key []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store.Delete(key)
}

// Iterator creates an iterator over a domain of keys
func (s *Store) Iterator(start, end []byte) types.Iterator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.Iterator(start, end)
}

// ReverseIterator creates a reverse iterator over a domain of keys
func (s *Store) ReverseIterator(start, end []byte) types.Iterator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.ReverseIterator(start, end)
}

// PrefixIterator creates an iterator over a domain of keys with a prefix
func (s *Store) PrefixIterator(prefix []byte) types.Iterator {
	end := calculatePrefixEnd(prefix)
	return s.Iterator(prefix, end)
}

// ReversePrefixIterator creates a reverse iterator over a domain of keys with a prefix
func (s *Store) ReversePrefixIterator(prefix []byte) types.Iterator {
	end := calculatePrefixEnd(prefix)
	return s.ReverseIterator(prefix, end)
}

// calculatePrefixEnd returns the end key for prefix iteration
func calculatePrefixEnd(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}

	end := make([]byte, len(prefix))
	copy(end, prefix)

	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end[:i+1]
		}
	}

	// If we got here, we had a prefix of all 0xff values
	return bytes.Repeat([]byte{0xff}, len(prefix))
}
