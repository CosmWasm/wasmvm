package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// RegisterHostFunctions registers all required host functions with the wazero runtime.
// It provides detailed logging and ensures all CosmWasm‐required functions are implemented.
func RegisterHostFunctions(runtime wazero.Runtime, env *RuntimeEnvironment) (wazero.CompiledModule, error) {
	// Create a builder for the "env" module.
	builder := runtime.NewHostModuleBuilder("env")

	// Register Memory Management Functions.
	registerMemoryFunctions(builder, env)

	// Register Debug Functions.
	registerDebugFunctions(builder, env)

	// Register Storage Functions.
	registerStorageFunctions(builder, env)

	// Register Iterator Functions.
	registerIteratorFunctions(builder, env)

	// Register Address Functions.
	registerAddressFunctions(builder, env)

	// Register Query Functions.
	registerQueryFunctions(builder, env)

	// Register Crypto Functions.
	registerCryptoFunctions(builder, env)
	// (Note: registerCryptoFunctions no longer calls registerIteratorFunctions.)

	// Compile and return the module.
	return builder.Compile(context.Background())
}

// --- Registration Helpers ---

// registerCryptoFunctions registers cryptographic functions like secp256k1_verify and ed25519_verify.
func registerCryptoFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// secp256k1 Verify Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, pubkeyPtr uint32) uint32 {
			fmt.Printf("Called secp256k1_verify(hash_ptr=0x%x, sig_ptr=0x%x, pubkey_ptr=0x%x)\n", hashPtr, sigPtr, pubkeyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256k1Verify(ctx, m, hashPtr, sigPtr, pubkeyPtr)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "pubkey_ptr").
		Export("secp256k1_verify")

	// secp256k1 Recover Pubkey Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, recID uint32) uint64 {
			fmt.Printf("Called secp256k1_recover_pubkey(hash_ptr=0x%x, sig_ptr=0x%x, rec_id=%d)\n", hashPtr, sigPtr, recID)
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256k1RecoverPubkey(ctx, m, hashPtr, sigPtr, recID)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "rec_id").
		Export("secp256k1_recover_pubkey")

	// ed25519 Verify Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr, sigPtr, pubkeyPtr uint32) uint32 {
			fmt.Printf("Called ed25519_verify(msg_ptr=0x%x, sig_ptr=0x%x, pubkey_ptr=0x%x)\n", msgPtr, sigPtr, pubkeyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostEd25519Verify(ctx, m, msgPtr, sigPtr, pubkeyPtr)
		}).
		WithParameterNames("msg_ptr", "sig_ptr", "pubkey_ptr").
		Export("ed25519_verify")

	// ed25519 Batch Verify Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgsPtr, sigsPtr, pubkeysPtr uint32) uint32 {
			fmt.Printf("Called ed25519_batch_verify(msgs_ptr=0x%x, sigs_ptr=0x%x, pubkeys_ptr=0x%x)\n", msgsPtr, sigsPtr, pubkeysPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostEd25519BatchVerify(ctx, m, msgsPtr, sigsPtr, pubkeysPtr)
		}).
		WithParameterNames("msgs_ptr", "sigs_ptr", "pubkeys_ptr").
		Export("ed25519_batch_verify")

	// BLS12-381 Aggregate G1 Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr1, ptr2 uint32) uint32 {
			fmt.Printf("Called bls12_381_aggregate_g1(ptr1=0x%x, ptr2=0x%x)\n", ptr1, ptr2)
			ctx = context.WithValue(ctx, envKey, env)
			return hostBls12381AggregateG1(ctx, m, ptr1, ptr2)
		}).
		WithParameterNames("ptr1", "ptr2").
		Export("bls12_381_aggregate_g1")

	// BLS12-381 Aggregate G2 Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr1, ptr2 uint32) uint32 {
			fmt.Printf("Called bls12_381_aggregate_g2(ptr1=0x%x, ptr2=0x%x)\n", ptr1, ptr2)
			ctx = context.WithValue(ctx, envKey, env)
			return hostBls12381AggregateG2(ctx, m, ptr1, ptr2)
		}).
		WithParameterNames("ptr1", "ptr2").
		Export("bls12_381_aggregate_g2")

	// BLS12-381 Pairing Equality Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, a1Ptr, a2Ptr, b1Ptr, b2Ptr uint32) uint32 {
			fmt.Printf("Called bls12_381_pairing_equality(a1_ptr=0x%x, a2_ptr=0x%x, b1_ptr=0x%x, b2_ptr=0x%x)\n",
				a1Ptr, a2Ptr, b1Ptr, b2Ptr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostBls12381PairingEquality(ctx, m, a1Ptr, a2Ptr, 0, 0, b1Ptr, b2Ptr, 0, 0)
		}).
		WithParameterNames("a1_ptr", "a2_ptr", "b1_ptr", "b2_ptr").
		Export("bls12_381_pairing_equality")

	// BLS12-381 Hash to G1 Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
			fmt.Printf("Called bls12_381_hash_to_g1(hash_ptr=0x%x, hash_len=%d, dst_ptr=0x%x, dst_len=%d)\n",
				hashPtr, hashLen, dstPtr, dstLen)
			ctx = context.WithValue(ctx, envKey, env)
			return hostBls12381HashToG1(ctx, m, hashPtr, hashLen, dstPtr, dstLen)
		}).
		WithParameterNames("hash_ptr", "hash_len", "dst_ptr", "dst_len").
		Export("bls12_381_hash_to_g1")

	// BLS12-381 Hash to G2 Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, hashLen, dstPtr, dstLen uint32) uint32 {
			fmt.Printf("Called bls12_381_hash_to_g2(hash_ptr=0x%x, hash_len=%d, dst_ptr=0x%x, dst_len=%d)\n",
				hashPtr, hashLen, dstPtr, dstLen)
			ctx = context.WithValue(ctx, envKey, env)
			return hostBls12381HashToG2(ctx, m, hashPtr, hashLen, dstPtr, dstLen)
		}).
		WithParameterNames("hash_ptr", "hash_len", "dst_ptr", "dst_len").
		Export("bls12_381_hash_to_g2")

	// Secp256r1 Verify Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr, sigPtr, pubkeyPtr uint32) uint32 {
			fmt.Printf("Called secp256r1_verify(msg_ptr=0x%x, sig_ptr=0x%x, pubkey_ptr=0x%x)\n",
				msgPtr, sigPtr, pubkeyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256r1Verify(ctx, m, msgPtr, 32, sigPtr, 64, pubkeyPtr, 33)
		}).
		WithParameterNames("msg_ptr", "sig_ptr", "pubkey_ptr").
		Export("secp256r1_verify")

	// Secp256r1 Recover Pubkey Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashPtr, sigPtr, recoveryParam uint32) uint64 {
			fmt.Printf("Called secp256r1_recover_pubkey(hash_ptr=0x%x, sig_ptr=0x%x, recovery=%d)\n",
				hashPtr, sigPtr, recoveryParam)
			ctx = context.WithValue(ctx, envKey, env)
			resultPtr, resultLen := hostSecp256r1RecoverPubkey(ctx, m, hashPtr, 32, sigPtr, 64, recoveryParam)
			return (uint64(resultPtr) << 32) | uint64(resultLen)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "recovery").
		Export("secp256r1_recover_pubkey")
}

