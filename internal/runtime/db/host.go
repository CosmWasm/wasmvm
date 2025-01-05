package db

import (
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// HostFunctions provides database-related host functions
type HostFunctions struct {
	store     *Store
	mem       *memory.HostFunctions
	allocator *memory.Allocator
}

// NewHostFunctions creates a new set of database host functions
func NewHostFunctions(store types.KVStore, mem *memory.HostFunctions, allocator *memory.Allocator) *HostFunctions {
	return &HostFunctions{
		store:     New(store),
		mem:       mem,
		allocator: allocator,
	}
}

// Get reads a value from the store
func (h *HostFunctions) Get(keyPtr uint32) (uint32, error) {
	// Read key region
	keyRegion, err := h.mem.ReadRegion(keyPtr)
	if err != nil {
		return 0, err
	}

	// Read key from memory
	key, err := h.mem.ReadString(keyRegion)
	if err != nil {
		return 0, err
	}

	// Get value from store
	value := h.store.Get([]byte(key))
	if value == nil {
		return 0, nil
	}

	// Write value to memory and return pointer
	ptr, _, err := h.allocator.Allocate(value)
	if err != nil {
		return 0, err
	}

	return ptr, nil
}

// Set writes a value to the store
func (h *HostFunctions) Set(keyPtr, valuePtr uint32) error {
	// Read key region
	keyRegion, err := h.mem.ReadRegion(keyPtr)
	if err != nil {
		return err
	}

	// Read value region
	valueRegion, err := h.mem.ReadRegion(valuePtr)
	if err != nil {
		return err
	}

	// Read key and value from memory
	key, err := h.mem.ReadString(keyRegion)
	if err != nil {
		return err
	}

	value, err := h.mem.ReadString(valueRegion)
	if err != nil {
		return err
	}

	// Set value in store
	h.store.Set([]byte(key), []byte(value))
	return nil
}

// Delete removes a key from the store
func (h *HostFunctions) Delete(keyPtr uint32) error {
	// Read key region
	keyRegion, err := h.mem.ReadRegion(keyPtr)
	if err != nil {
		return err
	}

	// Read key from memory
	key, err := h.mem.ReadString(keyRegion)
	if err != nil {
		return err
	}

	// Delete from store
	h.store.Delete([]byte(key))
	return nil
}
