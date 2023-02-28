package main

import (
	"fmt"
	"os"

	wasmvm "github.com/CosmWasm/wasmvm"
)

const (
	SupportedFeatures = "staking"
	PrintDebug        = true
	MemoryLimit       = 32  // MiB
	CacheSize         = 100 // MiB
)

// This is just a demo to ensure we can compile a static go binary
func main() {
	file := os.Args[1]
	fmt.Printf("Running %s...\n", file)
	bz, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded!")

	err = os.MkdirAll("tmp", 0o755)
	if err != nil {
		panic(err)
	}
	vm, err := wasmvm.NewVM("tmp", SupportedFeatures, MemoryLimit, PrintDebug, CacheSize)
	if err != nil {
		panic(err)
	}

	checksum, err := vm.StoreCode(bz)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Stored code with checksum: %X\n", checksum)

	vm.Cleanup()
	fmt.Println("finished")
}