// registerIteratorFunctions registers iterator-related functions like db_scan, db_next, db_next_key, and db_next_value.
func registerIteratorFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// DB Scan Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, startPtr, startLen, order uint32) uint32 {
			fmt.Printf("Called db_scan(start_ptr=0x%x, start_len=%d, order=%d)\n", startPtr, startLen, order)
			ctx = context.WithValue(ctx, envKey, env)
			env.gasUsed += gasCostIteratorCreate + (uint64(startLen) * gasPerByte)
			return hostScan(ctx, m, startPtr, startLen, order)
		}).
		WithParameterNames("start_ptr", "start_len", "order").
		Export("db_scan")

	// DB Next Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			fmt.Printf("Called db_next(iter_id=%d)\n", iterID)
			ctx = context.WithValue(ctx, envKey, env)
			env.gasUsed += gasCostIteratorNext
			return hostNext(ctx, m, iterID)
		}).
		WithParameterNames("iter_id").
		Export("db_next")

	// DB Next Key Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			fmt.Printf("Called db_next_key(iter_id=%d)\n", iterID)
			ctx = context.WithValue(ctx, envKey, env)
			env.gasUsed += gasCostIteratorNext
			ptr, _, _ := hostNextKey(ctx, m, uint64(iterID), 0)
			return ptr
		}).
		WithParameterNames("iter_id").
		Export("db_next_key")

	// DB Next Value Function.
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
}

