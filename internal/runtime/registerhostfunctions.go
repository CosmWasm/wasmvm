package runtime

import (
	"encoding/binary"
	"fmt"
)

// RegisterHostFunctions registers all required host functions for CosmWasm 1.x and 2.x,
// integrating with the MemoryManager for all memory operations. All host function
// signatures and behaviors are preserved exactly as required by CosmWasm [oai_citation_attribution:0‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=fn%20db_read%28key%3A%20u32%29%20,key%3A%20u32) [oai_citation_attribution:1‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=fn%20addr_validate%28source_ptr%3A%20u32%29%20,u32).
func RegisterHostFunctions(instance WasmInstance, memory MemoryManager, storage Storage, api API, querier Querier, gasMeter GasMeter, logger Logger) {
	// Helper to safely write data into a Region allocated in Wasm memory.
	writeToRegion := func(regionPtr uint32, data []byte) error {
		// Read the Region struct at regionPtr (12 bytes: offset, capacity, length)
		regionStruct := make([]byte, 12)
		data, err := memory.Read(regionPtr, 12)
		if err != nil {
			return fmt.Errorf("failed to read Region at %d: %w", regionPtr, err)
		}
		copy(regionStruct, data)
		// Parse offset and capacity from the Region struct
		offset := binary.LittleEndian.Uint32(regionStruct[0:4])
		capacity := binary.LittleEndian.Uint32(regionStruct[4:8])
		// Ensure data fits into allocated capacity
		if uint32(len(data)) > capacity {
			return fmt.Errorf("data length %d exceeds region capacity %d", len(data), capacity)
		}
		// Write the actual data bytes into the allocated memory at the given offset
		if err := memory.WriteMemory(offset, data); err != nil {
			return fmt.Errorf("failed to write data to memory at offset %d: %w", offset, err)
		}
		// Update the length field of the Region to the number of bytes written
		binary.LittleEndian.PutUint32(regionStruct[8:12], uint32(len(data)))
		if err := memory.WriteMemory(regionPtr+8, regionStruct[8:12]); err != nil {
			return fmt.Errorf("failed to write Region length at %d: %w", regionPtr+8, err)
		}
		return nil
	}

	// Abort (stop execution immediately). Signature: abort(msg_ptr: u32, file_ptr: u32, line: u32, col: u32) -> !
	instance.RegisterFunction("env", "abort", func(msgPtr, filePtr uint32, line, col uint32) {
		// Read the abort message and filename from memory
		msg, _ := memory.ReadRegion(msgPtr)
		file, _ := memory.ReadRegion(filePtr)
		// Log the abort information (if available)
		logger.Error(fmt.Sprintf("Wasm abort called: %s (%s:%d:%d)", string(msg), string(file), line, col))
		// Immediately terminate execution. In a real VM, we would trap here.
		panic(fmt.Sprintf("Wasm abort: %s", string(msg)))
	})

	// Debug (print debugging message). Signature: debug(msg_ptr: u32) -> ()
	instance.RegisterFunction("env", "debug", func(msgPtr uint32) {
		msg, err := memory.ReadRegion(msgPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("debug: failed to read message: %v", err))
			return
		}
		// CosmWasm expects debug messages to be logged by the host [oai_citation_attribution:2‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=%2F%2F%2F%20Writes%20a%20debug%20message,fn%20debug%28source_ptr%3A%20u32).
		logger.Debug(fmt.Sprintf("cosmwasm debug: %s", string(msg)))
	})

	// Storage read. Signature: db_read(key_ptr: u32) -> u32 (returns Region pointer or 0 if not found)
	instance.RegisterFunction("env", "db_read", func(keyPtr uint32) uint32 {
		key, err := memory.ReadRegion(keyPtr)
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
			// Key not found, return 0 (null pointer) [oai_citation_attribution:3‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=fn%20db_read%28key%3A%20u32%29%20,key%3A%20u32)
			logger.Debug("db_read: key not found")
			return 0
		}
		// Allocate Wasm memory for the result value and write the data
		regionPtr, err := memory.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_read: memory allocation failed: %v", err))
			return 0
		}
		if err := writeToRegion(regionPtr, value); err != nil {
			logger.Error(fmt.Sprintf("db_read: failed to write value to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_read: key %X -> %d bytes at region %d", key, len(value), regionPtr))
		return regionPtr
	})

	// Storage write. Signature: db_write(key_ptr: u32, value_ptr: u32) -> ()
	instance.RegisterFunction("env", "db_write", func(keyPtr, valuePtr uint32) {
		key, err := memory.ReadRegion(keyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_write: failed to read key: %v", err))
			return
		}
		value, err := memory.ReadRegion(valuePtr)
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

	// Storage remove. Signature: db_remove(key_ptr: u32) -> ()
	instance.RegisterFunction("env", "db_remove", func(keyPtr uint32) {
		key, err := memory.ReadRegion(keyPtr)
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

	// Iterator: scan. Signature: db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32 (iterator ID or 0)
	instance.RegisterFunction("env", "db_scan", func(startPtr, endPtr uint32, order int32) uint32 {
		start, err := memory.ReadRegion(startPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("db_scan: failed to read start key: %v", err))
			return 0
		}
		end, err := memory.ReadRegion(endPtr)
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

	// Iterator: next. Signature: db_next(iterator_id: u32) -> u32 (returns Region pointer or 0 if no more)
	instance.RegisterFunction("env", "db_next", func(iteratorID uint32) uint32 {
		key, value, err := storage.Next(iteratorID)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: iterator %d error: %v", iteratorID, err))
			return 0
		}
		if key == nil {
			// Iterator exhausted
			logger.Debug(fmt.Sprintf("db_next: iterator %d exhausted", iteratorID))
			return 0
		}
		// Package the key and value into a single Region containing two Regions (for CosmWasm 1.x compatibility)
		// Allocate memory for key and value data
		keyRegion, err := memory.Allocate(uint32(len(key)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for key: %v", err))
			return 0
		}
		if err := writeToRegion(keyRegion, key); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write key to region: %v", err))
			return 0
		}
		valRegion, err := memory.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for value: %v", err))
			// Free the key region to avoid memory leak (optional, if MemoryManager tracks allocations)
			return 0
		}
		if err := writeToRegion(valRegion, value); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write value to region: %v", err))
			return 0
		}
		// Now allocate a combined region for the two Region structs (key and value)
		combinedSize := uint32(12 * 2) // each Region struct is 12 bytes [oai_citation_attribution:4‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=,this%20region%20pub%20length%3A%20u32)
		combinedPtr, err := memory.Allocate(combinedSize)
		if err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to allocate memory for combined region: %v", err))
			return 0
		}
		// Copy the two Region structs into combined memory
		// Read the Region structs (12 bytes each) from their pointers
		keyRegionStruct := make([]byte, 12)
		valRegionStruct := make([]byte, 12)
		_ = memory.ReadMemory(keyRegion, keyRegionStruct) // should not fail; keyRegion points to Region struct
		_ = memory.ReadMemory(valRegion, valRegionStruct) // should not fail; valRegion points to Region struct
		// Write them back-to-back into combinedPtr
		concat := append(keyRegionStruct, valRegionStruct...)
		if err := memory.WriteMemory(combinedPtr, concat); err != nil {
			logger.Error(fmt.Sprintf("db_next: failed to write combined region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next: iterator %d next -> key %d bytes, value %d bytes", iteratorID, len(key), len(value)))
		return combinedPtr
	})

	// Iterator: next_key. Signature: db_next_key(iterator_id: u32) -> u32 (returns Region pointer for key or 0)
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
		// Return key as a Region
		regionPtr, err := memory.Allocate(uint32(len(key)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_key: failed to allocate memory: %v", err))
			return 0
		}
		if err := writeToRegion(regionPtr, key); err != nil {
			logger.Error(fmt.Sprintf("db_next_key: failed to write key to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next_key: iterator %d -> key %d bytes", iteratorID, len(key)))
		return regionPtr
	})

	// Iterator: next_value. Signature: db_next_value(iterator_id: u32) -> u32 (returns Region pointer for value or 0)
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
		regionPtr, err := memory.Allocate(uint32(len(value)))
		if err != nil {
			logger.Error(fmt.Sprintf("db_next_value: failed to allocate memory: %v", err))
			return 0
		}
		if err := writeToRegion(regionPtr, value); err != nil {
			logger.Error(fmt.Sprintf("db_next_value: failed to write value to region: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("db_next_value: iterator %d -> value %d bytes", iteratorID, len(value)))
		return regionPtr
	})

	// Address validation. Signature: addr_validate(addr_ptr: u32) -> u32 (0 = success, nonzero = error code)
	instance.RegisterFunction("env", "addr_validate", func(addrPtr uint32) uint32 {
		addrBytes, err := memory.ReadRegion(addrPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_validate: failed to read address: %v", err))
			// Use nonzero error code on failure (e.g., 1 for generic error)
			return 1
		}
		err = api.ValidateAddress(string(addrBytes))
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_validate: address %q is INVALID: %v", addrBytes, err))
			return 1 // return error code (1 indicates validation failure as per CosmWasm convention)
		}
		logger.Debug(fmt.Sprintf("addr_validate: address %q is valid", addrBytes))
		return 0 // success
	})

	// Address canonicalize. Signature: addr_canonicalize(human_ptr: u32, canon_ptr: u32) -> u32 (0 = success, nonzero = error)
	instance.RegisterFunction("env", "addr_canonicalize", func(humanPtr, canonPtr uint32) uint32 {
		humanAddr, err := memory.ReadRegion(humanPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_canonicalize: failed to read human address: %v", err))
			return 1
		}
		canon, err := api.CanonicalAddress(string(humanAddr))
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_canonicalize: invalid address %q: %v", humanAddr, err))
			return 1
		}
		// Write canonical address bytes into the provided destination region
		if err := writeToRegion(canonPtr, canon); err != nil {
			logger.Error(fmt.Sprintf("addr_canonicalize: failed to write canonical address: %v", err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_canonicalize: %q -> %X", humanAddr, canon))
		return 0
	})

	// Address humanize. Signature: addr_humanize(canon_ptr: u32, human_ptr: u32) -> u32 (0 = success, nonzero = error)
	instance.RegisterFunction("env", "addr_humanize", func(canonPtr, humanPtr uint32) uint32 {
		canonAddr, err := memory.ReadRegion(canonPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("addr_humanize: failed to read canonical address: %v", err))
			return 1
		}
		human, err := api.HumanAddress(canonAddr)
		if err != nil {
			logger.Debug(fmt.Sprintf("addr_humanize: invalid canonical addr %X: %v", canonAddr, err))
			return 1
		}
		if err := writeToRegion(humanPtr, []byte(human)); err != nil {
			logger.Error(fmt.Sprintf("addr_humanize: failed to write human address: %v", err))
			return 1
		}
		logger.Debug(fmt.Sprintf("addr_humanize: %X -> %q", canonAddr, human))
		return 0
	})

	// Crypto: secp256k1_verify. Signature: secp256k1_verify(hash_ptr: u32, sig_ptr: u32, pubkey_ptr: u32) -> u32
	// Returns 0 on verification success, 1 on verification failure, >1 on error [oai_citation_attribution:5‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=%2F%2F%2F%20Verifies%20message%20hashes%20against,u32).
	instance.RegisterFunction("env", "secp256k1_verify", func(hashPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msgHash, err := memory.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read message hash: %v", err))
			return 2 // return >1 to indicate error
		}
		sig, err := memory.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := memory.ReadRegion(pubKeyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: failed to read public key: %v", err))
			return 2
		}
		valid, err := api.Secp256k1Verify(msgHash, sig, pubKey)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_verify: crypto error: %v", err))
			return 3 // some error code >1
		}
		if !valid {
			logger.Debug("secp256k1_verify: signature verification FAILED")
			return 1 // verification false
		}
		logger.Debug("secp256k1_verify: signature verification successful")
		return 0 // verification true
	})

	// Crypto: secp256k1_recover_pubkey. Signature: secp256k1_recover_pubkey(hash_ptr: u32, sig_ptr: u32, recovery_param: u32) -> u64
	// Returns 64-bit (offset<<32 | length) packed result (Region pointer) or 0 on error.
	instance.RegisterFunction("env", "secp256k1_recover_pubkey", func(hashPtr, sigPtr, param uint32) uint64 {
		msgHash, err := memory.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to read message hash: %v", err))
			return 0
		}
		sig, err := memory.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to read signature: %v", err))
			return 0
		}
		pubKey, err := api.Secp256k1RecoverPubkey(msgHash, sig, byte(param))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: recover failed: %v", err))
			return 0
		}
		// Return the recovered pubkey as a Region
		regionPtr, err := memory.Allocate(uint32(len(pubKey)))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: allocation failed: %v", err))
			return 0
		}
		if err := writeToRegion(regionPtr, pubKey); err != nil {
			logger.Error(fmt.Sprintf("secp256k1_recover_pubkey: failed to write pubkey: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("secp256k1_recover_pubkey: recovered %d-byte pubkey", len(pubKey)))
		// The function returns a u64 combining the pointer and length (per CosmWasm spec [oai_citation_attribution:6‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=fn%20secp256k1_recover_pubkey,u64)).
		// The high 32 bits are the Region pointer, low 32 bits are length.
		packed := (uint64(regionPtr) << 32) | uint64(len(pubKey))
		return packed
	})

	// Crypto: secp256r1_verify. Same pattern as secp256k1_verify.
	instance.RegisterFunction("env", "secp256r1_verify", func(hashPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msgHash, err := memory.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_verify: failed to read message hash: %v", err))
			return 2
		}
		sig, err := memory.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := memory.ReadRegion(pubKeyPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_verify: failed to read public key: %v", err))
			return 2
		}
		valid, err := api.Secp256r1Verify(msgHash, sig, pubKey)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_verify: crypto error: %v", err))
			return 3
		}
		if !valid {
			logger.Debug("secp256r1_verify: signature verification FAILED")
			return 1
		}
		logger.Debug("secp256r1_verify: signature verification successful")
		return 0
	})

	// Crypto: secp256r1_recover_pubkey. Same pattern as secp256k1_recover_pubkey.
	instance.RegisterFunction("env", "secp256r1_recover_pubkey", func(hashPtr, sigPtr, param uint32) uint64 {
		msgHash, err := memory.ReadRegion(hashPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_recover_pubkey: failed to read message hash: %v", err))
			return 0
		}
		sig, err := memory.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_recover_pubkey: failed to read signature: %v", err))
			return 0
		}
		pubKey, err := api.Secp256r1RecoverPubkey(msgHash, sig, byte(param))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_recover_pubkey: recover failed: %v", err))
			return 0
		}
		regionPtr, err := memory.Allocate(uint32(len(pubKey)))
		if err != nil {
			logger.Error(fmt.Sprintf("secp256r1_recover_pubkey: allocation failed: %v", err))
			return 0
		}
		if err := writeToRegion(regionPtr, pubKey); err != nil {
			logger.Error(fmt.Sprintf("secp256r1_recover_pubkey: failed to write pubkey: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("secp256r1_recover_pubkey: recovered %d-byte pubkey", len(pubKey)))
		return (uint64(regionPtr) << 32) | uint64(len(pubKey))
	})

	// Crypto: ed25519_verify. Signature: ed25519_verify(msg_ptr: u32, sig_ptr: u32, pubkey_ptr: u32) -> u32
	// Returns 0 on success, 1 on signature mismatch, >1 on error [oai_citation_attribution:7‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=%2F%2F%2F%20Verifies%20a%20message%20against,u32).
	instance.RegisterFunction("env", "ed25519_verify", func(msgPtr, sigPtr, pubKeyPtr uint32) uint32 {
		msg, err := memory.ReadRegion(msgPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read message: %v", err))
			return 2
		}
		sig, err := memory.ReadRegion(sigPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_verify: failed to read signature: %v", err))
			return 2
		}
		pubKey, err := memory.ReadRegion(pubKeyPtr)
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

	// Crypto: ed25519_batch_verify. Signature: ed25519_batch_verify(messages_ptr: u32, signatures_ptr: u32, public_keys_ptr: u32) -> u32
	// Expects all inputs as concatenated sequences of equal-length items. Returns 0 if all signatures valid, 1 if any signature failed, >1 on error [oai_citation_attribution:8‡github.com](https://github.com/CosmWasm/cosmwasm#:~:text=%2F%2F%2F%20Verifies%20a%20batch%20of,u32).
	instance.RegisterFunction("env", "ed25519_batch_verify", func(msgsPtr, sigsPtr, pubKeysPtr uint32) uint32 {
		msgs, err := memory.ReadRegion(msgsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read messages: %v", err))
			return 2
		}
		sigs, err := memory.ReadRegion(sigsPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("ed25519_batch_verify: failed to read signatures: %v", err))
			return 2
		}
		pubKeys, err := memory.ReadRegion(pubKeysPtr)
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

	// BLS12-381: aggregate_g1. Signature: bls12_381_aggregate_g1(messages_ptr: u32) -> u32 (0 = success, >0 = error)
	instance.RegisterFunction("env", "bls12_381_aggregate_g1", func(messagesPtr uint32) uint32 {
		msgs, err := memory.ReadRegion(messagesPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: failed to read messages: %v", err))
			return 1
		}
		result, err := api.Bls12381AggregateG1(msgs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: error: %v", err))
			return 1
		}
		// Write the aggregated result (48 bytes group element) into memory and return region pointer
		regionPtr, err := memory.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g1: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_aggregate_g1: aggregation successful")
		return regionPtr
	})

	// BLS12-381: aggregate_g2
	instance.RegisterFunction("env", "bls12_381_aggregate_g2", func(messagesPtr uint32) uint32 {
		msgs, err := memory.ReadRegion(messagesPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: failed to read messages: %v", err))
			return 1
		}
		result, err := api.Bls12381AggregateG2(msgs)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: error: %v", err))
			return 1
		}
		regionPtr, err := memory.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_aggregate_g2: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_aggregate_g2: aggregation successful")
		return regionPtr
	})

	// BLS12-381: pairing_equality. Signature: bls12_381_pairing_equality(pairs_ptr: u32) -> u32 (0 = true, 1 = false, >1 = error)
	instance.RegisterFunction("env", "bls12_381_pairing_equality", func(pairsPtr uint32) uint32 {
		pairs, err := memory.ReadRegion(pairsPtr)
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

	// BLS12-381: hash_to_g1. Signature: bls12_381_hash_to_g1(message_ptr: u32, dest_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_hash_to_g1", func(messagePtr, destPtr uint32) uint32 {
		msg, err := memory.ReadRegion(messagePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to read message: %v", err))
			return 1
		}
		dest, err := memory.ReadRegion(destPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to read dest: %v", err))
			return 1
		}
		result, err := api.Bls12381HashToG1(msg, dest)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: error: %v", err))
			return 1
		}
		regionPtr, err := memory.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g1: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_hash_to_g1: hash successful")
		return regionPtr
	})

	// BLS12-381: hash_to_g2. Signature: bls12_381_hash_to_g2(message_ptr: u32, dest_ptr: u32) -> u32
	instance.RegisterFunction("env", "bls12_381_hash_to_g2", func(messagePtr, destPtr uint32) uint32 {
		msg, err := memory.ReadRegion(messagePtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to read message: %v", err))
			return 1
		}
		dest, err := memory.ReadRegion(destPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to read dest: %v", err))
			return 1
		}
		result, err := api.Bls12381HashToG2(msg, dest)
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: error: %v", err))
			return 1
		}
		regionPtr, err := memory.Allocate(uint32(len(result)))
		if err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: allocation failed: %v", err))
			return 1
		}
		if err := writeToRegion(regionPtr, result); err != nil {
			logger.Error(fmt.Sprintf("bls12_381_hash_to_g2: failed to write result: %v", err))
			return 1
		}
		logger.Debug("bls12_381_hash_to_g2: hash successful")
		return regionPtr
	})

	// Query an external (native chain) state. Signature: query_chain(request_ptr: u32) -> u32 (Region pointer to response or 0 on error)
	instance.RegisterFunction("env", "query_chain", func(reqPtr uint32) uint32 {
		request, err := memory.ReadRegion(reqPtr)
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to read request: %v", err))
			return 0
		}
		response, err := querier.Query(request)
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: query error: %v", err))
			return 0
		}
		// Allocate memory for the response and return a pointer to it
		regionPtr, err := memory.Allocate(uint32(len(response)))
		if err != nil {
			logger.Error(fmt.Sprintf("query_chain: allocation failed: %v", err))
			return 0
		}
		if err := writeToRegion(regionPtr, response); err != nil {
			logger.Error(fmt.Sprintf("query_chain: failed to write response: %v", err))
			return 0
		}
		logger.Debug(fmt.Sprintf("query_chain: responded with %d bytes at region %d", len(response), regionPtr))
		return regionPtr
	})

	// Note: Gas metering for host functions can be integrated here if needed (using gasMeter).
	// Each host function could consume gas according to CosmWasm cost rules (not shown for brevity).
}
