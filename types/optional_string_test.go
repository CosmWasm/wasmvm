package types

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOptionalStringDefaultValue(t *testing.T) {
	var os OptionalString
	assert.Equal(t, false, os.Set)
}

func TestNewOptionalString(t *testing.T) {
	os := NewOptionalStringSet("abc")
	assert.Equal(t, "abc", os.String())

	os = NewOptionalStringSet("")
	assert.Equal(t, "", os.String())

	os = NewOptionalStringUnset()
	assert.Equal(t, "", os.String())
}

func TestOptionalStringToString(t *testing.T) {
	os := NewOptionalStringSet("abc")
	assert.Equal(t, OptionalString{Set: true, Value: "abc"}, os)

	os = NewOptionalStringSet("")
	assert.Equal(t, OptionalString{Set: true, Value: ""}, os)

	os = NewOptionalStringUnset()
	assert.Equal(t, false, os.Set)
}

func TestOptionalStringMarshalJSON(t *testing.T) {
	alice := NewOptionalStringSet("alice")
	bytes, err := json.Marshal(alice)
	require.NoError(t, err)
	assert.Equal(t, []byte("\"alice\""), bytes)

	empty := NewOptionalStringSet("")
	bytes, err = json.Marshal(empty)
	require.NoError(t, err)
	assert.Equal(t, []byte("\"\""), bytes)

	unset := NewOptionalStringUnset()
	bytes, err = json.Marshal(unset)
	require.NoError(t, err)
	assert.Equal(t, []byte("null"), bytes)
}

func TestOptionalStringUnmarshalJSON(t *testing.T) {
	var os OptionalString

	err := json.Unmarshal([]byte("null"), &os)
	require.NoError(t, err)
	assert.Equal(t, false, os.Set)

	err = json.Unmarshal([]byte("\"\""), &os)
	require.NoError(t, err)
	assert.Equal(t, OptionalString{Set: true, Value: ""}, os)

	err = json.Unmarshal([]byte("\"abc\""), &os)
	require.NoError(t, err)
	assert.Equal(t, OptionalString{Set: true, Value: "abc"}, os)

	err = json.Unmarshal([]byte("123"), &os)
	// TODO: Use ErrorContains once released (https://github.com/stretchr/testify/commit/6990a05d54)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal number into Go value of type string")

	err = json.Unmarshal([]byte("[]"), &os)
	// TODO: Use ErrorContains once released (https://github.com/stretchr/testify/commit/6990a05d54)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal array into Go value of type string")

	err = json.Unmarshal([]byte("false"), &os)
	// TODO: Use ErrorContains once released (https://github.com/stretchr/testify/commit/6990a05d54)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal bool into Go value of type string")

	err = json.Unmarshal([]byte(""), &os)
	// TODO: Use ErrorContains once released (https://github.com/stretchr/testify/commit/6990a05d54)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected end of JSON input")
}
