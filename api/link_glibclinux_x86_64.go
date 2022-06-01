//go:build linux && !muslc && amd64 && !custom_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm.x86_64
import "C"
