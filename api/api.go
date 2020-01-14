package api

/*
#include "bindings.h"

// typedefs for _cgo functions
typedef int32_t (*human_address_fn)(Buffer, Buffer);
typedef int32_t (*canonical_address_fn)(Buffer, Buffer);

// forward declarations (api_cgo.go)
int32_t cHumanAddress_cgo(Buffer canon, Buffer human);
int32_t cCanonicalAddress_cgo(Buffer human, Buffer canon);
*/
import "C"

import "unsafe"

type GoAPI struct {
    HumanAddress func(canon []byte) string
    CanonicalAddress func(human string) []byte
}

// var DBvtable = C.DB_vtable{
// 	c_get: (C.get_fn)(C.cGet_cgo),
// 	c_set: (C.set_fn)(C.cSet_cgo),
// }

func buildApi(api GoAPI) C.GoApi {
	return C.GoApi{
	    c_human_address: (C.human_address_fn)(C.cHumanAddress_cgo),
	    c_canonical_address: (C.canonical_address_fn)(C.cCanonicalAddress_cgo),
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
