package host

import (
	"encoding/binary"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
)

// --- Minimal Host Interfaces ---
// WasmInstance is a minimal interface for a WASM contract instance.
type WasmInstance interface {
	// RegisterFunction registers a host function with the instance.
	RegisterFunction(module, name string, fn interface{})
}

// MemoryManager is imported from our memory package.
type MemoryManager = memory.MemoryManager

// Storage represents contract storage.
type Storage interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Scan(start, end []byte, order int32) (uint32, error)
	Next(iteratorID uint32) (key []byte, value []byte, err error)
}

// API aliases types.GoAPI.
type API = types.GoAPI

// Querier aliases types.Querier.
type Querier = types.Querier

// GasMeter aliases types.GasMeter.
type GasMeter = types.GasMeter

// Logger is a simple logging interface.
type Logger interface {
	Debug(args ...interface{})
	Error(args ...interface{})
}

// --- Runtime Environment ---
// RuntimeEnvironment holds all execution context for a contract call.
type RuntimeEnvironment struct {
	DB        types.KVStore
	API       API
	Querier   Querier
	Gas       GasMeter
	GasConfig types.GasConfig

	// internal gas limit and gas used for host functions:
	gasLimit uint64
	gasUsed  uint64

	// Iterator management.
	iterators      map[uint64]map[uint64]types.Iterator
	iteratorsMutex types.RWMutex // alias for sync.RWMutex from types package if desired
	nextCallID     uint64
	nextIterID     uint64
}

// --- Helper: writeToRegion ---
// writeToRegion uses MemoryManager to update a Region struct and write the provided data.
func writeToRegion(mem MemoryManager, regionPtr uint32, data []byte) error {
	regionStruct, err := mem.Read(regionPtr, 12)
	if err != nil {
		return fmt.Errorf("failed to read Region at %d: %w", regionPtr, err)
	}
	offset := binary.LittleEndian.Uint32(regionStruct[0:4])
	capacity := binary.LittleEndian.Uint32(regionStruct[4:8])
	if uint32(len(data)) > capacity {
		return fmt.Errorf("data length %d exceeds region capacity %d", len(data), capacity)
	}
	if err := mem.Write(offset, data); err != nil {
		return fmt.Errorf("failed to write data to memory at offset %d: %w", offset, err)
	}
	binary.LittleEndian.PutUint32(regionStruct[8:12], uint32(len(data)))
	if err := mem.Write(regionPtr+8, regionStruct[8:12]); err != nil {
		return fmt.Errorf("failed to write Region length at %d: %w", regionPtr+8, err)
	}
	return nil
}

// --- RegisterHostFunctions ---
// RegisterHostFunctions registers all host functions with the provided module builder
func RegisterHostFunctions(mod wazero.HostModuleBuilder) {
	// Abort: abort(msg_ptr: u32, file_ptr: u32, line: u32, col: u32) -> !
	mod.NewFunctionBuilder().WithFunc(Abort).Export("abort")

	// Debug: debug(msg_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(Debug).Export("debug")

	// db_read: db_read(key_ptr: u32) -> u32 (returns Region pointer or 0 if not found)
	mod.NewFunctionBuilder().WithFunc(DbRead).Export("db_read")

	// db_write: db_write(key_ptr: u32, value_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(DbWrite).Export("db_write")

	// db_remove: db_remove(key_ptr: u32) -> ()
	mod.NewFunctionBuilder().WithFunc(DbRemove).Export("db_remove")

	// db_scan: db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbScan).Export("db_scan")

	// db_next: db_next(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNext).Export("db_next")

	// db_next_key: db_next_key(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNextKey).Export("db_next_key")

	// db_next_value: db_next_value(iterator_id: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(DbNextValue).Export("db_next_value")

	// addr_validate: addr_validate(addr_ptr: u32) -> u32 (0 = success, nonzero = error)
	mod.NewFunctionBuilder().WithFunc(AddrValidate).Export("addr_validate")

	// addr_canonicalize: addr_canonicalize(human_ptr: u32, canon_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(AddrCanonicalize).Export("canonicalize_address")

	// addr_humanize: addr_humanize(canon_ptr: u32, human_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(hostHumanizeAddress).Export("humanize_address")

	// Crypto operations
	mod.NewFunctionBuilder().WithFunc(Secp256k1Verify).Export("secp256k1_verify")
	mod.NewFunctionBuilder().WithFunc(Secp256k1RecoverPubkey).Export("secp256k1_recover_pubkey")
	mod.NewFunctionBuilder().WithFunc(Ed25519Verify).Export("ed25519_verify")
	mod.NewFunctionBuilder().WithFunc(Ed25519BatchVerify).Export("ed25519_batch_verify")

	// BLS crypto operations
	mod.NewFunctionBuilder().WithFunc(Bls12381AggregateG1).Export("bls12_381_aggregate_g1")
	mod.NewFunctionBuilder().WithFunc(Bls12381AggregateG2).Export("bls12_381_aggregate_g2")
	mod.NewFunctionBuilder().WithFunc(Bls12381PairingCheck).Export("bls12_381_pairing_equality")
	mod.NewFunctionBuilder().WithFunc(Bls12381HashToG1).Export("bls12_381_hash_to_g1")
	mod.NewFunctionBuilder().WithFunc(Bls12381HashToG2).Export("bls12_381_hash_to_g2")

	// query_chain: query_chain(request_ptr: u32) -> u32
	mod.NewFunctionBuilder().WithFunc(QueryChain).Export("query_chain")
}
