package runtime

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/tetratelabs/wazero/api"
)

const (
	wasmPageSize     = uint32(65536) // 64KB page size
	firstPageOffset  = wasmPageSize  // Start at second page to avoid conflicts
	maxMemoryPages   = uint32(2048)  // Maximum memory pages (128MB)
	alignmentSize    = uint32(8)     // Memory alignment boundary
	regionStructSize = uint32(12)    // Size of a Region struct (3 uint32s)
)

// Region represents a contiguous section of memory that follows
// CosmWasm's memory passing convention
type Region struct {
	Offset   uint32
	Capacity uint32
	Length   uint32
}

// ToBytes serializes the Region according to CosmWasm's expected format
func (r *Region) ToBytes() []byte {
	buf := make([]byte, regionStructSize)
	binary.LittleEndian.PutUint32(buf[0:4], r.Offset)
	binary.LittleEndian.PutUint32(buf[4:8], r.Capacity)
	binary.LittleEndian.PutUint32(buf[8:12], r.Length)
	return buf
}

// RegionFromBytes deserializes a Region from bytes
func RegionFromBytes(data []byte, ok bool) (*Region, error) {
	if !ok || uint32(len(data)) < regionStructSize {
		return nil, fmt.Errorf("invalid region data: length=%d, expected=%d", len(data), regionStructSize)
	}

	region := &Region{
		Offset:   binary.LittleEndian.Uint32(data[0:4]),
		Capacity: binary.LittleEndian.Uint32(data[4:8]),
		Length:   binary.LittleEndian.Uint32(data[8:12]),
	}

	// Validate region fields
	if region.Length > region.Capacity {
		return nil, fmt.Errorf("invalid region: length %d exceeds capacity %d", region.Length, region.Capacity)
	}

	if region.Offset%alignmentSize != 0 {
		return nil, fmt.Errorf("invalid region: offset %d not aligned to %d", region.Offset, alignmentSize)
	}

	// Ensure capacity is aligned
	if region.Capacity%alignmentSize != 0 {
		return nil, fmt.Errorf("invalid region: capacity %d not aligned to %d", region.Capacity, alignmentSize)
	}

	// Check for potential overflow in offset + capacity
	if region.Offset > math.MaxUint32-region.Capacity {
		return nil, fmt.Errorf("invalid region: offset %d + capacity %d would overflow", region.Offset, region.Capacity)
	}

	return region, nil
}

// Validate checks if a Region is valid for the given memory size
func (r *Region) Validate(memorySize uint32) error {
	// Check for null region
	if r == nil {
		return fmt.Errorf("null region")
	}

	// Check alignment
	if r.Offset%alignmentSize != 0 {
		return fmt.Errorf("region offset %d not aligned to %d", r.Offset, alignmentSize)
	}
	if r.Capacity%alignmentSize != 0 {
		return fmt.Errorf("region capacity %d not aligned to %d", r.Capacity, alignmentSize)
	}

	// Check bounds
	if r.Length > r.Capacity {
		return fmt.Errorf("region length %d exceeds capacity %d", r.Length, r.Capacity)
	}

	// Check for overflow in offset + capacity calculation
	if r.Offset > math.MaxUint32-r.Capacity {
		return fmt.Errorf("region offset %d + capacity %d would overflow", r.Offset, r.Capacity)
	}

	// Check memory bounds
	if uint64(r.Offset)+uint64(r.Capacity) > uint64(memorySize) {
		return fmt.Errorf("region end %d exceeds memory size %d", r.Offset+r.Capacity, memorySize)
	}

	// Check minimum capacity
	if r.Capacity == 0 {
		return fmt.Errorf("region capacity cannot be zero")
	}

	// Check first page boundary
	if r.Offset < firstPageOffset {
		return fmt.Errorf("region offset %d is below first page boundary %d", r.Offset, firstPageOffset)
	}

	return nil
}

type memoryManager struct {
	memory     api.Memory
	gasState   *GasState
	nextOffset uint32
	size       uint32
}

