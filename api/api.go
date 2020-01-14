package api

/*
#include "bindings.h"

// typedefs for _cgo functions
typedef int32_t (*human_address_fn)(api_t*, Buffer, Buffer);
typedef int32_t (*canonical_address_fn)(api_t*, Buffer, Buffer);

// forward declarations (api_cgo.go)
int32_t cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer human);
int32_t cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer canon);
*/
import "C"

import "unsafe"

type HumanAddress func([]byte) string
type CanonicalAddress func(string) []byte

type GoAPI struct {
    HumanAddress HumanAddress
    CanonicalAddress CanonicalAddress
}

var api_vtable = C.GoApi_vtable{
	c_human_address: (C.human_address_fn)(C.cHumanAddress_cgo),
	c_canonical_address: (C.canonical_address_fn)(C.cCanonicalAddress_cgo),
}

func buildAPI(api GoAPI) C.GoApi {
	return C.GoApi{
		state:  (*C.api_t)(unsafe.Pointer(&api)),
		vtable: DBvtable,
	}
}

//export cHumanAddress
func cHumanAddress(ptr *C.api_t, canon C.Buffer, human C.Buffer) i32 {
	api := (*GoAPI)(unsafe.Pointer(ptr))
	c := receiveSlice(canon)
	h := api.HumanAddress(c)
	if len(h) == 0 {
		return 0
	}
	return writeToBuffer(human, []byte(h))
}

//export cCanonicalAddress
func cCanonicalAddress(ptr *C.api_t, human C.Buffer, canon C.Buffer) i32 {
	api := (*GoAPI)(unsafe.Pointer(ptr))
	h := string(receiveSlice(human))
	c := api.CanonicalAddress(h)
	if len(c) == 0 {
		return 0
	}
	return writeToBuffer(canon, c)
}
