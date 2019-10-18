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

func Create(dataDir string, wasm []byte) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	code := sendSlice(wasm)
	id, err := C.create(dir, code)
	return receiveSlice(id), getError(err)
}

func Instantiate(dataDir string, contractId []byte, params []byte, msg []byte, store KVStore, gasLimit int64) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	id := sendSlice(contractId)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	res, err := C.instantiate(dir, id, p, m, db, i64(gasLimit))
	return receiveSlice(res), getError(err)
}

func Handle(dataDir string, contractId []byte, params []byte, msg []byte, store KVStore, gasLimit int64) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	id := sendSlice(contractId)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	res, err := C.instantiate(dir, id, p, m, db, i64(gasLimit))
	return receiveSlice(res), getError(err)
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
