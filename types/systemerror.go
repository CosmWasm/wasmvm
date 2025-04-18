package types

import (
	"fmt"
	"reflect"
)

// SystemError captures all errors returned from the Rust code as SystemError.
// Exactly one of the fields should be set.
type SystemError struct {
	InvalidRequest     *InvalidRequest     `json:"invalid_request,omitempty"`
	InvalidResponse    *InvalidResponse    `json:"invalid_response,omitempty"`
	NoSuchContract     *NoSuchContract     `json:"no_such_contract,omitempty"`
	NoSuchCode         *NoSuchCode         `json:"no_such_code,omitempty"`
	Unknown            *Unknown            `json:"unknown,omitempty"`
	UnsupportedRequest *UnsupportedRequest `json:"unsupported_request,omitempty"`
}

var (
	_ error = SystemError{}
	_ error = InvalidRequest{}
	_ error = InvalidResponse{}
	_ error = NoSuchContract{}
	_ error = Unknown{}
	_ error = UnsupportedRequest{}
)

func (a SystemError) Error() string {
	switch {
	case a.InvalidRequest != nil:
		return a.InvalidRequest.Error()
	case a.InvalidResponse != nil:
		return a.InvalidResponse.Error()
	case a.NoSuchContract != nil:
		return a.NoSuchContract.Error()
	case a.NoSuchCode != nil:
		return a.NoSuchCode.Error()
	case a.Unknown != nil:
		return a.Unknown.Error()
	case a.UnsupportedRequest != nil:
		return a.UnsupportedRequest.Error()
	default:
		panic("unknown error variant")
	}
}

// InvalidRequest represents an invalid request error
type InvalidRequest struct {
	Err     string `json:"error"`
	Request []byte `json:"request"`
}

func (e InvalidRequest) Error() string {
	return fmt.Sprintf("invalid request: %s - original request: %s", e.Err, string(e.Request))
}

// InvalidResponse represents an invalid response error
type InvalidResponse struct {
	Err      string `json:"error"`
	Response []byte `json:"response"`
}

func (e InvalidResponse) Error() string {
	return fmt.Sprintf("invalid response: %s - original response: %s", e.Err, string(e.Response))
}

// NoSuchContract represents a missing contract error
type NoSuchContract struct {
	Addr string `json:"addr,omitempty"`
}

func (e NoSuchContract) Error() string {
	return fmt.Sprintf("no such contract: %s", e.Addr)
}

// NoSuchCode represents a missing code error
type NoSuchCode struct {
	CodeID uint64 `json:"code_id,omitempty"`
}

func (e NoSuchCode) Error() string {
	return fmt.Sprintf("no such code: %d", e.CodeID)
}

// Unknown represents an unknown error
type Unknown struct{}

func (Unknown) Error() string {
	return "unknown system error"
}

// UnsupportedRequest represents an unsupported request error
type UnsupportedRequest struct {
	Kind string `json:"kind,omitempty"`
}

func (e UnsupportedRequest) Error() string {
	return fmt.Sprintf("unsupported request: %s", e.Kind)
}

// convertErrorToSystemError converts a specific error type to a SystemError
func convertErrorToSystemError(err error) *SystemError {
	switch t := err.(type) {
	case SystemError:
		return &t
	case *SystemError:
		return t
	default:
		return nil
	}
}

// convertSpecificError converts a specific error type to a SystemError
func convertSpecificError(err error) *SystemError {
	switch t := err.(type) {
	case InvalidRequest:
		return &SystemError{InvalidRequest: &t}
	case *InvalidRequest:
		return &SystemError{InvalidRequest: t}
	case InvalidResponse:
		return &SystemError{InvalidResponse: &t}
	case *InvalidResponse:
		return &SystemError{InvalidResponse: t}
	case NoSuchContract:
		return &SystemError{NoSuchContract: &t}
	case *NoSuchContract:
		return &SystemError{NoSuchContract: t}
	case NoSuchCode:
		return &SystemError{NoSuchCode: &t}
	case *NoSuchCode:
		return &SystemError{NoSuchCode: t}
	case Unknown:
		return &SystemError{Unknown: &t}
	case *Unknown:
		return &SystemError{Unknown: t}
	case UnsupportedRequest:
		return &SystemError{UnsupportedRequest: &t}
	case *UnsupportedRequest:
		return &SystemError{UnsupportedRequest: t}
	default:
		return nil
	}
}

// ToSystemError will try to convert the given error to an SystemError.
// This is important to returning any Go error back to Rust.
//
// If it is already StdError, return self.
// If it is an error, which could be a sub-field of StdError, embed it.
// If it is anything else, **return nil**
//
// This may return nil on an unknown error, whereas ToStdError will always create
// a valid error type.
func ToSystemError(err error) *SystemError {
	if isNil(err) {
		return nil
	}

	if result := convertErrorToSystemError(err); result != nil {
		return result
	}

	return convertSpecificError(err)
}

// check if an interface is nil (even if it has type info).
func isNil(i any) bool {
	if i == nil {
		return true
	}
	if reflect.TypeOf(i).Kind() == reflect.Ptr {
		// IsNil panics if you try it on a struct (not a pointer)
		return reflect.ValueOf(i).IsNil()
	}
	// if we aren't a pointer, can't be nil, can we?
	return false
}
