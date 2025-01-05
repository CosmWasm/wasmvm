package memory

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// Allocator handles memory allocation in Wasm modules
type Allocator struct {
	memory  api.Memory
	module  api.Module
	manager *Manager
	host    *HostFunctions
}

// NewAllocator creates a new memory allocator
func NewAllocator(memory api.Memory, module api.Module) *Allocator {
	manager := New(memory)
	return &Allocator{
		memory:  memory,
		module:  module,
		manager: manager,
		host:    NewHostFunctions(memory),
	}
}

// Allocate allocates memory and writes data to it
func (a *Allocator) Allocate(data []byte) (uint32, uint32, error) {
	if data == nil {
		return 0, 0, nil
	}

	// Get the allocate function
	allocate := a.module.ExportedFunction("allocate")
	if allocate == nil {
		return 0, 0, fmt.Errorf("allocate function not found in WASM module")
	}

	// Allocate memory for the Region struct (12 bytes) and the data
	size := uint32(len(data))
	results, err := allocate.Call(context.Background(), uint64(size+regionSize))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate memory: %w", err)
	}
	ptr := uint32(results[0])

	// Create and write the Region struct
	region := &Region{
		Offset:   ptr + regionSize, // Data starts after the Region struct
		Capacity: size,
		Length:   size,
	}

	// Validate the region before writing
	if err := validateRegion(region); err != nil {
		if err := a.Deallocate(ptr); err != nil {
			// Log deallocation error but return the original error
			fmt.Printf("failed to deallocate memory after validation error: %v\n", err)
		}
		return 0, 0, fmt.Errorf("invalid region: %w", err)
	}

	// Write the Region struct
	if err := a.host.WriteRegion(ptr, region); err != nil {
		if err := a.Deallocate(ptr); err != nil {
			fmt.Printf("failed to deallocate memory after write error: %v\n", err)
		}
		return 0, 0, fmt.Errorf("failed to write region: %w", err)
	}

	// Write the actual data
	if err := a.manager.WriteBytes(region.Offset, data); err != nil {
		if err := a.Deallocate(ptr); err != nil {
			fmt.Printf("failed to deallocate memory after data write error: %v\n", err)
		}
		return 0, 0, fmt.Errorf("failed to write data to memory: %w", err)
	}

	return ptr, size, nil
}

// Deallocate frees allocated memory
func (a *Allocator) Deallocate(ptr uint32) error {
	deallocate := a.module.ExportedFunction("deallocate")
	if deallocate == nil {
		return fmt.Errorf("deallocate function not found in WASM module")
	}

	_, err := deallocate.Call(context.Background(), uint64(ptr))
	if err != nil {
		return fmt.Errorf("failed to deallocate memory: %w", err)
	}

	return nil
}

// Read reads data from allocated memory
func (a *Allocator) Read(ptr uint32) ([]byte, error) {
	if ptr == 0 {
		return nil, nil
	}

	// Read the Region struct
	region, err := a.host.ReadRegion(ptr)
	if err != nil {
		return nil, fmt.Errorf("failed to read region: %w", err)
	}

	// Read the actual data
	data, err := a.manager.ReadBytes(region.Offset, region.Length)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory: %w", err)
	}

	// Make a copy to ensure we own the data
	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}
