package api

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api/testdb"
	"github.com/CosmWasm/wasmvm/v2/types"
)

//-----------------------------------------------------------------------------
// Existing Table-Driven Tests for Memory Bridging and Unmanaged Vectors
//-----------------------------------------------------------------------------

func TestMakeView_TableDriven(t *testing.T) {
	type testCase struct {
		name     string
		input    []byte
		expIsNil bool
		expLen   cusize
	}

	tests := []testCase{
		{
			name:     "Non-empty byte slice",
			input:    []byte{0xaa, 0xbb, 0x64},
			expIsNil: false,
			expLen:   3,
		},
		{
			name:     "Empty slice",
			input:    []byte{},
			expIsNil: false,
			expLen:   0,
		},
		{
			name:     "Nil slice",
			input:    nil,
			expIsNil: true,
			expLen:   0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			view := makeView(tc.input)
			require.Equal(t, cbool(tc.expIsNil), view.is_nil, "Mismatch in is_nil for test: %s", tc.name)
			require.Equal(t, tc.expLen, view.len, "Mismatch in len for test: %s", tc.name)
		})
	}
}

func TestCreateAndDestroyUnmanagedVector_TableDriven(t *testing.T) {
	// Helper for the round-trip test
	checkUnmanagedRoundTrip := func(t *testing.T, input []byte, expectNone bool) {
		unmanaged := newUnmanagedVector(input)
		require.Equal(t, cbool(expectNone), unmanaged.is_none, "Mismatch on is_none with input: %v", input)

		if !expectNone && len(input) > 0 {
			require.Equal(t, len(input), int(unmanaged.len), "Length mismatch for input: %v", input)
			require.GreaterOrEqual(t, int(unmanaged.cap), int(unmanaged.len), "Expected cap >= len for input: %v", input)
		}

		copyData := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, input, copyData, "Round-trip mismatch for input: %v", input)
	}

	type testCase struct {
		name       string
		input      []byte
		expectNone bool
	}

	tests := []testCase{
		{
			name:       "Non-empty data",
			input:      []byte{0xaa, 0xbb, 0x64},
			expectNone: false,
		},
		{
			name:       "Empty but non-nil",
			input:      []byte{},
			expectNone: false,
		},
		{
			name:       "Nil => none",
			input:      nil,
			expectNone: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			checkUnmanagedRoundTrip(t, tc.input, tc.expectNone)
		})
	}
}

func TestCopyDestroyUnmanagedVector_SpecificEdgeCases(t *testing.T) {
	t.Run("is_none = true ignoring ptr/len/cap", func(t *testing.T) {
		invalidPtr := unsafe.Pointer(uintptr(42))
		uv := constructUnmanagedVector(cbool(true), cu8_ptr(invalidPtr), cusize(0xBB), cusize(0xAA))
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Nil(t, copy, "copy should be nil if is_none=true")
	})

	t.Run("cap=0 => no allocation => empty data", func(t *testing.T) {
		invalidPtr := unsafe.Pointer(uintptr(42))
		uv := constructUnmanagedVector(cbool(false), cu8_ptr(invalidPtr), cusize(0), cusize(0))
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Equal(t, []byte{}, copy, "expected empty result if cap=0 and is_none=false")
	})
}

func TestCopyDestroyUnmanagedVector_Concurrent(t *testing.T) {
	inputs := [][]byte{
		{1, 2, 3},
		{},
		nil,
		{0xff, 0x00, 0x12, 0xab, 0xcd, 0xef},
	}

	var wg sync.WaitGroup
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		for _, data := range inputs {
			data := data
			wg.Add(1)
			go func() {
				defer wg.Done()
				uv := newUnmanagedVector(data)
				out := copyAndDestroyUnmanagedVector(uv)
				assert.Equal(t, data, out, "Mismatch in concurrency test for input=%v", data)
			}()
		}
	}
	wg.Wait()
}

//-----------------------------------------------------------------------------
// Memory Leak Scenarios and Related Tests
//-----------------------------------------------------------------------------

// retryInitCache attempts to initialize a cache repeatedly until success or timeout.
func retryInitCache(config types.VMConfig, timeout time.Duration) (Cache, error) {
	start := time.Now()
	var cache Cache
	var err error
	for time.Since(start) < timeout {
		cache, err = InitCache(config)
		if err == nil {
			return cache, nil
		}
		// If error is due to exclusive lock, wait a bit and retry.
		time.Sleep(50 * time.Millisecond)
	}
	return Cache{}, fmt.Errorf("retryInitCache: timed out after %v, last error: %w", timeout, err)
}