func newMemoryManager(memory api.Memory, gasState *GasState) *memoryManager {
	// Initialize memory with one page if empty
	if memory.Size() == 0 {
		if _, ok := memory.Grow(1); !ok {
			panic("failed to initialize memory with one page")
		}
	}

	// Ensure memory size is valid
	size := memory.Size()
	if size > maxMemoryPages*wasmPageSize {
		panic(fmt.Sprintf("memory size %d exceeds maximum allowed (%d pages)", size, maxMemoryPages))
	}

	// Initialize memory manager with proper alignment
	mm := &memoryManager{
		memory:     memory,
		gasState:   gasState,
		nextOffset: align(firstPageOffset, alignmentSize),
		size:       size,
	}

	return mm
}

// align ensures the offset meets CosmWasm alignment requirements
func align(offset uint32, alignment uint32) uint32 {
	return (offset + alignment - 1) & ^(alignment - 1)
}

// ensureAlignment enforces memory alignment requirements
func ensureAlignment(offset uint32, printDebug bool) uint32 {
	aligned := align(offset, alignmentSize)
	if aligned != offset && printDebug {
		fmt.Printf("[DEBUG] Aligning offset from 0x%x to 0x%x\n", offset, aligned)
	}
	return aligned
}

// readMemory is a helper to read bytes from memory with bounds checking
func readMemory(memory api.Memory, offset uint32, length uint32) ([]byte, error) {
	// Check for zero length
	if length == 0 {
		return nil, fmt.Errorf("zero length read")
	}

	// Calculate aligned length
	alignedLen := align(length, alignmentSize)

	// Check for potential overflow in offset + length calculation
	if offset > math.MaxUint32-alignedLen {
		return nil, fmt.Errorf("memory access would overflow: offset=%d, length=%d", offset, alignedLen)
	}

	// Ensure we're not reading past memory bounds
	if uint64(offset)+uint64(alignedLen) > uint64(memory.Size()) {
		return nil, fmt.Errorf("read would exceed memory bounds: offset=%d, length=%d, memory_size=%d",
			offset, alignedLen, memory.Size())
	}

	// Ensure offset is properly aligned
	if offset%alignmentSize != 0 {
		return nil, fmt.Errorf("unaligned memory read: offset=%d must be aligned to %d", offset, alignmentSize)
	}

	// Ensure offset is after first page
	if offset < firstPageOffset {
		return nil, fmt.Errorf("read offset %d is below first page boundary %d", offset, firstPageOffset)
	}

	// Read aligned data
	alignedData, ok := memory.Read(offset, alignedLen)
	if !ok {
		return nil, fmt.Errorf("failed to read %d bytes from memory at offset %d", alignedLen, offset)
	}

	// Validate data is not null
	if alignedData == nil {
		return nil, fmt.Errorf("null data read from memory")
	}

	// Return only the requested length
	data := alignedData[:length]

	return data, nil
}

// writeMemory is a helper to write bytes to memory with bounds checking
func writeMemory(memory api.Memory, offset uint32, data []byte, printDebug bool) error {
	// Check for null data
	if data == nil {
		return fmt.Errorf("null data")
	}

	// Check for potential overflow in offset + length calculation
	dataLen := uint32(len(data))
	alignedLen := align(dataLen, alignmentSize)

	if offset > math.MaxUint32-alignedLen {
		return fmt.Errorf("memory access would overflow: offset=%d, length=%d", offset, alignedLen)
	}

	// Ensure we're not writing past memory bounds
	if uint64(offset)+uint64(alignedLen) > uint64(memory.Size()) {
		return fmt.Errorf("write would exceed memory bounds: offset=%d, length=%d, memory_size=%d",
			offset, alignedLen, memory.Size())
	}

	// Ensure the write is aligned
	if offset%alignmentSize != 0 {
		return fmt.Errorf("unaligned memory write: offset=%d must be aligned to %d", offset, alignmentSize)
	}

	// Ensure offset is after first page
	if offset < firstPageOffset {
		return fmt.Errorf("write offset %d is below first page boundary %d", offset, firstPageOffset)
	}

	if printDebug {
		fmt.Printf("[DEBUG] Writing %d bytes to memory at offset 0x%x (aligned to %d)\n", dataLen, offset, alignedLen)
		if len(data) < 1024 {
			fmt.Printf("[DEBUG] Data: %s\n", string(data))
		}
	}

	// Create aligned buffer and copy data
	alignedData := make([]byte, alignedLen)
	copy(alignedData, data)

	// Write aligned data to memory
	if !memory.Write(offset, alignedData) {
		return fmt.Errorf("failed to write %d bytes to memory at offset %d", alignedLen, offset)
	}

	return nil
}

