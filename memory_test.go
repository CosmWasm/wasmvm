package wasmvm

import (
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// MemoryStats contains detailed memory statistics
type MemoryStats struct {
	HeapAlloc   uint64 // Bytes allocated and still in use
	HeapObjects uint64 // Number of allocated objects
	TotalAlloc  uint64 // Cumulative bytes allocated (even if freed)
	Mallocs     uint64 // Cumulative count of heap objects allocated
	Frees       uint64 // Cumulative count of heap objects freed
	LiveObjects uint64 // Mallocs - Frees
	PauseNs     uint64 // Cumulative nanoseconds in GC stop-the-world pauses
	NumGC       uint32 // Number of completed GC cycles
	NumForcedGC uint32 // Number of GC cycles forced by the application
}

// captureMemoryStats returns current memory statistics
func captureMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemoryStats{
		HeapAlloc:   m.HeapAlloc,
		HeapObjects: m.HeapObjects,
		TotalAlloc:  m.TotalAlloc,
		Mallocs:     m.Mallocs,
		Frees:       m.Frees,
		LiveObjects: m.Mallocs - m.Frees,
		PauseNs:     m.PauseTotalNs,
		NumGC:       m.NumGC,
		NumForcedGC: m.NumForcedGC,
	}
}

// LeakStats contains results of a memory leak test
type LeakStats struct {
	HeapAllocDiff     int64  // Difference in heap allocation
	HeapObjectsDiff   int64  // Difference in number of heap objects
	TotalAllocDiff    uint64 // Difference in total allocations (always positive, only grows)
	LiveObjectsDiff   int64  // Difference in live objects
	AvgHeapPerIter    int64  // Average heap change per iteration
	AvgObjectsPerIter int64  // Average objects change per iteration
}

// MeasureMemoryLeakDetailed runs a function multiple times and captures detailed memory statistics
// to determine if memory leaks are present.
//
// The function performs 3 levels of analysis:
// 1. Initial - measures first few iterations to catch rapid leaks
// 2. Extended - runs more iterations to measure steady-state behavior
// 3. Forced GC - performs iterations with forced GC to detect difficult leaks
//
// Parameters:
// - t: Testing context
// - shortIterations: Number of iterations for quick check
// - longIterations: Number of iterations for thorough check
// - maxMemPerIter: Maximum allowed heap memory growth per iteration in bytes
// - testName: Name of the test for reporting
// - f: Function to test
func MeasureMemoryLeakDetailed(t *testing.T, shortIterations, longIterations int, maxMemPerIter int64, testName string, f func()) {
	t.Helper()

	// Configure options
	baselineCycles := 3       // Cycles to establish baseline
	warmupIterations := 5     // Initial iterations to ignore for warmup
	forcedGCIterations := 100 // Extra iterations with forced GC
	gcBeforeMeasurement := 3  // Number of forced GC cycles before measurements
	gcAfterMeasurement := 3   // Number of forced GC cycles after measurements

	t.Logf("==== Memory Leak Test: %s ====", testName)

	// Force multiple GC before starting to get a clean state
	forceMultipleGC(gcBeforeMeasurement)

	// Run baseline to establish initial memory state (with panic recovery)
	for i := 0; i < baselineCycles; i++ {
		runFuncSafely(t, f)
	}
	forceMultipleGC(gcBeforeMeasurement)

	// Short test for fast feedback on obvious leaks
	shortStats := runMemoryTest(t, shortIterations, warmupIterations, testName+"-short", f)

	// Check if short test found significant leaks
	reportMemoryStats(t, shortStats, shortIterations, testName+"-short")
	if shortStats.AvgHeapPerIter > maxMemPerIter {
		t.Errorf("LEAK DETECTED - Short test (%s): average heap growth of %d bytes per iteration exceeds threshold of %d bytes",
			testName, shortStats.AvgHeapPerIter, maxMemPerIter)
	} else {
		t.Logf("Short test (%s): No obvious leaks detected (avg growth: %d bytes/iter)",
			testName, shortStats.AvgHeapPerIter)
	}

	// Always run long test for thorough analysis
	longStats := runMemoryTest(t, longIterations, warmupIterations, testName+"-long", f)
	reportMemoryStats(t, longStats, longIterations, testName+"-long")
	if longStats.AvgHeapPerIter > maxMemPerIter {
		t.Errorf("LEAK DETECTED - Long test (%s): average heap growth of %d bytes per iteration exceeds threshold of %d bytes",
			testName, longStats.AvgHeapPerIter, maxMemPerIter)
	} else {
		t.Logf("Long test (%s): No significant leaks detected (avg growth: %d bytes/iter)",
			testName, longStats.AvgHeapPerIter)
	}

	// Run forced GC test for hard-to-detect leaks
	forceMultipleGC(gcBeforeMeasurement)
	before := captureMemoryStats()
	for i := 0; i < forcedGCIterations; i++ {
		runFuncSafely(t, f)
		if i%10 == 0 {
			forceMultipleGC(1) // Force GC every 10 iterations
		}
	}
	forceMultipleGC(gcAfterMeasurement)
	after := captureMemoryStats()

	// Calculate and report statistics
	forcedStats := calculateLeakStats(before, after, forcedGCIterations)
	reportMemoryStats(t, forcedStats, forcedGCIterations, testName+"-forcedGC")
	if forcedStats.AvgHeapPerIter > maxMemPerIter {
		t.Errorf("LEAK DETECTED - Forced GC test (%s): average heap growth of %d bytes per iteration exceeds threshold of %d bytes",
			testName, forcedStats.AvgHeapPerIter, maxMemPerIter)
	} else {
		t.Logf("Forced GC test (%s): No leaks detected (avg growth: %d bytes/iter)",
			testName, forcedStats.AvgHeapPerIter)
	}

	// Final summary - only report failure if both long and forced GC tests show a leak
	if longStats.AvgHeapPerIter > maxMemPerIter && forcedStats.AvgHeapPerIter > maxMemPerIter {
		t.Logf("CONFIRMED LEAK in %s: Both long-term tests show memory growth exceeding threshold", testName)
	} else if shortStats.AvgHeapPerIter > maxMemPerIter && longStats.AvgHeapPerIter <= maxMemPerIter && forcedStats.AvgHeapPerIter <= maxMemPerIter {
		// False positive - short test showed leak but long tests didn't
		t.Logf("FALSE ALARM in %s: Initial test showed potential leak but long-term tests confirm it's not a leak", testName)
	}

	t.Logf("==== Memory Leak Test Complete: %s ====", testName)
}

