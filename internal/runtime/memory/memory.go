package memory

import (
	"context"
	"errors"

	"github.com/tetratelabs/wazero/api"
)

// WasmMemory is an alias for the wazero Memory interface.
type WasmMemory = api.Memory

// Region in Go for clarity (optional; we can also handle without this struct)
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// MemoryManager manages a Wasm instance's memory and allocation.
type MemoryManager struct {
	Memory       WasmMemory                   // interface to Wasm memory (e.g., provides Read, Write)
	WasmAllocate func(uint32) (uint32, error) // function to call Wasm allocate
	Deallocate   func(uint32) error           // function to call Wasm deallocate
	MemorySize   uint32                       // size of the memory (for bounds checking, if available)
}

// NewMemoryManager creates and initializes a MemoryManager from the given module.
// It retrieves the exported "allocate" and "deallocate" functions and the Wasm memory,
// and sets the memorySize field.
func NewMemoryManager(module api.Module) (*MemoryManager, error) {
	allocFn := module.ExportedFunction("allocate")
	deallocFn := module.ExportedFunction("deallocate")
	mem := module.Memory()
	if allocFn == nil || deallocFn == nil || mem == nil {
		return nil, errors.New("missing required exports: allocate, deallocate, or memory")
	}

	// Get the current memory size.
	size := mem.Size()

	// Create wrapper functions that call the exported functions.
	allocateWrapper := func(requestSize uint32) (uint32, error) {
		results, err := allocFn.Call(context.Background(), uint64(requestSize))
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 0, errors.New("allocate returned no results")
		}
		return uint32(results[0]), nil
	}

	deallocateWrapper := func(ptr uint32) error {
		_, err := deallocFn.Call(context.Background(), uint64(ptr))
		return err
	}

	return &MemoryManager{
		Memory:       mem,
		WasmAllocate: allocateWrapper,
		Deallocate:   deallocateWrapper,
		MemorySize:   size,
	}, nil
}

// Read copies `length` bytes from Wasm memory at the given offset into a new byte slice.
func (m *MemoryManager) Read(offset uint32, length uint32) ([]byte, error) {
	if offset+length > m.MemorySize {
		return nil, errors.New("memory read out of bounds")
	}
	data, ok := m.Memory.Read(offset, uint32(length))
	if !ok {
		return nil, errors.New("failed to read memory")
	}
	return data, nil
}

// Write copies the given data into Wasm memory starting at the given offset.
func (m *MemoryManager) Write(offset uint32, data []byte) error {
	length := uint32(len(data))
	if offset+length > m.MemorySize {
		return errors.New("memory write out of bounds")
	}
	if !m.Memory.Write(offset, data) {
		return errors.New("failed to write memory")
	}
	return nil
}

// ReadRegion reads a Region (offset, capacity, length) from Wasm memory and returns the pointed bytes.
func (m *MemoryManager) ReadRegion(regionPtr uint32) ([]byte, error) {
	// Read 12 bytes for Region struct
	const regionSize = 12
	raw, err := m.Read(regionPtr, regionSize)
	if err != nil {
		return nil, err
	}
	// Parse Region struct (little-endian u32s)
	if len(raw) != regionSize {
		return nil, errors.New("invalid region struct size")
	}
	region := Region{
		Offset:   littleEndianToUint32(raw[0:4]),
		Capacity: littleEndianToUint32(raw[4:8]),
		Length:   littleEndianToUint32(raw[8:12]),
	}
	// Basic sanity checks
	if region.Offset+region.Length > m.MemorySize {
		return nil, errors.New("region out of bounds")
	}
	if region.Length > region.Capacity {
		return nil, errors.New("region length exceeds capacity")
	}
	// Read the actual data
	return m.Read(region.Offset, region.Length)
}

// Allocate requests a new memory region of given size from the Wasm instance.
func (m *MemoryManager) Allocate(size uint32) (uint32, error) {
	// Call the contract's allocate function via the provided callback
	offset, err := m.WasmAllocate(size)
	if err != nil {
		return 0, err
	}
	if offset == 0 {
		// A zero offset might indicate allocation failure (if contract uses 0 as null)
		return 0, errors.New("allocation failed")
	}
	// Optionally, ensure offset is within memory bounds (if allocate doesn't already guarantee it)
	if offset >= m.MemorySize {
		return 0, errors.New("allocation returned out-of-bounds pointer")
	}
	return offset, nil
}

// Free releases previously allocated memory back to the contract.
func (m *MemoryManager) Free(offset uint32) error {
	return m.Deallocate(offset)
}

// CreateRegion allocates a Region struct in Wasm memory for a given data buffer.
func (m *MemoryManager) CreateRegion(dataOffset, dataLength uint32) (uint32, error) {
	const regionSize = 12
	regionPtr, err := m.Allocate(regionSize)
	if err != nil {
		return 0, err
	}
	// Build the region struct in little-endian bytes
	reg := make([]byte, regionSize)
	putUint32LE(reg[0:4], dataOffset)
	putUint32LE(reg[4:8], dataLength)  // capacity = length (we allocate exactly length)
	putUint32LE(reg[8:12], dataLength) // length = actual data length
	// Write the struct into memory
	if err := m.Write(regionPtr, reg); err != nil {
		m.Free(regionPtr) // free the region struct allocation if writing fails
		return 0, err
	}
	return regionPtr, nil
}

// Utility: convert 4 bytes little-endian to uint32
func littleEndianToUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// Utility: write uint32 as 4 little-endian bytes
func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v & 0xFF)
	b[1] = byte((v >> 8) & 0xFF)
	b[2] = byte((v >> 16) & 0xFF)
	b[3] = byte((v >> 24) & 0xFF)
}
