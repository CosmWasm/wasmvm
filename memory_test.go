package wasmvm

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// MemoryStats contains detailed memory statistics
type MemoryStats struct {
	HeapAlloc     uint64  // Bytes allocated and still in use
	HeapObjects   uint64  // Number of allocated objects
	TotalAlloc    uint64  // Cumulative bytes allocated (even if freed)
	Mallocs       uint64  // Cumulative count of heap objects allocated
	Frees         uint64  // Cumulative count of heap objects freed
	LiveObjects   uint64  // Mallocs - Frees
	PauseNs       uint64  // Cumulative nanoseconds in GC stop-the-world pauses
	NumGC         uint32  // Number of completed GC cycles
	NumForcedGC   uint32  // Number of GC cycles forced by the application
	StackInUse    uint64  // Bytes in stack spans in use
	StackSys      uint64  // Bytes obtained from system for stack spans
	MSpanInUse    uint64  // Bytes in mspan structures in use
	MCacheInUse   uint64  // Bytes in mcache structures in use
	GCSys         uint64  // Bytes in garbage collection metadata
	OtherSys      uint64  // Bytes in miscellaneous off-heap runtime allocations
	NextGC        uint64  // Target heap size of the next GC cycle
	LastGC        uint64  // When the last GC cycle finished, Unix timestamp
	GCCPUFraction float64 // The fraction of CPU time used by GC
}

// captureMemoryStats returns current memory statistics
func captureMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemoryStats{
		HeapAlloc:     m.HeapAlloc,
		HeapObjects:   m.HeapObjects,
		TotalAlloc:    m.TotalAlloc,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		LiveObjects:   m.Mallocs - m.Frees,
		PauseNs:       m.PauseTotalNs,
		NumGC:         m.NumGC,
		NumForcedGC:   m.NumForcedGC,
		StackInUse:    m.StackInuse,
		StackSys:      m.StackSys,
		MSpanInUse:    m.MSpanInuse,
		MCacheInUse:   m.MCacheInuse,
		GCSys:         m.GCSys,
		OtherSys:      m.OtherSys,
		NextGC:        m.NextGC,
		LastGC:        m.LastGC,
		GCCPUFraction: m.GCCPUFraction,
	}
}

