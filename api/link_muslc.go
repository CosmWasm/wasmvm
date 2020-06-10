// +build linux,muslc
// +build 386

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm_muslc
import "C"
