package api

// #include "bindings.h"
import "C"

import (
	"fmt"
)

// LibwasmvmVersion returns the version of the loaded library
// at runtime. This can be used to verify if the loaded version
// matches the expected version.
func LibwasmvmVersion() (string, error) {
	version, err := C.version_number()
	if err != nil {
		return "", err
	}
	patch := version >> 0 & 0xFFFF
	minor := version >> 16 & 0xFFFF
	major := version >> 32 & 0xFFFF
	error := version >> 48 & 0xFFFF
	if error != 0 {
		return "", fmt.Errorf("Error code from version_number call: %d", error)
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}
