package ffi

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	bindingsOnce sync.Once

	// Cache management
	init_cache         func(config uintptr, errOut uintptr) uintptr
	store_code         func(cache uintptr, wasm uintptr, checked bool, persist bool, errOut uintptr) uintptr
	remove_wasm        func(cache uintptr, checksum uintptr, errOut uintptr)
	load_wasm          func(cache uintptr, checksum uintptr, errOut uintptr) uintptr
	pin                func(cache uintptr, checksum uintptr, errOut uintptr)
	unpin              func(cache uintptr, checksum uintptr, errOut uintptr)
	analyze_code       func(cache uintptr, checksum uintptr, errOut uintptr) uintptr
	get_metrics        func(cache uintptr, errOut uintptr) uintptr
	get_pinned_metrics func(cache uintptr, errOut uintptr) uintptr
	release_cache      func(cache uintptr)

	// Execution entry-points
	instantiate       func(cache, checksum, env, info, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	execute           func(cache, checksum, env, info, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	migrate           func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	migrate_with_info func(cache, checksum, env, msg, migrateInfo, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	sudo              func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	reply             func(cache, checksum, env, replyArg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	query             func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr

	// IBC helpers
	ibc_channel_open         func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_channel_connect      func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_channel_close        func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_packet_receive       func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_packet_ack           func(cache, checksum, env, ack, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_packet_timeout       func(cache, checksum, env, packet, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_source_callback      func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_destination_callback func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc2_packet_receive      func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr

	// misc helpers
	new_unmanaged_vector     func(nil bool, ptr uintptr, length uintptr) uintptr
	destroy_unmanaged_vector func(v UnmanagedVector)
)

func ensureBindingsLoaded() {
	bindingsOnce.Do(func() {
		ensureLoaded()

		// Core cache ops
		purego.RegisterLibFunc(&init_cache, dlHandle, "init_cache")
		purego.RegisterLibFunc(&store_code, dlHandle, "store_code")
		purego.RegisterLibFunc(&remove_wasm, dlHandle, "remove_wasm")
		purego.RegisterLibFunc(&load_wasm, dlHandle, "load_wasm")
		purego.RegisterLibFunc(&pin, dlHandle, "pin")
		purego.RegisterLibFunc(&unpin, dlHandle, "unpin")
		purego.RegisterLibFunc(&analyze_code, dlHandle, "analyze_code")
		purego.RegisterLibFunc(&get_metrics, dlHandle, "get_metrics")
		purego.RegisterLibFunc(&get_pinned_metrics, dlHandle, "get_pinned_metrics")
		purego.RegisterLibFunc(&release_cache, dlHandle, "release_cache")

		// Execution entry-points
		purego.RegisterLibFunc(&instantiate, dlHandle, "instantiate")
		purego.RegisterLibFunc(&execute, dlHandle, "execute")
		purego.RegisterLibFunc(&migrate, dlHandle, "migrate")
		purego.RegisterLibFunc(&migrate_with_info, dlHandle, "migrate_with_info")
		purego.RegisterLibFunc(&sudo, dlHandle, "sudo")
		purego.RegisterLibFunc(&reply, dlHandle, "reply")
		purego.RegisterLibFunc(&query, dlHandle, "query")

		// IBC helpers
		purego.RegisterLibFunc(&ibc_channel_open, dlHandle, "ibc_channel_open")
		purego.RegisterLibFunc(&ibc_channel_connect, dlHandle, "ibc_channel_connect")
		purego.RegisterLibFunc(&ibc_channel_close, dlHandle, "ibc_channel_close")
		purego.RegisterLibFunc(&ibc_packet_receive, dlHandle, "ibc_packet_receive")
		purego.RegisterLibFunc(&ibc_packet_ack, dlHandle, "ibc_packet_ack")
		purego.RegisterLibFunc(&ibc_packet_timeout, dlHandle, "ibc_packet_timeout")
		purego.RegisterLibFunc(&ibc_source_callback, dlHandle, "ibc_source_callback")
		purego.RegisterLibFunc(&ibc_destination_callback, dlHandle, "ibc_destination_callback")
		purego.RegisterLibFunc(&ibc2_packet_receive, dlHandle, "ibc2_packet_receive")

		// helper destructors
		purego.RegisterLibFunc(&new_unmanaged_vector, dlHandle, "new_unmanaged_vector")
		purego.RegisterLibFunc(&destroy_unmanaged_vector, dlHandle, "destroy_unmanaged_vector")
	})
}

func EnsureBindingsLoaded() { ensureBindingsLoaded() }

func InitCache(config uintptr, errOut uintptr) uintptr { return init_cache(config, errOut) }
func StoreCode(cache uintptr, wasm uintptr, checked bool, persist bool, errOut uintptr) uintptr {
	return store_code(cache, wasm, checked, persist, errOut)
}
func RemoveWasm(cache uintptr, checksum uintptr, errOut uintptr) {
	remove_wasm(cache, checksum, errOut)
}
func LoadWasm(cache uintptr, checksum uintptr, errOut uintptr) uintptr {
	return load_wasm(cache, checksum, errOut)
}
func Pin(cache uintptr, checksum uintptr, errOut uintptr)   { pin(cache, checksum, errOut) }
func Unpin(cache uintptr, checksum uintptr, errOut uintptr) { unpin(cache, checksum, errOut) }
func AnalyzeCode(cache uintptr, checksum uintptr, errOut uintptr) uintptr {
	return analyze_code(cache, checksum, errOut)
}
func GetMetrics(cache uintptr, errOut uintptr) uintptr { return get_metrics(cache, errOut) }
func GetPinnedMetrics(cache uintptr, errOut uintptr) uintptr {
	return get_pinned_metrics(cache, errOut)
}
func ReleaseCache(cache uintptr) { release_cache(cache) }

func Instantiate(cache, checksum, env, info, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return instantiate(cache, checksum, env, info, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func Execute(cache, checksum, env, info, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return execute(cache, checksum, env, info, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func Migrate(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return migrate(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func MigrateWithInfo(cache, checksum, env, msg, migrateInfo, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return migrate_with_info(cache, checksum, env, msg, migrateInfo, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func Sudo(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return sudo(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func Reply(cache, checksum, env, replyArg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return reply(cache, checksum, env, replyArg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func Query(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return query(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcChannelOpen(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_channel_open(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcChannelConnect(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_channel_connect(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcChannelClose(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_channel_close(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcPacketReceive(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_packet_receive(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcPacketAck(cache, checksum, env, ack, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_packet_ack(cache, checksum, env, ack, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcPacketTimeout(cache, checksum, env, packet, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_packet_timeout(cache, checksum, env, packet, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcSourceCallback(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_source_callback(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func IbcDestinationCallback(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc_destination_callback(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}
func Ibc2PacketReceive(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr {
	return ibc2_packet_receive(cache, checksum, env, msg, db, api, querier, gasLimit, printDebug, gasReport, errOut)
}

func NewUnmanagedVector(nil bool, ptr uintptr, length uintptr) UnmanagedVector {
	ret := new_unmanaged_vector(nil, ptr, length)
	return *(*UnmanagedVector)(unsafe.Pointer(ret))
}

func DestroyUnmanagedVector(v UnmanagedVector) { destroy_unmanaged_vector(v) }
