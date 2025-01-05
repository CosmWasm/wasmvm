package memory

import (
	"encoding/binary"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

// Manager handles memory operations for Wasm modules
type Manager struct {
	mu     sync.RWMutex
	memory api.Memory
}

// New creates a new memory manager
func New(memory api.Memory) *Manager {
	return &Manager{
		memory: memory,
	}
}

// ReadBytes reads a byte slice from Wasm memory
func (m *Manager) ReadBytes(offset uint32, length uint32) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if the memory access is within bounds
	if uint64(offset)+uint64(length) > uint64(m.memory.Size()) {
		return nil, ErrInvalidMemoryAccess
	}

	data, ok := m.memory.Read(offset, length)
	if !ok {
		return nil, ErrMemoryReadFailed
	}

	return data, nil
}

// WriteBytes writes a byte slice to Wasm memory
func (m *Manager) WriteBytes(offset uint32, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if the memory access is within bounds
	if uint64(offset)+uint64(len(data)) > uint64(m.memory.Size()) {
		return ErrInvalidMemoryAccess
	}

	ok := m.memory.Write(offset, data)
	if !ok {
		return ErrMemoryWriteFailed
	}

	return nil
}

// ReadUint32 reads a uint32 from Wasm memory
func (m *Manager) ReadUint32(offset uint32) (uint32, error) {
	data, err := m.ReadBytes(offset, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

// WriteUint32 writes a uint32 to Wasm memory
func (m *Manager) WriteUint32(offset uint32, value uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, value)
	return m.WriteBytes(offset, buf)
}

// ReadString reads a string from Wasm memory
func (m *Manager) ReadString(offset uint32, length uint32) (string, error) {
	data, err := m.ReadBytes(offset, length)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteString writes a string to Wasm memory
func (m *Manager) WriteString(offset uint32, s string) error {
	return m.WriteBytes(offset, []byte(s))
}

// ReadUint32FromBytes reads a uint32 from a byte slice
func (m *Manager) ReadUint32FromBytes(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data)
}

// WriteUint32ToBytes writes a uint32 to a byte slice
func (m *Manager) WriteUint32ToBytes(data []byte, value uint32) {
	binary.LittleEndian.PutUint32(data, value)
}
