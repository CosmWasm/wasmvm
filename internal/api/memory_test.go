//go:build cgo

package api

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestMakeView(t *testing.T) {
	data := []byte{0xaa, 0xbb, 0x64}
	dataView := makeView(data)
	require.Equal(t, cbool(false), dataView.is_nil)
	require.Equal(t, cusize(3), dataView.len)

	empty := []byte{}
	emptyView := makeView(empty)
	require.Equal(t, cbool(false), emptyView.is_nil)
	require.Equal(t, cusize(0), emptyView.len)

	nilView := makeView(nil)
	require.Equal(t, cbool(true), nilView.is_nil)
}

func TestCreateAndDestroyUnmanagedVector(t *testing.T) {
	// non-empty
	{
		original := []byte{0xaa, 0xbb, 0x64}
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(false), unmanaged.is_none)
		require.Equal(t, uint64(3), uint64(unmanaged.len))
		require.GreaterOrEqual(t, uint64(3), uint64(unmanaged.cap)) // Rust implementation decides this
		copied := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, original, copied)
	}

	// empty
	{
		original := []byte{}
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(false), unmanaged.is_none)
		require.Equal(t, uint64(0), uint64(unmanaged.len))
		require.GreaterOrEqual(t, uint64(0), uint64(unmanaged.cap)) // Rust implementation decides this
		copied := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, original, copied)
	}

	// none
	{
		var original []byte
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(true), unmanaged.is_none)
		// We must not make assumptions on the other fields in this case
		copied := copyAndDestroyUnmanagedVector(unmanaged)
		require.Nil(t, copied)
	}
}

// Like the test above but without `newUnmanagedVector` calls.
// Since only Rust can actually create them, we only test edge cases here.
//
//go:nocheckptr
func TestCopyDestroyUnmanagedVector(t *testing.T) {
	{
		// ptr, cap and len broken. Do not access those values when is_none is true
		base := unsafe.Pointer(&struct{ x byte }{}) //nolint:gosec  // This is a test-only code that requires unsafe pointer for low-level memory testing
		invalid_ptr := unsafe.Add(base, 42)
		uv := constructUnmanagedVector(cbool(true), cu8_ptr(invalid_ptr), cusize(0xBB), cusize(0xAA))
		copied := copyAndDestroyUnmanagedVector(uv)
		require.Nil(t, copied)
	}
	{
		// Capacity is 0, so no allocation happened. Do not access the pointer.
		base := unsafe.Pointer(&struct{ x byte }{}) //nolint:gosec  // This is a test-only code that requires unsafe pointer for low-level memory testing
		invalid_ptr := unsafe.Add(base, 42)
		uv := constructUnmanagedVector(cbool(false), cu8_ptr(invalid_ptr), cusize(0), cusize(0))
		copied := copyAndDestroyUnmanagedVector(uv)
		require.Equal(t, []byte{}, copied)
	}
}