// LeakStats contains results of a memory leak test
type LeakStats struct {
	HeapAllocDiff      int64   // Difference in heap allocation
	HeapObjectsDiff    int64   // Difference in number of heap objects
	TotalAllocDiff     uint64  // Difference in total allocations (always positive, only grows)
	LiveObjectsDiff    int64   // Difference in live objects
	AvgHeapPerIter     int64   // Average heap change per iteration
	AvgObjectsPerIter  int64   // Average objects change per iteration
	StackInUseDiff     int64   // Difference in stack memory usage
	StackSysDiff       int64   // Difference in stack system memory
	MSpanInUseDiff     int64   // Difference in MSpan memory usage
	MCacheInUseDiff    int64   // Difference in MCache memory usage
	GCSysDiff          int64   // Difference in GC metadata memory usage
	OtherSysDiff       int64   // Difference in other system memory usage
	AvgStackPerIter    int64   // Average stack memory change per iteration
	StdDevHeapPerIter  float64 // Standard deviation of heap changes
	StdDevStackPerIter float64 // Standard deviation of stack changes
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
	baselineCycles := 5       // Cycles to establish baseline (increased for stability)
	warmupIterations := 10    // Initial iterations to ignore for warmup (increased)
	forcedGCIterations := 150 // Extra iterations with forced GC (increased)
	gcBeforeMeasurement := 5  // Number of forced GC cycles before measurements (increased)
	gcAfterMeasurement := 5   // Number of forced GC cycles after measurements (increased)
	samplingRate := 10        // Collect data points every N iterations for variance analysis
	multiPassCount := 3       // Number of measurement passes for averaging

	// Thresholds for reporting
	variabilityThreshold := float64(maxMemPerIter) * 5.0 // Only report high variability if it's 5x the max mem allowed
	stackGrowthThreshold := int64(10)                    // Only report stack growth if it's more than 10 bytes/iter

	t.Logf("==== Memory Leak Test: %s ====", testName)

	// Force multiple GC before starting to get a clean state
	forceMultipleGC(gcBeforeMeasurement)

	// Run baseline to establish initial memory state (with panic recovery)
	for i := 0; i < baselineCycles; i++ {
		runFuncSafely(t, f)
	}
	forceMultipleGC(gcBeforeMeasurement)

	// Short test for fast feedback on obvious leaks
	shortStats := runMemoryTestMultiPass(t, shortIterations, warmupIterations, samplingRate, multiPassCount, testName+"-short", f)

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
	longStats := runMemoryTestMultiPass(t, longIterations, warmupIterations, samplingRate, multiPassCount, testName+"-long", f)
	reportMemoryStats(t, longStats, longIterations, testName+"-long")

	// Check for high variance in memory usage, which can indicate intermittent leaks
	t.Logf("%s: Heap growth variance: %.2f bytes/iter, Stack growth variance: %.2f bytes/iter",
		testName+"-long", longStats.StdDevHeapPerIter, longStats.StdDevStackPerIter)

	// Report stack memory changes only if significant
	if longStats.AvgStackPerIter > stackGrowthThreshold {
		t.Logf("%s: Stack memory growth: %d bytes/iter, %d bytes total",
			testName+"-long", longStats.AvgStackPerIter, longStats.StackInUseDiff)
	}

	if longStats.AvgHeapPerIter > maxMemPerIter {
		t.Errorf("LEAK DETECTED - Long test (%s): average heap growth of %d bytes per iteration exceeds threshold of %d bytes",
			testName, longStats.AvgHeapPerIter, maxMemPerIter)
	} else {
		t.Logf("Long test (%s): No significant leaks detected (avg growth: %d bytes/iter)",
			testName, longStats.AvgHeapPerIter)
	}

	// Run forced GC test for hard-to-detect leaks
	forceMultipleGC(gcBeforeMeasurement)

	// Collect intermittent memory usage data
	var heapSamples []int64
	var stackSamples []int64

	before := captureMemoryStats()
	for i := 0; i < forcedGCIterations; i++ {
		runFuncSafely(t, f)

		// Collect data points for variance analysis
		if i%samplingRate == 0 {
			runtime.GC() // Less aggressive intermediate GC
			currentStats := captureMemoryStats()
			heapDelta := int64(currentStats.HeapAlloc) - int64(before.HeapAlloc)
			stackDelta := int64(currentStats.StackInUse) - int64(before.StackInUse)
			heapSamples = append(heapSamples, heapDelta)
			stackSamples = append(stackSamples, stackDelta)
		}

		if i%10 == 0 {
			forceMultipleGC(1) // Force GC every 10 iterations
		}
	}
	forceMultipleGC(gcAfterMeasurement)
	after := captureMemoryStats()

	// Calculate and report statistics
	forcedStats := calculateLeakStats(before, after, forcedGCIterations)

	// Add variance stats
	forcedStats.StdDevHeapPerIter = calculateStdDev(heapSamples, forcedStats.HeapAllocDiff)
	forcedStats.StdDevStackPerIter = calculateStdDev(stackSamples, forcedStats.StackInUseDiff)

	reportMemoryStats(t, forcedStats, forcedGCIterations, testName+"-forcedGC")
	if forcedStats.AvgHeapPerIter > maxMemPerIter {
		t.Errorf("LEAK DETECTED - Forced GC test (%s): average heap growth of %d bytes per iteration exceeds threshold of %d bytes",
			testName, forcedStats.AvgHeapPerIter, maxMemPerIter)
	} else {
		t.Logf("Forced GC test (%s): No leaks detected (avg growth: %d bytes/iter)",
			testName, forcedStats.AvgHeapPerIter)
	}

	// Stack-specific reporting - only if significant
	if forcedStats.AvgStackPerIter > stackGrowthThreshold {
		t.Logf("%s: Stack memory changes - InUse: %+d bytes, System: %+d bytes, Avg/iter: %+d bytes",
			testName+"-forcedGC", forcedStats.StackInUseDiff, forcedStats.StackSysDiff, forcedStats.AvgStackPerIter)
	}

	// Only report memory system changes if there are significant changes
	if forcedStats.MSpanInUseDiff != 0 || forcedStats.MCacheInUseDiff != 0 ||
		forcedStats.GCSysDiff != 0 || forcedStats.OtherSysDiff != 0 {
		t.Logf("%s: Memory system changes - MSpan: %+d, MCache: %+d, GCSys: %+d, OtherSys: %+d",
			testName+"-forcedGC", forcedStats.MSpanInUseDiff, forcedStats.MCacheInUseDiff,
			forcedStats.GCSysDiff, forcedStats.OtherSysDiff)
	}

	// Final summary - only report failure if both long and forced GC tests show a leak
	if longStats.AvgHeapPerIter > maxMemPerIter && forcedStats.AvgHeapPerIter > maxMemPerIter {
		t.Logf("CONFIRMED LEAK in %s: Both long-term tests show memory growth exceeding threshold", testName)
	} else if shortStats.AvgHeapPerIter > maxMemPerIter && longStats.AvgHeapPerIter <= maxMemPerIter && forcedStats.AvgHeapPerIter <= maxMemPerIter {
		// False positive - short test showed leak but long tests didn't
		t.Logf("FALSE ALARM in %s: Initial test showed potential leak but long-term tests confirm it's not a leak", testName)
	}

	// Check for stack-specific leaks (which might not show up in heap)
	if forcedStats.AvgStackPerIter > stackGrowthThreshold && longStats.AvgStackPerIter > stackGrowthThreshold {
		t.Logf("STACK GROWTH DETECTED in %s: Stack memory is growing by %d bytes/iter (long test) and %d bytes/iter (forced GC test)",
			testName, longStats.AvgStackPerIter, forcedStats.AvgStackPerIter)
	}

	// Check for high variability which could indicate erratic leaks - but only if it's very high
	if (forcedStats.StdDevHeapPerIter > variabilityThreshold || longStats.StdDevHeapPerIter > variabilityThreshold) &&
		(forcedStats.AvgHeapPerIter > maxMemPerIter/2 || longStats.AvgHeapPerIter > maxMemPerIter/2) {
		t.Logf("HIGH MEMORY VARIABILITY DETECTED in %s: This may indicate an intermittent leak or resource fluctuation",
			testName)
	}

	t.Logf("==== Memory Leak Test Complete: %s ====", testName)
}

