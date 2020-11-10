package main

import (
	"fmt"
	wasmvm "github.com/CosmWasm/wasmvm"
	"io/ioutil"
	"os"
)

const SUPPORTED_FEATURES = "staking"
const PRINT_DEBUG = true
const CACHE_SIZE = 100 // MiB

// This is just a demo to ensure we can compile a static go binary
func main() {
	file := os.Args[1]
	fmt.Printf("Running %s...\n", file)
	bz, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded!")

	os.MkdirAll("tmp", 0755)
	vm, err := wasmvm.NewVM("tmp", SUPPORTED_FEATURES, PRINT_DEBUG, CACHE_SIZE)
	if err != nil {
		panic(err)
	}

	id, err := vm.Create(bz)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got code id: %X\n", id)

	vm.Cleanup()
	fmt.Println("finished")
}
