package host

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/memory"
	"github.com/CosmWasm/wasmvm/v2/types"
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

// NewRuntimeEnvironment creates a new RuntimeEnvironment.
func NewRuntimeEnvironment(db types.KVStore, api API, querier Querier, gasLimit uint64) *RuntimeEnvironment {
	return &RuntimeEnvironment{
		DB:        db,
		API:       api,
		Querier:   querier,
		Gas:       types.NewGasState(gasLimit),
		gasLimit:  gasLimit,
		gasUsed:   0,
		GasConfig: types.DefaultGasConfig(),
		iterators: make(map[uint64]map[uint64]types.Iterator),
	}
}

func (env *RuntimeEnvironment) StartCall() uint64 {
	// For simplicity we don’t show the locking here, but in production this should be thread‐safe.
	env.nextCallID++
	env.iterators[env.nextCallID] = make(map[uint64]types.Iterator)
	return env.nextCallID
}

func (env *RuntimeEnvironment) StoreIterator(callID uint64, iter types.Iterator) uint64 {
	env.nextIterID++
	if env.iterators[callID] == nil {
		env.iterators[callID] = make(map[uint64]types.Iterator)
	}
	env.iterators[callID][env.nextIterID] = iter
	return env.nextIterID
}

func (env *RuntimeEnvironment) GetIterator(callID, iterID uint64) types.Iterator {
	if callMap, exists := env.iterators[callID]; exists {
		return callMap[iterID]
	}
	return nil
}

