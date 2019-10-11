package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
import "C"

func Add(a int32, b int32) int32 {
	return (int32)(C.add((C.int32_t)(a), (C.int32_t)(b)))
}