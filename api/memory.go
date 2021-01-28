package api

/*
#include "bindings.h"
*/
import "C"

import "unsafe"

// makeView creates a view into the given byte slice what allows Rust code to read it.
// The byte slice is managed by Go and will be garbage collected. Use runtime.KeepAlive
// to ensure the byte slice lives long enough.
func makeView(s []byte) C.ByteSliceView {
	if s == nil {
		return C.ByteSliceView{is_nil: true, ptr: cu8_ptr(nil), len: cusize(0)}
	}

	// In Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
	// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
	// below with `&data[0]`. https://play.golang.org/p/xvDY3g9OqUk
	if len(s) == 0 {
		return C.ByteSliceView{is_nil: false, ptr: cu8_ptr(nil), len: cusize(0)}
	}

	return C.ByteSliceView{
		is_nil: false,
		ptr:    cu8_ptr(unsafe.Pointer(&s[0])),
		len:    cusize(len(s)),
	}
}

func allocateRust(data []byte) C.Buffer {
	var ret C.Buffer
	if data == nil {
		// Just return a null buffer
		ret = C.Buffer{
			ptr: cu8_ptr(nil),
			len: cusize(0),
			cap: cusize(0),
		}
		// in Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
		// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
		// below with `&data[0]`.
		// https://play.golang.org/p/xvDY3g9OqUk
		// Additionally, the pointer field in a Rust vector is a NonNull pointer. This means that when
		// the vector is empty and no heap allocation is made, it needs to put _some_ value there instead.
		// At the time of writing, it uses the alignment of the generic type T, which in this case equals 1.
		// But because that is an internal detail that we can't rely on in future versions, we still call out
		// to Rust and ask it to build an empty vector for us.
		// https://play.rust-lang.org/?version=stable&mode=debug&edition=2018&gist=01ced0731171c8226e2c28634a7e41d7
	} else if len(data) == 0 {
		// This will create an empty vector
		ret = C.allocate_rust(cu8_ptr(nil), cusize(0))
	} else {
		// This will allocate a proper vector with content and return a description of it
		ret = C.allocate_rust(cu8_ptr(unsafe.Pointer(&data[0])), cusize(len(data)))
	}
	return ret
}

// Take an owned vector that was passed to us, copy it, and then free it on the Rust side.
// This should only be used for vectors that will never be observed again on the Rust side
func receiveVector(b C.Buffer) []byte {
	if bufIsNil(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.len))
	C.free_rust(b)
	return res
}

// copyU8Slice copies the contents of an Option<&[u8]> that was allocated on the Rust side.
// Returns nil if and only if the source is None.
func copyU8Slice(view C.U8SliceView) []byte {
	if view.is_none {
		return nil
	}
	if view.len == 0 {
		// In this case, we don't want to look into the ptr
		return []byte{}
	}
	// C.GoBytes create a copy (https://stackoverflow.com/a/40950744/2013738)
	res := C.GoBytes(unsafe.Pointer(view.ptr), cint(view.len))
	return res
}

func bufIsNil(b C.Buffer) bool {
	return b.ptr == cu8_ptr(nil)
}
