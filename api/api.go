package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
import "C"

import "unsafe"

// nice aliases to the rust names
type i32 = C.int32_t
type u8 = C.uint8_t
type u8_ptr = *C.uint8_t
type usize = C.uintptr_t
type cint = C.int


func Add(a int32, b int32) int32 {
	return (int32)(C.add(i32(a), i32(b)))
}

func Greet(name []byte) []byte {
	buf := sliceToBuffer(name)
	raw := C.greet(buf)
	res := copyBuffer(raw)
	// make sure to free after call
	freeOurBuf(buf)
	freeTheirBuf(raw)
	return res
}

/*** To memory module **/

func sliceToBuffer(s []byte) *C.Buffer {
	if s == nil {
		return nil;
	}
	return &C.Buffer{
		ptr: u8_ptr(C.CBytes(s)),
		size: usize(len(s)),
	}
}

func freeOurBuf(buf *C.Buffer) {
	if buf != nil && buf.ptr != u8_ptr(nil) {
		C.free(unsafe.Pointer(buf.ptr))
	}
}

func freeTheirBuf(buf *C.Buffer) {
	if buf != nil && buf.ptr != u8_ptr(nil) {
		C.free_rust(buf)
	}
}

func copyBuffer(b *C.Buffer) []byte {
	return C.GoBytes(unsafe.Pointer(b.ptr), cint(b.size))
}