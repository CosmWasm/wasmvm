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
	// Non-empty vector
	{
		original := []byte{0xaa, 0xbb, 0x64}
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(false), unmanaged.is_none)
		require.Equal(t, 3, int(unmanaged.len))
		require.GreaterOrEqual(t, 3, int(unmanaged.cap)) // Rust implementation decides cap
		copy := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, original, copy)
	}

	// Empty vector
	{
		original := []byte{}
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(false), unmanaged.is_none)
		require.Equal(t, 0, int(unmanaged.len))
		require.GreaterOrEqual(t, 0, int(unmanaged.cap)) // Rust implementation decides cap
		copy := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, original, copy)
	}

	// None (nil slice)
	{
		var original []byte
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(true), unmanaged.is_none)
		// Fields other than is_none are not guaranteed in this scenario
		copy := copyAndDestroyUnmanagedVector(unmanaged)
		require.Nil(t, copy)
	}
}

// TestCopyDestroyUnmanagedVector checks edge cases without newUnmanagedVector calls.
//
//go:nocheckptr
func TestCopyDestroyUnmanagedVector(t *testing.T) {
	// If is_none is true, do not access pointer, len, or cap values
	{
		invalidPtr := unsafe.Pointer(uintptr(42))
		uv := constructUnmanagedVector(cbool(true), cu8_ptr(invalidPtr), cusize(0xBB), cusize(0xAA))
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Nil(t, copy)
	}

	// Capacity is 0, so no allocation happened. Do not access the pointer.
	{
		invalidPtr := unsafe.Pointer(uintptr(42))
		uv := constructUnmanagedVector(cbool(false), cu8_ptr(invalidPtr), cusize(0), cusize(0))
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Equal(t, []byte{}, copy)
	}
}
