package api

/*
#include "bindings.h"
#include <stdio.h>

// imports from db.go
void cSet(db_t *ptr, Buffer key, Buffer val);
int64_t cGet(db_t *ptr, Buffer key, Buffer val);

// Gateway function
int64_t cGet_cgo(db_t *ptr, Buffer key, Buffer val) {
	return cGet(ptr, key, val);
}

// Gateway function
void cSet_cgo(db_t *ptr, Buffer key, Buffer val) {
	cSet(ptr, key, val);
}

// imports from api.go
int32_t cHumanAddress(api_t *ptr, Buffer canon, Buffer human);
int32_t cCanonicalAddress(api_t *ptr, Buffer human, Buffer canon);

// Gateway function
int32_t cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer canon) {
    return cCanonicalAddress(ptr, human, canon);
}

// Gateway function
int32_t cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer human) {
    return cHumanAddress(ptr, canon, human);
}
*/
import "C"