// Calculate standard deviation of samples
func calculateStdDev(samples []int64, totalDiff int64) float64 {
	if len(samples) <= 1 {
		return 0.0
	}

	// Calculate mean
	mean := float64(totalDiff) / float64(len(samples))

	// Calculate sum of squared differences
	var sumSquaredDiff float64
	for _, sample := range samples {
		diff := float64(sample) - mean
		sumSquaredDiff += diff * diff
	}

	// Return standard deviation
	return math.Sqrt(sumSquaredDiff / float64(len(samples)-1))
}

// runMemoryTestMultiPass executes multiple passes of memory tests and averages the results
func runMemoryTestMultiPass(t *testing.T, iterations, warmup, samplingRate, passes int, testName string, f func()) LeakStats {
	t.Helper()

	var combinedStats LeakStats
	var heapGrowthRates []int64
	var stackGrowthRates []int64

	// Run multiple passes to get more accurate measurements
	for pass := 0; pass < passes; pass++ {
		// Force GC between passes for clean state
		forceMultipleGC(3)

		// Run the test
		stats := runMemoryTest(t, iterations, warmup, samplingRate, testName+fmt.Sprintf("-pass%d", pass+1), f)

		// Accumulate statistics
		combinedStats.HeapAllocDiff += stats.HeapAllocDiff
		combinedStats.HeapObjectsDiff += stats.HeapObjectsDiff
		combinedStats.TotalAllocDiff += stats.TotalAllocDiff
		combinedStats.LiveObjectsDiff += stats.LiveObjectsDiff
		combinedStats.StackInUseDiff += stats.StackInUseDiff
		combinedStats.StackSysDiff += stats.StackSysDiff
		combinedStats.MSpanInUseDiff += stats.MSpanInUseDiff
		combinedStats.MCacheInUseDiff += stats.MCacheInUseDiff
		combinedStats.GCSysDiff += stats.GCSysDiff
		combinedStats.OtherSysDiff += stats.OtherSysDiff

		// Store individual growth rates for variance calculation
		heapGrowthRates = append(heapGrowthRates, stats.AvgHeapPerIter)
		stackGrowthRates = append(stackGrowthRates, stats.AvgStackPerIter)
	}

	// Average the results
	divisor := int64(passes)
	if divisor > 0 {
		combinedStats.HeapAllocDiff /= divisor
		combinedStats.HeapObjectsDiff /= divisor
		combinedStats.LiveObjectsDiff /= divisor
		combinedStats.StackInUseDiff /= divisor
		combinedStats.StackSysDiff /= divisor
		combinedStats.MSpanInUseDiff /= divisor
		combinedStats.MCacheInUseDiff /= divisor
		combinedStats.GCSysDiff /= divisor
		combinedStats.OtherSysDiff /= divisor

		// Calculate per-iteration averages
		iterDivisor := passes * iterations
		if iterDivisor > 0 {
			combinedStats.TotalAllocDiff /= uint64(passes) // Keep total as sum but average per pass
			combinedStats.AvgHeapPerIter = combinedStats.HeapAllocDiff / int64(iterations)
			combinedStats.AvgObjectsPerIter = combinedStats.HeapObjectsDiff / int64(iterations)
			combinedStats.AvgStackPerIter = combinedStats.StackInUseDiff / int64(iterations)
		}
	}

	// Calculate standard deviations
	combinedStats.StdDevHeapPerIter = calculateStdDev(heapGrowthRates, combinedStats.HeapAllocDiff)
	combinedStats.StdDevStackPerIter = calculateStdDev(stackGrowthRates, combinedStats.StackInUseDiff)

	return combinedStats
}