// registerAddressFunctions registers address-related functions like addr_validate, addr_canonicalize, and addr_humanize.
func registerAddressFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// Address Validation Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
			fmt.Printf("Called addr_validate(addr_ptr=0x%x)\n", addrPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostValidateAddress(ctx, m, addrPtr)
		}).
		WithParameterNames("addr_ptr").
		Export("addr_validate")

	// Address Canonicalization Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			fmt.Printf("Called addr_canonicalize(addr_ptr=0x%x, addr_len=%d)\n", addrPtr, addrLen)
			ctx = context.WithValue(ctx, envKey, env)
			return hostCanonicalizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		Export("addr_canonicalize")

	// Address Humanization Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			fmt.Printf("Called addr_humanize(addr_ptr=0x%x, addr_len=%d)\n", addrPtr, addrLen)
			ctx = context.WithValue(ctx, envKey, env)
			return hostHumanizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		Export("addr_humanize")
}

// registerMemoryFunctions registers memory management functions like allocate and deallocate.
func registerMemoryFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// Allocate Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, size uint32) uint32 {
			fmt.Printf("Called allocate(size=%d)\n", size)
			ctx = context.WithValue(ctx, envKey, env)
			ptr, err := allocateInContract(ctx, m, size)
			if err != nil {
				return 0
			}
			return ptr
		}).
		WithParameterNames("size").
		Export("allocate")

	// Deallocate Function (no-op).
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr uint32) {
			fmt.Printf("Called deallocate(ptr=0x%x)\n", ptr)
			// Deallocate is a no-op in our implementation.
		}).
		WithParameterNames("ptr").
		Export("deallocate")
}

// registerDebugFunctions registers debug-related functions like debug and abort.
func registerDebugFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// Debug Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr uint32) {
			fmt.Printf("Called debug(msg_ptr=0x%x)\n", msgPtr)
			ctx = context.WithValue(ctx, envKey, env)
			hostDebug(ctx, m, msgPtr)
		}).
		WithParameterNames("msg_ptr").
		Export("debug")

	// Abort Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr uint32) {
			mem := m.Memory()
			// Try to read, say, 256 bytes from msgPtr (or until a null terminator)
			msgBytes, err := readMemory(mem, msgPtr, 256)
			var msg string
			if err == nil {
				// Optionally trim at the first 0 byte:
				for i, b := range msgBytes {
					if b == 0 {
						msgBytes = msgBytes[:i]
						break
					}
				}
				msg = string(msgBytes)
			} else {
				msg = fmt.Sprintf("<error reading abort message: %v>", err)
			}
			fmt.Printf("Called abort(msg_ptr=0x%x): %s\n", msgPtr, msg)
			panic(fmt.Sprintf("contract aborted at pointer 0x%x: %s", msgPtr, msg))
		}).
		WithParameterNames("msg_ptr").
		Export("abort")
}

// registerStorageFunctions registers storage-related functions like db_read, db_write, and db_remove.
func registerStorageFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// DB Read Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) uint32 {
			fmt.Printf("Called db_read(key_ptr=0x%x)\n", keyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostDbRead(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_read")

	// DB Write Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, valuePtr uint32) {
			fmt.Printf("Called db_write(key_ptr=0x%x, value_ptr=0x%x)\n", keyPtr, valuePtr)
			ctx = context.WithValue(ctx, envKey, env)
			hostDbWrite(ctx, m, keyPtr, valuePtr)
		}).
		WithParameterNames("key_ptr", "value_ptr").
		Export("db_write")

	// DB Remove Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) {
			fmt.Printf("Called db_remove(key_ptr=0x%x)\n", keyPtr)
			ctx = context.WithValue(ctx, envKey, env)
			hostDbRemove(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_remove")
}

// registerQueryFunctions registers query-related functions like query_chain.
func registerQueryFunctions(builder wazero.HostModuleBuilder, env *RuntimeEnvironment) {
	// Query Chain Function.
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, reqPtr uint32) uint32 {
			fmt.Printf("Called query_chain(req_ptr=0x%x)\n", reqPtr)
			ctx = context.WithValue(ctx, envKey, env)
			return hostQueryChain(ctx, m, reqPtr)
		}).
		WithParameterNames("req_ptr").
		Export("query_chain")
}
