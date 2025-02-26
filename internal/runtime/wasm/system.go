package wasm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// StoreCode compiles and stores a new Wasm code blob, returning its checksum and gas used for compilation.
func (vm *WazeroVM) StoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:] // as []byte
	// Simulate compilation gas cost
	compileCost := uint64(len(code)) * (3 * 140_000) // CostPerByte = 3 * 140k, as per CosmWasm gas schedule
	if gasLimit < compileCost {
		// Not enough gas provided to compile this code
		return cs, compileCost, OutOfGasError{}
	}
	// If code is already stored, we can avoid recompiling (but still charge gas).
	codeHash := [32]byte(checksum)
	vm.cacheMu.Lock()
	alreadyStored := vm.codeStore[codeHash] != nil
	vm.cacheMu.Unlock()
	if !alreadyStored {
		// Insert code into storage
		vm.cacheMu.Lock()
		vm.codeStore[codeHash] = code
		vm.cacheMu.Unlock()
		vm.logger.Info("Stored new contract code", "checksum", hex.EncodeToString(checksum[:]), "size", len(code))
	} else {
		vm.logger.Debug("StoreCode called for already stored code", "checksum", hex.EncodeToString(checksum[:]))
	}
	// Compile module immediately to ensure it is valid and cached.
	vm.cacheMu.Lock()
	_, compErr := vm.getCompiledModule(codeHash)
	vm.cacheMu.Unlock()
	if compErr != nil {
		return cs, compileCost, compErr
	}
	return cs, compileCost, nil
}

// SimulateStoreCode estimates gas needed to store the given code, without actually storing it.
func (vm *WazeroVM) SimulateStoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:]
	cost := uint64(len(code)) * (3 * 140_000) // same formula as compileCost
	if gasLimit < cost {
		return cs, cost, OutOfGasError{}
	}
	// We do not compile or store the code in simulation.
	return cs, cost, nil
}

// GetCode returns the original Wasm bytes for the given code checksum.
func (vm *WazeroVM) GetCode(checksum Checksum) ([]byte, error) {
	codeHash := [32]byte{}
	copy(codeHash[:], checksum) // convert to array key
	vm.cacheMu.RLock()
	code := vm.codeStore[codeHash]
	vm.cacheMu.RUnlock()
	if code == nil {
		return nil, fmt.Errorf("code for %x not found", checksum)
	}
	return code, nil
}

