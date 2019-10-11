package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
import "C"

type i32 = C.int32_t


func Add(a int32, b int32) int32 {
	return (int32)(C.add(i32(a), i32(b)))
}