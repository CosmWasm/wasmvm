//go:build linux && !muslc && arm64 && !custom_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm.aarch64
import "C"
