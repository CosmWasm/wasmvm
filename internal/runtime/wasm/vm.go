package wasm

import (
	"context"
	"fmt"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/crypto"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/gas"
	"github.com/CosmWasm/wasmvm/v2/internal/runtime/host"
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
func NewWazeroVM() (*WazeroVM, error) {
	// Create runtime with proper context
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	// Set default values
	cacheSize := 100                        // Default cache size
	memoryLimit := uint32(32 * 1024 * 1024) // Default memory limit (32MB)
	gasConfig := gas.DefaultGasConfig()

	// Create the VM instance
	vm := &WazeroVM{
		runtime:     runtime,
		codeStore:   make(map[[32]byte][]byte),
		memoryCache: make(map[[32]byte]*cacheItem),
		pinned:      make(map[[32]byte]*cacheItem),
		cacheOrder:  make([][32]byte, 0),
		cacheSize:   cacheSize,
		gasConfig:   gasConfig,
		memoryLimit: memoryLimit,
		logger:      nil, // Will use default logger
	}

	// Initialize crypto handler
	err := crypto.SetupCryptoHandlers()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize crypto handlers: %w", err)
	}

	// Register host modules
	hostModule := vm.runtime.NewHostModuleBuilder("env")

	// Register core host functions from host package
	host.RegisterHostFunctions(hostModule)

	// Register crypto host functions
	crypto.RegisterHostFunctions(hostModule)

	// Instantiate the host module
	_, err = hostModule.Instantiate(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	return vm, nil
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
