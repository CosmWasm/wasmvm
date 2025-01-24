package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// requiredHostFunctions defines the set of functions that CosmWasm contracts expect
// Source: https://github.com/CosmWasm/cosmwasm/blob/main/packages/std/src/imports.rs
var requiredHostFunctions = map[string]struct{}{
	// Memory management
	"allocate":   {},
	"deallocate": {},

	// Debug operations
	"debug": {},
	"abort": {},

	// DB operations
	"db_read":       {},
	"db_write":      {},
	"db_remove":     {},
	"db_scan":       {},
	"db_next":       {},
	"db_next_key":   {},
	"db_next_value": {},

	// Address operations
	"addr_validate":     {},
	"addr_canonicalize": {},
	"addr_humanize":     {},

	// Crypto operations
	"secp256k1_verify":           {},
	"secp256k1_recover_pubkey":   {},
	"ed25519_verify":             {},
	"ed25519_batch_verify":       {},
	"bls12_381_aggregate_g1":     {},
	"bls12_381_aggregate_g2":     {},
	"bls12_381_pairing_equality": {},
	"bls12_381_hash_to_g1":       {},
	"bls12_381_hash_to_g2":       {},
}

// RegisterHostFunctions registers all required host functions with the wazero runtime.
// It provides detailed logging of the registration process and function execution.
func RegisterHostFunctions(runtime wazero.Runtime, env *RuntimeEnvironment) (wazero.CompiledModule, error) {
	fmt.Printf("\n=== Starting Host Function Registration ===\n")
	startTime := time.Now()
	registeredFuncs := make(map[string]bool)

	// Helper function to log function registration
	logRegistration := func(name string) {
		registeredFuncs[name] = true
		fmt.Printf("Registered host function: %s\n", name)
	}

	builder := runtime.NewHostModuleBuilder("env")

	// Memory Management Functions
	fmt.Printf("\nRegistering Memory Management Functions...\n")

	// Register abort function for error handling
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, code uint32) {
			fmt.Printf("Called abort with code: 0x%x\n", code)
			ctx = context.WithValue(ctx, envKey, env)
			hostAbort(ctx, m, code)
		}).
		WithParameterNames("code").
		Export("abort")
	logRegistration("abort")

	// Allocate function - critical for contract memory management
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, size uint32) uint32 {
			fmt.Printf("Called allocate(size=%d)\n", size)
			env.gasUsed += uint64((size + 1023) / 1024) // 1 gas per 1KB, minimum 1
			memory := m.Memory()
			if memory == nil {
				panic("no memory exported")
			}
			currentBytes := memory.Size()
			pageSize := uint32(65536)
			if size > currentBytes {
				pagesToGrow := (size - currentBytes + pageSize - 1) / pageSize
				if _, ok := memory.Grow(pagesToGrow); !ok {
					panic("failed to grow memory")
				}
			}
			ptr := currentBytes
			fmt.Printf("Allocated %d bytes at ptr=0x%x\n", size, ptr)
			return ptr
		}).
		WithParameterNames("size").
		WithResultNames("ptr").
		Export("allocate")
	logRegistration("allocate")

	// Deallocate function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr uint32) {
			fmt.Printf("Called deallocate(ptr=0x%x)\n", ptr)
			env.gasUsed += 1
		}).
		WithParameterNames("ptr").
		Export("deallocate")
	logRegistration("deallocate")

	// Storage Functions
	fmt.Printf("\nRegistering Storage Functions...\n")

	// DB read operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) uint32 {
			fmt.Printf("Called db_read(key_ptr=0x%x)\n", keyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostDbRead(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_read")
	logRegistration("db_read")

	// DB write operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, valuePtr uint32) {
			fmt.Printf("Called db_write(key_ptr=0x%x, value_ptr=0x%x)\n", keyPtr, valuePtr)
			ctx = context.WithValue(ctx, envKey, env)
			hostDbWrite(ctx, m, keyPtr, valuePtr)
		}).
		WithParameterNames("key_ptr", "value_ptr").
		Export("db_write")
	logRegistration("db_write")

	// DB remove operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) {
			fmt.Printf("Called db_remove(key_ptr=0x%x)\n", keyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			hostDbRemove(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_remove")
	logRegistration("db_remove")

	// Iterator Functions
	fmt.Printf("\nRegistering Iterator Functions...\n")

	// DB scan operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, startPtr, startLen, order uint32) uint32 {
			fmt.Printf("Called db_scan(start_ptr=0x%x, start_len=%d, order=%d)\n", startPtr, startLen, order)
			env.gasUsed += gasCostIteratorCreate + (uint64(startLen) * gasPerByte)
			return hostScan(ctx, m, startPtr, startLen, order)
		}).
		WithParameterNames("start_ptr", "start_len", "order").
		Export("db_scan")
	logRegistration("db_scan")

	// DB next operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			fmt.Printf("Called db_next(iter_id=%d)\n", iterID)
			env.gasUsed += gasCostIteratorNext
			return hostNext(ctx, m, iterID)
		}).
		WithParameterNames("iter_id").
		Export("db_next")
	logRegistration("db_next")

	// DB next key operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			fmt.Printf("Called db_next_key(iter_id=%d)\n", iterID)
			env.gasUsed += gasCostIteratorNext
			ctx = context.WithValue(ctx, envKey, env)
			ptr, _, _ := hostNextKey(ctx, m, uint64(iterID), 0)
			return ptr
		}).
		WithParameterNames("iter_id").
		Export("db_next_key")
	logRegistration("db_next_key")

	// DB next value operation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			fmt.Printf("Called db_next_value(iter_id=%d)\n", iterID)
			ctx = context.WithValue(ctx, envKey, env)
			callID := uint64(iterID >> 16)
			actualIterID := uint64(iterID & 0xFFFF)
			ptr, _, _ := hostNextValue(ctx, m, callID, actualIterID)
			return ptr
		}).
		WithParameterNames("iter_id").
		Export("db_next_value")
	logRegistration("db_next_value")

	// Address Functions
	fmt.Printf("\nRegistering Address Functions...\n")

	// Address validation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
			fmt.Printf("Called addr_validate(addr_ptr=0x%x)\n", addrPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostValidateAddress(ctx, m, addrPtr)
		}).
		WithParameterNames("addr_ptr").
		Export("addr_validate")
	logRegistration("addr_validate")

	// Address canonicalization
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			fmt.Printf("Called addr_canonicalize(addr_ptr=0x%x, addr_len=%d)\n", addrPtr, addrLen)
			ctx = context.WithValue(ctx, envKey, env)
			return hostCanonicalizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		Export("addr_canonicalize")
	logRegistration("addr_canonicalize")

	// Address humanization
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			fmt.Printf("Called addr_humanize(addr_ptr=0x%x, addr_len=%d)\n", addrPtr, addrLen)
			ctx = context.WithValue(ctx, envKey, env)
			return hostHumanizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		Export("addr_humanize")
	logRegistration("addr_humanize")

	// Query Functions
	fmt.Printf("\nRegistering Query Functions...\n")

	// Query chain
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, reqPtr uint32) uint32 {
			fmt.Printf("Called query_chain(req_ptr=0x%x)\n", reqPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostQueryChain(ctx, m, reqPtr)
		}).
		WithParameterNames("req_ptr").
		Export("query_chain")
	logRegistration("query_chain")

	// Crypto Functions
	fmt.Printf("\nRegistering Crypto Functions...\n")

	// secp256k1 verification
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, pubkeyPtr uint32) uint32 {
			fmt.Printf("Called secp256k1_verify(hash_ptr=0x%x, sig_ptr=0x%x, pubkey_ptr=0x%x)\n",
				hashPtr, sigPtr, pubkeyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256k1Verify(ctx, m, hashPtr, sigPtr, pubkeyPtr)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "pubkey_ptr").
		Export("secp256k1_verify")
	logRegistration("secp256k1_verify")

	// secp256r1 (NIST P-256) verification
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, pubkeyPtr uint32) uint32 {
			fmt.Printf("Called secp256r1_verify(hash_ptr=0x%x, sig_ptr=0x%x, pubkey_ptr=0x%x)\n",
				hashPtr, sigPtr, pubkeyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			// Assuming standard sizes for NIST P-256: 32 bytes for hash, 64 bytes for signature, 33 bytes for compressed pubkey
			return hostSecp256r1Verify(ctx, m, hashPtr, 32, sigPtr, 64, pubkeyPtr, 33)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "pubkey_ptr").
		WithResultNames("result").
		Export("secp256r1_verify")
	logRegistration("secp256r1_verify")

	// secp256r1 (NIST P-256) public key recovery
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, recID uint32) uint64 {
			fmt.Printf("Called secp256r1_recover_pubkey(hash_ptr=0x%x, sig_ptr=0x%x, rec_id=%d)\n",
				hashPtr, sigPtr, recID)
			ctx = context.WithValue(ctx, envKey, env)
			// Assuming standard sizes for NIST P-256: 32 bytes for hash, 64 bytes for signature
			resultPtr, resultLen := hostSecp256r1RecoverPubkey(ctx, m, hashPtr, 32, sigPtr, 64, recID)
			// Pack the pointer and length into a uint64
			return (uint64(resultLen) << 32) | uint64(resultPtr)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "rec_id").
		WithResultNames("result").
		Export("secp256r1_recover_pubkey")
	logRegistration("secp256r1_recover_pubkey")

	// ed25519 verification
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr, sigPtr, pubkeyPtr uint32) uint32 {
			fmt.Printf("Called ed25519_verify(msg_ptr=0x%x, sig_ptr=0x%x, pubkey_ptr=0x%x)\n",
				msgPtr, sigPtr, pubkeyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostEd25519Verify(ctx, m, msgPtr, sigPtr, pubkeyPtr)
		}).
		WithParameterNames("msg_ptr", "sig_ptr", "pubkey_ptr").
		Export("ed25519_verify")
	logRegistration("ed25519_verify")

	// BLS12-381 G1 point aggregation with correct signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr1, ptr2 uint32) uint32 {
			fmt.Printf("Called bls12_381_aggregate_g1(ptr1=0x%x, ptr2=0x%x)\n", ptr1, ptr2)
			ctx = context.WithValue(ctx, envKey, env)
			mem := m.Memory()

			// Read both G1 points (48 bytes each)
			element1, err := readMemory(mem, ptr1, 48)
			if err != nil {
				panic(fmt.Sprintf("failed to read first G1 point: %v", err))
			}
			element2, err := readMemory(mem, ptr2, 48)
			if err != nil {
				panic(fmt.Sprintf("failed to read second G1 point: %v", err))
			}

			// Perform aggregation with both points
			result, err := BLS12381AggregateG1([][]byte{element1, element2})
			if err != nil {
				panic(fmt.Sprintf("failed to aggregate G1 points: %v", err))
			}

			// Allocate memory for result
			resultPtr, err := allocateInContract(ctx, m, uint32(len(result)))
			if err != nil {
				panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
			}

			// Write result
			if err := writeMemory(mem, resultPtr, result); err != nil {
				panic(fmt.Sprintf("failed to write result: %v", err))
			}

			return resultPtr
		}).
		WithParameterNames("ptr1", "ptr2").
		WithResultNames("result_ptr").
		Export("bls12_381_aggregate_g1")
	logRegistration("bls12_381_aggregate_g1")

	// BLS12-381 G2 point aggregation
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr1, ptr2 uint32) uint32 {
			fmt.Printf("Called bls12_381_aggregate_g2(ptr1=0x%x, ptr2=0x%x)\n", ptr1, ptr2)
			ctx = context.WithValue(ctx, envKey, env)
			mem := m.Memory()

			// Read both G2 points (96 bytes each)
			element1, err := readMemory(mem, ptr1, 96)
			if err != nil {
				panic(fmt.Sprintf("failed to read first G2 point: %v", err))
			}
			element2, err := readMemory(mem, ptr2, 96)
			if err != nil {
				panic(fmt.Sprintf("failed to read second G2 point: %v", err))
			}

			// Perform aggregation with both points
			result, err := BLS12381AggregateG2([][]byte{element1, element2})
			if err != nil {
				panic(fmt.Sprintf("failed to aggregate G2 points: %v", err))
			}

			// Allocate memory for result
			resultPtr, err := allocateInContract(ctx, m, uint32(len(result)))
			if err != nil {
				panic(fmt.Sprintf("failed to allocate memory for result: %v", err))
			}

			// Write result
			if err := writeMemory(mem, resultPtr, result); err != nil {
				panic(fmt.Sprintf("failed to write result: %v", err))
			}

			return resultPtr
		}).
		WithParameterNames("ptr1", "ptr2").
		WithResultNames("result_ptr").
		Export("bls12_381_aggregate_g2")
	logRegistration("bls12_381_aggregate_g2")

	// BLS12-381 pairing equality check
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, a1Ptr, a2Ptr, b1Ptr, b2Ptr uint32) uint32 {
			fmt.Printf("Called bls12_381_pairing_equality(a1_ptr=0x%x, a2_ptr=0x%x, b1_ptr=0x%x, b2_ptr=0x%x)\n",
				a1Ptr, a2Ptr, b1Ptr, b2Ptr)
			ctx = context.WithValue(ctx, envKey, env)
			// Assuming standard sizes: 48 bytes for G1 points, 96 bytes for G2 points
			return hostBls12381PairingEquality(ctx, m, a1Ptr, 48, a2Ptr, 96, b1Ptr, 48, b2Ptr, 96)
		}).
		WithParameterNames("a1_ptr", "a2_ptr", "b1_ptr", "b2_ptr").
		WithResultNames("result").
		Export("bls12_381_pairing_equality")
	logRegistration("bls12_381_pairing_equality")

	// BLS12-381 hash to G1
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, hashLen, domainPtr, domainLen uint32) uint32 {
			fmt.Printf("Called bls12_381_hash_to_g1(hash_ptr=0x%x, hash_len=%d, domain_ptr=0x%x, domain_len=%d)\n",
				hashPtr, hashLen, domainPtr, domainLen)
			ctx = context.WithValue(ctx, envKey, env)
			resultPtr, _ := hostBls12381HashToG1(ctx, m, hashPtr, hashLen)
			return resultPtr
		}).
		WithParameterNames("hash_ptr", "hash_len", "domain_ptr", "domain_len").
		WithResultNames("result_ptr").
		Export("bls12_381_hash_to_g1")
	logRegistration("bls12_381_hash_to_g1")

	// BLS12-381 hash to G2
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, hashLen, domainPtr, domainLen uint32) uint32 {
			fmt.Printf("Called bls12_381_hash_to_g2(hash_ptr=0x%x, hash_len=%d, domain_ptr=0x%x, domain_len=%d)\n",
				hashPtr, hashLen, domainPtr, domainLen)
			ctx = context.WithValue(ctx, envKey, env)
			resultPtr, _ := hostBls12381HashToG2(ctx, m, hashPtr, hashLen)
			return resultPtr
		}).
		WithParameterNames("hash_ptr", "hash_len", "domain_ptr", "domain_len").
		WithResultNames("result_ptr").
		Export("bls12_381_hash_to_g2")
	logRegistration("bls12_381_hash_to_g2")

	// Add secp256k1 public key recovery
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, recID uint32) uint64 {
			fmt.Printf("Called secp256k1_recover_pubkey(hash_ptr=0x%x, sig_ptr=0x%x, rec_id=%d)\n",
				hashPtr, sigPtr, recID)
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256k1RecoverPubkey(ctx, m, hashPtr, sigPtr, recID)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "rec_id").
		Export("secp256k1_recover_pubkey")
	logRegistration("secp256k1_recover_pubkey")

	// Add ed25519 batch verification
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgsPtr, sigsPtr, pubkeysPtr uint32) uint32 {
			fmt.Printf("Called ed25519_batch_verify(msgs_ptr=0x%x, sigs_ptr=0x%x, pubkeys_ptr=0x%x)\n",
				msgsPtr, sigsPtr, pubkeysPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostEd25519BatchVerify(ctx, m, msgsPtr, sigsPtr, pubkeysPtr)
		}).
		WithParameterNames("msgs_ptr", "sigs_ptr", "pubkeys_ptr").
		WithResultNames("result").
		Export("ed25519_batch_verify")
	logRegistration("ed25519_batch_verify")

	// Debug Functions
	fmt.Printf("\nRegistering Debug Functions...\n")

	// Debug logging
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr uint32) {
			fmt.Printf("Called debug(msg_ptr=0x%x)\n", msgPtr)
			ctx = context.WithValue(ctx, envKey, env)
			hostDebug(ctx, m, msgPtr)
		}).
		WithParameterNames("msg_ptr").
		Export("debug")
	logRegistration("debug")

	// Check for missing required functions
	fmt.Printf("\n=== Checking Required Host Functions ===\n")
	var missing []string
	for required := range requiredHostFunctions {
		if !registeredFuncs[required] {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		fmt.Printf("WARNING: Missing %d required host functions:\n", len(missing))
		for _, name := range missing {
			fmt.Printf("  - %s\n", name)
		}
	}

	// Registration summary
	fmt.Printf("\n=== Registration Summary ===\n")
	fmt.Printf("Registered %d host functions in %v\n", len(registeredFuncs), time.Since(startTime))
	fmt.Printf("Memory model: wazero with 64KB pages\n")
	fmt.Printf("Gas metering: enabled\n")
	fmt.Printf("===================================\n\n")

	// Compile and return the module
	return builder.Compile(context.Background())
}

// Helper function to debug memory contents
func DebugMemory(mem api.Memory, ptr uint32, size uint32) {
	if data, ok := mem.Read(ptr, size); ok {
		fmt.Printf("Memory dump at ptr=0x%x (size=%d):\n", ptr, size)
		fmt.Printf("Hex: %x\n", data)
		fmt.Printf("ASCII: %s\n", strings.Map(func(r rune) rune {
			if r >= 32 && r <= 126 {
				return r
			}
			return '.'
		}, string(data)))
	} else {
		fmt.Printf("Failed to read memory at ptr=0x%x size=%d\n", ptr, size)
	}
}

func mustReadMemoryString(m api.Module, ptr uint32) string {
	mem := m.Memory()
	data, ok := mem.Read(ptr, 1024) // Read up to 1024 bytes
	if !ok {
		return ""
	}
	// Find null terminator
	length := 0
	for length < len(data) && data[length] != 0 {
		length++
	}
	return string(data[:length])
}