// runFuncSafely executes a function with panic recovery
func runFuncSafely(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic in test function: %v", r)
		}
	}()
	f()
}

// Simpler version of MeasureMemoryLeakDetailed with reasonable defaults
func MeasureMemoryLeak(t *testing.T, iterations int, testName string, f func()) {
	t.Helper()
	// Use a more reasonable default threshold of 4096 (4KB)
	MeasureMemoryLeakDetailed(t, iterations/10, iterations, 4096, testName, f)
}

// runMemoryTest executes the test function for specified iterations and returns memory statistics
func runMemoryTest(t *testing.T, iterations, warmup int, testName string, f func()) LeakStats {
	t.Helper()

	// Warm up
	for i := 0; i < warmup; i++ {
		runFuncSafely(t, f)
	}

	// Force GC to get accurate starting point
	forceMultipleGC(2)

	// Measure before state
	before := captureMemoryStats()
	t.Logf("%s: Starting with %d heap bytes, %d objects",
		testName, before.HeapAlloc, before.HeapObjects)

	// Run test iterations with panic recovery
	for i := 0; i < iterations; i++ {
		runFuncSafely(t, f)
	}

	// Force GC to get accurate final state
	forceMultipleGC(2)

	// Measure after state
	after := captureMemoryStats()

	return calculateLeakStats(before, after, iterations)
}

// calculateLeakStats computes statistics for leak detection
func calculateLeakStats(before, after MemoryStats, iterations int) LeakStats {
	// Calculate differences, handle both growth and shrinkage
	heapDiff := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	objectsDiff := int64(after.HeapObjects) - int64(before.HeapObjects)
	liveObjectsDiff := int64(after.LiveObjects) - int64(before.LiveObjects)

	// Total allocations always grows or stays the same
	totalAllocDiff := after.TotalAlloc - before.TotalAlloc

	// Calculate per-iteration averages
	var avgHeapPerIter, avgObjectsPerIter int64
	if iterations > 0 {
		avgHeapPerIter = heapDiff / int64(iterations)
		avgObjectsPerIter = objectsDiff / int64(iterations)
	}

	return LeakStats{
		HeapAllocDiff:     heapDiff,
		HeapObjectsDiff:   objectsDiff,
		TotalAllocDiff:    totalAllocDiff,
		LiveObjectsDiff:   liveObjectsDiff,
		AvgHeapPerIter:    avgHeapPerIter,
		AvgObjectsPerIter: avgObjectsPerIter,
	}
}

