package wasm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

// StoreCode compiles and stores a new Wasm code blob, returning its checksum and gas used for compilation.
func (vm *WazeroVM) StoreCode(code []byte, gasLimit uint64) (Checksum, uint64, error) {
	checksum := sha256.Sum256(code)
	cs := checksum[:] // as []byte
	// Simulate compilation gas cost [oai_citation_attribution:39‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=%2F%2F%20Benchmarks%20and%20numbers%20,were%20discussed%20in):
	compileCost := uint64(len(code)) * (3 * 140_000) // CostPerByte = 3 * 140k, as per CosmWasm gas schedule [oai_citation_attribution:40‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=%2F%2F%20https%3A%2F%2Fgithub.com%2FCosmWasm%2Fwasmd%2Fpull%2F634%23issuecomment).
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
		vm.logger.Info().Str("checksum", hex.EncodeToString(checksum[:])).Int("size", len(code)).Msg("Stored new contract code")
	} else {
		vm.logger.Warn().Str("checksum", hex.EncodeToString(checksum[:])).Msg("StoreCode called for already stored code")
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
	cost := uint64(len(code)) * (3 * 140_000) // same formula as compileCost [oai_citation_attribution:41‡github.com](https://github.com/CosmWasm/wasmvm/blob/main/lib_libwasmvm.go#:~:text=%2F%2F%20https%3A%2F%2Fgithub.com%2FCosmWasm%2Fwasmd%2Fpull%2F634%23issuecomment)
	if gasLimit < cost {
		return cs, cost, OutOfGasError{}
	}
	// We do not compile or store the code in simulation.
	return cs, cost, nil
}

// GetCode returns the original Wasm bytes for the given code checksum.
func (vm *WazeroVM) GetCode(checksum Checksum) ([]byte, error) {
	var hash [32]byte
	if len(checksum) == 32 {
		copy(hash[:], checksum[:32])
	} else {
		return nil, fmt.Errorf("invalid checksum length")
	}
	vm.cacheMu.RLock()
	code, ok := vm.codeStore[hash]
	vm.cacheMu.RUnlock()
	if !ok {
		return nil, types.NoSuchCode{CodeID: 0} // or a generic not found error
	}
	return code, nil
}

// RemoveCode removes the compiled module (if any) for the given code checksum from memory caches.
func (vm *WazeroVM) RemoveCode(checksum Checksum) error {
	var hash [32]byte
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	if item, ok := vm.pinned[hash]; ok {
		// Remove from pinned cache
		_ = item.compiled.Close(context.Background())
		delete(vm.pinned, hash)
		vm.logger.Info().Str("checksum", hex.EncodeToString(hash[:])).Msg("Removed pinned contract from memory")
		return nil
	}
	if item, ok := vm.memoryCache[hash]; ok {
		_ = item.compiled.Close(context.Background())
		delete(vm.memoryCache, hash)
		// Remove from LRU order slice
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		vm.logger.Info().Str("checksum", hex.EncodeToString(hash[:])).Msg("Removed contract from in-memory cache")
		return nil
	}
	// If not in caches, nothing to remove.
	vm.logger.Debug().Str("checksum", hex.EncodeToString(hash[:])).Msg("RemoveCode called but code not in memory cache")
	return nil
}

// Pin moves the given code's compiled module into the pinned cache (preventing eviction) [oai_citation_attribution:42‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=In%20order%20to%20add%20a,hash%20of%20the%20Wasm%20blob).
func (vm *WazeroVM) Pin(checksum Checksum) error {
	var hash [32]byte
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	// Ensure compiled module is loaded
	item, inMem := vm.memoryCache[hash]
	if !inMem {
		// If not in memory cache, maybe not compiled yet; compile it.
		if vm.codeStore[hash] == nil {
			return types.NoSuchCode{CodeID: 0}
		}
		compiled, err := vm.runtime.CompileModule(context.Background(), vm.codeStore[hash])
		if err != nil {
			return fmt.Errorf("compilation failed: %w", err)
		}
		item = &cacheItem{compiled: compiled, size: uint64(len(vm.codeStore[hash])), hits: 0}
	} else {
		// Remove from LRU structures
		for i, h := range vm.cacheOrder {
			if h == hash {
				vm.cacheOrder = append(vm.cacheOrder[:i], vm.cacheOrder[i+1:]...)
				break
			}
		}
		delete(vm.memoryCache, hash)
	}
	// Add to pinned cache
	vm.pinned[hash] = item
	vm.logger.Info().Str("checksum", hex.EncodeToString(hash[:])).Msg("Pinned contract code in memory")
	return nil
}

