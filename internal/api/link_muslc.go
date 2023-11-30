//go:build linux && muslc && !sys_wasmvm

package api

// TODO: once we switch to builders 0018+, split this linking statement in x86_64 and arm64 like we do with the glibc case

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm_muslc
import "C"
