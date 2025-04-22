package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/CosmWasm/wasmvm/v2/internal/api"
)

func main() {
	// Enable detailed vector debugging
	api.EnableVectorDebug(true)

	// Create a simple buffer to test with
	testData := []byte("test data for vector debugging")

	// Create a vector
	vector := api.NewSafeUnmanagedVector(testData)
	fmt.Printf("Created vector with length: %d\n", vector.Length())

	// First consumption - should work fine
	bytes := vector.ToBytesAndDestroy()
	fmt.Printf("First consumption result: %s\n", string(bytes))

	// Second consumption - should fail with warning
	bytes = vector.ToBytesAndDestroy()
	if bytes == nil {
		fmt.Println("Second consumption correctly returned nil")
	}

	// Create more vectors to see counter behavior
	vector1 := api.NewSafeUnmanagedVector([]byte("vector 1"))
	vector2 := api.NewSafeUnmanagedVector([]byte("vector 2"))
	vector3 := api.NewSafeUnmanagedVector([]byte("vector 3"))

	// Only consume some of them
	vector1.ToBytesAndDestroy()
	vector3.ToBytesAndDestroy()

	// Check stats
	created, consumed := api.GetVectorStats()
	fmt.Printf("Vector stats - Created: %d, Consumed: %d\n", created, consumed)

	// Make sure vector2 is used (to avoid linter warnings)
	fmt.Printf("Vector 2 length: %d\n", vector2.Length())

	// Trigger GC to demonstrate finalizer behavior
	fmt.Println("Triggering garbage collection to demonstrate finalizer...")
	runtime.GC()

	// Wait briefly for finalizers to run
	time.Sleep(100 * time.Millisecond)

	// Check stats again after GC
	created, consumed = api.GetVectorStats()
	fmt.Printf("Vector stats after GC - Created: %d, Consumed: %d\n", created, consumed)
}
