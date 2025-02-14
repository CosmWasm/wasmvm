package wasm

import (
	"context"
	"crypto/sha256"
	"sync"

	"github.com/rs/zerolog"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Checksum is a 32-byte SHA-256 digest identifying a Wasm code blob [oai_citation_attribution:0‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=In%20order%20to%20add%20a,hash%20of%20the%20Wasm%20blob).
type Checksum []byte

// OutOfGasError is returned when execution runs out of gas.
type OutOfGasError struct{}

func (e OutOfGasError) Error() string { return "Out of gas" }

// WazeroVM is a CosmWasm VM runtime using Wazero.
type WazeroVM struct {
	runtime     wazero.Runtime          // Wazero runtime instance
	envModule   api.Module              // Instantiated host module "env"
	logger      zerolog.Logger          // Structured logger (zerolog)
	cacheMu     sync.RWMutex            // Synchronizes access to caches
	memoryCache map[[32]byte]*cacheItem // LRU cache for compiled modules [oai_citation_attribution:1‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=2.%20,a%20separate%20cache)
	cacheOrder  [][32]byte              // Order of cached keys for LRU (oldest first)
	pinned      map[[32]byte]*cacheItem // Pinned modules (separate cache) [oai_citation_attribution:2‡docs.cosmwasm.com](https://docs.cosmwasm.com/core/architecture/pinning#:~:text=Both%20memory%20caches%20%282,towards%20its%20cache%20size%20limit)
	codeStore   map[[32]byte][]byte     // Storage of original Wasm code bytes
	cacheSize   int                     // Max number of modules in memory cache
	hitsMemory  uint32                  // Metrics: LRU cache hits
	hitsPinned  uint32                  // Metrics: pinned cache hits
	misses      uint32                  // Metrics: cache misses (compilations)
	// Database API
	dbRead   api.GoModuleFunction
	dbWrite  api.GoModuleFunction
	dbRemove api.GoModuleFunction
	dbScan   api.GoModuleFunction
	dbNext   api.GoModuleFunction
	// Address API
	addrValidate     api.GoModuleFunction
	addrCanonicalize api.GoModuleFunction
	addrHumanize     api.GoModuleFunction
	// Cryptography API
	secp256k1Verify        api.GoModuleFunction
	secp256k1RecoverPubkey api.GoModuleFunction
	secp256r1Verify        api.GoModuleFunction
	secp256r1RecoverPubkey api.GoModuleFunction
	ed25519Verify          api.GoModuleFunction
	ed25519BatchVerify     api.GoModuleFunction
	blsAggregateG1         api.GoModuleFunction
	blsAggregateG2         api.GoModuleFunction
	blsPairingEquality     api.GoModuleFunction
	blsHashToG1            api.GoModuleFunction
	blsHashToG2            api.GoModuleFunction
	// Misc API
	debugPrint api.GoModuleFunction
	queryChain api.GoModuleFunction
}

// cacheItem holds a compiled module and metadata for caching.
type cacheItem struct {
	compiled wazero.CompiledModule // The compiled Wasm module
	size     uint64                // Original Wasm bytecode size
	hits     uint32                // Number of times this module was used (for pinned metrics)
}

// NewWazeroVM initializes a new WazeroVM with the given cache size and logger.
func NewWazeroVM(cacheSize int, logger zerolog.Logger) (*WazeroVM, error) {
	vm := &WazeroVM{
		runtime:     wazero.NewRuntime(context.Background()),
		logger:      logger,
		memoryCache: make(map[[32]byte]*cacheItem),
		pinned:      make(map[[32]byte]*cacheItem),
		codeStore:   make(map[[32]byte][]byte),
		cacheSize:   cacheSize,
	}
	// Build and instantiate the host "env" module with all required host functions [oai_citation_attribution:3‡docs.rs](https://docs.rs/crate/cosmwasm-vm/latest/source/src/compatibility.rs#:~:text=%2F%2F%2F%20Lists%20all%20imports%20we,env.addr_canonicalize) [oai_citation_attribution:4‡docs.rs](https://docs.rs/crate/cosmwasm-std/latest/source/src/imports.rs#:~:text=,u32).
	err := vm.buildEnvModule(context.Background())
	if err != nil {
		return nil, err
	}
	logger.Info().Msg("Wazero runtime initialized")
	return vm, nil
}

// buildEnvModule defines and instantiates the host module "env" with all CosmWasm host functions.
func (vm *WazeroVM) buildEnvModule(ctx context.Context) error {
	builder := vm.runtime.NewHostModuleBuilder("env")
	// Database API functions
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.dbRead, []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("db_read")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.dbWrite, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).Export("db_write")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.dbRemove, []api.ValueType{api.ValueTypeI32}, nil).Export("db_remove")
	// Iteration (optional, feature "iterator")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.dbScan, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("db_scan")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.dbNext, []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("db_next")
	// Address API
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.addrValidate, []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("addr_validate")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.addrCanonicalize, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("addr_canonicalize")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.addrHumanize, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("addr_humanize")
	// Cryptography API (secp256k1, secp256r1, ed25519, BLS12-381) [oai_citation_attribution:5‡docs.rs](https://docs.rs/crate/cosmwasm-std/latest/source/src/imports.rs#:~:text=%2F%2F%2F%20Verifies%20message%20hashes%20against,u32) [oai_citation_attribution:6‡docs.rs](https://docs.rs/crate/cosmwasm-std/latest/source/src/imports.rs#:~:text=fn%20secp256r1_verify,u32) [oai_citation_attribution:7‡docs.rs](https://docs.rs/crate/cosmwasm-std/latest/source/src/imports.rs#:~:text=%2F%2F%2F%20Verifies%20a%20message%20against,u32)
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.secp256k1Verify, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("secp256k1_verify")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.secp256k1RecoverPubkey, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).Export("secp256k1_recover_pubkey")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.secp256r1Verify, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("secp256r1_verify")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.secp256r1RecoverPubkey, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).Export("secp256r1_recover_pubkey")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.ed25519Verify, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("ed25519_verify")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.ed25519BatchVerify, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("ed25519_batch_verify")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.blsAggregateG1, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("bls12_381_aggregate_g1")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.blsAggregateG2, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("bls12_381_aggregate_g2")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.blsPairingEquality, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("bls12_381_pairing_equality")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.blsHashToG1, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("bls12_381_hash_to_g1")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.blsHashToG2, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("bls12_381_hash_to_g2")
	// Misc
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.debugPrint, []api.ValueType{api.ValueTypeI32}, nil).Export("debug")
	builder.NewFunctionBuilder().WithGoModuleFunction(vm.queryChain, []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("query_chain")

	// Instantiate the host module
	mod, err := builder.Instantiate(ctx)
	if err != nil {
		return err
	}
	vm.envModule = mod
	vm.logger.Debug().Msg("Host module 'env' instantiated with CosmWasm host functions")
	return nil
}

// computeChecksum returns the SHA-256 checksum of the given code.
func computeChecksum(code []byte) [32]byte {
	return sha256.Sum256(code)
}
