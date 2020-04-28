package types

import (
	"fmt"
)

// ApiError captures all errors returned from the Rust code as StdError.SerializeErr
// Exactly one of the fields should be set.
type ApiError struct {
	Base64Err      *SourceErr     `json:"base64_err,omitempty"`
	ContractErr    *MsgErr        `json:"contract_err,omitempty"`
	DynContractErr *MsgErr        `json:"dyn_contract_err,omitempty"`
	NotFound       *NotFoundErr   `json:"not_found,omitempty"`
	NullPointer    *struct{}      `json:"null_pointer,omitempty"`
	ParseErr       *JSONErr       `json:"parse_err,omitempty"`
	SerializeErr   *JSONErr       `json:"serialize_err,omitempty"`
	Unauthorized   *struct{}      `json:"unauthorized,omitempty"`
	UnderflowErr   *UnderflowErr  `json:"underflow_err,omitempty"`
	Utf8Err        *SourceErr     `json:"utf8_err,omitempty"`
	Utf8StringErr  *SourceErr     `json:"utf8_string_err,omitempty"`
	ValidationErr  *ValidationErr `json:"validation_err,omitempty"`
}

var _ error = (*ApiError)(nil)

func (a *ApiError) Error() string {
	if a == nil {
		return "(nil)"
	}
	switch {
	case a.Base64Err != nil:
		return fmt.Sprintf("base64: %#v", a.Base64Err)
	case a.ContractErr != nil:
		return fmt.Sprintf("contract: %#v", a.ContractErr)
	case a.DynContractErr != nil:
		return fmt.Sprintf("dyn_contract: %#v", a.DynContractErr)
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
	case a.Utf8Err != nil:
		return fmt.Sprintf("utf8: %#v", a.Utf8Err)
	case a.Utf8StringErr != nil:
		return fmt.Sprintf("utf8: %#v", a.Utf8StringErr)
	case a.ValidationErr != nil:
		return fmt.Sprintf("validation: %#v", a.ValidationErr)
	default:
		return "unknown error variant"
	}
}

type NotFoundErr struct {
	Kind string `json:"kind,omitempty"`
}

// JsonErr is used for ParseErr and SerializeErr
type JSONErr struct {
	Kind   string `json:"kind,omitempty"`
	Source string `json:"source,omitempty"`
}

type ValidationErr struct {
	Field string `json:"field,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

// SourceErr is a generic type for errors that only have Source field
type SourceErr struct {
	Source string `json:"source,omitempty"`
}

// MsgErr is a generic type for errors that only have Msg field
type MsgErr struct {
	Msg string `json:"msg,omitempty"`
}

type UnderflowErr struct {
	Minuend    string `json:"minuend,omitempty"`
	Subtrahend string `json:"subtrahend,omitempty"`
}
