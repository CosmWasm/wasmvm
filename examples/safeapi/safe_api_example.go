package main

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
)

// Demonstrate how to use the SafeUnmanagedVector functions to prevent double-free issues
func SafeVectorExample() {
	fmt.Println("Example of using safer FFI functions")

	// Enable vector debugging to track consumption
	api.EnableVectorDebug(true)

	// Example data to process
	testData := []byte("Test data for safer FFI functions")

	// Create a vector
	safeVector := api.NewSafeUnmanagedVector(testData)
	fmt.Printf("Created vector with length: %d\n", safeVector.Length())

	// Example of consuming the data safely
	bytes := safeVector.ToBytesAndDestroy()
	fmt.Printf("Vector data: %s\n", string(bytes))

	// Example of using the vector conversion function
	// This would typically be used with vectors returned from FFI calls
	safeVector2 := api.NewSafeUnmanagedVector([]byte("Another test vector"))
	fmt.Printf("Second vector length: %d\n", safeVector2.Length())

	// Get data from second vector
	bytes2 := safeVector2.ToBytesAndDestroy()
	fmt.Printf("Second vector data: %s\n", string(bytes2))

	// Check vector stats
	created, consumed := api.GetVectorStats()
	fmt.Printf("Vector stats - Created: %d, Consumed: %d\n", created, consumed)

	fmt.Println("This approach prevents double-free issues commonly seen in FFI code")
}

func main() {
	SafeVectorExample()
}
