package api

/*
#include "bindings.h"
#include <stdio.h>

// imports (db)
GoResult cSet(db_t *ptr, Buffer key, Buffer val);
GoResult cGet(db_t *ptr, Buffer key, Buffer *val);
GoResult cDelete(db_t *ptr, Buffer key);
GoResult cScan(db_t *ptr, Buffer start, Buffer end, int32_t order, GoIter *out);
// imports (iterator)
GoResult cNext(iterator_t *ptr, Buffer *key, Buffer *val);
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
GoResult cScan_cgo(db_t *ptr, Buffer start, Buffer end, int32_t order, GoIter *out) {
	return cScan(ptr, start, end, order, out);
}

// Gateway functions (iterator)
GoResult cNext_cgo(iterator_t *ptr, Buffer *key, Buffer *val) {
	return cNext(ptr, key, val);
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