// ensureMemory grows memory if needed to accommodate required size with overflow protection
func (mm *memoryManager) ensureMemory(required uint32) error {
	currentSize := mm.memory.Size()

	// Check for potential overflow in offset + required calculation
	if mm.nextOffset > math.MaxUint32-required {
		return fmt.Errorf("memory size calculation would overflow: offset=%d, required=%d",
			mm.nextOffset, required)
	}

	// Calculate total required size including alignment padding
	requiredSize := mm.nextOffset + required
	alignedSize := align(requiredSize, wasmPageSize)

	// Verify aligned size didn't overflow
	if alignedSize < requiredSize {
		return fmt.Errorf("aligned size calculation overflow: required=%d, aligned=%d",
			requiredSize, alignedSize)
	}

	if alignedSize > currentSize {
		// Calculate pages needed with overflow protection
		if alignedSize > maxMemoryPages*wasmPageSize {
			return fmt.Errorf("required memory exceeds maximum allowed (%d pages)", maxMemoryPages)
		}

		pagesToGrow := (alignedSize - currentSize + wasmPageSize - 1) / wasmPageSize

		// Charge gas for memory growth
		growthSize := pagesToGrow * wasmPageSize
		if err := mm.gasState.ConsumeMemory(growthSize); err != nil {
			return fmt.Errorf("insufficient gas for memory growth: %w", err)
		}

		// Grow memory
		if _, ok := mm.memory.Grow(pagesToGrow); !ok {
			return fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}

		// Update size after growth
		mm.size = mm.memory.Size()
	}
	return nil
}

// writeAlignedData writes data to memory with proper alignment and returns the write location and actual data length
func (mm *memoryManager) writeAlignedData(data []byte, printDebug bool) (uint32, uint32, error) {
	// Check for null data
	if data == nil {
		return 0, 0, fmt.Errorf("null data")
	}

	// Calculate aligned length
	dataLen := uint32(len(data))
	alignedLen := align(dataLen, alignmentSize)

	// Ensure we have enough memory
	if mm.nextOffset > math.MaxUint32-alignedLen {
		return 0, 0, fmt.Errorf("memory allocation would overflow: offset=%d, length=%d", mm.nextOffset, alignedLen)
	}

	// Write data to memory
	if err := writeMemory(mm.memory, mm.nextOffset, data, printDebug); err != nil {
		return 0, 0, fmt.Errorf("failed to write data: %w", err)
	}

	// Store current offset and update for next write
	writeOffset := mm.nextOffset
	mm.nextOffset += alignedLen

	// Return write location and actual data length
	return writeOffset, dataLen, nil
}

// prepareRegions allocates and prepares memory regions for input data
func (mm *memoryManager) prepareRegions(env, info, msg []byte) (*Region, *Region, *Region, error) {
	// Prepare env region
	envOffset, envLen, err := mm.writeAlignedData(env, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write env data: %w", err)
	}
	envRegion := &Region{
		Offset:   envOffset,
		Capacity: align(uint32(envLen), alignmentSize),
		Length:   uint32(envLen),
	}
	if err := envRegion.Validate(mm.size); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid env region: %w", err)
	}

	// Prepare info region
	infoOffset, infoLen, err := mm.writeAlignedData(info, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write info data: %w", err)
	}
	infoRegion := &Region{
		Offset:   infoOffset,
		Capacity: align(uint32(infoLen), alignmentSize),
		Length:   uint32(infoLen),
	}
	if err := infoRegion.Validate(mm.size); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid info region: %w", err)
	}

	// Prepare msg region
	msgOffset, msgLen, err := mm.writeAlignedData(msg, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write msg data: %w", err)
	}
	msgRegion := &Region{
		Offset:   msgOffset,
		Capacity: align(uint32(msgLen), alignmentSize),
		Length:   uint32(msgLen),
	}
	if err := msgRegion.Validate(mm.size); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid msg region: %w", err)
	}

	return envRegion, infoRegion, msgRegion, nil
}

