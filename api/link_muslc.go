//go:build linux && muslc
// +build linux,muslc

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm_muslc
import "C"
