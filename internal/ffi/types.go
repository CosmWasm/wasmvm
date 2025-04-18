// Package ffi contains pure‑Go representations of the C ABI structs that
// libwasmvm expects.  They are safe to use with purego.RegisterLibFunc and
// must exactly match the field layout in internal/api/bindings.h.
package ffi

import (
	"unsafe"
)

// ByteSliceView mirrors the C struct with the same name.
// It represents an Option<&[u8]> coming from Go → Rust.
type ByteSliceView struct {
	IsNil bool    // whether the slice is nil
	Ptr   *uint8  // element 0
	Len   uintptr // length in bytes
}

// MakeByteSliceView converts a Go byte slice into a view that can be passed to
// Rust.  The caller MUST keep the original slice alive until after the FFI
// call returns.
func MakeByteSliceView(b []byte) ByteSliceView {
	if b == nil {
		return ByteSliceView{IsNil: true}
	}
	if len(b) == 0 {
		return ByteSliceView{IsNil: false, Ptr: nil, Len: 0}
	}
	return ByteSliceView{IsNil: false, Ptr: &b[0], Len: uintptr(len(b))}
}

// UnmanagedVector mirrors the Rust side vector that crosses FFI boundaries.
type UnmanagedVector struct {
	IsNone bool
	Ptr    *uint8
	Len    uintptr
	Cap    uintptr
}

// GasReport mirrors the struct returned from many libwasmvm calls.
type GasReport struct {
	Limit          uint64
	Remaining      uint64
	UsedExternally uint64
	UsedInternally uint64
}

// copyAndDestroyUnmanagedVector copies the data out of an UnmanagedVector and
// then frees it on the Rust side via destroy_unmanaged_vector.  This helper
// assumes ensureBindingsLoaded() has already been called.
func CopyAndDestroyUnmanagedVector(v UnmanagedVector) []byte {
	if v.IsNone {
		// represents None / null
		return nil
	}
	if v.Cap == 0 {
		return []byte{}
	}
	// we must copy because the memory is owned by Rust
	out := unsafe.Slice((*byte)(unsafe.Pointer(v.Ptr)), v.Len)
	dup := append([]byte(nil), out...) // copy
	destroy_unmanaged_vector(v)
	return dup
}

// VTable types mirroring C structs

type DbVtable struct {
	ReadDB   uintptr
	WriteDB  uintptr
	RemoveDB uintptr
	ScanDB   uintptr
}

type IteratorVtable struct {
	Next    uintptr
	NextKey uintptr
	NextVal uintptr
}

type GoApiVtable struct {
	Humanize     uintptr
	Canonicalize uintptr
	Validate     uintptr
}

type QuerierVtable struct {
	QueryExternal uintptr
}

// Main FFI struct types corresponding to C definitions

type GasMeter struct { // Opaque struct - size doesn't matter as it's passed by pointer
	_ byte // Placeholder
}

type Db struct {
	GasMeter *GasMeter // Corresponds to gas_meter_t*
	State    uintptr   // Corresponds to db_t* (unsafe.Pointer to Go state)
	Vtable   DbVtable
}

type GoApi struct {
	State  uintptr // Corresponds to api_t* (unsafe.Pointer to Go API)
	Vtable GoApiVtable
}

type GoQuerier struct {
	State  uintptr // Corresponds to querier_t* (unsafe.Pointer to Go Querier)
	Vtable QuerierVtable
}
