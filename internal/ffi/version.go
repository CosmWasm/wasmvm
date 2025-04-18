package ffi

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	dlHandle uintptr
	once     sync.Once

	versionStr func() uintptr // C: const char* version_str(void)
)

func dlname() string {
	switch runtime.GOOS {
	case "darwin":
		return "libwasmvm.dylib"
	case "linux":
		return "libwasmvm.so"
	case "windows":
		return "wasmvm.dll"
	default:
		panic("unsupported OS for purego wasmvm")
	}
}

func ensureLoaded() {
	once.Do(func() {
		h, err := purego.Dlopen(dlname(), purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			panic(err)
		}
		dlHandle = h
		// register every C symbol we need, one by one
		purego.RegisterLibFunc(&versionStr, dlHandle, "version_str")
	})
}

// Version calls C's version_str() without cgo.
func Version() string {
	ensureLoaded()
	cptr := versionStr() // char*
	return cString(cptr)
}

// cString converts a C NUL‑terminated string referenced by the given pointer
// (as uintptr) into a Go string. It is equivalent to C.GoString but implemented
// without cgo so that this file can be built when CGO is disabled.
func cString(c uintptr) string {
	// Convert the uintptr to an unsafe.Pointer while keeping it opaque to the
	// compiler.
	ptr := unsafe.Pointer(c)
	if ptr == nil {
		return ""
	}

	// Determine the length by scanning until the terminating NUL.
	var n uintptr
	for {
		if *(*byte)(unsafe.Add(ptr, n)) == 0 {
			break
		}
		n++
	}

	// Build a slice that references the underlying memory and convert to
	// string. The memory is owned by the C side; we make a copy to ensure the
	// Go string is safe after the call returns.
	bytes := unsafe.Slice((*byte)(ptr), n)
	return string(bytes)
}
