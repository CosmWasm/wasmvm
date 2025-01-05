package iterator

import (
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// Manager handles database iterators
type Manager struct {
	mu        sync.RWMutex
	iterators map[uint64]types.Iterator
	nextID    uint64
}

// New creates a new iterator manager
func New() *Manager {
	return &Manager{
		iterators: make(map[uint64]types.Iterator),
		nextID:    1,
	}
}

// Create stores an iterator and returns its ID
func (m *Manager) Create(iter types.Iterator) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextID
	m.iterators[id] = iter
	m.nextID++
	return id
}

// Get retrieves an iterator by its ID
func (m *Manager) Get(id uint64) (types.Iterator, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iter, exists := m.iterators[id]
	return iter, exists
}

// Remove deletes an iterator
func (m *Manager) Remove(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if iter, exists := m.iterators[id]; exists {
		iter.Close()
		delete(m.iterators, id)
	}
}

// RemoveAll deletes all iterators
func (m *Manager) RemoveAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, iter := range m.iterators {
		iter.Close()
	}
	m.iterators = make(map[uint64]types.Iterator)
}
