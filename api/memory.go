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

func receiveSlice(b C.Buffer) []byte {
	if emptyBuf(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.len))
	C.free_rust(b)
	return res
}

func freeAfterSend(b C.Buffer) {
	if !emptyBuf(b) {
		C.free(unsafe.Pointer(b.ptr))
	}
}

func emptyBuf(b C.Buffer) bool {
	return b.ptr == u8_ptr(nil)
}
