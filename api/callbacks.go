package api

/*
#include "bindings.h"

// typedefs for _cgo functions (db)
typedef GoResult (*read_db_fn)(db_t *ptr, Buffer key, Buffer *val);
typedef GoResult (*write_db_fn)(db_t *ptr, Buffer key, Buffer val);
// and api
typedef GoResult (*humanize_address_fn)(api_t*, Buffer, Buffer*);
typedef GoResult (*canonicalize_address_fn)(api_t*, Buffer, Buffer*);

// forward declarations (db)
GoResult cGet_cgo(db_t *ptr, Buffer key, Buffer *val);
GoResult cSet_cgo(db_t *ptr, Buffer key, Buffer val);
// and api
GoResult cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer *human);
GoResult cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer *canon);
*/
import "C"

import (
	"fmt"
	"log"
	"unsafe"
)

import cosmosStoreTypes "github.com/cosmos/cosmos-sdk/store/types"

// Note: we have to include all exports in the same file (at least since they both import bindings.h),
// or get odd cgo build errors about duplicate definitions

func recoverPanic(ret *C.GoResult) {
	rec := recover()
	switch rec.(type) {
	case nil:
		// Do nothing, there was no panic
	// These two cases are for types thrown in panics from this module:
	// https://github.com/cosmos/cosmos-sdk/blob/4ffabb65a5c07dbb7010da397535d10927d298c1/store/types/gas.go
	// ErrorOutOfGas needs to be propagated through the rust code and back into go code, where it should
	// probably be thrown in a panic again.
	// TODO figure out how to pass the text in its `Descriptor` field through all the FFI
	// TODO handle these cases on the Rust side in the first place
	case cosmosStoreTypes.ErrorOutOfGas:
		*ret = C.GoResult_OutOfGas
	// Looks like this error is not treated specially upstream:
	// https://github.com/cosmos/cosmos-sdk/blob/4ffabb65a5c07dbb7010da397535d10927d298c1/baseapp/baseapp.go#L818-L853
	// but this needs to be periodically verified, in case they do start checking for this type
	case cosmosStoreTypes.ErrorGasOverflow:
		log.Printf("Panic in Go callback: %#v\n", rec)
		*ret = C.GoResult_Panic
	default:
		log.Printf("Panic in Go callback: %#v\n", rec)
		*ret = C.GoResult_Panic
	}
}

/****** DB ********/

type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
}

var db_vtable = C.DB_vtable{
	read_db: (C.read_db_fn)(C.cGet_cgo),
	write_db: (C.write_db_fn)(C.cSet_cgo),
}

// contract: original pointer/struct referenced must live longer than C.DB struct
// since this is only used internally, we can verify the code that this is the case
func buildDB(kv KVStore) C.DB {
	return C.DB{
		state:  (*C.db_t)(unsafe.Pointer(&kv)),
		vtable: db_vtable,
	}
}

//export cGet
func cGet(ptr *C.db_t, key C.Buffer, val *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if val == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := kv.Get(k)
	// v will equal nil when the key is missing
	// https://github.com/cosmos/cosmos-sdk/blob/1083fa948e347135861f88e07ec76b0314296832/store/types/store.go#L174
	if v != nil {
		*val = allocateRust(v)
	}
	// else: the Buffer on the rust side is initialised as a "null" buffer,
	// so if we don't write a non-null address to it, it will understand that
	// the key it requested does not exist in the kv store

	return C.GoResult_Ok
}

//export cSet
func cSet(ptr *C.db_t, key C.Buffer, val C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)

	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := receiveSlice(val)
	kv.Set(k, v)
	return C.GoResult_Ok
}

/***** GoAPI *******/

type HumanAddress func([]byte) (string, error)
type CanonicalAddress func(string) ([]byte, error)

type GoAPI struct {
	HumanAddress     HumanAddress
	CanonicalAddress CanonicalAddress
}

var api_vtable = C.GoApi_vtable{
	humanize_address:     (C.humanize_address_fn)(C.cHumanAddress_cgo),
	canonicalize_address: (C.canonicalize_address_fn)(C.cCanonicalAddress_cgo),
}

// contract: original pointer/struct referenced must live longer than C.GoApi struct
// since this is only used internally, we can verify the code that this is the case
func buildAPI(api *GoAPI) C.GoApi {
	return C.GoApi{
		state:  (*C.api_t)(unsafe.Pointer(api)),
		vtable: api_vtable,
	}
}

//export cHumanAddress
func cHumanAddress(ptr *C.api_t, canon C.Buffer, human *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if human == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	api := (*GoAPI)(unsafe.Pointer(ptr))
	c := receiveSlice(canon)
	h, err := api.HumanAddress(c)
	if err != nil {
		return C.GoResult_Other
	}
	if len(h) == 0 {
		panic(fmt.Sprintf("`api.HumanAddress()` returned an empty string for %q", c))
	}
	*human = allocateRust([]byte(h))
	return C.GoResult_Ok
}

//export cCanonicalAddress
func cCanonicalAddress(ptr *C.api_t, human C.Buffer, canon *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if canon == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	api := (*GoAPI)(unsafe.Pointer(ptr))
	h := string(receiveSlice(human))
	c, err := api.CanonicalAddress(h)
	if err != nil {
		return C.GoResult_Other
	}
	if len(c) == 0 {
		panic(fmt.Sprintf("`api.CanonicalAddress()` returned an empty string for %q", h))
	}
	*canon = allocateRust(c)

	// If we do not set canon to a meaningful value, then the other side will interpret that as an empty result.
	return C.GoResult_Ok
}
