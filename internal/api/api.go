// Package api defines core interfaces and error types for the WASM VM
package api

import (
	"fmt"
)

// ErrorOutOfGas represents an out of gas error with a descriptive message
type ErrorOutOfGas struct {
	Descriptor string
}

// Error implements the error interface
func (e ErrorOutOfGas) Error() string {
	return fmt.Sprintf("out of gas: %s", e.Descriptor)
}
