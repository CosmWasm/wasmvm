package runtime

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// RegisterHostFunctions registers all host functions with the wazero runtime
func RegisterHostFunctions(runtime wazero.Runtime, env *RuntimeEnvironment) (wazero.CompiledModule, error) {
	builder := runtime.NewHostModuleBuilder("env")

	// Register abort function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, code uint32) {
			ctx = context.WithValue(ctx, envKey, env)
			hostAbort(ctx, m, code)
		}).
		WithParameterNames("code").
		Export("abort")

	// Register DB functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen uint32) (uint32, uint32) {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge gas for read operation (1 gas per byte read)
			env.gasUsed += uint64(keyLen)
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			return hostGet(ctx, m, keyPtr, keyLen)
		}).
		WithParameterNames("key_ptr", "key_len").
		Export("db_get")

	// Register query_chain with i32_i32 signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, reqPtr uint32) uint32 {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Read request from memory to calculate gas
			mem := m.Memory()
			req, err := readMemory(mem, reqPtr, 4) // Read length prefix first
			if err != nil {
				panic(fmt.Sprintf("failed to read request length: %v", err))
			}
			reqLen := binary.LittleEndian.Uint32(req)

			// Charge gas for query operation (10 gas per byte queried)
			env.gasUsed += uint64(reqLen) * 10
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			return hostQueryChain(ctx, m, reqPtr)
		}).
		WithParameterNames("request").
		WithResultNames("result").
		Export("query_chain")

		// Missing critical host functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module) uint32 {
			return hostGetAllocation(ctx, m)
		}).
		Export("get_allocation")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr uint32) {
			hostDeallocate(ctx, m, ptr)
		}).
		WithParameterNames("ptr").
		Export("deallocate")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge gas for write operation (2 gas per byte written)
			env.gasUsed += uint64(keyLen+valLen) * 2
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			hostSet(ctx, m, keyPtr, keyLen, valPtr, valLen)
		}).
		WithParameterNames("key_ptr", "key_len", "val_ptr", "val_len").
		Export("db_set")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge gas for write operation (2 gas per byte written)
			env.gasUsed += uint64(keyLen+valLen) * 2
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			hostSet(ctx, m, keyPtr, keyLen, valPtr, valLen)
		}).
		WithParameterNames("key_ptr", "key_len", "val_ptr", "val_len").
		Export("db_write")

	// Register interface_version_8 function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module) {
			// This is just a marker function that doesn't need to do anything
		}).
		Export("interface_version_8")

		// Register allocate function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, size uint32) uint32 {
			// We will ignore the `size` for now and just return a fixed offset
			// to show a minimal approach. For a real approach, see notes in the explanation.

			// Charge gas for allocation (1 gas per 1KB, minimum 1 gas)
			gasCharge := (size + 1023) / 1024 // Round up to nearest KB
			if gasCharge == 0 {
				gasCharge = 1
			}
			env.gasUsed += uint64(gasCharge)
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			// Allocate memory in the Wasm module
			memory := m.Memory()
			if memory == nil {
				panic("no memory exported")
			}

			// Calculate required pages for the allocation
			currentBytes := memory.Size()
			requiredBytes := size
			pageSize := uint32(65536) // 64KB

			// Grow memory if needed
			if requiredBytes > currentBytes {
				pagesToGrow := (requiredBytes - currentBytes + pageSize - 1) / pageSize
				if _, ok := memory.Grow(uint32(pagesToGrow)); !ok {
					panic("failed to grow memory")
				}
			}

			// Return the pointer to the allocated memory
			ptr := currentBytes
			return ptr
		}).
		WithParameterNames("size").
		WithResultNames("ptr").
		Export("allocate")

	// Register deallocate function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr uint32) {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge minimal gas for deallocation
			env.gasUsed += 1
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}
			// In our implementation, we don't need to explicitly deallocate
			// as we rely on the Wasm runtime's memory management
		}).
		WithParameterNames("ptr").
		Export("deallocate")

	// Register BLS12-381 functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, g1sPtr, outPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, _ := hostBls12381AggregateG1(ctx, m, g1sPtr)
			return ptr
		}).
		WithParameterNames("g1s_ptr", "out_ptr").
		WithResultNames("result").
		Export("bls12_381_aggregate_g1")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, g2sPtr, outPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, _ := hostBls12381AggregateG2(ctx, m, g2sPtr)
			return ptr
		}).
		WithParameterNames("g2s_ptr", "out_ptr").
		WithResultNames("result").
		Export("bls12_381_aggregate_g2")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, psPtr, qsPtr, rPtr, sPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostBls12381PairingEquality(ctx, m, psPtr, 0, qsPtr, 0, rPtr, 0, sPtr, 0)
		}).
		WithParameterNames("ps_ptr", "qs_ptr", "r_ptr", "s_ptr").
		WithResultNames("result").
		Export("bls12_381_pairing_equality")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashFunction, msgPtr, dstPtr, outPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, _ := hostBls12381HashToG1(ctx, m, msgPtr, hashFunction)
			return ptr
		}).
		WithParameterNames("hash_function", "msg_ptr", "dst_ptr", "out_ptr").
		WithResultNames("result").
		Export("bls12_381_hash_to_g1")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hashFunction, msgPtr, dstPtr, outPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, _ := hostBls12381HashToG2(ctx, m, msgPtr, hashFunction)
			return ptr
		}).
		WithParameterNames("hash_function", "msg_ptr", "dst_ptr", "out_ptr").
		WithResultNames("result").
		Export("bls12_381_hash_to_g2")

	// SECP256r1 functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, messageHashPtr, signaturePtr, publicKeyPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256r1Verify(ctx, m, messageHashPtr, 0, signaturePtr, 0, publicKeyPtr, 0)
		}).
		WithParameterNames("message_hash_ptr", "signature_ptr", "public_key_ptr").
		WithResultNames("result").
		Export("secp256r1_verify")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, messageHashPtr, signaturePtr, recoveryParam uint32) uint64 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, len := hostSecp256r1RecoverPubkey(ctx, m, messageHashPtr, 0, signaturePtr, 0, recoveryParam)
			return (uint64(len) << 32) | uint64(ptr)
		}).
		WithParameterNames("message_hash_ptr", "signature_ptr", "recovery_param").
		WithResultNames("result").
		Export("secp256r1_recover_pubkey")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, startPtr, startLen, order uint32) uint32 {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge gas for scan operation (gasCostIteratorCreate + 1 gas per byte scanned)
			env.gasUsed += gasCostIteratorCreate + uint64(startLen)
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			return hostScan(ctx, m, startPtr, startLen, order)
		}).
		WithParameterNames("start_ptr", "start_len", "order").
		WithResultNames("iter_id").
		Export("db_scan")

	// db_next
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge gas for next operation
			env.gasUsed += gasCostIteratorNext
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			return hostNext(ctx, m, iterID)
		}).
		WithParameterNames("iter_id").
		WithResultNames("kv_region_ptr").
		Export("db_next")

	// db_next_value
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			// Get environment from context
			env := ctx.Value(envKey).(*RuntimeEnvironment)

			// Charge gas for next value operation
			env.gasUsed += gasCostIteratorNext
			if env.gasUsed > env.Gas.GasConsumed() {
				panic("out of gas")
			}

			// Extract call_id and iter_id from the packed uint32
			callID := uint64(iterID >> 16)
			actualIterID := uint64(iterID & 0xFFFF)
			ptr, _, _ := hostNextValue(ctx, m, callID, actualIterID)
			return ptr
		}).
		WithParameterNames("iter_id").
		WithResultNames("value_ptr").
		Export("db_next_value")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostHumanizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("result").
		Export("addr_humanize")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostValidateAddress(ctx, m, addrPtr)
		}).
		WithParameterNames("addr_ptr").
		WithResultNames("result").
		Export("addr_validate")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, addrPtr, addrLen uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostCanonicalizeAddress(ctx, m, addrPtr, addrLen)
		}).
		WithParameterNames("addr_ptr", "addr_len").
		WithResultNames("result").
		Export("addr_canonicalize")

	// Register Query functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, reqPtr, reqLen, gasLimit uint32) (uint32, uint32) {
			ctx = context.WithValue(ctx, envKey, env)
			return hostQueryExternal(ctx, m, reqPtr, reqLen, gasLimit)
		}).
		WithParameterNames("req_ptr", "req_len", "gas_limit").
		WithResultNames("res_ptr", "res_len").
		Export("querier_query")

	// Register secp256k1_verify function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hash_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256k1Verify(ctx, m, hash_ptr, sig_ptr, pubkey_ptr)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "pubkey_ptr").
		WithResultNames("result").
		Export("secp256k1_verify")

	// Register DB read/write/remove functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostDbRead(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_read")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr, valuePtr uint32) {
			ctx = context.WithValue(ctx, envKey, env)
			hostDbWrite(ctx, m, keyPtr, valuePtr)
		}).
		WithParameterNames("key_ptr", "value_ptr").
		Export("db_write")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, keyPtr uint32) {
			ctx = context.WithValue(ctx, envKey, env)
			hostDbRemove(ctx, m, keyPtr)
		}).
		WithParameterNames("key_ptr").
		Export("db_remove")

	// db_close_iterator
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, callID, iterID uint64) {
			ctx = context.WithValue(ctx, envKey, env)
			hostCloseIterator(ctx, m, callID, iterID)
		}).
		WithParameterNames("call_id", "iter_id").
		Export("db_close_iterator")

	// Register secp256k1_recover_pubkey function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, hash_ptr, sig_ptr, rec_id uint32) uint64 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostSecp256k1RecoverPubkey(ctx, m, hash_ptr, sig_ptr, rec_id)
		}).
		WithParameterNames("hash_ptr", "sig_ptr", "rec_id").
		WithResultNames("result").
		Export("secp256k1_recover_pubkey")

	// Register ed25519_verify function with i32i32i32_i32 signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msg_ptr, sig_ptr, pubkey_ptr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostEd25519Verify(ctx, m, msg_ptr, sig_ptr, pubkey_ptr)
		}).
		WithParameterNames("msg_ptr", "sig_ptr", "pubkey_ptr").
		WithResultNames("result").
		Export("ed25519_verify")

	// Register ed25519_batch_verify function with i32i32i32_i32 signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgs_ptr, sigs_ptr, pubkeys_ptr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			return hostEd25519BatchVerify(ctx, m, msgs_ptr, sigs_ptr, pubkeys_ptr)
		}).
		WithParameterNames("msgs_ptr", "sigs_ptr", "pubkeys_ptr").
		WithResultNames("result").
		Export("ed25519_batch_verify")

	// Register debug function with i32_v signature
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, msgPtr uint32) {
			ctx = context.WithValue(ctx, envKey, env)
			hostDebug(ctx, m, msgPtr)
		}).
		WithParameterNames("msg_ptr").
		Export("debug")

	// db_next_key
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, iterID uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, _, _ := hostNextKey(ctx, m, uint64(iterID), 0)
			return ptr
		}).
		WithParameterNames("iter_id").
		WithResultNames("key_ptr").
		Export("db_next_key")

	return builder.Compile(context.Background())
}
