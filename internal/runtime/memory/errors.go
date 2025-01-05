package memory

import "errors"

var (
	// ErrInvalidMemoryAccess is returned when trying to access invalid memory regions
	ErrInvalidMemoryAccess = errors.New("invalid memory access")
	// ErrMemoryReadFailed is returned when memory read operation fails
	ErrMemoryReadFailed = errors.New("memory read failed")
	// ErrMemoryWriteFailed is returned when memory write operation fails
	ErrMemoryWriteFailed = errors.New("memory write failed")
)
