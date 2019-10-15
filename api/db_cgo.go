package api

/*
#include "bindings.h"

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
*/
import "C"
