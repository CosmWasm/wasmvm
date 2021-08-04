package api

import (
	"testing"

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
		require.Equal(t, 3, int(unmanaged.len))
		require.GreaterOrEqual(t, 3, int(unmanaged.cap)) // Rust implementation decides this
		copy := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, original, copy)
	}

	// empty
	{
		original := []byte{}
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(false), unmanaged.is_none)
		require.Equal(t, 0, int(unmanaged.len))
		require.GreaterOrEqual(t, 0, int(unmanaged.cap)) // Rust implementation decides this
		copy := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, original, copy)
	}

	// none
	{
		var original []byte
		unmanaged := newUnmanagedVector(original)
		require.Equal(t, cbool(true), unmanaged.is_none)
		// We must not make assumtions on the other fields in this case
		copy := copyAndDestroyUnmanagedVector(unmanaged)
		require.Nil(t, copy)
	}
}