// RemoveCode removes the Wasm bytes and any cached compiled module.
func (vm *WazeroVM) RemoveCode(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// First check if it's pinned (priority cache)
	if item, ok := vm.pinned[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.pinned, hash)
		vm.logger.Info("Removed pinned contract from memory", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	if item, ok := vm.memoryCache[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.memoryCache, hash)
		// Also need to remove from LRU ordering
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.logger.Info("Removed contract from in-memory cache", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	// If not in caches, nothing to remove.
	vm.logger.Debug("RemoveCode called but code not in memory cache", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// Pin marks the module with the given checksum as pinned, meaning it won't be removed by the LRU cache.
func (vm *WazeroVM) Pin(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// If already pinned, nothing to do.
	if _, ok := vm.pinned[hash]; ok {
		return nil
	}
	// See if it's in memory cache, move it to pinned.
	memItem, memOk := vm.memoryCache[hash]
	if memOk {
		delete(vm.memoryCache, hash)
		// Remove from LRU order slice
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		// Add to pinned cache directly
		vm.pinned[hash] = memItem
		vm.logger.Info("Pinned contract code in memory", "checksum", hex.EncodeToString(hash[:]))
		return nil
	}
	// Not in mem cache, fetch from code store & compile.
	code, ok := vm.codeStore[hash]
	if !ok {
		return fmt.Errorf("code %x not found", hash)
	}
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return fmt.Errorf("pinning compilation failed: %w", err)
	}
	item := &cacheItem{
		compiled: compiled,
		size:     uint64(len(code)),
		hits:     0,
	}
	// Add to pinned cache
	vm.pinned[hash] = item
	vm.logger.Info("Pinned contract code in memory", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// Unpin marks the module with the given checksum as unpinned, allowing it to be removed by the LRU cache.
func (vm *WazeroVM) Unpin(checksum Checksum) error {
	hash := [32]byte{}
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// If not pinned, nothing to do.
	item, ok := vm.pinned[hash]
	if !ok {
		return nil
	}
	// Move from pinned to memory cache
	delete(vm.pinned, hash)
	vm.memoryCache[hash] = item
	vm.cacheOrder = append(vm.cacheOrder, hash) // add to end (most recently used)
	// If memoryCache is now over capacity, evict the LRU item
	if len(vm.memoryCache) > vm.cacheSize {
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background())
			delete(vm.memoryCache, oldest)
			vm.logger.Debug("Evicted module after unpin (LRU)", "checksum", hex.EncodeToString(oldest[:]))
		}
	}
	vm.logger.Info("Unpinned contract code", "checksum", hex.EncodeToString(hash[:]))
	return nil
}

// AnalyzeCode statically analyzes the Wasm bytecode and returns capabilities and features it requires.
func (vm *WazeroVM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error) {
	hash := [32]byte{}
	copy(hash[:], checksum)

	// Get the module (either from cache or fresh compile)
	vm.cacheMu.Lock()
	module, err := vm.getCompiledModule(hash)
	vm.cacheMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Create base report
	report := types.AnalysisReport{
		HasIBCEntryPoints:    false,
		RequiredCapabilities: "",
	}

	// First, check exports for IBC entry points
	exports := module.ExportedFunctions()
	for name := range exports {
		// Check for IBC exports
		if name == "ibc_channel_open" || name == "ibc_channel_connect" || name == "ibc_channel_close" ||
			name == "ibc_packet_receive" || name == "ibc_packet_ack" || name == "ibc_packet_timeout" {
			report.HasIBCEntryPoints = true
			break
		}
	}

	// Get the module's imports to check for required capabilities
	var requiredCapabilities string

	// Helper to add capabilities without duplicates
	addCapability := func(cap string) {
		if requiredCapabilities == "" {
			requiredCapabilities = cap
		} else {
			// Check if already present
			found := false
			for _, c := range []string{requiredCapabilities} {
				if c == cap {
					found = true
					break
				}
			}
			if !found {
				requiredCapabilities = requiredCapabilities + "," + cap
			}
		}
	}

	// Check imports to determine capabilities
	for _, imp := range module.ImportedFunctions() {
		impModule, impName, _ := imp.Import()

		if impModule == "env" {
			// Check for capability-indicating imports
			switch impName {
			case "secp256k1_verify", "secp256k1_recover_pubkey":
				addCapability("secp256k1")
			case "ed25519_verify", "ed25519_batch_verify":
				addCapability("ed25519")
			case "addr_humanize", "addr_canonicalize", "addr_validate":
				addCapability("cosmwasm_1_1")
			case "bls12_381_aggregate_g1", "bls12_381_aggregate_g2":
				addCapability("cosmwasm_1_4")
			}
		}
	}

	report.RequiredCapabilities = requiredCapabilities
	return &report, nil
}

// GetMetrics returns aggregated metrics about cache usage.
func (vm *WazeroVM) GetMetrics() (*types.Metrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	m := &types.Metrics{
		HitsPinnedMemoryCache:     uint32(vm.hitsPinned),
		HitsMemoryCache:           uint32(vm.hitsMemory),
		HitsFsCache:               0, // we are not using FS cache in this implementation
		Misses:                    uint32(vm.misses),
		ElementsPinnedMemoryCache: uint64(len(vm.pinned)),
		ElementsMemoryCache:       uint64(len(vm.memoryCache)),
		SizePinnedMemoryCache:     0,
		SizeMemoryCache:           0,
	}
	// Calculate sizes
	for _, item := range vm.pinned {
		m.SizePinnedMemoryCache += item.size
	}
	for _, item := range vm.memoryCache {
		m.SizeMemoryCache += item.size
	}
	return m, nil
}

// GetPinnedMetrics returns detailed metrics for each pinned contract.
func (vm *WazeroVM) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	var entries []types.PerModuleEntry
	for hash, item := range vm.pinned {
		entries = append(entries, types.PerModuleEntry{
			Checksum: hash[:],
			Metrics: types.PerModuleMetrics{
				Hits: uint32(item.hits),
				Size: item.size,
			},
		})
	}
	// Sort entries by checksum for consistency
	sort.Slice(entries, func(i, j int) bool {
		return hex.EncodeToString(entries[i].Checksum) < hex.EncodeToString(entries[j].Checksum)
	})
	return &types.PinnedMetrics{PerModule: entries}, nil
}
