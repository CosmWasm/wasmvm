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

type Cache struct {
	ptr *C.cache_t
}

func InitCache(dataDir string, cacheSize uint64) (Cache, error) {
	dir := sendSlice([]byte(dataDir))
	errmsg := C.Buffer{}
	ptr, err := C.init_cache(dir, usize(cacheSize), &errmsg)
	if err != nil {
		return Cache{}, errorWithMessage(err, errmsg)
	}
	return Cache{ptr: ptr}, nil
}

func ReleaseCache(cache Cache) {
	C.release_cache(cache.ptr)
}

func Create(cache Cache, wasm []byte) ([]byte, error) {
	code := sendSlice(wasm)
	errmsg := C.Buffer{}
	id, err := C.create(cache.ptr, code, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(id), nil
}

func GetCode(cache Cache, contractID []byte) ([]byte, error) {
	id := sendSlice(contractID)
	errmsg := C.Buffer{}
	code, err := C.get_code(cache.ptr, id, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(code), nil
}

func Instantiate(cache Cache, contractID []byte, params []byte, msg []byte, store KVStore, gasLimit int64) ([]byte, error) {
	id := sendSlice(contractID)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	errmsg := C.Buffer{}
	res, err := C.instantiate(cache.ptr, id, p, m, db, i64(gasLimit), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

func Handle(cache Cache, contractID []byte, params []byte, msg []byte, store KVStore, gasLimit int64) ([]byte, error) {
	id := sendSlice(contractID)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	errmsg := C.Buffer{}
	res, err := C.handle(cache.ptr, id, p, m, db, i64(gasLimit), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

func Query(cache Cache, contractID []byte, path []byte, data []byte, store KVStore, gasLimit int64) ([]byte, error) {
	id := sendSlice(contractID)
	p := sendSlice(path)
	d := sendSlice(data)
	db := buildDB(store)
	errmsg := C.Buffer{}
	res, err := C.query(cache.ptr, id, p, d, db, i64(gasLimit), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

/**** To error module ***/

func errorWithMessage(err error, b C.Buffer) error {
	msg := receiveSlice(b)
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}
