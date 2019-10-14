package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
import "C"

import "fmt"
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

func Divide(a, b int32) (int32, error) {
	res, err := C.divide(i32(a), i32(b))
	if err != nil {
		return 0, getError()
	}
	return int32(res), nil
}

func RandomMessage(guess int32) (string, error) {
	res, err := C.may_panic(i32(guess))
	if err != nil {
		return "", getError()
	}
	return string(receiveSlice(res)), nil
}


/**** To error module ***/

// returns the last error message (or nil if none returned)
func getError() error {
	// TODO: add custom error type
	msg := receiveSlice(C.get_last_error())
	if msg == nil {
		return nil
	}
	return fmt.Errorf("%s", string(msg))
}

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
	if emptyBuf(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.size))
	C.free_rust(b)
	return res
}

func freeAfterSend(b C.Buffer) {
	if !emptyBuf(b) {
		C.free(unsafe.Pointer(b.ptr))
	}
}

func emptyBuf(b C.Buffer) bool {
	return b.ptr == u8_ptr(nil) || b.size == usize(0)
}


