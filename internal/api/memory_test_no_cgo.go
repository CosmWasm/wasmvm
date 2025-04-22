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

// TestCopyDestroyUnmanagedVectorNoCgo tests copying and destroying unmanaged vectors without CGO
func TestCopyDestroyUnmanagedVectorNoCgo(t *testing.T) {
	{
		// ptr, cap and len broken. Do not access those values when is_none is true
		base := unsafe.Pointer(&struct{ x byte }{}) //nolint:gosec  // This is a test-only code that requires unsafe pointer for low-level memory testing
		invalid_ptr := unsafe.Add(base, 42)
		uv := constructUnmanagedVector(true, cu8_ptr(invalid_ptr), 0xBB, 0xAA)

		// Use safer approach to copy and destroy
		safeVec := CopyAndDestroyToSafeVector(uv)
		copied := safeVec.ToBytesAndDestroy()
		require.Nil(t, copied)
	}
	{
		// Capacity is 0, so no allocation happened. Do not access the pointer.
		base := unsafe.Pointer(&struct{ x byte }{}) //nolint:gosec  // This is a test-only code that requires unsafe pointer for low-level memory testing
		invalid_ptr := unsafe.Add(base, 42)
		uv := constructUnmanagedVector(false, cu8_ptr(invalid_ptr), 0, 0)

		// Use safer approach to copy and destroy
		safeVec := CopyAndDestroyToSafeVector(uv)
		copied := safeVec.ToBytesAndDestroy()
		require.Equal(t, []byte{}, copied)
	}
}
