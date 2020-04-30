package types

import (
	"fmt"
)

// ApiError captures all errors returned from the Rust code as StdError.SerializeErr
// Exactly one of the fields should be set.
type ApiError struct {
	GenericErr    *MsgErr       `json:"generic_err,omitempty"`
	InvalidBase64 *MsgErr       `json:"invalid_base64,omitempty"`
	InvalidUtf8   *MsgErr       `json:"invalid_utf8,omitempty"`
	NotFound      *NotFoundErr  `json:"not_found,omitempty"`
	NullPointer   *struct{}     `json:"null_pointer,omitempty"`
	ParseErr      *ParseErr     `json:"parse_err,omitempty"`
	SerializeErr  *SerializeErr `json:"serialize_err,omitempty"`
	Unauthorized  *struct{}     `json:"unauthorized,omitempty"`
	UnderflowErr  *UnderflowErr `json:"underflow_err,omitempty"`
}

var _ error = (*ApiError)(nil)

func (a *ApiError) Error() string {
	if a == nil {
		return "(nil)"
	}
	switch {
	case a.GenericErr != nil:
		return fmt.Sprintf("generic: %#v", a.GenericErr)
	case a.InvalidBase64 != nil:
		return fmt.Sprintf("base64: %#v", a.InvalidBase64)
	case a.InvalidUtf8 != nil:
		return fmt.Sprintf("utf8: %#v", a.InvalidUtf8)
	case a.NotFound != nil:
		return fmt.Sprintf("not_found: %#v", a.NotFound)
	case a.NullPointer != nil:
		return fmt.Sprintf("null_pointer")
	case a.ParseErr != nil:
		return fmt.Sprintf("parse: %#v", a.ParseErr)
	case a.SerializeErr != nil:
		return fmt.Sprintf("serialize: %#v", a.SerializeErr)
	case a.Unauthorized != nil:
		return fmt.Sprintf("unauthorized")
	case a.UnderflowErr != nil:
		return fmt.Sprintf("underflow: %#v", a.UnderflowErr)
	default:
		return "unknown error variant"
	}
}

type NotFoundErr struct {
	Kind string `json:"kind,omitempty"`
}

type ParseErr struct {
	Target string `json:"target,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

type SerializeErr struct {
	Source string `json:"source,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

// MsgErr is a generic type for errors that only have Msg field
type MsgErr struct {
	Msg string `json:"msg,omitempty"`
}

type UnderflowErr struct {
	Minuend    string `json:"minuend,omitempty"`
	Subtrahend string `json:"subtrahend,omitempty"`
}