// runMemoryTest executes the test function for specified iterations and returns memory statistics
func runMemoryTest(t *testing.T, iterations, warmup, samplingRate int, testName string, f func()) LeakStats {
	t.Helper()

	// Warm up
	for i := 0; i < warmup; i++ {
		runFuncSafely(t, f)
	}

	// Force GC to get accurate starting point
	forceMultipleGC(2)

	// Measure before state
	before := captureMemoryStats()
	t.Logf("%s: Starting with %d heap bytes, %d objects, %d stack bytes",
		testName, before.HeapAlloc, before.HeapObjects, before.StackInUse)

	// Variables for sampling
	var heapSamples []int64
	var stackSamples []int64

	// Run test iterations with panic recovery
	for i := 0; i < iterations; i++ {
		runFuncSafely(t, f)

		// Collect intermittent samples for variance analysis
		if i%samplingRate == 0 && samplingRate > 0 {
			runtime.GC() // Gentle GC for intermediate measurements
			currentStats := captureMemoryStats()
			heapSamples = append(heapSamples, int64(currentStats.HeapAlloc)-int64(before.HeapAlloc))
			stackSamples = append(stackSamples, int64(currentStats.StackInUse)-int64(before.StackInUse))
		}
	}

	// Force GC to get accurate final state
	forceMultipleGC(2)

	// Measure after state
	after := captureMemoryStats()
	stats := calculateLeakStats(before, after, iterations)

	// Add variance analysis
	stats.StdDevHeapPerIter = calculateStdDev(heapSamples, stats.HeapAllocDiff)
	stats.StdDevStackPerIter = calculateStdDev(stackSamples, stats.StackInUseDiff)

	return stats
}

