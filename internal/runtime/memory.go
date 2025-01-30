package runtime

import (
	"bytes"
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
	buf := make([]byte, 12) // No length prefix
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
	if r.Offset%8 != 0 {
		return fmt.Errorf("region offset %d not aligned to 8 bytes", r.Offset)
	}
	if r.Capacity%8 != 0 {
		return fmt.Errorf("region capacity %d not aligned to 8 bytes", r.Capacity)
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

	// Ensure the offset is properly aligned for the memory model
	if r.Offset%alignmentSize != 0 {
		return fmt.Errorf("region offset %d not aligned to required boundary %d", r.Offset, alignmentSize)
	}

	// Ensure capacity is aligned for the memory model
	if r.Capacity%alignmentSize != 0 {
		return fmt.Errorf("region capacity %d not aligned to required boundary %d", r.Capacity, alignmentSize)
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
	// Don't add padding for JSON data
	if len(data) > 0 && (data[0] == '{' || data[0] == '[') {
		if !memory.Write(offset, data) {
			return fmt.Errorf("failed to write %d bytes to memory at offset %d", len(data), offset)
		}
		return nil
	}

	// Add padding for non-JSON data
	alignedLen := align(uint32(len(data)), alignmentSize)
	alignedData := make([]byte, alignedLen)
	copy(alignedData, data)

	if !memory.Write(offset, alignedData) {
		return fmt.Errorf("failed to write %d bytes to memory at offset %d", alignedLen, offset)
	}
	return nil
}

// writeAlignedData writes data to memory with proper alignment and returns the write location and actual data length
func (mm *memoryManager) writeAlignedData(data []byte, printDebug bool) (uint32, uint32, error) {
	fmt.Printf("\n=== Memory Write Operation ===\n")
	fmt.Printf("Memory interface details:\n")
	fmt.Printf("- Current pages: %d\n", mm.memory.Size()/wasmPageSize)
	fmt.Printf("- Page size: %d\n", wasmPageSize)
	fmt.Printf("- Current memory size: %d bytes\n", mm.memory.Size())

	// Check for null data
	if data == nil {
		fmt.Printf("ERROR: Null data provided\n")
		return 0, 0, fmt.Errorf("null data")
	}

	// Calculate aligned length without null terminator
	dataLen := uint32(len(data))
	alignedLen := align(dataLen, 8)

	fmt.Printf("Write details:\n")
	fmt.Printf("- Original length: %d\n", dataLen)
	fmt.Printf("- Aligned length: %d\n", alignedLen)
	fmt.Printf("- Current offset: 0x%x\n", mm.nextOffset)
	fmt.Printf("- Memory size: %d\n", mm.size)

	// Check for overflow
	if mm.nextOffset > math.MaxUint32-alignedLen {
		fmt.Printf("ERROR: Memory allocation would overflow: offset=0x%x, length=%d\n",
			mm.nextOffset, alignedLen)
		return 0, 0, fmt.Errorf("memory allocation would overflow: offset=%d, length=%d",
			mm.nextOffset, alignedLen)
	}

	// Check if we need to grow memory
	requiredSize := mm.nextOffset + alignedLen
	if requiredSize > mm.size {
		oldSize := mm.size
		pagesToGrow := (requiredSize - mm.size + wasmPageSize - 1) / wasmPageSize

		fmt.Printf("Growing memory:\n")
		fmt.Printf("- Current size: %d bytes (%d pages)\n", oldSize, oldSize/wasmPageSize)
		fmt.Printf("- Growing by: %d pages\n", pagesToGrow)
		fmt.Printf("- New size will be: %d bytes (%d pages)\n",
			oldSize+pagesToGrow*wasmPageSize, (oldSize+pagesToGrow*wasmPageSize)/wasmPageSize)

		if _, ok := mm.memory.Grow(pagesToGrow); !ok {
			fmt.Printf("ERROR: Failed to grow memory\n")
			return 0, 0, fmt.Errorf("failed to grow memory by %d pages", pagesToGrow)
		}
		mm.size = mm.memory.Size()
	}

	// Create aligned buffer without null padding
	alignedData := make([]byte, alignedLen)
	copy(alignedData, data)

	// Fill remaining space with zeros but don't add null terminator
	for i := dataLen; i < alignedLen; i++ {
		alignedData[i] = 0
	}

	if printDebug {
		fmt.Printf("[DEBUG] Writing %d bytes to memory at offset 0x%x (aligned to %d)\n",
			dataLen, mm.nextOffset, alignedLen)
		if len(data) < 1024 {
			fmt.Printf("[DEBUG] Data: %s\n", string(data))
			fmt.Printf("[DEBUG] Hex: %x\n", data)
		}
	}

	// Write aligned data to memory
	if err := writeMemory(mm.memory, mm.nextOffset, alignedData, false); err != nil {
		fmt.Printf("ERROR: Failed to write data: %v\n", err)
		return 0, 0, fmt.Errorf("failed to write data: %w", err)
	}

	// Store current offset and update for next write
	writeOffset := mm.nextOffset
	mm.nextOffset += alignedLen

	fmt.Printf("Write successful:\n")
	fmt.Printf("- Write offset: 0x%x\n", writeOffset)
	fmt.Printf("- Next offset: 0x%x\n", mm.nextOffset)
	fmt.Printf("=== End Memory Write ===\n\n")

	// Verify the written data
	if verifyData, ok := mm.memory.Read(writeOffset, dataLen); ok {
		if !bytes.Equal(data, verifyData) {
			return 0, 0, fmt.Errorf("data verification failed after write")
		}
	}

	return writeOffset, dataLen, nil
}

// prepareRegions allocates and prepares memory regions for input data
// prepareRegions allocates and prepares memory regions for input data
func (mm *memoryManager) prepareRegions(env, info, msg []byte) (*Region, *Region, *Region, error) {
	fmt.Printf("\n=== PrepareRegions Start ===\n")
	fmt.Printf("Initial memory state:\n")
	fmt.Printf("- Total size: %d bytes (%d pages)\n", mm.size, mm.size/wasmPageSize)
	fmt.Printf("- Current offset: 0x%x\n", mm.nextOffset)

	// Log input data details
	fmt.Printf("\nInput Data Details:\n")
	fmt.Printf("env:  %d bytes - %s\n", len(env), previewData(env))
	fmt.Printf("info: %d bytes - %s\n", len(info), previewData(info))
	fmt.Printf("msg:  %d bytes - %s\n", len(msg), previewData(msg))

	// Validate JSON data before writing
	fmt.Printf("\nValidating JSON data...\n")
	for i, data := range []struct {
		name string
		data []byte
	}{
		{"env", env},
		{"info", info},
		{"msg", msg},
	} {
		if len(data.data) > 0 && data.data[0] == '{' {
			var js interface{}
			if err := json.Unmarshal(data.data, &js); err != nil {
				fmt.Printf("ERROR: Invalid JSON in %s data: %v\n", data.name, err)
				return nil, nil, nil, fmt.Errorf("invalid JSON data at index %d: %w", i, err)
			}
			fmt.Printf("%s: Valid JSON confirmed\n", data.name)
		}
	}

	// Calculate total required size with alignment
	totalSize := uint32(0)
	for _, data := range [][]byte{env, info, msg} {
		totalSize += align(uint32(len(data)), alignmentSize)
	}
	totalSize += align(3*regionStructSize, alignmentSize) // Space for region structs

	// Check if we need to grow memory
	if mm.nextOffset+totalSize > mm.size {
		requiredPages := (mm.nextOffset + totalSize - mm.size + wasmPageSize - 1) / wasmPageSize
		fmt.Printf("\nGrowing memory:\n")
		fmt.Printf("- Current size: %d bytes (%d pages)\n", mm.size, mm.size/wasmPageSize)
		fmt.Printf("- Growing by: %d pages\n", requiredPages)
		fmt.Printf("- Required size: %d bytes\n", mm.nextOffset+totalSize)

		if _, ok := mm.memory.Grow(requiredPages); !ok {
			return nil, nil, nil, fmt.Errorf("failed to grow memory by %d pages", requiredPages)
		}
		mm.size = mm.memory.Size()
		fmt.Printf("- New size: %d bytes (%d pages)\n", mm.size, mm.size/wasmPageSize)
	}

	// Ensure proper alignment for data section
	mm.nextOffset = align(firstPageOffset, alignmentSize)
	fmt.Printf("\nMemory Layout:\n")
	fmt.Printf("- Starting offset: 0x%x\n", mm.nextOffset)

	// Write env data
	fmt.Printf("\nWriting env data...\n")
	envOffset, envLen, err := mm.writeAlignedData(env, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write env data: %w", err)
	}
	fmt.Printf("- Written at: 0x%x, length: %d\n", envOffset, envLen)

	// Write info data
	fmt.Printf("\nWriting info data...\n")
	infoOffset, infoLen, err := mm.writeAlignedData(info, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write info data: %w", err)
	}
	fmt.Printf("- Written at: 0x%x, length: %d\n", infoOffset, infoLen)

	// Write msg data
	fmt.Printf("\nWriting msg data...\n")
	msgOffset, msgLen, err := mm.writeAlignedData(msg, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write msg data: %w", err)
	}
	fmt.Printf("- Written at: 0x%x, length: %d\n", msgOffset, msgLen)

	// Create regions with proper alignment
	envRegion := &Region{
		Offset:   envOffset,
		Capacity: align(uint32(envLen), alignmentSize),
		Length:   uint32(envLen),
	}

	infoRegion := &Region{
		Offset:   infoOffset,
		Capacity: align(uint32(infoLen), alignmentSize),
		Length:   uint32(infoLen),
	}

	msgRegion := &Region{
		Offset:   msgOffset,
		Capacity: align(uint32(msgLen), alignmentSize),
		Length:   uint32(msgLen),
	}

	// Validate regions
	fmt.Printf("\nValidating regions:\n")

	validateAndVerifyRegion := func(name string, region *Region) error {
		fmt.Printf("\n%s Region:\n", name)
		fmt.Printf("- Offset: 0x%x\n", region.Offset)
		fmt.Printf("- Capacity: %d (0x%x)\n", region.Capacity, region.Capacity)
		fmt.Printf("- Length: %d (0x%x)\n", region.Length, region.Length)

		// Validate region structure
		if err := region.Validate(mm.size); err != nil {
			fmt.Printf("ERROR: Region validation failed: %v\n", err)
			return fmt.Errorf("invalid %s region: %w", name, err)
		}

		// Verify data at region
		if data, ok := mm.memory.Read(region.Offset, region.Length); ok {
			fmt.Printf("- Data verification: %s\n", previewData(data))

			// For JSON data, verify it can be parsed
			if len(data) > 0 && data[0] == '{' {
				var js interface{}
				if err := json.Unmarshal(data, &js); err != nil {
					fmt.Printf("ERROR: Invalid JSON in region data: %v\n", err)
					return fmt.Errorf("invalid JSON in %s region: %w", name, err)
				}
			}
		} else {
			fmt.Printf("ERROR: Could not read data at region\n")
			return fmt.Errorf("failed to read %s region data", name)
		}

		return nil
	}

	// Validate all regions
	if err := validateAndVerifyRegion("Environment", envRegion); err != nil {
		return nil, nil, nil, err
	}
	if err := validateAndVerifyRegion("Info", infoRegion); err != nil {
		return nil, nil, nil, err
	}
	if err := validateAndVerifyRegion("Message", msgRegion); err != nil {
		return nil, nil, nil, err
	}

	// Verify no overlaps
	fmt.Printf("\nChecking for overlaps...\n")
	regions := [][2]uint32{
		{envRegion.Offset, envRegion.Offset + envRegion.Capacity},
		{infoRegion.Offset, infoRegion.Offset + infoRegion.Capacity},
		{msgRegion.Offset, msgRegion.Offset + msgRegion.Capacity},
	}

	for i := 0; i < len(regions); i++ {
		for j := i + 1; j < len(regions); j++ {
			if regions[i][1] > regions[j][0] && regions[j][1] > regions[i][0] {
				return nil, nil, nil, fmt.Errorf(
					"memory regions overlap: [0x%x-0x%x] and [0x%x-0x%x]",
					regions[i][0], regions[i][1], regions[j][0], regions[j][1],
				)
			}
		}
	}

	// Final memory state
	fmt.Printf("\nFinal Memory State:\n")
	fmt.Printf("- Total size: %d bytes (%d pages)\n", mm.size, mm.size/wasmPageSize)
	fmt.Printf("- Next offset: 0x%x\n", mm.nextOffset)
	fmt.Printf("- Regions prepared: env, info, msg\n")
	fmt.Printf("=== PrepareRegions Complete ===\n\n")

	return envRegion, infoRegion, msgRegion, nil
}

// Helper function to preview data content
func previewData(data []byte) string {
	if len(data) == 0 {
		return "<empty>"
	}

	preview := data
	if len(preview) > 32 {
		preview = preview[:32]
	}

	// For JSON data, try to pretty print
	if len(data) > 0 && data[0] == '{' {
		var js interface{}
		if err := json.Unmarshal(data, &js); err == nil {
			return fmt.Sprintf("JSON: %s... (%d bytes)", string(preview), len(data))
		}
	}

	// For non-JSON data, show hex
	return fmt.Sprintf("Hex: %x... (%d bytes)", preview, len(data))
}

// Helper function to align offset to boundary
func align(offset uint32, alignment uint32) uint32 {
	return (offset + alignment - 1) & ^(alignment - 1)
}

// writeRegions writes the regions to memory and returns their pointers
func (mm *memoryManager) writeRegions(env, info, msg *Region) (uint32, uint32, uint32, error) {
	fmt.Printf("\n=== Writing Region Structs to Memory ===\n")

	// Write env region
	envRegionBytes := env.ToBytes()
	fmt.Printf("Environment Region bytes: %x\n", envRegionBytes)
	envPtr, envLen, err := mm.writeAlignedData(envRegionBytes, true)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to write env region: %w", err)
	}
	if envLen != regionStructSize {
		return 0, 0, 0, fmt.Errorf("unexpected env region size: got %d, want %d", envLen, regionStructSize)
	}
	fmt.Printf("Wrote env region at ptr=0x%x\n", envPtr)

	// Verify region alignment
	if envPtr%8 != 0 {
		return 0, 0, 0, fmt.Errorf("env ptr 0x%x not 8-byte aligned", envPtr)
	}

	// Write info region if present
	var infoPtr uint32
	if info != nil {
		infoRegionBytes := info.ToBytes()
		fmt.Printf("Info Region bytes: %x\n", infoRegionBytes)
		infoPtr, envLen, err = mm.writeAlignedData(infoRegionBytes, true)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to write info region: %w", err)
		}
		if envLen != regionStructSize {
			return 0, 0, 0, fmt.Errorf("unexpected info region size: got %d, want %d", envLen, regionStructSize)
		}
		fmt.Printf("Wrote info region at ptr=0x%x\n", infoPtr)

		// Verify region alignment
		if infoPtr%8 != 0 {
			return 0, 0, 0, fmt.Errorf("info ptr 0x%x not 8-byte aligned", infoPtr)
		}
	}

	// Write msg region if present
	var msgPtr uint32
	if msg != nil {
		msgRegionBytes := msg.ToBytes()
		fmt.Printf("Message Region bytes: %x\n", msgRegionBytes)
		msgPtr, envLen, err = mm.writeAlignedData(msgRegionBytes, true)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to write msg region: %w", err)
		}
		if envLen != regionStructSize {
			return 0, 0, 0, fmt.Errorf("unexpected msg region size: got %d, want %d", envLen, regionStructSize)
		}
		fmt.Printf("Wrote msg region at ptr=0x%x\n", msgPtr)

		// Verify region alignment
		if msgPtr%8 != 0 {
			return 0, 0, 0, fmt.Errorf("msg ptr 0x%x not 8-byte aligned", msgPtr)
		}
	}

	fmt.Printf("=== Successfully Wrote Region Structs ===\n")
	return envPtr, infoPtr, msgPtr, nil
}

// readRegionData reads data from a Region with proper alignment and validation
func readRegionData(memory api.Memory, region *Region, printDebug bool) ([]byte, error) {
	data, err := readMemory(memory, region.Offset, region.Length)
	if err != nil {
		return nil, err
	}

	// Remove null terminators for JSON data
	if len(data) > 0 && data[0] == '{' {
		// Find first null terminator
		nullIndex := bytes.IndexByte(data, 0)
		if nullIndex != -1 {
			data = data[:nullIndex]
		}
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
