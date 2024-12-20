package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// MemoryAllocator manages memory allocation for the WASM module
type MemoryAllocator struct {
	mu sync.Mutex
	// Start of the memory region we manage
	heapStart uint32
	// Current position of the allocation pointer
	current uint32
	// Map of allocated memory regions: offset -> size
	allocations map[uint32]uint32
	// List of freed regions that can be reused
	freeList []memoryRegion
}

type memoryRegion struct {
	offset uint32
	size   uint32
}

// NewMemoryAllocator creates a new memory allocator starting at the specified offset
func NewMemoryAllocator(heapStart uint32) *MemoryAllocator {
	return &MemoryAllocator{
		heapStart:   heapStart,
		current:     heapStart,
		allocations: make(map[uint32]uint32),
		freeList:    make([]memoryRegion, 0),
	}
}

// Allocate allocates a new region of memory of the specified size
// Returns the offset where the memory was allocated
func (m *MemoryAllocator) Allocate(mem api.Memory, size uint32) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try to reuse a freed region first
	for i, region := range m.freeList {
		if region.size >= size {
			// Remove this region from free list
			m.freeList = append(m.freeList[:i], m.freeList[i+1:]...)
			m.allocations[region.offset] = size
			return region.offset, nil
		}
	}

	// No suitable freed region found, allocate new memory
	offset := m.current

	// Check if we have enough memory
	memSize := mem.Size()
	if offset+size > memSize {
		// Try to grow memory
		pages := ((offset + size - memSize) / uint32(65536)) + 1
		newSize, ok := mem.Grow(pages)
		if !ok {
			return 0, fmt.Errorf("failed to grow memory")
		}
		if newSize == 0 {
			return 0, fmt.Errorf("failed to grow memory: maximum memory size exceeded")
		}
	}

	m.current += size
	m.allocations[offset] = size
	return offset, nil
}

// Free releases the memory at the specified offset
func (m *MemoryAllocator) Free(offset uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	size, exists := m.allocations[offset]
	if !exists {
		return fmt.Errorf("attempt to free unallocated memory at offset %d", offset)
	}

	delete(m.allocations, offset)
	m.freeList = append(m.freeList, memoryRegion{offset: offset, size: size})
	return nil
}

// WriteMemory writes data to memory at the specified offset
func WriteMemory(mem api.Memory, offset uint32, data []byte) error {
	if !mem.Write(offset, data) {
		return fmt.Errorf("failed to write %d bytes at offset %d", len(data), offset)
	}
	return nil
}

// ReadMemory reads data from memory at the specified offset and length
func ReadMemory(mem api.Memory, offset, length uint32) ([]byte, error) {
	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("failed to read %d bytes at offset %d", length, offset)
	}
	return data, nil
}

// RegisterMemoryManagement sets up memory management for the module
func RegisterMemoryManagement(builder wazero.HostModuleBuilder, allocator *MemoryAllocator) {
	// Allocate memory
	builder.NewFunctionBuilder().
		WithFunc(func(_ context.Context, mod api.Module, size uint32) uint32 {
			offset, err := allocator.Allocate(mod.Memory(), size)
			if err != nil {
				panic(err) // In real code, handle this better
			}
			return offset
		}).
		Export("allocate")

	// Free memory
	builder.NewFunctionBuilder().
		WithFunc(func(_ context.Context, _ api.Module, offset uint32) {
			if err := allocator.Free(offset); err != nil {
				panic(err) // In real code, handle this better
			}
		}).
		Export("deallocate")
}