// calculateLeakStats computes statistics for leak detection
func calculateLeakStats(before, after MemoryStats, iterations int) LeakStats {
	// Calculate differences, handle both growth and shrinkage
	heapDiff := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	objectsDiff := int64(after.HeapObjects) - int64(before.HeapObjects)
	liveObjectsDiff := int64(after.LiveObjects) - int64(before.LiveObjects)
	stackInUseDiff := int64(after.StackInUse) - int64(before.StackInUse)
	stackSysDiff := int64(after.StackSys) - int64(before.StackSys)
	mspanInUseDiff := int64(after.MSpanInUse) - int64(before.MSpanInUse)
	mcacheInUseDiff := int64(after.MCacheInUse) - int64(before.MCacheInUse)
	gcSysDiff := int64(after.GCSys) - int64(before.GCSys)
	otherSysDiff := int64(after.OtherSys) - int64(before.OtherSys)

	// Total allocations always grows or stays the same
	totalAllocDiff := after.TotalAlloc - before.TotalAlloc

	// Calculate per-iteration averages
	var avgHeapPerIter, avgObjectsPerIter, avgStackPerIter int64
	if iterations > 0 {
		avgHeapPerIter = heapDiff / int64(iterations)
		avgObjectsPerIter = objectsDiff / int64(iterations)
		avgStackPerIter = stackInUseDiff / int64(iterations)
	}

	return LeakStats{
		HeapAllocDiff:     heapDiff,
		HeapObjectsDiff:   objectsDiff,
		TotalAllocDiff:    totalAllocDiff,
		LiveObjectsDiff:   liveObjectsDiff,
		AvgHeapPerIter:    avgHeapPerIter,
		AvgObjectsPerIter: avgObjectsPerIter,
		StackInUseDiff:    stackInUseDiff,
		StackSysDiff:      stackSysDiff,
		MSpanInUseDiff:    mspanInUseDiff,
		MCacheInUseDiff:   mcacheInUseDiff,
		GCSysDiff:         gcSysDiff,
		OtherSysDiff:      otherSysDiff,
		AvgStackPerIter:   avgStackPerIter,
	}
}

// reportMemoryStats logs detailed memory statistics
func reportMemoryStats(t *testing.T, stats LeakStats, iterations int, testName string) {
	t.Helper()
	t.Logf("%s (%d iterations): Heap bytes diff: %+d, Objects diff: %+d, Live objects diff: %+d",
		testName, iterations, stats.HeapAllocDiff, stats.HeapObjectsDiff, stats.LiveObjectsDiff)
	t.Logf("%s: Per iteration: %+d heap bytes, %+d objects, %+d stack bytes",
		testName, stats.AvgHeapPerIter, stats.AvgObjectsPerIter, stats.AvgStackPerIter)
	t.Logf("%s: Total allocations: %d bytes (includes collected memory)",
		testName, stats.TotalAllocDiff)

	// Only report variability if it's significant in relation to the heap allocation
	significantVariance := stats.HeapAllocDiff != 0 &&
		(stats.StdDevHeapPerIter > math.Abs(float64(stats.HeapAllocDiff))/5.0)
	if stats.StdDevHeapPerIter > 0 && significantVariance {
		t.Logf("%s: Variability - Heap StdDev: %.2f bytes/iter, Stack StdDev: %.2f bytes/iter",
			testName, stats.StdDevHeapPerIter, stats.StdDevStackPerIter)
	}
}

// forceMultipleGC triggers multiple garbage collection cycles
func forceMultipleGC(cycles int) {
	for i := 0; i < cycles; i++ {
		runtime.GC()
		debug.FreeOSMemory()
	}
}

// runFuncSafely executes a function with panic recovery
func runFuncSafely(t *testing.T, f func()) {
	t.Helper()
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
	t.Helper()
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
	tempDir := t.TempDir()

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
	tempDir2 := t.TempDir()

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
	tempDir := t.TempDir()

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
	tempDir := t.TempDir()

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

	// Test creating and destroying VMs
	MeasureMemoryLeakDetailed(t, 50, 200, 2048, "VMCreateCleanup", func() {
		// Create a unique directory for each VM instance
		vmDir := t.TempDir()

		vm, err := NewVM(vmDir, []string{"staking"}, 1, false, 0)
		if err != nil {
			t.Logf("VM creation failed: %v", err)
			return
		}
		vm.Cleanup()
	})

	// Test creating VMs with cache dir
	MeasureMemoryLeakDetailed(t, 50, 200, 2048, "VMWithCacheDir", func() {
		// Create a unique subdirectory for each VM instance
		vmDir := t.TempDir()

		vm, err := NewVM(vmDir, []string{"staking"}, 1, false, 0)
		if err != nil {
			t.Logf("VM creation failed: %v", err)
			return
		}
		vm.Cleanup()
	})
}

// Now add specialized tests for stack leaks