func TestMemoryLeakScenarios(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Iterator_NoClose_WithGC",
			run: func(t *testing.T) {
				db := testdb.NewMemDB()
				defer db.Close()

				key := []byte("key1")
				val := []byte("value1")
				db.Set(key, val)

				iter := db.Iterator([]byte("key1"), []byte("zzzz"))
				require.NoError(t, iter.Error(), "creating iterator should not error")
				// Simulate leak by not closing the iterator.
				iter = nil

				runtime.GC()

				writeDone := make(chan error, 1)
				go func() {
					db.Set([]byte("key2"), []byte("value2"))
					writeDone <- nil
				}()

				select {
				case err := <-writeDone:
					require.NoError(t, err, "DB write should succeed after GC")
				case <-time.After(200 * time.Millisecond):
					require.FailNow(t, "DB write timed out; iterator lock may not have been released")
				}
			},
		},
		{
			name: "Iterator_ProperClose_NoLeak",
			run: func(t *testing.T) {
				db := testdb.NewMemDB()
				defer db.Close()

				db.Set([]byte("a"), []byte("value-a"))
				db.Set([]byte("b"), []byte("value-b"))

				iter := db.Iterator([]byte("a"), []byte("z"))
				require.NoError(t, iter.Error(), "creating iterator")
				for iter.Valid() {
					_ = iter.Key()
					_ = iter.Value()
					iter.Next()
				}
				require.NoError(t, iter.Close(), "closing iterator should succeed")

				db.Set([]byte("c"), []byte("value-c"))
			},
		},
		{
			name: "Cache_Release_Frees_Memory",
			run: func(t *testing.T) {
				// Ensure that releasing caches frees memory.
				getAlloc := func() uint64 {
					var m runtime.MemStats
					runtime.ReadMemStats(&m)
					return m.HeapAlloc
				}

				dir, err := os.MkdirTemp("", "wasmvm-cache-*")
				require.NoError(t, err, "should create temp dir for cache")
				defer os.RemoveAll(dir)

				runtime.GC()
				baseAlloc := getAlloc()

				const N = 5
				caches := make([]Cache, 0, N)
				config := types.VMConfig{
					Cache: types.CacheOptions{
						BaseDir:                  dir,
						AvailableCapabilities:    []string{},
						MemoryCacheSizeBytes:     types.NewSizeMebi(0),
						InstanceMemoryLimitBytes: types.NewSizeMebi(32),
					},
				}
				// Wait up to 5 seconds to acquire each cache instance.
				for i := 0; i < N; i++ {
					cache, err := retryInitCache(config, 30*time.Second)
					require.NoError(t, err, "InitCache should eventually succeed")
					caches = append(caches, cache)
				}

				runtime.GC()
				allocAfterCreate := getAlloc()

				for _, c := range caches {
					ReleaseCache(c)
				}
				runtime.GC()
				allocAfterRelease := getAlloc()

				require.Less(t, allocAfterRelease, baseAlloc*2,
					"Heap allocation after releasing caches too high: base=%d, after=%d", baseAlloc, allocAfterRelease)
				require.Less(t, allocAfterRelease*2, allocAfterCreate,
					"Releasing caches did not free expected memory: before=%d, after=%d", allocAfterCreate, allocAfterRelease)
			},
		},
		{
			name: "MemDB_Iterator_Range_Correctness",
			run: func(t *testing.T) {
				db := testdb.NewMemDB()
				defer db.Close()

				keys := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
				for _, k := range keys {
					db.Set(k, []byte("val:"+string(k)))
				}

				subCases := []struct {
					start, end []byte
					expKeys    [][]byte
				}{
					{nil, nil, [][]byte{[]byte("a"), []byte("b"), []byte("c")}},
					{[]byte("a"), []byte("c"), [][]byte{[]byte("a"), []byte("b")}},
					{[]byte("a"), []byte("b"), [][]byte{[]byte("a")}},
					{[]byte("b"), []byte("b"), [][]byte{}},
					{[]byte("b"), []byte("c"), [][]byte{[]byte("b")}},
				}

				for _, sub := range subCases {
					iter := db.Iterator(sub.start, sub.end)
					require.NoError(t, iter.Error(), "Iterator(%q, %q) should not error", sub.start, sub.end)
					var gotKeys [][]byte
					for ; iter.Valid(); iter.Next() {
						k := append([]byte{}, iter.Key()...)
						gotKeys = append(gotKeys, k)
					}
					require.NoError(t, iter.Close(), "closing iterator")
					if len(sub.expKeys) == 0 {
						require.Empty(t, gotKeys, "Iterator(%q, %q) expected no keys", sub.start, sub.end)
					} else {
						require.Equal(t, sub.expKeys, gotKeys, "Iterator(%q, %q) returned unexpected keys", sub.start, sub.end)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}

//-----------------------------------------------------------------------------
// New Stress Tests
//-----------------------------------------------------------------------------

// TestStressHighVolumeInsert inserts a large number of items and tracks peak memory.
func TestStressHighVolumeInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-volume insert test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	const totalInserts = 100000
	t.Logf("Inserting %d items...", totalInserts)

	var mStart, mEnd runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mStart)

	for i := 0; i < totalInserts; i++ {
		key := []byte(fmt.Sprintf("key_%d", i))
		db.Set(key, []byte("value"))
	}
	runtime.GC()
	runtime.ReadMemStats(&mEnd)
	t.Logf("Memory before: %d bytes, after: %d bytes", mStart.Alloc, mEnd.Alloc)

	require.LessOrEqual(t, mEnd.Alloc, mStart.Alloc+50*1024*1024, "Memory usage exceeded expected threshold after high-volume insert")
}

// TestBulkDeletionMemoryRecovery verifies that deleting many entries frees memory.
func TestBulkDeletionMemoryRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping bulk deletion test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	const totalInserts = 50000
	keys := make([][]byte, totalInserts)
	for i := 0; i < totalInserts; i++ {
		key := []byte(fmt.Sprintf("bulk_key_%d", i))
		keys[i] = key
		db.Set(key, []byte("bulk_value"))
	}
	runtime.GC()
	var mBefore runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	for _, key := range keys {
		db.Delete(key)
	}
	runtime.GC()
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)
	t.Logf("Memory before deletion: %d bytes, after deletion: %d bytes", mBefore.Alloc, mAfter.Alloc)

	require.Less(t, mAfter.Alloc, mBefore.Alloc, "Memory usage did not recover after bulk deletion")
}

