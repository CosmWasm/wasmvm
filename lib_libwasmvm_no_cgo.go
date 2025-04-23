//go:build !cgo

package wasmvm

import (
	"errors"
)

// This file provides stub implementations for types and functions that depend
// on CGo details provided by libwasmvm, allowing the package to compile
// even when CGo is disabled or the system library is not linked.

// VM is a stub implementation for non-CGo builds.
type VM struct {
	// Add fields here if needed for non-CGo logic, otherwise empty.
}

var errNoCgo = errors.New("wasmvm library compiled without CGo support or libwasmvm linking")

// NewVM is a stub implementation for non-CGo builds.
func NewVM(dataDir string, supportedCapabilities []string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error) {
	return nil, errNoCgo
}

// Cleanup is a stub implementation for non-CGo builds.
func (v *VM) Cleanup() {
	// No-op
}

// StoreCode is a stub implementation for non-CGo builds.
func (v *VM) StoreCode(code WasmCode, gasLimit uint64) (Checksum, uint64, error) {
	return Checksum{}, 0, errNoCgo
}
