// Package main provides a demo application showcasing the usage of the wasmvm library.
package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	wasmvm "github.com/CosmWasm/wasmvm/v2"
)

// Constants for VM configuration
const (
	// PrintDebug enables debug printing when true
	PrintDebug = true
	// MemoryLimit defines the memory limit in MiB
	MemoryLimit = 32
	// CacheSize defines the cache size in MiB
	CacheSize = 100
	// DefaultDirMode is the default directory permission mode
	DefaultDirMode = 0o755
)

// Constants for exit codes
const (
	ExitSuccess = 0
	ExitError   = 1
)

// Constants for array indices and lengths
const (
	MinArgsLength = 2
	FilePathIndex = 1
)

// SupportedCapabilities defines the list of supported staking capabilities.
var SupportedCapabilities = []string{"staking"}

// exitCode tracks the code that the program will exit with.
var exitCode = 0

// printError prints an error message, sets the exit code, and returns the write error (if any)
func printError(format string, args ...any) error {
	_, err := fmt.Fprintf(os.Stderr, format, args...)
	exitCode = ExitError
	return err // Return potential write error
}

// printInfo prints an informational message and returns the write error (if any)
func printInfo(format string, args ...any) error {
	_, err := fmt.Fprintf(os.Stdout, format, args...)
	return err // Return potential write error
}

// handleVersion prints the libwasmvm version
func handleVersion() error {
	libwasmvmVersion, err := wasmvm.LibwasmvmVersion()
	if err != nil {
		return printError("Error getting libwasmvm version: %v\n", err) // Propagate error
	}
	return printInfo("libwasmvm: %s\n", libwasmvmVersion) // Propagate error
}

// validateFilePath checks if the file path is valid
func validateFilePath(file string) (string, error) {
	cleanPath := filepath.Clean(file)
	if filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		err := errors.New("invalid file path")
		return "", printError("Error: %v\n", err) // Propagate error
	}
	return cleanPath, nil
}

// setupVM creates and initializes the VM
func setupVM() (*wasmvm.VM, error) {
	if err := os.MkdirAll("tmp", DefaultDirMode); err != nil {
		return nil, printError("Error creating tmp directory: %v\n", err) // Propagate error
	}

	vm, err := wasmvm.NewVM("tmp", SupportedCapabilities, MemoryLimit, PrintDebug, CacheSize)
	if err != nil {
		return nil, printError("Error creating VM: %v\n", err) // Propagate error
	}
	return vm, nil
}

// loadAndStoreWasm loads wasm bytecode from a file and stores it in the VM
func loadAndStoreWasm(vm *wasmvm.VM, filePath string) error {
	// Use the validated filePath (cleanPath from main)
	bz, err := os.ReadFile(filePath) //nolint:gosec // Path validated before calling this function
	if err != nil {
		return printError("Error reading file: %v\n", err) // Propagate error
	}
	if err := printInfo("Loaded!\n"); err != nil {
		return err // Handle printInfo error
	}

	checksum, _, err := vm.StoreCode(bz, math.MaxUint64)
	if err != nil {
		return printError("Error storing code: %v\n", err) // Propagate error
	}
	return printInfo("Stored code with checksum: %X\n", checksum) // Propagate error
}

func run() error {
	if len(os.Args) < MinArgsLength {
		return printError("Usage: %s <path-to-wasm-file>\n", os.Args[0])
	}

	file := os.Args[FilePathIndex]

	if file == "version" {
		return handleVersion()
	}

	if err := printInfo("Running %s...\n", file); err != nil {
		return err
	}

	cleanPath, err := validateFilePath(file)
	if err != nil {
		return err
	}

	vm, err := setupVM()
	if err != nil {
		return err
	}
	defer vm.Cleanup()

	if err := loadAndStoreWasm(vm, cleanPath); err != nil {
		return err
	}

	return printInfo("finished\n")
}

// main is the entry point for the demo application that tests wasmvm functionality.
func main() {
	// Defer the os.Exit call until the very end
	defer func() {
		os.Exit(exitCode)
	}()

	// Run the main application logic
	if err := run(); err != nil {
		// If run() returned an error not already printed by printError,
		// print it now. This handles potential fmt.Fprintf errors.
		if exitCode == ExitSuccess { // Check if printError was already called
			if _, printErr := fmt.Fprintf(os.Stderr, "Unhandled error: %v\n", err); printErr != nil {
				// If we can't even print the error, there's not much else to do
				// but at least we tried.
			}
			exitCode = ExitError
		}
		// No return needed here, defer will handle exit
	}
}
