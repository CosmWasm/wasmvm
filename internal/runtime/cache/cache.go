package cache

import (
	"crypto/sha256"
	"sync"

	"github.com/tetratelabs/wazero"
)

// Cache manages compiled Wasm modules
type Cache struct {
	mu              sync.RWMutex
	codeCache       map[string][]byte // stores raw Wasm bytes
	compiledModules map[string]wazero.CompiledModule
	pinnedModules   map[string]struct{}
	moduleHits      map[string]uint32
	moduleSizes     map[string]uint64
}

// New creates a new cache instance
func New() *Cache {
	return &Cache{
		codeCache:       make(map[string][]byte),
		compiledModules: make(map[string]wazero.CompiledModule),
		pinnedModules:   make(map[string]struct{}),
		moduleHits:      make(map[string]uint32),
		moduleSizes:     make(map[string]uint64),
	}
}

// Save stores a Wasm module in the cache
func (c *Cache) Save(code []byte) []byte {
	checksum := sha256.Sum256(code)
	key := string(checksum[:])

	c.mu.Lock()
	defer c.mu.Unlock()

	c.codeCache[key] = code
	c.moduleSizes[key] = uint64(len(code))
	return checksum[:]
}

// Load retrieves a Wasm module from the cache
func (c *Cache) Load(checksum []byte) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	code, exists := c.codeCache[string(checksum)]
	if exists {
		c.moduleHits[string(checksum)]++
	}
	return code, exists
}

// Pin marks a module as pinned in memory
func (c *Cache) Pin(checksum []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pinnedModules[string(checksum)] = struct{}{}
}

// Unpin removes the pin from a module
func (c *Cache) Unpin(checksum []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pinnedModules, string(checksum))
}

// Remove deletes a module from the cache if it's not pinned
func (c *Cache) Remove(checksum []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := string(checksum)
	if _, isPinned := c.pinnedModules[key]; isPinned {
		return false
	}

	delete(c.codeCache, key)
	delete(c.compiledModules, key)
	delete(c.moduleHits, key)
	delete(c.moduleSizes, key)
	return true
}

// SaveCompiledModule stores a compiled module in the cache
func (c *Cache) SaveCompiledModule(checksum []byte, module wazero.CompiledModule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compiledModules[string(checksum)] = module
}

// LoadCompiledModule retrieves a compiled module from the cache
func (c *Cache) LoadCompiledModule(checksum []byte) (wazero.CompiledModule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	module, exists := c.compiledModules[string(checksum)]
	return module, exists
}
