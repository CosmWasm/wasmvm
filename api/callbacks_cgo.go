package api

/*
#include "bindings.h"
#include <stdio.h>

// imports (db)
GoResult cSet(db_t *ptr, Buffer key, Buffer val);
GoResult cGet(db_t *ptr, Buffer key, Buffer *val);
GoResult cDelete(db_t *ptr, Buffer key);
// imports (api)
GoResult cHumanAddress(api_t *ptr, Buffer canon, Buffer *human);
GoResult cCanonicalAddress(api_t *ptr, Buffer human, Buffer *canon);

// Gateway functions (db)
GoResult cGet_cgo(db_t *ptr, Buffer key, Buffer *val) {
	return cGet(ptr, key, val);
}
GoResult cSet_cgo(db_t *ptr, Buffer key, Buffer val) {
	return cSet(ptr, key, val);
}
GoResult cDelete_cgo(db_t *ptr, Buffer key) {
	return cDelete(ptr, key);
}

// Gateway functions (api)
GoResult cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer *canon) {
    return cCanonicalAddress(ptr, human, canon);
}
GoResult cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer *human) {
    return cHumanAddress(ptr, canon, human);
}
*/
import "C"

// We need these gateway functions to allow calling back to a go function from the c code.
// At least I didn't discover a cleaner way.
// Also, this needs to be in a different file than `callbacks.go`, as we cannot create functions
// in the same file that has //export directives. Only import header types
