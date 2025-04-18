//go:build !cgo

package api

import (
	"errors"
	"os"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// This file provides stub implementations for types and functions that depend
// on CGo details, allowing the package to compile even when CGo is disabled.

// Cache is a stub implementation for non-CGo builds.
type Cache struct {
	// Add fields here if needed for non-CGo logic, otherwise empty.
	// We need the lockfile field to satisfy the struct definition used elsewhere.
	lockfile os.File
}

// Querier is redefined here for non-CGo builds as it's used by ContractCallParams
type Querier = types.Querier

// ContractCallParams is redefined here for non-CGo builds.
// Note: It uses the non-CGo stub version of Cache.
type ContractCallParams struct {
	Cache      Cache
	Checksum   []byte
	Env        []byte
	Info       []byte
	Msg        []byte
	GasMeter   *types.GasMeter
	Store      types.KVStore
	API        *types.GoAPI
	Querier    *Querier
	GasLimit   uint64
	PrintDebug bool
}

var errNoCgo = errors.New("wasmvm library compiled without CGo support")

// Instantiate is a stub implementation for non-CGo builds.
func Instantiate(params ContractCallParams) ([]byte, types.GasReport, error) {
	return nil, types.GasReport{}, errNoCgo
}

// Execute is a stub implementation for non-CGo builds.
func Execute(params ContractCallParams) ([]byte, types.GasReport, error) {
	return nil, types.GasReport{}, errNoCgo
}

// Query is a stub implementation for non-CGo builds.
func Query(params ContractCallParams) ([]byte, types.GasReport, error) {
	return nil, types.GasReport{}, errNoCgo
}

// Migrate is a stub implementation for non-CGo builds.
func Migrate(params ContractCallParams) ([]byte, types.GasReport, error) {
	return nil, types.GasReport{}, errNoCgo
}

// Sudo is a stub implementation for non-CGo builds.
func Sudo(params ContractCallParams) ([]byte, types.GasReport, error) {
	return nil, types.GasReport{}, errNoCgo
}

// Reply is a stub implementation for non-CGo builds.
func Reply(params ContractCallParams) ([]byte, types.GasReport, error) {
	return nil, types.GasReport{}, errNoCgo
}

// InitCache is a stub implementation for non-CGo builds.
func InitCache(config types.VMConfig) (Cache, error) {
	// Minimal implementation needed to satisfy Cache struct definition
	return Cache{}, errNoCgo
}

// ReleaseCache is a stub implementation for non-CGo builds.
func ReleaseCache(cache Cache) {
	// No-op
}

// StoreCode is a stub implementation for non-CGo builds.
func StoreCode(cache Cache, wasm []byte, persist bool) ([]byte, error) {
	return nil, errNoCgo
}

// GetCode is a stub implementation for non-CGo builds.
func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	return nil, errNoCgo
}

// RemoveCode is a stub implementation for non-CGo builds.
func RemoveCode(cache Cache, checksum []byte) error {
	return errNoCgo
}

// StoreCodeUnchecked is a stub implementation for non-CGo builds.
func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	return nil, errNoCgo
}

// Pin is a stub implementation for non-CGo builds.
func Pin(cache Cache, checksum []byte) error {
	return errNoCgo
}

// Unpin is a stub implementation for non-CGo builds.
func Unpin(cache Cache, checksum []byte) error {
	return errNoCgo
}

// GetMetrics is a stub implementation for non-CGo builds.
func GetMetrics(cache Cache) (*types.Metrics, error) {
	return nil, errNoCgo
}

// GetPinnedMetrics is a stub implementation for non-CGo builds.
func GetPinnedMetrics(cache Cache) (*types.PinnedMetrics, error) {
	return nil, errNoCgo
}

// Other CGo-dependent functions from lib.go might need stubs here too if used by non-CGo code.
// For now, we only add the ones directly causing errors in iterator_test.go.
