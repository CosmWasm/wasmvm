// Package ffi provides pure-Go bindings; it must always be built.

package ffi

import (
	"sync"

	"github.com/ebitengine/purego"
)

// This file registers *all* remaining libwasmvm symbols we need for a pure‑Go
// build.  Every variable below is a Go function pointer whose calling
// convention matches the corresponding C function exactly.  Higher‑level code
// can call these via unsafe.Pointer / uintptr as needed.

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

	// Execution entry‑points
	instantiate       func(cache, checksum, env, info, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	execute           func(cache, checksum, env, info, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	migrate           func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	migrate_with_info func(cache, checksum, env, msg, migrateInfo, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	sudo              func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	reply             func(cache, checksum, env, reply, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	query             func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr

	// IBC helpers (subset; extend as needed)
	ibc_channel_open    func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_channel_connect func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_channel_close   func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	ibc_packet_receive  func(cache, checksum, env, msg, db, api, querier uintptr, gasLimit uint64, printDebug bool, gasReport, errOut uintptr) uintptr
	// ... other IBC functions can be registered the same way.
)

// ensureBindingsLoaded must be called once before any of the function
// pointers above are used.  internal/api replacements should call this during
// their own init/ensure phase.
func ensureBindingsLoaded() {
	bindingsOnce.Do(func() {
		ensureLoaded() // Opens the shared object and initialises dlHandle

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

		// Execution entry‑points
		purego.RegisterLibFunc(&instantiate, dlHandle, "instantiate")
		purego.RegisterLibFunc(&execute, dlHandle, "execute")
		purego.RegisterLibFunc(&migrate, dlHandle, "migrate")
		purego.RegisterLibFunc(&migrate_with_info, dlHandle, "migrate_with_info")
		purego.RegisterLibFunc(&sudo, dlHandle, "sudo")
		purego.RegisterLibFunc(&reply, dlHandle, "reply")
		purego.RegisterLibFunc(&query, dlHandle, "query")

		// IBC helpers (partial set)
		purego.RegisterLibFunc(&ibc_channel_open, dlHandle, "ibc_channel_open")
		purego.RegisterLibFunc(&ibc_channel_connect, dlHandle, "ibc_channel_connect")
		purego.RegisterLibFunc(&ibc_channel_close, dlHandle, "ibc_channel_close")
		purego.RegisterLibFunc(&ibc_packet_receive, dlHandle, "ibc_packet_receive")
	})
}
