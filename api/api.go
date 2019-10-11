package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
//
// Buffer *toBuf(uint8_t *ptr, uintptr_t size) { Buffer *buf = malloc(sizeof(Buffer)); buf->ptr = ptr; buf->size = size; return buf; }
import "C"

import "unsafe"

// TODO: free after toBuf


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
	return copyBuffer(raw)
}

/*** To memory module **/

func sliceToBuffer(s []byte) *C.Buffer {
	if s == nil {
		return nil;
	}
	return C.toBuf(u8_ptr(&s[0]), usize(len(s)))
}

func copyBuffer(b *C.Buffer) []byte {
	return C.GoBytes(unsafe.Pointer(b.ptr), cint(b.size))
}