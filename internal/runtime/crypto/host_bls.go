package crypto

import (
	"encoding/binary"
	"fmt"
)

// BLS12381AggregateG1Host implements the BLS12-381 G1 aggregation host function
func (h *HostFunctions) BLS12381AggregateG1Host(elementsPtr uint32) (uint32, error) {
	// Read length prefix (4 bytes)
	lenBytes, err := h.mem.Manager().ReadBytes(elementsPtr, 4)
	if err != nil {
		return 0, fmt.Errorf("failed to read elements length: %w", err)
	}
	numElements := binary.LittleEndian.Uint32(lenBytes)

	// Read elements
	elements := make([][]byte, numElements)
	offset := elementsPtr + 4
	for i := uint32(0); i < numElements; i++ {
		// Read element length
		elemLenBytes, err := h.mem.Manager().ReadBytes(offset, 4)
		if err != nil {
			return 0, fmt.Errorf("failed to read element length: %w", err)
		}
		elemLen := binary.LittleEndian.Uint32(elemLenBytes)
		offset += 4

		// Read element data
		element, err := h.mem.Manager().ReadBytes(offset, elemLen)
		if err != nil {
			return 0, fmt.Errorf("failed to read element data: %w", err)
		}
		elements[i] = element
		offset += elemLen
	}

	// Perform aggregation
	result, err := h.verifier.BLS12381AggregateG1(elements)
	if err != nil {
		return 0, fmt.Errorf("failed to aggregate G1 points: %w", err)
	}

	// Write result to memory
	ptr, _, err := h.allocator.Allocate(result)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate memory for result: %w", err)
	}

	return ptr, nil
}

// BLS12381AggregateG2Host implements the BLS12-381 G2 aggregation host function
func (h *HostFunctions) BLS12381AggregateG2Host(elementsPtr uint32) (uint32, error) {
	// Read length prefix (4 bytes)
	lenBytes, err := h.mem.Manager().ReadBytes(elementsPtr, 4)
	if err != nil {
		return 0, fmt.Errorf("failed to read elements length: %w", err)
	}
	numElements := binary.LittleEndian.Uint32(lenBytes)

	// Read elements
	elements := make([][]byte, numElements)
	offset := elementsPtr + 4
	for i := uint32(0); i < numElements; i++ {
		// Read element length
		elemLenBytes, err := h.mem.Manager().ReadBytes(offset, 4)
		if err != nil {
			return 0, fmt.Errorf("failed to read element length: %w", err)
		}
		elemLen := binary.LittleEndian.Uint32(elemLenBytes)
		offset += 4

		// Read element data
		element, err := h.mem.Manager().ReadBytes(offset, elemLen)
		if err != nil {
			return 0, fmt.Errorf("failed to read element data: %w", err)
		}
		elements[i] = element
		offset += elemLen
	}

	// Perform aggregation
	result, err := h.verifier.BLS12381AggregateG2(elements)
	if err != nil {
		return 0, fmt.Errorf("failed to aggregate G2 points: %w", err)
	}

	// Write result to memory
	ptr, _, err := h.allocator.Allocate(result)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate memory for result: %w", err)
	}

	return ptr, nil
}

// BLS12381HashToG1Host implements the BLS12-381 hash-to-G1 host function
func (h *HostFunctions) BLS12381HashToG1Host(msgPtr uint32) (uint32, error) {
	// Read message region
	msgRegion, err := h.mem.ReadRegion(msgPtr)
	if err != nil {
		return 0, err
	}

	// Read message from memory
	msg, err := h.mem.ReadString(msgRegion)
	if err != nil {
		return 0, err
	}

	// Hash to G1
	result, err := h.verifier.BLS12381HashToG1([]byte(msg))
	if err != nil {
		return 0, fmt.Errorf("failed to hash to G1: %w", err)
	}

	// Write result to memory
	ptr, _, err := h.allocator.Allocate(result)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate memory for result: %w", err)
	}

	return ptr, nil
}

// BLS12381HashToG2Host implements the BLS12-381 hash-to-G2 host function
func (h *HostFunctions) BLS12381HashToG2Host(msgPtr uint32) (uint32, error) {
	// Read message region
	msgRegion, err := h.mem.ReadRegion(msgPtr)
	if err != nil {
		return 0, err
	}

	// Read message from memory
	msg, err := h.mem.ReadString(msgRegion)
	if err != nil {
		return 0, err
	}

	// Hash to G2
	result, err := h.verifier.BLS12381HashToG2([]byte(msg))
	if err != nil {
		return 0, fmt.Errorf("failed to hash to G2: %w", err)
	}

	// Write result to memory
	ptr, _, err := h.allocator.Allocate(result)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate memory for result: %w", err)
	}

	return ptr, nil
}

// BLS12381PairingEqualityHost implements the BLS12-381 pairing equality check host function
func (h *HostFunctions) BLS12381PairingEqualityHost(a1Ptr, a2Ptr, b1Ptr, b2Ptr uint32) (uint32, error) {
	// Read all regions
	a1Region, err := h.mem.ReadRegion(a1Ptr)
	if err != nil {
		return 0, err
	}

	a2Region, err := h.mem.ReadRegion(a2Ptr)
	if err != nil {
		return 0, err
	}

	b1Region, err := h.mem.ReadRegion(b1Ptr)
	if err != nil {
		return 0, err
	}

	b2Region, err := h.mem.ReadRegion(b2Ptr)
	if err != nil {
		return 0, err
	}

	// Read all points from memory
	a1, err := h.mem.ReadString(a1Region)
	if err != nil {
		return 0, err
	}

	a2, err := h.mem.ReadString(a2Region)
	if err != nil {
		return 0, err
	}

	b1, err := h.mem.ReadString(b1Region)
	if err != nil {
		return 0, err
	}

	b2, err := h.mem.ReadString(b2Region)
	if err != nil {
		return 0, err
	}

	// Check pairing equality
	result, err := h.verifier.BLS12381PairingEquality([]byte(a1), []byte(a2), []byte(b1), []byte(b2))
	if err != nil {
		return 0, fmt.Errorf("failed to check pairing equality: %w", err)
	}

	if result {
		return 1, nil
	}
	return 0, nil
}
