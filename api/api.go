package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
import "C"

import "fmt"

// nice aliases to the rust names
type i32 = C.int32_t
type i64 = C.int64_t
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
	return receiveSlice(raw)
}

func Divide(a, b int32) (int32, error) {
	res, err := C.divide(i32(a), i32(b))
	return int32(res), getError(err)
}

func RandomMessage(guess int32) (string, error) {
	res, err := C.may_panic(i32(guess))
	if err != nil {
		return "", getError(err)
	}
	return string(receiveSlice(res)), nil
}

func UpdateDB(kv KVStore, key []byte) error {
	db := buildDB(kv)
	buf := sendSlice(key)
	_, err := C.update_db(db, buf)
	return getError(err)
}

/**** To error module ***/

// returns the last error message (or nil if none returned)
// err is assumed to be the result of errno, and this only queries if err != nil
// so you can safely use it to wrap all returns (eg. it will be a noop if err == nil)
func getError(err error) error {
	if err == nil {
		return nil
	}
	// TODO: add custom error type
	msg := receiveSlice(C.get_last_error())
	if msg == nil {
		return nil
	}
	return fmt.Errorf("%s", string(msg))
}
