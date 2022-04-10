package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLibwasmvmVersion(t *testing.T) {
	version, err := LibwasmvmVersion()
	require.NoError(t, err)
	require.Equal(t, "1.0.0", version)
}
