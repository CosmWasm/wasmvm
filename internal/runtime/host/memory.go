package host

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// readMemory reads length bytes from memory at the given offset
func readMemory(mem api.Memory, offset, length uint32) ([]byte, error) {
	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d, length %d", offset, length)
	}
	return data, nil
}

// writeMemory writes data to memory at the given offset
func writeMemory(mem api.Memory, offset uint32, data []byte, allowGrow bool) error {
	if !mem.Write(offset, data) {
		return fmt.Errorf("failed to write %d bytes to memory at offset %d", len(data), offset)
	}
	return nil
}