// Unpin removes the code from the pinned cache, allowing it to be managed by the LRU cache.
func (vm *WazeroVM) Unpin(checksum Checksum) error {
	var hash [32]byte
	copy(hash[:], checksum)
	vm.cacheMu.Lock()
	defer vm.cacheMu.Unlock()
	item, wasPinned := vm.pinned[hash]
	if !wasPinned {
		return fmt.Errorf("code not pinned")
	}
	// Remove from pinned
	delete(vm.pinned, hash)
	// Insert into memory cache (at most-recent position)
	vm.memoryCache[hash] = item
	vm.cacheOrder = append(vm.cacheOrder, hash)
	if len(vm.memoryCache) > vm.cacheSize {
		// evict least used
		oldest := vm.cacheOrder[0]
		vm.cacheOrder = vm.cacheOrder[1:]
		if ci, ok := vm.memoryCache[oldest]; ok {
			_ = ci.compiled.Close(context.Background())
			delete(vm.memoryCache, oldest)
			vm.logger.Debug().Str("checksum", hex.EncodeToString(oldest[:])).Msg("Evicted module after unpin (LRU)")
		}
	}
	vm.logger.Info().Str("checksum", hex.EncodeToString(hash[:])).Msg("Unpinned contract code")
	return nil
}

// AnalyzeCode performs static analysis on the Wasm code to determine supported features and entry points.
func (vm *WazeroVM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error) {
	var hash [32]byte
	copy(hash[:], checksum)
	vm.cacheMu.RLock()
	code, ok := vm.codeStore[hash]
	vm.cacheMu.RUnlock()
	if !ok {
		return nil, types.NoSuchCode{CodeID: 0}
	}
	// Compile module (if not already compiled) to inspect it.
	compiled, err := vm.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}
	defer compiled.Close(context.Background())
	// Instantiate module to query exports.
	module, err := vm.runtime.InstantiateModule(context.Background(), compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("instantiate failed: %w", err)
	}
	defer module.Close(context.Background())
	report := types.AnalysisReport{
		HasIBCEntryPoints:      false,
		RequiredCapabilities:   "",
		Entrypoints:            []string{},
		ContractMigrateVersion: nil,
	}
	// Determine entry points present.
	exports := module.ExportedFunctionDefinitions()
	knownEntries := []string{"instantiate", "execute", "query", "migrate", "sudo", "reply",
		"ibc_channel_open", "ibc_channel_connect", "ibc_channel_close",
		"ibc_packet_receive", "ibc_packet_ack", "ibc_packet_timeout"}
	for name := range exports {
		// Check if this export is one of the known entry points.
		for _, entry := range knownEntries {
			if name == entry {
				report.Entrypoints = append(report.Entrypoints, name)
				if name == "ibc_channel_open" || name == "ibc_channel_connect" || name == "ibc_channel_close" ||
					name == "ibc_packet_receive" || name == "ibc_packet_ack" || name == "ibc_packet_timeout" {
					report.HasIBCEntryPoints = true
				}
			}
		}
		// Check for version requirement markers
		if name == "requires_cosmwasm_2_1" {
			report.RequiredCapabilities = appendCapability(report.RequiredCapabilities, "cosmwasm_2_1")
		}
		if name == "requires_cosmwasm_2_0" {
			report.RequiredCapabilities = appendCapability(report.RequiredCapabilities, "cosmwasm_2_0")
		}
		if name == "requires_cosmwasm_1_4" {
			report.RequiredCapabilities = appendCapability(report.RequiredCapabilities, "cosmwasm_1_4")
		}
		// Check for optional features by imports
		// (Note: in CosmWasm, presence of iterator imports indicates "iterator" capability)
		// We'll inspect import names via module.ImportedFunctions below.
	}
	// Check imports for iterator support
	importedFuncs := module.ImportedFunctionDefinitions()
	for _, f := range importedFuncs {
		modName, funcName, _ := f.Import()
		if modName == "env" && (funcName == "db_scan" || funcName == "db_next") {
			report.RequiredCapabilities = appendCapability(report.RequiredCapabilities, "iterator")
			break
		}
	}
	// Determine contract migrate version if present.
	migrateVerFn := module.ExportedFunction("contract_migrate_version")
	if migrateVerFn != nil {
		res, err := migrateVerFn.Call(context.Background())
		if err == nil && len(res) > 0 {
			ver := uint64(res[0])
			report.ContractMigrateVersion = &ver
		}
	}
	// Sort entrypoints for deterministic order
	sort.Strings(report.Entrypoints)
	return &report, nil
}

// GetMetrics returns aggregated metrics about cache usage.
func (vm *WazeroVM) GetMetrics() (*types.Metrics, error) {
	vm.cacheMu.RLock()
	defer vm.cacheMu.RUnlock()
	m := &types.Metrics{
		HitsPinnedMemoryCache:     vm.hitsPinned,
		HitsMemoryCache:           vm.hitsMemory,
		HitsFsCache:               0, // we are not using FS cache in this implementation
		Misses:                    vm.misses,
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
				Hits: item.hits,
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

// appendCapability adds a capability string to the RequiredCapabilities field, avoiding duplicates and formatting with commas.
func appendCapability(capStr string, cap string) string {
	if capStr == "" {
		return cap
	}
	// avoid duplicate
	for _, c := range []string{capStr} {
		if c == cap {
			return capStr
		}
	}
	return capStr + "," + cap
}
