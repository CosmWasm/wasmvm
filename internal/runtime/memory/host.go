package memory

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
)

// Constants for memory management
const (
	// Memory page size in WebAssembly (64KB)
	wasmPageSize = 65536

	// Size of a Region struct in bytes (3x4 bytes)
	regionSize = 12
)

// Region describes data allocated in Wasm's linear memory
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// validateRegion performs plausibility checks on a Region
func validateRegion(region *Region) error {
	if region.Offset == 0 {
		return fmt.Errorf("region has zero offset")
	}
	if region.Length > region.Capacity {
		return fmt.Errorf("region length %d exceeds capacity %d", region.Length, region.Capacity)
	}
	if uint64(region.Offset)+uint64(region.Capacity) > math.MaxUint32 {
		return fmt.Errorf("region out of range: offset %d, capacity %d", region.Offset, region.Capacity)
	}
	return nil
}

// HostFunctions provides memory-related host functions
type HostFunctions struct {
	manager *Manager
}

// NewHostFunctions creates a new set of memory host functions
func NewHostFunctions(memory api.Memory) *HostFunctions {
	return &HostFunctions{
		manager: New(memory),
	}
}

// Manager returns the underlying memory manager
func (h *HostFunctions) Manager() *Manager {
	return h.manager
}

// ReadRegion reads a Region struct from Wasm memory
func (h *HostFunctions) ReadRegion(offset uint32) (*Region, error) {
	data, err := h.manager.ReadBytes(offset, regionSize)
	if err != nil {
		return nil, err
	}

	region := &Region{
		Offset:   h.manager.ReadUint32FromBytes(data[0:4]),
		Capacity: h.manager.ReadUint32FromBytes(data[4:8]),
		Length:   h.manager.ReadUint32FromBytes(data[8:12]),
	}

	if err := validateRegion(region); err != nil {
		return nil, err
	}

	return region, nil
}

// ReadString reads a string from a Region in Wasm memory
func (h *HostFunctions) ReadString(region *Region) (string, error) {
	data, err := h.manager.ReadBytes(region.Offset, region.Length)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteRegion writes a Region struct to Wasm memory
func (h *HostFunctions) WriteRegion(offset uint32, region *Region) error {
	data := make([]byte, regionSize)
	h.manager.WriteUint32ToBytes(data[0:4], region.Offset)
	h.manager.WriteUint32ToBytes(data[4:8], region.Capacity)
	h.manager.WriteUint32ToBytes(data[8:12], region.Length)
	return h.manager.WriteBytes(offset, data)
}
