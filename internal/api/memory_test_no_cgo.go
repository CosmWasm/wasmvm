package api

/*
#include "bindings.h"
*/
import "C"

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestCopyDestroyUnmanagedVectorNoCgo(t *testing.T) {
	{
		// ptr, cap and len broken. Do not access those values when is_none is true
		base := unsafe.Pointer(&struct{ x byte }{}) //nolint:gosec
		invalid_ptr := unsafe.Add(base, 42)
		uv := constructUnmanagedVector(true, cu8_ptr(invalid_ptr), 0xBB, 0xAA)
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Nil(t, copy)
	}
	{
		// Capacity is 0, so no allocation happened. Do not access the pointer.
		base := unsafe.Pointer(&struct{ x byte }{}) //nolint:gosec
		invalid_ptr := unsafe.Add(base, 42)
		uv := constructUnmanagedVector(false, cu8_ptr(invalid_ptr), 0, 0)
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Equal(t, []byte{}, copy)
	}
}