// writeRegions writes the regions to memory and returns their pointers
func (mm *memoryManager) writeRegions(env, info, msg *Region) (uint32, uint32, uint32, error) {
	// Write env region
	envPtr, envLen, err := mm.writeAlignedData(env.ToBytes(), false)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write env region: %w", err)
	}
	if envLen != regionStructSize {
		return 0, 0, 0, fmt.Errorf("unexpected env region size: got %d, want %d", envLen, regionStructSize)
	}

	// Write info region
	infoPtr, infoLen, err := mm.writeAlignedData(info.ToBytes(), false)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write info region: %w", err)
	}
	if infoLen != regionStructSize {
		return 0, 0, 0, fmt.Errorf("unexpected info region size: got %d, want %d", infoLen, regionStructSize)
	}

	// Write msg region
	msgPtr, msgLen, err := mm.writeAlignedData(msg.ToBytes(), false)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write msg region: %w", err)
	}
	if msgLen != regionStructSize {
		return 0, 0, 0, fmt.Errorf("unexpected msg region size: got %d, want %d", msgLen, regionStructSize)
	}

	return envPtr, infoPtr, msgPtr, nil
}

// readRegionData reads data from a Region with proper alignment and validation
func readRegionData(memory api.Memory, region *Region, printDebug bool) ([]byte, error) {
	// Validate region
	if region == nil {
		return nil, fmt.Errorf("null region")
	}

	// Validate region fields
	if region.Length > region.Capacity {
		return nil, fmt.Errorf("region length %d exceeds capacity %d", region.Length, region.Capacity)
	}

	// Read data from memory
	data, err := readMemory(memory, region.Offset, region.Length)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory: %w", err)
	}

	// If this looks like JSON data, validate it
	if len(data) > 0 && data[0] == '{' {
		var js interface{}
		if err := json.Unmarshal(data, &js); err != nil {
			if printDebug {
				fmt.Printf("[DEBUG] JSON validation failed: %v\n", err)
				// Print the problematic section
				errPos := 0
				if serr, ok := err.(*json.SyntaxError); ok {
					errPos = int(serr.Offset)
				}
				start := errPos - 20
				if start < 0 {
					start = 0
				}
				end := errPos + 20
				if end > len(data) {
					end = len(data)
				}
				fmt.Printf("[DEBUG] JSON error context: %q\n", string(data[start:end]))
				fmt.Printf("[DEBUG] Full data: %s\n", string(data))
			}
			return nil, fmt.Errorf("invalid JSON data: %w", err)
		}

		// Re-marshal to ensure consistent formatting
		cleanData, err := json.Marshal(js)
		if err != nil {
			return nil, fmt.Errorf("failed to re-marshal JSON: %w", err)
		}
		data = cleanData
	}

	return data, nil
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
	if mm.nextOffset+allocSize > mm.size {
		pagesToGrow := (mm.nextOffset + allocSize - mm.size + wasmPageSize - 1) / wasmPageSize
		if printDebug {
			fmt.Printf("[DEBUG] Growing memory by %d pages (current size: %d, needed: %d)\n",
				pagesToGrow, mm.size/wasmPageSize, (mm.nextOffset+allocSize)/wasmPageSize)
		}
		grown, ok := mm.memory.Grow(pagesToGrow)
		if !ok || grown == 0 {
			return 0, 0, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Write data to memory
	if !mm.memory.Write(mm.nextOffset, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory")
	}

	// Store current offset and update for next write
	offset := mm.nextOffset
	mm.nextOffset += allocSize

	if printDebug {
		fmt.Printf("[DEBUG] Wrote %d bytes at offset 0x%x (page-aligned size: %d)\n",
			len(data), offset, allocSize)
	}

	return offset, allocSize, nil
}

// readMemoryAndDeserialize reads memory at the given offset and size, and attempts to deserialize it
func readMemoryAndDeserialize(memory api.Memory, offset, size uint32) (string, error) {
	data, ok := memory.Read(offset, size)
	if !ok {
		return "", fmt.Errorf("failed to read memory at offset=%d size=%d", offset, size)
	}

	if readable := tryDeserializeMemory(data); readable != "" {
		return readable, nil
	}

	// If no readable text found, return the traditional hex dump
	return fmt.Sprintf("As hex: %s", hex.EncodeToString(data)), nil
}

// tryDeserializeMemory attempts to extract readable text from a memory dump
func tryDeserializeMemory(data []byte) string {
	var results []string

	// First try to interpret as UTF-8 text
	if str := tryUTF8(data); str != "" {
		results = append(results, "As text: "+str)
	}

	// Try to find null-terminated C strings
	if strs := tryCStrings(data); len(strs) > 0 {
		results = append(results, "As C strings: "+strings.Join(strs, ", "))
	}

	// Try to find JSON fragments
	if json := tryJSON(data); json != "" {
		results = append(results, "As JSON: "+json)
	}

	// Always include hex representation in a readable format
	hexStr := formatHexDump(data)
	results = append(results, "As hex dump:\n"+hexStr)

	return strings.Join(results, "\n")
}

// tryUTF8 attempts to interpret the given data as UTF-8 text.
// If it's valid UTF-8, we return the string; otherwise, we return an empty string.
func tryUTF8(data []byte) string {
	// A simple check to see if the bytes are valid UTF-8:
	if isValidUTF8(data) {
		return string(data)
	}
	return ""
}

// isValidUTF8 checks if the entire byte slice is valid UTF-8
func isValidUTF8(data []byte) bool {
	// This is a simple approach that relies on Go's built-in UTF-8 checking.
	// It returns true if the byte slice is entirely valid UTF-8.
	for len(data) > 0 {
		r, size := rune(data[0]), 1
		if r >= 0x80 {
			// If it's not ASCII, decode the multi-byte sequence:
			r, size = decodeRune(data)
			// If invalid, return false
			if r == 0xFFFD && size == 1 {
				return false
			}
		}
		data = data[size:]
	}
	return true
}

// decodeRune decodes the first UTF-8 encoded rune in the provided byte slice.
// If invalid, it returns the replacement rune 0xFFFD.
func decodeRune(b []byte) (rune, int) {
	r, size := utf8DecodeRune(b)
	return r, size
}

// utf8DecodeRune is a minimal re-implementation of Go's utf8.DecodeRune.
// For actual usage, consider using the standard library: utf8.DecodeRune()
func utf8DecodeRune(b []byte) (r rune, size int) {
	if len(b) == 0 {
		return utf8.RuneError, 0
	}
	return utf8.DecodeRune(b)
}

// tryCStrings scans for null-terminated strings within the data slice.
// For each 0x00 encountered, it treats that as the end of a string.
func tryCStrings(data []byte) []string {
	var result []string
	start := 0
	for i, b := range data {
		if b == 0 {
			if i > start {
				result = append(result, string(data[start:i]))
			}
			start = i + 1
		}
	}
	// If there's non-null data after the last null, treat that as a string too:
	if start < len(data) {
		result = append(result, string(data[start:]))
	}
	return result
}

// tryJSON attempts to interpret the given data as JSON.
// If it's valid, we'll reformat it (indent for clarity).
// Otherwise, returns an empty string.
func tryJSON(data []byte) string {
	var js interface{}
	if err := json.Unmarshal(data, &js); err == nil {
		// If valid, pretty-print the JSON
		out, err := json.MarshalIndent(js, "", "  ")
		if err == nil {
			return string(out)
		}
	}
	return ""
}

// formatHexDump returns a hex dump of the data, typically 16 bytes per line.
func formatHexDump(data []byte) string {
	// hex.Dump is a convenient helper from the standard library that
	// formats the given bytes in a block of hex plus ASCII.
	return hex.Dump(data)
}