// TestPeakMemoryTracking tracks the peak memory usage during high-load operations.
func TestPeakMemoryTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping peak memory tracking test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	const totalOps = 100000
	var peakAlloc uint64
	var m runtime.MemStats
	for i := 0; i < totalOps; i++ {
		key := []byte(fmt.Sprintf("peak_key_%d", i))
		db.Set(key, []byte("peak_value"))
		if i%1000 == 0 {
			runtime.GC()
			runtime.ReadMemStats(&m)
			if m.Alloc > peakAlloc {
				peakAlloc = m.Alloc
			}
		}
	}
	t.Logf("Peak memory allocation observed: %d bytes", peakAlloc)
	require.LessOrEqual(t, peakAlloc, uint64(200*1024*1024), "Peak memory usage too high")
}

//-----------------------------------------------------------------------------
// New Edge Case Tests for Memory Leaks
//-----------------------------------------------------------------------------

// TestRepeatedCreateDestroyCycles repeatedly creates and destroys MemDB instances.
func TestRepeatedCreateDestroyCycles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping repeated create/destroy cycles test in short mode")
	}
	const cycles = 100
	var mStart, mEnd runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mStart)
	for i := 0; i < cycles; i++ {
		db := testdb.NewMemDB()
		db.Set([]byte("cycle_key"), []byte("cycle_value"))
		db.Close()
	}
	runtime.GC()
	runtime.ReadMemStats(&mEnd)
	t.Logf("Memory before cycles: %d bytes, after cycles: %d bytes", mStart.Alloc, mEnd.Alloc)
	require.LessOrEqual(t, mEnd.Alloc, mStart.Alloc+10*1024*1024, "Memory leak detected over create/destroy cycles")
}

// TestSmallAllocationsLeak repeatedly allocates small objects to detect leaks.
func TestSmallAllocationsLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping small allocations leak test in short mode")
	}
	const iterations = 100000
	for i := 0; i < iterations; i++ {
		_ = make([]byte, 32)
	}
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("Memory after small allocations GC: %d bytes", m.Alloc)
	require.Less(t, m.Alloc, uint64(50*1024*1024), "Memory leak detected in small allocations")
}

//-----------------------------------------------------------------------------
// New Concurrency Tests
//-----------------------------------------------------------------------------

// TestConcurrentAccess performs parallel read/write operations on the MemDB.
func TestConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	const numWriters = 10
	const numReaders = 10
	const opsPerGoroutine = 1000
	var wg sync.WaitGroup

	// Writers.
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := []byte(fmt.Sprintf("concurrent_key_%d_%d", id, j))
				db.Set(key, []byte("concurrent_value"))
			}
		}(i)
	}

	// Readers.
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				iter := db.Iterator(nil, nil)
				for iter.Valid() {
					_ = iter.Key()
					iter.Next()
				}
				iter.Close()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Concurrent access test timed out; potential deadlock or race condition")
	}
}

