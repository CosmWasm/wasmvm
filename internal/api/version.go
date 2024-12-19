package api

// Just define a constant version here
const wasmvmVersion = "6.9.0"

// LibwasmvmVersion returns the version of this library as a string.
func LibwasmvmVersion() (string, error) {
	// Since we're no longer using cgo, we return the hardcoded version.
	return wasmvmVersion, nil
}