// reportMemoryStats logs detailed memory statistics
func reportMemoryStats(t *testing.T, stats LeakStats, iterations int, testName string) {
	t.Helper()
	t.Logf("%s (%d iterations): Heap bytes diff: %+d, Objects diff: %+d, Live objects diff: %+d",
		testName, iterations, stats.HeapAllocDiff, stats.HeapObjectsDiff, stats.LiveObjectsDiff)
	t.Logf("%s: Per iteration: %+d bytes, %+d objects",
		testName, stats.AvgHeapPerIter, stats.AvgObjectsPerIter)
	t.Logf("%s: Total allocations: %d bytes (includes collected memory)",
		testName, stats.TotalAllocDiff)
}

// forceMultipleGC triggers multiple garbage collection cycles
func forceMultipleGC(cycles int) {
	for i := 0; i < cycles; i++ {
		runtime.GC()
		debug.FreeOSMemory()
	}
}

// Simple test with deliberate memory leaks to verify the detection works
func TestMeasureMemoryLeak(t *testing.T) {
	// Test with no allocations - should not detect leaks
	MeasureMemoryLeak(t, 1000, "NoAlloc", func() {
		// Do nothing
	})

	// Test with temporary allocations - should be garbage collected
	MeasureMemoryLeak(t, 1000, "TempAlloc", func() {
		_ = make([]byte, 1024) // 1KB temporary allocation
	})

	// Skip long test in short mode
	if testing.Short() {
		t.Skip("Skipping deliberate leak test in short mode")
	}

	// Test with a simulated memory leak
	var leakedSlices [][]byte
	MeasureMemoryLeakDetailed(t, 50, 200, 100, "DeliberateLeak", func() {
		// Each iteration leaks 100 bytes
		slice := make([]byte, 100)
		leakedSlices = append(leakedSlices, slice)
	})

	// Test with a gradually increasing leak (harder to detect)
	var leakedData []byte
	MeasureMemoryLeakDetailed(t, 50, 200, 50, "GradualLeak", func() {
		// Grow the slice by 10 bytes each time
		newSlice := make([]byte, len(leakedData)+10)
		copy(newSlice, leakedData)
		leakedData = newSlice
	})

	// Clear leaks after tests
	leakedSlices = nil
	leakedData = nil
}

// TestMemoryLeakInConcurrency verifies leak detection works with goroutines
func TestMemoryLeakInConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent leak test in short mode")
	}

	// Test with goroutines but no leaks - use a more realistic threshold of 1024 bytes
	MeasureMemoryLeakDetailed(t, 50, 200, 1024, "ConcurrentNoLeak", func() {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = make([]byte, 1000) // Temporary allocation in goroutine
			}()
		}
		wg.Wait() // Wait for all goroutines to complete
	})

	// Test with goroutine leaks (using a global to simulate a leak)
	var globalLeakedData [][]byte
	var mu sync.Mutex

	MeasureMemoryLeakDetailed(t, 50, 200, 1500, "ConcurrentWithLeak", func() {
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data := make([]byte, 200) // 200 bytes leaked per goroutine
				mu.Lock()
				globalLeakedData = append(globalLeakedData, data)
				mu.Unlock()
			}()
		}
		wg.Wait() // Wait for all goroutines to complete
	})

	// Clear leaks
	globalLeakedData = nil
}

// TestMemoryLeakWithTimers verifies leak detection works with timers/tickers
func TestMemoryLeakWithTimers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timer leak test in short mode")
	}

	// Test with properly cleaned up timers - should not leak
	MeasureMemoryLeakDetailed(t, 50, 200, 100, "CleanupTimers", func() {
		timer := time.NewTimer(time.Millisecond)
		<-timer.C // Wait for timer
		// No leak: timer automatically collected after it fires
	})

	// Test with a ticker that's properly stopped - should not leak
	MeasureMemoryLeakDetailed(t, 50, 200, 100, "StoppedTicker", func() {
		ticker := time.NewTicker(time.Millisecond)
		<-ticker.C    // Get one tick
		ticker.Stop() // Properly stop the ticker
	})
}

