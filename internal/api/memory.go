package api

import (
	"unsafe"

	"github.com/CosmWasm/wasmvm/v2/internal/ffi"
)

// U8SliceView represents a slice view of uint8 values
type U8SliceView struct {
	IsNone uint8
	Ptr    *uint8
	Len    uintptr
}

// makeView creates a view into the given byte slice for Rust to read.
// The byte slice is managed by Go and will be garbage collected.
func makeView(s []byte) ByteSliceView {
	if s == nil {
		return ByteSliceView{IsNil: 1}
	}
	if len(s) == 0 {
		return ByteSliceView{IsNil: 0, Ptr: nil, Len: 0}
	}
	return ByteSliceView{
		IsNil: 0,
		Ptr:   (*uint8)(unsafe.Pointer(&s[0])),
		Len:   uintptr(len(s)),
	}
}

// newUnmanagedVector creates a new UnmanagedVector from a byte slice
func newUnmanagedVector(data []byte) ffi.UnmanagedVector {
	var nilFlag bool = false
	var ptr *uint8
	var length uintptr

	if data == nil {
		nilFlag = true
	} else if len(data) == 0 {
		ptr = nil
		length = 0
	} else {
		ptr = (*uint8)(unsafe.Pointer(&data[0]))
		length = uintptr(len(data))
	}

	// Call the Rust function via purego
	vec := ffi.NewUnmanagedVector(nilFlag, uintptr(unsafe.Pointer(ptr)), length)
	return vec
}

// copyU8Slice copies the contents of a U8SliceView
func copyU8Slice(view ffi.ByteSliceView) []byte {
	if view.IsNil {
		return nil
	}
	if view.Len == 0 {
		return []byte{}
	}
	// Copy the data
	out := make([]byte, view.Len)
	copy(out, unsafe.Slice(view.Ptr, view.Len))
	return out
}

// OptionalU64 represents an optional 64-bit unsigned integer
type OptionalU64 struct {
	IsSome uint8
	Value  uint64
}

// optionalU64ToPtr converts OptionalU64 to *uint64
func optionalU64ToPtr(val OptionalU64) *uint64 {
	if val.IsSome != 0 {
		return &val.Value
	}
	return nil
}
