package api

/*
#include <string.h> // memcpy
#include "bindings.h"

// memcpy helper
int64_t write_to_buffer(Buffer dest, uint8_t *data, int64_t len) {
    if (len > dest.size) {
    	return -dest.size;
    }
	memcpy(dest.ptr, data, len);
	return len;
}

*/
import "C"

import "unsafe"

func writeToBuffer(buf C.Buffer, data []byte) i64 {
	return C.write_to_buffer(buf, u8_ptr(unsafe.Pointer(&data[0])), i64(len(data)))
}

func sendSlice(data []byte) C.Buffer {
	if len(data) == 0 {
		return C.Buffer{ptr: u8_ptr(nil), size: usize(0)}
	}
	res := C.Buffer{
		ptr:  u8_ptr(unsafe.Pointer(&data[0])),
		size: usize(len(data)),
	}
	return res
}

func receiveSlice(b C.Buffer) []byte {
	if emptyBuf(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.size))
	C.free_rust(b)
	return res
}

func emptyBuf(b C.Buffer) bool {
	return b.ptr == u8_ptr(nil) || b.size == usize(0)
}