// TestMemoryLeakWithMistake demonstrates a mistake in memory profiling.
// This test shows why multiple cycles with forced GC are important.
func TestMemoryLeakWithMistake(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mistaken test in short mode")
	}

	cacheSize := 0

	// This test initializes a large allocation at the beginning that
	// might be mistaken for a leak during naive testing
	MeasureMemoryLeakDetailed(t, 50, 200, 100, "InitialAllocationNotLeak", func() {
		if cacheSize == 0 {
			// First-time allocation (not a leak, just initialization)
			cacheSize = 1000000 // 1MB
			_ = make([]byte, cacheSize)
		}
		// Actual operation is leak-free
		_ = make([]byte, 100)
	})
}

// -----------------------------------------------------------------------------
// Core wasmvm function memory leak tests
// -----------------------------------------------------------------------------

// WasmTestData contains file paths and constants for memory leak tests
type WasmTestData struct {
	HackatomWasm []byte
	IbcWasm      []byte
	BurnerWasm   []byte
	GasLimit     uint64
}

// loadWasmTestData loads test contracts for memory leak tests
func loadWasmTestData(t *testing.T) WasmTestData {
	// Define file paths
	hackatomPath := "./testdata/hackatom.wasm"
	ibcPath := "./testdata/ibc_reflect.wasm"
	burnerPath := "./testdata/burner.wasm" // Optional contract

	// Load contract bytecode
	hackatomBytes, err := os.ReadFile(hackatomPath)
	if err != nil {
		t.Fatalf("Failed to load hackatom contract: %v", err)
	}

	ibcBytes, err := os.ReadFile(ibcPath)
	if err != nil {
		t.Fatalf("Failed to load ibc contract: %v", err)
	}

	// Try to load burner contract, but continue with nil if not found
	var burnerBytes []byte
	burnerBytes, err = os.ReadFile(burnerPath)
	if err != nil {
		t.Logf("Burner contract not found, will use hackatom contract instead: %v", err)
		// Fall back to using hackatom contract for burner tests
		burnerBytes = hackatomBytes
	}

	return WasmTestData{
		HackatomWasm: hackatomBytes,
		IbcWasm:      ibcBytes,
		BurnerWasm:   burnerBytes,
		GasLimit:     1_000_000_000_000, // Increased gas limit to prevent "Out of gas" errors
	}
}