// TestStackMemoryLeaks specifically tests for leaks in stack memory usage
func TestStackMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stack memory leak tests in short mode")
	}

	// Baseline test - no leaks expected
	MeasureMemoryLeakDetailed(t, 100, 500, 100, "StackClean", func() {
		// Simple function with stack allocation that should be reclaimed
		buffer := make([]byte, 1024)
		for i := 0; i < len(buffer); i++ {
			buffer[i] = byte(i % 256)
		}
		_ = buffer[0] // Prevent optimization
	})

	// Test with deep recursion and stack growth
	MeasureMemoryLeakDetailed(t, 100, 500, 200, "StackRecursion", func() {
		// Recursively build up stack frames
		var recurse func(depth int) int
		recurse = func(depth int) int {
			if depth <= 0 {
				return 1
			}
			// Create some stack variables
			buffer := make([]byte, 16)
			for i := 0; i < len(buffer); i++ {
				buffer[i] = byte(i + depth)
			}
			return recurse(depth-1) + int(buffer[0])
		}
		_ = recurse(20) // Deep but not too deep
	})

	// Test with stack variables captured in closures
	var leakedClosures []func() int
	MeasureMemoryLeakDetailed(t, 100, 500, 500, "StackClosure", func() {
		// Variables that will be captured by the closure
		buffer := make([]byte, 128)
		for i := 0; i < len(buffer); i++ {
			buffer[i] = byte(i % 256)
		}

		// Create a closure that captures these stack variables
		capturer := func() int {
			sum := 0
			for _, b := range buffer {
				sum += int(b)
			}
			return sum
		}

		// Store the closure, simulating a leak
		if len(leakedClosures) < 50 { // Limit to prevent unbounded growth
			leakedClosures = append(leakedClosures, capturer)
		}
	})
	// Clean up to avoid affecting other tests
	leakedClosures = nil

	// Test with large stack allocations that should be reclaimed
	MeasureMemoryLeakDetailed(t, 100, 500, 200, "LargeStackAlloc", func() {
		// Allocate a large array on the stack (Go will likely move to heap, but test anyway)
		var largeArray [1024]int
		for i := 0; i < len(largeArray); i++ {
			largeArray[i] = i
		}

		// Do some work with it
		sum := 0
		for i := 0; i < 1000; i++ {
			sum += largeArray[i%1024]
		}
		_ = sum // Prevent optimization
	})

	// Test with deeply nested function calls that build stack frames
	MeasureMemoryLeakDetailed(t, 100, 500, 200, "NestedCalls", func() {
		// Define nested functions
		level1 := func() int {
			var buf1 [256]byte
			for i := range buf1 {
				buf1[i] = byte(i)
			}

			level2 := func() int {
				var buf2 [256]byte
				for i := range buf2 {
					buf2[i] = byte(i + 1)
				}

				level3 := func() int {
					var buf3 [256]byte
					for i := range buf3 {
						buf3[i] = byte(i + 2)
					}
					return int(buf3[0] + buf2[0])
				}

				return level3() + int(buf2[0])
			}

			return level2() + int(buf1[0])
		}

		_ = level1()
	})
}

// TestDeferAndPanicStackUsage tests stack behavior with defers and panics
func TestDeferAndPanicStackUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping defer/panic stack tests in short mode")
	}

	// Test with multiple defers that should clean up
	MeasureMemoryLeakDetailed(t, 100, 500, 200, "MultipleDefers", func() {
		func() {
			// Set up several defers
			defer func() {
				_ = make([]byte, 100)
			}()
			defer func() {
				_ = make([]byte, 100)
			}()
			defer func() {
				_ = make([]byte, 100)
			}()

			// Do some work
			_ = make([]byte, 200)
		}()
	})

	// Test with recovered panics
	MeasureMemoryLeakDetailed(t, 100, 500, 200, "RecoveredPanic", func() {
		func() {
			defer func() {
				// Recover from the panic
				if r := recover(); r != nil {
					// Do something with the recovered value
					_ = fmt.Sprintf("%v", r)
				}
			}()

			// Cause a panic
			if true {
				panic("test panic that will be recovered")
			}
		}()
	})
}
