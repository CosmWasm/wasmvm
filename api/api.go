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
	buf := sendSlice(name)
	raw := C.greet(buf)
	// make sure to free after call
	freeAfterSend(buf)

	return receiveSlice(raw)
}

// TODO: add error handling example...

/*** To memory module **/

func sendSlice(s []byte) C.Buffer {
	if s == nil {
		return C.Buffer{ptr: u8_ptr(nil), size: usize(0)};
	}
	return C.Buffer{
		ptr: u8_ptr(C.CBytes(s)),
		size: usize(len(s)),
	}
}

func receiveSlice(b C.Buffer) []byte {
	if b.ptr == u8_ptr(nil) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.size))
	C.free_rust(b)
	return res
}

func freeAfterSend(buf C.Buffer) {
	if buf.ptr != u8_ptr(nil) {
		C.free(unsafe.Pointer(buf.ptr))
	}
}


