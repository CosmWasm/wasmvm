package api

/*
#include "bindings.h"
*/
import "C"

import "unsafe"

func allocateRust(data []byte) C.Buffer {
	return C.allocate_rust(u8_ptr(unsafe.Pointer(&data[0])), usize(len(data)))
}

func sendSlice(s []byte) C.Buffer {
	if s == nil {
		return C.Buffer{ptr: u8_ptr(nil), len: usize(0), cap: usize(0)}
	}
	return C.Buffer{
		ptr: u8_ptr(C.CBytes(s)),
		len: usize(len(s)),
		cap: usize(len(s)),
	}
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

// Copy the contents of a vector that was allocated on the Rust side.
// Unlike receiveVector, we do not free it, because it will be manually
// freed on the Rust side after control returns to it.
//This should be used in places like callbacks from Rust to Go.
func receiveSlice(b C.Buffer) []byte {
	if bufIsNil(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.len))
	return res
}

func freeAfterSend(b C.Buffer) {
	if !bufIsNil(b) {
		C.free(unsafe.Pointer(b.ptr))
	}
}

func bufIsNil(b C.Buffer) bool {
	return b.ptr == u8_ptr(nil)
}
