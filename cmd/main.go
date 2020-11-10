package main

import (
	"fmt"
	wasm "github.com/CosmWasm/go-cosmwasm"
	"io/ioutil"
	"os"
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

	os.MkdirAll("tmp", 0755)
	vm, err := wasm.NewVM("tmp", "staking", true)
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
