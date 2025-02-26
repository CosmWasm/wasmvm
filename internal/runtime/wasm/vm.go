package wasm

import (
	"context"
	"sync"

	wasmTypes "github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

type Checksum = []byte

type cacheItem struct {
	compiled wazero.CompiledModule
	size     uint64
	hits     uint64
}

// WazeroVM implements the CosmWasm VM using wazero
type WazeroVM struct {
	runtime     wazero.Runtime
	codeStore   map[[32]byte][]byte
	memoryCache map[[32]byte]*cacheItem
	pinned      map[[32]byte]*cacheItem
	cacheOrder  [][32]byte
	cacheSize   int
	cacheMu     sync.RWMutex

	// Cache statistics
	hitsPinned uint64
	hitsMemory uint64
	misses     uint64

	// Configuration
	gasConfig   wasmTypes.GasConfig
	memoryLimit uint32

	logger Logger
}

// Logger defines the logging interface used by WazeroVM
type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
}

// InfoLogger extends Logger with chainable methods for structured logging
type InfoLogger interface {
	Logger
	With(keyvals ...interface{}) Logger
}

// NewWazeroVM creates a new VM instance with wazero as the runtime
func NewWazeroVM(cacheSize int, memoryLimit uint32, gasConfig wasmTypes.GasConfig, logger Logger) *WazeroVM {
	// Create runtime with proper context
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	return &WazeroVM{
		runtime:     runtime,
		codeStore:   make(map[[32]byte][]byte),
		memoryCache: make(map[[32]byte]*cacheItem),
		pinned:      make(map[[32]byte]*cacheItem),
		cacheOrder:  make([][32]byte, 0),
		cacheSize:   cacheSize,
		gasConfig:   gasConfig,
		memoryLimit: memoryLimit,
		logger:      logger,
	}
}

// Close releases all resources held by the VM
func (vm *WazeroVM) Close() error {
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()

	ctx := context.Background()

	// Close all compiled modules
	for _, item := range vm.pinned {
		if err := item.compiled.Close(ctx); err != nil {
			vm.logger.Error("Error closing pinned module", "error", err)
		}
	}

	for _, item := range vm.memoryCache {
		if err := item.compiled.Close(ctx); err != nil {
			vm.logger.Error("Error closing cached module", "error", err)
		}
	}

	// Clear maps
	vm.pinned = make(map[[32]byte]*cacheItem)
	vm.memoryCache = make(map[[32]byte]*cacheItem)
	vm.cacheOrder = make([][32]byte, 0)

	// Close the runtime
	return vm.runtime.Close(ctx)
}
