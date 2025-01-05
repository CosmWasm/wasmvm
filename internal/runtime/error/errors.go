package error

import (
	"fmt"
)

// RuntimeError represents a generic runtime error
type RuntimeError struct {
	Msg string
	Err error
}

func (e *RuntimeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

// GasError represents an error related to gas consumption
type GasError struct {
	Wanted    uint64
	Available uint64
}

func (e *GasError) Error() string {
	return fmt.Sprintf("insufficient gas: required %d, but only %d available", e.Wanted, e.Available)
}

// ToWasmVMError converts internal errors to wasmvm types.SystemError
func ToWasmVMError(err error) error {
	return err
}