func (env *RuntimeEnvironment) EndCall(callID uint64) {
	delete(env.iterators, callID)
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
// RegisterHostFunctions registers all host functions. It uses the provided WasmInstance,
// MemoryManager, Storage, API, Querier, GasMeter, and Logger.
func RegisterHostFunctions(instance WasmInstance, mem MemoryManager, storage Storage, api API, querier Querier, gasMeter GasMeter, logger Logger) {
	// Abort: abort(msg_ptr: u32, file_ptr: u32, line: u32, col: u32) -> !
	instance.RegisterFunction("env", "abort", func(msgPtr, filePtr uint32, line, col uint32) {
		msg, _ := mem.ReadRegion(msgPtr)
		file, _ := mem.ReadRegion(filePtr)
		logger.Error(fmt.Sprintf("Wasm abort called: %s (%s:%d:%d)", string(msg), string(file), line, col))
		panic(fmt.Sprintf("Wasm abort: %s", string(msg)))
	})

	// Debug: debug(msg_ptr: u32) -> ()
	instance.RegisterFunction("env", "debug", func(msgPtr uint32) {
		msg, err := mem.ReadRegion(msgPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("debug: failed to read message: %v", err))
			return
		}
		logger.Debug("cosmwasm debug:", string(msg))
	})

	// db_read: db_read(key_ptr: u32) -> u32 (returns Region pointer or 0 if not found)
	instance.RegisterFunction("env", "db_read", func(keyPtr uint32) uint32 {
		key, err := mem.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: failed to read key region: %v", err))
			return 0
		}
		value, err := storage.Get(key)
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: storage error: %v", err))
			return 0
		}
		if value == nil {
			logger.Debug("db_read: key not found")
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: memory allocation failed: %v", err))
			return 0
		}
		if err := mem.Write(regionPtr, value); err != nil {
			logger.Error(fmt.Sprintf("db_read: failed to write value to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_read: key %X -> %d bytes at region %d", key, len(value), regionPtr))
		return regionPtr
	})

	// db_write: db_write(key_ptr: u32, value_ptr: u32) -> ()
	instance.RegisterFunction("env", "db_write", func(keyPtr, valuePtr uint32) {
		key, err := mem.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_write: failed to read key: %v", err))
			return
		}
		value, err := mem.ReadRegion(valuePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_write: failed to read value: %v", err))
			return
		}
		if err := storage.Set(key, value); err != nil {
			logger.Error(fmt.Sprintf("db_write: storage error: %v", err))
		} else {
			logger.Debug(fmt.Sprintf("db_write: stored %d bytes under key %X", len(value), key))
		}
	})

	// db_remove: db_remove(key_ptr: u32) -> ()
	instance.RegisterFunction("env", "db_remove", func(keyPtr uint32) {
		key, err := mem.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_remove: failed to read key: %v", err))
			return
		}
		if err := storage.Delete(key); err != nil {
			logger.Error(fmt.Sprintf("db_remove: storage error: %v", err))
		} else {
			logger.Debug(fmt.Sprintf("db_remove: removed key %X", key))
		}
	})

	// db_scan: db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32
	instance.RegisterFunction("env", "db_scan", func(startPtr, endPtr uint32, order int32) uint32 {
		start, err := mem.ReadRegion(startPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: failed to read start key: %v", err))
			return 0
		}
		end, err := mem.ReadRegion(endPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: failed to read end key: %v", err))
			return 0
		}
		iteratorID, err := storage.Scan(start, end, order)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: storage scan error: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_scan: created iterator %d for range [%X, %X], order %d", iteratorID, start, end, order))
		return iteratorID
	})

	// db_next: db_next(iterator_id: u32) -> u32
	instance.RegisterFunction("env", "db_next", func(iteratorID uint32) uint32 {
		key, value, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if key == nil {
			logger.Debug(fmt.Sprintf("db_next: iterator %d exhausted", iteratorID))
			return 0
		}
		// Allocate regions for key and value.
		keyRegion, err := mem.Allocate(uint32(len(key)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for key: %v", err))
			return 0
		}
		if err := writeToRegion(mem, keyRegion, key); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write key to region: %v", err))
			return 0
		}
		valRegion, err := mem.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for value: %v", err))
			return 0
		}
		if err := writeToRegion(mem, valRegion, value); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write value to region: %v", err))
			return 0
		}
		// Allocate a combined region for both key and value Region structs.
		combinedSize := uint32(12 * 2)
		combinedPtr, err := mem.Allocate(combinedSize)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for combined region: %v", err))
			return 0
		}
		keyRegionStruct, err := mem.Read(keyRegion, 12)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to read key region struct: %v", err))
			return 0
		}
		valRegionStruct, err := mem.Read(valRegion, 12)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to read value region struct: %v", err))
			return 0
		}
		concat := append(keyRegionStruct, valRegionStruct...)
		if err := mem.Write(combinedPtr, concat); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write combined region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next: iterator %d next -> key %d bytes, value %d bytes", iteratorID, len(key), len(value)))
		return combinedPtr
	})

	// db_next_key: db_next_key(iterator_id: u32) -> u32
	instance.RegisterFunction("env", "db_next_key", func(iteratorID uint32) uint32 {
		key, _, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_key: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if key == nil {
			logger.Debug(fmt.Sprintf("db_next_key: iterator %d exhausted", iteratorID))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(key)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_key: failed to allocate memory: %v", err))
			return 0
		}
		if err := writeToRegion(mem, regionPtr, key); err != nil {
			logger.Error(fmt.Sprintf("db_next_key: failed to write key to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next_key: iterator %d -> key %d bytes", iteratorID, len(key)))
		return regionPtr
	})

	// db_next_value: db_next_value(iterator_id: u32) -> u32
	instance.RegisterFunction("env", "db_next_value", func(iteratorID uint32) uint32 {
		_, value, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_value: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if value == nil {
			logger.Debug(fmt.Sprintf("db_next_value: iterator %d exhausted", iteratorID))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_value: failed to allocate memory: %v", err))
			return 0
		}
		if err := writeToRegion(mem, regionPtr, value); err != nil {
			logger.Error(fmt.Sprintf("db_next_value: failed to write value to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next_value: iterator %d -> value %d bytes", iteratorID, len(value)))
		return regionPtr
	})

	// addr_validate: addr_validate(addr_ptr: u32) -> u32 (0 = success, nonzero = error)
	instance.RegisterFunction("env", "addr_validate", func(addrPtr uint32) uint32 {
		addrBytes, err := mem.ReadRegion(addrPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_validate: failed to read address: %v", err))
			return 1
		}
		if err := api.ValidateAddress(string(addrBytes)); err != nil {
			logger.Debug(fmt.Sprintf("addr_validate: address %q is INVALID: %v", addrBytes, err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_validate: address %q is valid", addrBytes))
		return 0
	})

	// addr_canonicalize: addr_canonicalize(human_ptr: u32, canon_ptr: u32) -> u32
	instance.RegisterFunction("env", "addr_canonicalize", func(humanPtr, canonPtr uint32) uint32 {
		humanAddr, err := mem.ReadRegion(humanPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_canonicalize: failed to read human address: %v", err))
			return 1
		}
		canon, err := api.CanonicalAddress(string(humanAddr))
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_canonicalize: invalid address %q: %v", humanAddr, err))
			return 1
		}
		if err := writeToRegion(mem, canonPtr, canon); err != nil {
			logger.Error(fmt.Sprintf("addr_canonicalize: failed to write canonical address: %v", err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_canonicalize: %q -> %X", humanAddr, canon))
		return 0
	})

	// addr_humanize: addr_humanize(canon_ptr: u32, human_ptr: u32) -> u32
	instance.RegisterFunction("env", "addr_humanize", func(canonPtr, humanPtr uint32) uint32 {
		canonAddr, err := mem.ReadRegion(canonPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_humanize: failed to read canonical address: %v", err))
			return 1
		}
		human, err := api.HumanAddress(canonAddr)
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_humanize: invalid canonical addr %X: %v", canonAddr, err))
			return 1
		}
		if err := writeToRegion(mem, humanPtr, []byte(human)); err != nil {
			logger.Error(fmt.Sprintf("addr_humanize: failed to write human address: %v", err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_humanize: %X -> %q", canonAddr, human))
		return 0
	})

	// secp256k1_verify: secp256k1_verify(hash_ptr: u32, sig_ptr: u32, pubkey_ptr: u32) -> u32
	instance.RegisterFunction("env", "secp256k1_verify", func(hashPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msgHash, err := mem.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read message hash: %v", err))
			return 2
		}
		sig, err := mem.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := mem.ReadRegion(pubKeyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read public key: %v", err))
			return 2
		}
		valid, err := api.Secp256k1Verify(msgHash, sig, pubKey)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("secp256k1_verify: signature verification FAILED")
			return 1
		}
		logger.Debug("secp256k1_verify: signature verification successful")
		return 0
	})

	// secp256k1_recover_pubkey: secp256k1_recover_pubkey(hash_ptr: u32, sig_ptr: u32, recovery_param: u32) -> u64
	instance.RegisterFunction("env", "secp256k1_recover_pubkey", func(hashPtr, sigPtr, param uint32) uint64 {
		msgHash, err := mem.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to read message hash: %v", err))
			return 0
		}
		sig, err := mem.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to read signature: %v", err))
			return 0
		}
		pubKey, err := api.Secp256k1RecoverPubkey(msgHash, sig, byte(param))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: recover failed: %v", err))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(pubKey)))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: allocation failed: %v", err))
			return 0
		}
		if err := writeToRegion(mem, regionPtr, pubKey); err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to write pubkey: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("secp256k1_recover_pubkey: recovered %d-byte pubkey", len(pubKey)))
		return (uint64(regionPtr) << 32) | uint64(len(pubKey))
	})

	// ed25519_verify: ed25519_verify(msg_ptr: u32, sig_ptr: u32, pubkey_ptr: u32) -> u32
	instance.RegisterFunction("env", "ed25519_verify", func(msgPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msg, err := mem.ReadRegion(msgPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read message: %v", err))
			return 2
		}
		sig, err := mem.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := mem.ReadRegion(pubKeyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read public key: %v", err))
			return 2
		}
		valid, err := api.Ed25519Verify(msg, sig, pubKey)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("ed25519_verify: signature verification FAILED")
			return 1
		}
		logger.Debug("ed25519_verify: signature verification successful")
		return 0
	})

	// ed25519_batch_verify: ed25519_batch_verify(msgs_ptr: u32, sigs_ptr: u32, pubkeys_ptr: u32) -> u32
	instance.RegisterFunction("env", "ed25519_batch_verify", func(msgsPtr, sigsPtr, pubKeysPtr uint32) uint32 {
		msgs, err := mem.ReadRegion(msgsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read messages: %v", err))
			return 2
		}
		sigs, err := mem.ReadRegion(sigsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read signatures: %v", err))
			return 2
		}
		pubKeys, err := mem.ReadRegion(pubKeysPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read public keys: %v", err))
			return 2
		}
		valid, err := api.Ed25519BatchVerify(msgs, sigs, pubKeys)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("ed25519_batch_verify: batch verification FAILED")
			return 1
		}
		logger.Debug("ed25519_batch_verify: batch verification successful")
		return 0
	})

	// bls12_381_aggregate_g1: bls12_381_aggregate_g1(messages_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_aggregate_g1", func(messagesPtr uint32) uint32 {
		msgs, err := mem.ReadRegion(messagesPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: failed to read messages: %v", err))
			return 1
		}
		result, err := api.Bls12381AggregateG1(msgs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_aggregate_g1: aggregation successful")
		return regionPtr
	})

	// bls12_381_aggregate_g2: bls12_381_aggregate_g2(messages_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_aggregate_g2", func(messagesPtr uint32) uint32 {
		msgs, err := mem.ReadRegion(messagesPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: failed to read messages: %v", err))
			return 1
		}
		result, err := api.Bls12381AggregateG2(msgs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_aggregate_g2: aggregation successful")
		return regionPtr
	})

	// bls12_381_pairing_equality: bls12_381_pairing_equality(pairs_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_pairing_equality", func(pairsPtr uint32) uint32 {
		pairs, err := mem.ReadRegion(pairsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_pairing_equality: failed to read pairs: %v", err))
			return 2
		}
		equal, err := api.Bls12381PairingCheck(pairs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_pairing_equality: error: %v", err))
			return 3
		}
		if !equal {
			logger.Debug("bls12_381_pairing_equality: pairs are NOT equal")
			return 1
		}
		logger.Debug("bls12_381_pairing_equality: pairs are equal")
		return 0
	})

	// bls12_381_hash_to_g1: bls12_381_hash_to_g1(message_ptr: u32, dest_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_hash_to_g1", func(messagePtr, destPtr uint32) uint32 {
		msg, err := mem.ReadRegion(messagePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to read message: %v", err))
			return 1
		}
		dest, err := mem.ReadRegion(destPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to read dest: %v", err))
			return 1
		}
		result, err := api.Bls12381HashToG1(msg, dest)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_hash_to_g1: hash successful")
		return regionPtr
	})

	// bls12_381_hash_to_g2: bls12_381_hash_to_g2(message_ptr: u32, dest_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_hash_to_g2", func(messagePtr, destPtr uint32) uint32 {
		msg, err := mem.ReadRegion(messagePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to read message: %v", err))
			return 1
		}
		dest, err := mem.ReadRegion(destPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to read dest: %v", err))
			return 1
		}
		result, err := api.Bls12381HashToG2(msg, dest)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: error: %v", err))
			return 1
		}
		regionPtr, err := mem.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(mem, regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_hash_to_g2: hash successful")
		return regionPtr
	})

	// query_chain: query_chain(request_ptr: u32) -> u32
	instance.RegisterFunction("env", "query_chain", func(reqPtr uint32) uint32 {
		request, err := mem.ReadRegion(reqPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to read request: %v", err))
			return 0
		}
		response := types.RustQuery(querier, request, gasMeter.GasConsumed())
		serialized, err := json.Marshal(response)
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to serialize response: %v", err))
			return 0
		}
		regionPtr, err := mem.Allocate(uint32(len(serialized)))
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: allocation failed: %v", err))
			return 0
		}
		if err := mem.Write(regionPtr, serialized); err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to write response: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("query_chain: responded with %d bytes at region %d", len(serialized), regionPtr))
		return regionPtr
	})
}
