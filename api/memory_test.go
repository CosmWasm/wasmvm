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
