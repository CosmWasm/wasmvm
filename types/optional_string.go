package types

import (
	"encoding/json"
)

// OptionalString is a type that is able to represent JSON's null or string.
// It allows us to differentiate between a null field and an empty string.
// The corresponding Rust type is Option<String>.
// If you want to treat null the same way as the empty string, use .String()
// which maps the two to an empty string.
type OptionalString struct {
	Set bool
	/// Value is the string value when Set is true. When Set is unset, this field must be ignored.
	Value string
}

func NewOptionalStringUnset() OptionalString {
	return OptionalString{
		Set:   false,
		Value: "don't use me",
	}
}

func NewOptionalStringSet(value string) OptionalString {
	return OptionalString{
		Set:   true,
		Value: value,
	}
}

// String converts the OptionalString to a string by mapping unset
// to an empty string. Use this when you not need the differentiation anymore.
func (os OptionalString) String() string {
	if os.Set == false {
		return ""
	} else {
		return os.Value
	}
}

// MarshalJSON encodes a set OptionalString to a JSON string and an unset OptionalString to a JSON null
func (in OptionalString) MarshalJSON() ([]byte, error) {
	if in.Set == false {
		return []byte("null"), nil
	} else {
		return json.Marshal(in.Value)
	}
}

// UnmarshalJSON decodes a JSON string to a set OptionalString and null to an unset OptionalString
func (out *OptionalString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*out = NewOptionalStringUnset()
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*out = NewOptionalStringSet(value)
	return nil
}