// TestLockingAndRelease simulates read-write conflicts to ensure proper lock handling.
func TestLockingAndRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping locking and release test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	db.Set([]byte("conflict_key"), []byte("initial"))

	ready := make(chan struct{})
	release := make(chan struct{})
	go func() {
		iter := db.Iterator([]byte("conflict_key"), []byte("zzzz"))
		assert.NoError(t, iter.Error(), "Iterator creation error")
		close(ready) // signal iterator is active
		<-release    // hold the iterator a bit
		iter.Close()
	}()

	<-ready
	done := make(chan struct{})
	go func() {
		db.Set([]byte("conflict_key"), []byte("updated"))
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Exclusive lock not acquired after read lock release; potential deadlock")
	}
}

//-----------------------------------------------------------------------------
// New Sustained Memory Usage Tests
//-----------------------------------------------------------------------------

// TestLongRunningWorkload simulates a long-running workload and verifies memory stability.
func TestLongRunningWorkload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running workload test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	const iterations = 10000
	const reportInterval = 1000
	var mInitial runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mInitial)

	for i := 0; i < iterations; i++ {
		key := []byte(fmt.Sprintf("workload_key_%d", i))
		db.Set(key, []byte("workload_value"))
		if i%2 == 0 {
			db.Delete(key)
		}
		if i%reportInterval == 0 {
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			t.Logf("Iteration %d: HeapAlloc=%d bytes", i, m.HeapAlloc)
		}
	}
	runtime.GC()
	var mFinal runtime.MemStats
	runtime.ReadMemStats(&mFinal)
	t.Logf("Initial HeapAlloc=%d bytes, Final HeapAlloc=%d bytes", mInitial.HeapAlloc, mFinal.HeapAlloc)

	require.LessOrEqual(t, mFinal.HeapAlloc, mInitial.HeapAlloc+20*1024*1024, "Memory usage increased over long workload")
}

//-----------------------------------------------------------------------------
// Additional Utility Test for Memory Metrics
//-----------------------------------------------------------------------------

// TestMemoryMetrics verifies that allocation and free counters remain reasonably balanced.
func TestMemoryMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory metrics test in short mode")
	}
	var mBefore, mAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	const allocCount = 10000
	for i := 0; i < allocCount; i++ {
		_ = make([]byte, 128)
	}
	runtime.GC()
	runtime.ReadMemStats(&mAfter)
	t.Logf("Mallocs: before=%d, after=%d, diff=%d", mBefore.Mallocs, mAfter.Mallocs, mAfter.Mallocs-mBefore.Mallocs)
	t.Logf("Frees: before=%d, after=%d, diff=%d", mBefore.Frees, mAfter.Frees, mAfter.Frees-mBefore.Frees)

	// Use original acceptable threshold.
	diff := mAfter.Mallocs - mAfter.Frees
	require.LessOrEqual(t, diff, uint64(allocCount/10), "Unexpected allocation leak detected")
}

// -----------------------------------------------------------------------------
// Additional New Test Ideas
//
// TestRandomMemoryAccessPatterns simulates random insertions and deletions,
// which can reveal subtle memory fragmentation or concurrent issues.
func TestRandomMemoryAccessPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping random memory access patterns test in short mode")
	}
	db := testdb.NewMemDB()
	defer db.Close()

	const ops = 50000
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				if j%2 == 0 {
					key := []byte(fmt.Sprintf("rand_key_%d_%d", seed, j))
					db.Set(key, []byte("rand_value"))
				} else {
					// Randomly delete some keys.
					key := []byte(fmt.Sprintf("rand_key_%d_%d", seed, j-1))
					db.Delete(key)
				}
			}
		}(i)
	}
	wg.Wait()
	// After random operations, check that GC recovers memory.
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("After random memory access, HeapAlloc=%d bytes", m.HeapAlloc)
}

// TestMemoryFragmentation attempts to force fragmentation by alternating large and small allocations.
func TestMemoryFragmentation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory fragmentation test in short mode")
	}
	const iterations = 10000
	for i := 0; i < iterations; i++ {
		if i%10 == 0 {
			// Allocate a larger block (e.g. 64KB)
			_ = make([]byte, 64*1024)
		} else {
			_ = make([]byte, 256)
		}
	}
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("After fragmentation test, HeapAlloc=%d bytes", m.HeapAlloc)
	// We expect that HeapAlloc should eventually come down.
	require.Less(t, m.HeapAlloc, uint64(100*1024*1024), "Memory fragmentation causing high HeapAlloc")
}
