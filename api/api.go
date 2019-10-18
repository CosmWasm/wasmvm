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
	errmsg := C.Buffer{}
	id, err := C.create(dir, code, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(id), nil
}

func GetCode(dataDir string, contractID []byte) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	id := sendSlice(contractID)
	errmsg := C.Buffer{}
	code, err := C.get_code(dir, id, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(code), nil
}

func Instantiate(dataDir string, contractID []byte, params []byte, msg []byte, store KVStore, gasLimit int64) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	id := sendSlice(contractID)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	errmsg := C.Buffer{}
	res, err := C.instantiate(dir, id, p, m, db, i64(gasLimit), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

func Handle(dataDir string, contractID []byte, params []byte, msg []byte, store KVStore, gasLimit int64) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	id := sendSlice(contractID)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	errmsg := C.Buffer{}
	res, err := C.instantiate(dir, id, p, m, db, i64(gasLimit), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

func Query(dataDir string, contractID []byte, path []byte, data []byte, store KVStore, gasLimit int64) ([]byte, error) {
	dir := sendSlice([]byte(dataDir))
	id := sendSlice(contractID)
	p := sendSlice(path)
	d := sendSlice(data)
	db := buildDB(store)
	errmsg := C.Buffer{}
	res, err := C.query(dir, id, p, d, db, i64(gasLimit), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

func UpdateDB(kv KVStore, key []byte) error {
	db := buildDB(kv)
	buf := sendSlice(key)
	errmsg := C.Buffer{}
	_, err := C.update_db(db, buf, &errmsg)
	if err != nil {
		return errorWithMessage(err, errmsg)
	}
	return nil
}

/**** To error module ***/

func errorWithMessage(err error, b C.Buffer) error {
	msg := receiveSlice(b)
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}
