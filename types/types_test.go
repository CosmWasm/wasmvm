package types

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUint64JSON(t *testing.T) {
	var u Uint64

	// test unmarshal
	err := json.Unmarshal([]byte(`"123"`), &u)
	require.NoError(t, err)
	require.Equal(t, uint64(123), uint64(u))
	// test marshal
	bz, err := json.Marshal(u)
	require.NoError(t, err)
	require.Equal(t, `"123"`, string(bz))

	// test max value unmarshal
	err = json.Unmarshal([]byte(`"18446744073709551615"`), &u)
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), uint64(u))
	// test max value marshal
	bz, err = json.Marshal(Uint64(uint64(math.MaxUint64)))
	require.NoError(t, err)
	require.Equal(t, `"18446744073709551615"`, string(bz))

	// test max value + 1
	err = json.Unmarshal([]byte(`"18446744073709551616"`), &u)
	require.Error(t, err)

	// test unquoted unmarshal
	err = json.Unmarshal([]byte(`123`), &u)
	require.EqualError(t, err, "cannot unmarshal 123 into Uint64, expected string-encoded integer")

	// test empty string
	err = json.Unmarshal([]byte(`""`), &u)
	require.EqualError(t, err, "cannot unmarshal \"\" into Uint64, failed to parse integer")
}

func TestInt64JSON(t *testing.T) {
	var i Int64

	// test unmarshal
	err := json.Unmarshal([]byte(`"-123"`), &i)
	require.NoError(t, err)
	require.Equal(t, int64(-123), int64(i))
	// test marshal
	bz, err := json.Marshal(i)
	require.NoError(t, err)
	require.Equal(t, `"-123"`, string(bz))

	// test max value unmarshal
	err = json.Unmarshal([]byte(`"9223372036854775807"`), &i)
	require.NoError(t, err)
	require.Equal(t, int64(math.MaxInt64), int64(i))
	// test max value marshal
	bz, err = json.Marshal(Int64(int64(math.MaxInt64)))
	require.NoError(t, err)
	require.Equal(t, `"9223372036854775807"`, string(bz))

	// test max value + 1
	err = json.Unmarshal([]byte(`"9223372036854775808"`), &i)
	require.Error(t, err)

	// test min value unmarshal
	err = json.Unmarshal([]byte(`"-9223372036854775808"`), &i)
	require.NoError(t, err)
	require.Equal(t, int64(math.MinInt64), int64(i))
	// test min value marshal
	bz, err = json.Marshal(Int64(int64(math.MinInt64)))
	require.NoError(t, err)
	require.Equal(t, `"-9223372036854775808"`, string(bz))

	// test unquoted unmarshal
	err = json.Unmarshal([]byte(`-123`), &i)
	require.EqualError(t, err, "cannot unmarshal -123 into Int64, expected string-encoded integer")

	// test empty string
	err = json.Unmarshal([]byte(`""`), &i)
	require.EqualError(t, err, "cannot unmarshal \"\" into Int64, failed to parse integer")
}

func TestArraySerialization(t *testing.T) {
	var arr Array[string]

	// unmarshal empty
	err := json.Unmarshal([]byte(`[]`), &arr)
	require.NoError(t, err)
	require.Equal(t, Array[string]{}, arr)
	err = json.Unmarshal([]byte(`[ ]`), &arr)
	require.NoError(t, err)
	require.Equal(t, Array[string]{}, arr)
	err = json.Unmarshal([]byte(` []`), &arr)
	require.NoError(t, err)
	require.Equal(t, Array[string]{}, arr)

	// unmarshal null
	err = json.Unmarshal([]byte(`null`), &arr)
	require.NoError(t, err)
	require.Equal(t, Array[string]{}, arr)

	// unmarshal filled
	err = json.Unmarshal([]byte(`["a","b"]`), &arr)
	require.NoError(t, err)
	require.Equal(t, Array[string]{"a", "b"}, arr)

	// marshal filled
	bz, err := json.Marshal(arr)
	require.NoError(t, err)
	require.Equal(t, `["a","b"]`, string(bz))

	// marshal null
	bz, err = json.Marshal(Array[string](nil))
	require.NoError(t, err)
	require.Equal(t, `[]`, string(bz))

	// marshal empty
	bz, err = json.Marshal(Array[uint64]{})
	require.NoError(t, err)
	require.Equal(t, `[]`, string(bz))
}
