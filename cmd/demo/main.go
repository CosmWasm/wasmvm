package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	wasmvm "github.com/CosmWasm/wasmvm/v2"
)

// PrintDebug enables debug printing when true
const (
	PrintDebug = true
	// MemoryLimit defines the memory limit in MiB
	MemoryLimit = 32
	// CacheSize defines the cache size in MiB
	CacheSize = 100
)

// SupportedCapabilities defines the list of supported staking capabilities
var SupportedCapabilities = []string{"staking"}

// exitCode tracks the code that the program will exit with
var exitCode = 0

// main is the entry point for the demo application that tests wasmvm functionality
func main() {
	defer func() {
		os.Exit(exitCode)
	}()

	if len(os.Args) < 2 {
		fmt.Println("Usage: demo <file|version>")
		exitCode = 1
		return
	}

	file := os.Args[1]

	if file == "version" {
		libwasmvmVersion, err := wasmvm.LibwasmvmVersion()
		if err != nil {
			fmt.Printf("Error getting libwasmvm version: %v\n", err)
			exitCode = 1
			return
		}
		fmt.Printf("libwasmvm: %s\n", libwasmvmVersion)
		return
	}

	fmt.Printf("Running %s...\n", file)

	// Validate file path
	cleanPath := filepath.Clean(file)
	if filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		fmt.Println("Error: invalid file path")
		exitCode = 1
		return
	}

	bz, err := os.ReadFile(cleanPath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		exitCode = 1
		return
	}
	fmt.Println("Loaded!")

	err = os.MkdirAll("tmp", 0o750)
	if err != nil {
		fmt.Printf("Error creating tmp directory: %v\n", err)
		exitCode = 1
		return
	}
	vm, err := wasmvm.NewVM("tmp", SupportedCapabilities, MemoryLimit, PrintDebug, CacheSize)
	if err != nil {
		fmt.Printf("Error creating VM: %v\n", err)
		exitCode = 1
		return
	}
	defer vm.Cleanup()

	checksum, _, err := vm.StoreCode(bz, math.MaxUint64)
	if err != nil {
		fmt.Printf("Error storing code: %v\n", err)
		exitCode = 1
		return
	}
	fmt.Printf("Stored code with checksum: %X\n", checksum)

	fmt.Println("finished")
}
