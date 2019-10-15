package api

/*
#include "bindings.h"

// typedefs for _cgo functions
typedef int64_t (*get_fn)(db_t *ptr, Buffer key, Buffer val);
typedef void (*set_fn)(db_t *ptr, Buffer key, Buffer val);

// forward declarations (db_cgo.go)
int64_t cGet_cgo(db_t *ptr, Buffer key, Buffer val);
void cSet_cgo(db_t *ptr, Buffer key, Buffer val);
*/
import "C"

import "unsafe"

type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
}

func buildDB(kv KVStore) C.DB {
	return C.DB{
		state: (*C.db_t)(unsafe.Pointer(&kv)),
		c_get: (C.get_fn)(C.cGet_cgo),
		c_set: (C.set_fn)(C.cSet_cgo),
	}
}

//export cGet
func cGet(ptr *C.db_t, key C.Buffer, val C.Buffer) i64 {
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := kv.Get(k)
	if len(v) == 0 {
		return 0
	}
	return writeToBuffer(val, v)
}

//export cSet
func cSet(ptr *C.db_t, key C.Buffer, val C.Buffer) {
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := receiveSlice(val)
	kv.Set(k, v)
}
