package api

/*
#include "bindings.h"
#include <stdio.h>

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
