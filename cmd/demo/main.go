package main

import (
	"fmt"
	"io/ioutil"
	"os"

	wasmvm "github.com/CosmWasm/wasmvm"
)

const (
	SUPPORTED_FEATURES = "staking"
	PRINT_DEBUG        = true
	MEMORY_LIMIT       = 32  // MiB
	CACHE_SIZE         = 100 // MiB
)

// This is just a demo to ensure we can compile a static go binary
func main() {
	file := os.Args[1]
	fmt.Printf("Running %s...\n", file)
	bz, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded!")

	os.MkdirAll("tmp", 0o755)
	vm, err := wasmvm.NewVM("tmp", SUPPORTED_FEATURES, MEMORY_LIMIT, PRINT_DEBUG, CACHE_SIZE)
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