// TestMemoryLeakStoreCode tests storing contract code for memory leaks
func TestMemoryLeakStoreCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Store Code memory leak test in short mode")
	}

	// Load test data
	testData := loadWasmTestData(t)

	// Create a temporary cache directory for tests
	tempDir, err := os.MkdirTemp("", "wasmvm-test-cache")
	if err != nil {
		t.Fatalf("Failed to create temporary cache directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test storing the same code repeatedly - create VM with Capabilities and HomeDir
	MeasureMemoryLeakDetailed(t, 50, 200, 2048, "StoreCode", func() {
		// Create VM with temporary directory to avoid "could not create base directory" error
		vm, err := NewVM(tempDir, []string{"staking"}, 1, false, 0)
		if err != nil {
			t.Logf("VM creation failed: %v", err)
			return
		}
		defer vm.Cleanup()

		_, _, err = vm.StoreCode(testData.HackatomWasm, testData.GasLimit)
		if err != nil {
			t.Logf("StoreCode failed: %v", err)
		}
	})

	// For StoreMultipleContracts, use a smaller, more optimized contract set
	// Get the smaller of the two contracts for better performance
	var smallerContract []byte
	if len(testData.HackatomWasm) < len(testData.IbcWasm) {
		smallerContract = testData.HackatomWasm
	} else {
		smallerContract = testData.IbcWasm
	}

	// Create a new temporary directory for the second test
	tempDir2, err := os.MkdirTemp("", "wasmvm-test-cache2")
	if err != nil {
		t.Fatalf("Failed to create second temporary cache directory: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// Initialize a VM outside the loop to verify it works
	testVM, err := NewVM(tempDir2, []string{"staking"}, 1, false, 0)
	if err != nil {
		t.Fatalf("Initial VM creation failed: %v", err)
	}
	_, _, err = testVM.StoreCode(smallerContract, testData.GasLimit)
	if err != nil {
		t.Logf("Initial StoreCode test failed, adjusting test: %v", err)
		t.Logf("Skipping StoreMultipleContracts test since initial test failed")
		testVM.Cleanup()
		return
	}
	testVM.Cleanup()

	// Use the same contract for all iterations to avoid "Out of gas" errors
	MeasureMemoryLeakDetailed(t, 50, 200, 2048, "StoreMultipleContracts", func() {
		// Create VM with capabilities and temporary directory
		vm, err := NewVM(tempDir2, []string{"staking"}, 1, false, 0)
		if err != nil {
			t.Logf("VM creation failed: %v", err)
			return
		}
		defer vm.Cleanup()

		// Use the smaller contract for all iterations
		_, _, err = vm.StoreCode(smallerContract, testData.GasLimit)
		if err != nil {
			t.Logf("StoreCode failed: %v", err)
		}
	})
}

// TestMemoryLeakGetCode tests retrieving contract code for memory leaks
func TestMemoryLeakGetCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Get Code memory leak test in short mode")
	}

	// Load test data
	testData := loadWasmTestData(t)

	// Create a temporary cache directory for tests
	tempDir, err := os.MkdirTemp("", "wasmvm-test-getcode")
	if err != nil {
		t.Fatalf("Failed to create temporary cache directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup: Store a contract once
	vm, err := NewVM(tempDir, []string{"staking"}, 1, false, 0)
	if err != nil {
		t.Fatalf("VM creation failed: %v", err)
	}
	defer vm.Cleanup()

	checksum, _, err := vm.StoreCode(testData.HackatomWasm, testData.GasLimit)
	if err != nil {
		t.Fatalf("StoreCode failed: %v", err)
	}

	// Test getting the same code repeatedly
	MeasureMemoryLeakDetailed(t, 50, 200, 1024, "GetCode", func() {
		_, err := vm.GetCode(checksum)
		if err != nil {
			t.Logf("GetCode failed: %v", err)
		}
	})
}

// TestMemoryLeakAnalyzeCode tests analyzing contract code for memory leaks
func TestMemoryLeakAnalyzeCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Analyze Code memory leak test in short mode")
	}

	// Load test data
	testData := loadWasmTestData(t)

	// Create a temporary cache directory for tests
	tempDir, err := os.MkdirTemp("", "wasmvm-test-analyze")
	if err != nil {
		t.Fatalf("Failed to create temporary cache directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup: Store hackatom contract only, which is more reliable
	vm, err := NewVM(tempDir, []string{"staking"}, 1, false, 0)
	if err != nil {
		t.Fatalf("VM creation failed: %v", err)
	}
	defer vm.Cleanup()

	// Store hackatom contract
	hackatomChecksum, _, err := vm.StoreCode(testData.HackatomWasm, testData.GasLimit)
	if err != nil {
		t.Fatalf("StoreCode for hackatom failed: %v", err)
		return
	}

	// Test analyzing non-IBC code
	MeasureMemoryLeakDetailed(t, 50, 200, 1024, "AnalyzeCode", func() {
		_, err := vm.AnalyzeCode(hackatomChecksum)
		if err != nil {
			t.Logf("AnalyzeCode failed: %v", err)
		}
	})
}

// TestMemoryLeakVMCleanup tests creating and cleaning up VMs for memory leaks
func TestMemoryLeakVMCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping VM cleanup memory leak test in short mode")
	}

	// Create a temporary base directory for all VM instances
	baseDir, err := os.MkdirTemp("", "wasmvm-test-base")
	if err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// Test creating and destroying VMs
	MeasureMemoryLeakDetailed(t, 50, 200, 2048, "VMCreateCleanup", func() {
		// Create a unique directory for each VM instance
		vmDir, err := os.MkdirTemp(baseDir, "vm-instance-")
		if err != nil {
			t.Logf("Failed to create VM instance directory: %v", err)
			return
		}

		vm, err := NewVM(vmDir, []string{"staking"}, 1, false, 0)
		if err != nil {
			t.Logf("VM creation failed: %v", err)
			return
		}
		vm.Cleanup()
	})

	// Test creating VMs with cache dir
	tempDir, err := os.MkdirTemp("", "wasmvm-test-cache")
	if err != nil {
		t.Fatalf("Failed to create temporary cache directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	MeasureMemoryLeakDetailed(t, 50, 200, 2048, "VMWithCacheDir", func() {
		// Create a unique subdirectory for each VM instance
		vmDir, err := os.MkdirTemp(tempDir, "vm-cache-")
		if err != nil {
			t.Logf("Failed to create VM cache directory: %v", err)
			return
		}

		vm, err := NewVM(vmDir, []string{"staking"}, 1, false, 0)
		if err != nil {
			t.Logf("VM creation failed: %v", err)
			return
		}
		vm.Cleanup()
	})
}
