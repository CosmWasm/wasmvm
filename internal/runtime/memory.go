package runtime

import (
	"encoding/binary"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

func (mm *memoryManager) ensureMemory(required uint32) error {
	currentSize := mm.memory.Size()
	requiredSize := mm.nextPtr + required

	if requiredSize > currentSize {
		pagesToGrow := (requiredSize - currentSize + wasmPageSize - 1) / wasmPageSize
		if pagesToGrow > maxMemoryPages {
			return fmt.Errorf("required memory exceeds maximum allowed")
		}

		if _, ok := mm.memory.Grow(pagesToGrow); !ok {
			return fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
	}

	return nil
}

func (mm *memoryManager) writeData(data []byte) (uint32, error) {
	if len(data) == 0 {
		return 0, nil
	}

	// Charge gas for memory usage
	mm.gasUsed += uint64(len(data)) * gasPerByte
	if mm.gasUsed > mm.gasLimit {
		return 0, fmt.Errorf("out of gas")
	}

	// Write data
	ptr := mm.nextPtr
	if err := writeMemory(mm.memory, ptr, data); err != nil {
		return 0, err
	}

	// Update next pointer with alignment
	mm.nextPtr = align(ptr+uint32(len(data)), alignmentSize)
	return ptr, nil
}

func (mm *memoryManager) writeRegion(region *Region) (uint32, error) {
	data := make([]byte, regionStructSize)
	binary.LittleEndian.PutUint32(data[0:4], region.Offset)
	binary.LittleEndian.PutUint32(data[4:8], region.Capacity)
	binary.LittleEndian.PutUint32(data[8:12], region.Length)

	ptr := mm.nextPtr
	if err := writeMemory(mm.memory, ptr, data); err != nil {
		return 0, err
	}

	mm.nextPtr = align(ptr+regionStructSize, alignmentSize)
	return ptr, nil
}

type memoryManager struct {
	memory   api.Memory
	gasLimit uint64
	gasUsed  uint64
	nextPtr  uint32
}

type dataLocations struct {
	envRegionPtr  uint32
	infoRegionPtr uint32
	msgRegionPtr  uint32
}

func newMemoryManager(memory api.Memory, gasLimit uint64) *memoryManager {
	return &memoryManager{
		memory:   memory,
		gasLimit: gasLimit,
		gasUsed:  0,
		nextPtr:  firstPageOffset,
	}
}

func (mm *memoryManager) writeContractData(env, info, msg []byte) (*dataLocations, error) {
	// Calculate sizes with alignment
	envSize := align(uint32(len(env)), alignmentSize)
	infoSize := align(uint32(len(info)), alignmentSize)
	msgSize := align(uint32(len(msg)), alignmentSize)

	// Total required memory including Region structs
	totalSize := envSize + infoSize + msgSize + (3 * regionStructSize)

	// Ensure enough memory
	if err := mm.ensureMemory(totalSize); err != nil {
		return nil, err
	}

	// Write data
	envPtr, err := mm.writeData(env)
	if err != nil {
		return nil, err
	}

	infoPtr, err := mm.writeData(info)
	if err != nil {
		return nil, err
	}

	msgPtr, err := mm.writeData(msg)
	if err != nil {
		return nil, err
	}

	// Create and write regions
	regions := &dataLocations{}
	regions.envRegionPtr, err = mm.writeRegion(&Region{
		Offset:   envPtr,
		Capacity: envSize,
		Length:   uint32(len(env)),
	})
	if err != nil {
		return nil, err
	}

	regions.infoRegionPtr, err = mm.writeRegion(&Region{
		Offset:   infoPtr,
		Capacity: infoSize,
		Length:   uint32(len(info)),
	})
	if err != nil {
		return nil, err
	}

	regions.msgRegionPtr, err = mm.writeRegion(&Region{
		Offset:   msgPtr,
		Capacity: msgSize,
		Length:   uint32(len(msg)),
	})
	if err != nil {
		return nil, err
	}

	return regions, nil
}

// writeToMemory writes data to memory and returns the offset where it was written
func (mm *memoryManager) writeToMemory(data []byte, printDebug bool) (uint32, uint32, error) {
	dataSize := uint32(len(data))
	if dataSize == 0 {
		return 0, 0, nil
	}

	// Calculate pages needed for data
	pagesNeeded := (dataSize + wasmPageSize - 1) / wasmPageSize
	allocSize := pagesNeeded * wasmPageSize

	// Check if we need to grow memory
	if mm.nextPtr+allocSize > mm.memory.Size() {
		pagesToGrow := (mm.nextPtr + allocSize - mm.memory.Size() + wasmPageSize - 1) / wasmPageSize
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages (current size: %d, needed: %d)\n",
				pagesToGrow, mm.memory.Size()/wasmPageSize, (mm.nextPtr+allocSize)/wasmPageSize)
		}
		grown, ok := mm.memory.Grow(pagesToGrow)
		if !ok || grown == 0 {
			return 0, 0, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.nextPtr = mm.memory.Size()
	}

	// Write data to memory
	if !mm.memory.Write(mm.nextPtr, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory")
	}

	// Store current offset and update for next write
	offset := mm.nextPtr
	mm.nextPtr += allocSize

	if printDebug {
		fmt.Printf("[DEBUG] Wrote %d bytes at offset 0x%x (page-aligned size: %d)\n",
			len(data), offset, allocSize)
	}

	return offset, allocSize, nil
}
