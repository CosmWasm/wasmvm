package api

import (
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

//-------------------------------------
// Example tests for memory bridging
//-------------------------------------

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
			require.Equal(t, cbool(tc.expIsNil), view.is_nil,
				"Mismatch in is_nil for test: %s", tc.name)
			require.Equal(t, tc.expLen, view.len,
				"Mismatch in len for test: %s", tc.name)
		})
	}
}

func TestCreateAndDestroyUnmanagedVector_TableDriven(t *testing.T) {
	// Helper for the round-trip test
	checkUnmanagedRoundTrip := func(t *testing.T, input []byte, expectNone bool) {
		unmanaged := newUnmanagedVector(input)
		require.Equal(t, cbool(expectNone), unmanaged.is_none,
			"Mismatch on is_none with input: %v", input)

		if !expectNone && len(input) > 0 {
			require.Equal(t, len(input), int(unmanaged.len),
				"Length mismatch for input: %v", input)
			require.GreaterOrEqual(t, int(unmanaged.cap), int(unmanaged.len),
				"Expected cap >= len for input: %v", input)
		}

		copyData := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, input, copyData,
			"Round-trip mismatch for input: %v", input)
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
		require.Equal(t, []byte{}, copy,
			"expected empty result if cap=0 and is_none=false")
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
				assert.Equal(t, data, out,
					"Mismatch in concurrency test for input=%v", data)
			}()
		}
	}
	wg.Wait()
}

// table-driven tests to highlight potential memory/resource leaks in the API changes.
func TestMemoryLeakScenarios(t *testing.T) {
	// Define test cases in a table-driven style.
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Iterator_NoClose_WithGC",
			run: func(t *testing.T) {
				// This test simulates not closing an iterator and relying on GC to cleanup.
				// Expected behavior: GC triggers any finalizers to release resources (no memory/lock leak).
				// If the iterator is not properly freed (no finalizer), a write will hang, indicating a leak.
				db := testdb.NewMemDB()
				defer db.Close() // ensure DB is closed at end to avoid any persistent resource usage

				// Prepare some data in the DB.
				key := []byte("key1")
				val := []byte("value1")
				db.Set(key, val)

				// Create an iterator over a range including the key. Do NOT close it explicitly.
				iter := db.Iterator([]byte("key1"), []byte("zzzz"))
				require.NoError(t, iter.Error(), "creating iterator should not error")
				// Normally, users must call iter.Close(), but here we simulate a bug (forgetting to close).
				// Remove the only reference to the iterator so it becomes garbage.
				iter = nil

				// Force garbage collection to run any finalizers for the iterator.
				runtime.GC()

				// Now attempt to write to the DB. If the iterator's lock was not freed, this write will block.
				writeDone := make(chan error, 1)
				go func() {
					// Try to set a new key; this will block if the iterator's read-lock is still held (leak).
					db.Set([]byte("key2"), []byte("value2"))
					writeDone <- nil
				}()

				// Wait for the write to complete or time out.
				select {
				case err := <-writeDone:
					// The write completed. We expect it to succeed if no leak (lock freed via finalizer).
					require.NoError(t, err, "DB write should not error after iterator is garbage-collected")
				case <-time.After(200 * time.Millisecond):
					// Timeout: The write likely hung due to an unfreed lock (iterator leak).
					// This indicates a memory/lock leak because the iterator was not closed and not finalized.
					require.FailNow(t,
						"DB write timed out, indicating the iterator's lock was never released (potential leak)",
					)
				}
			},
		},
		{
			name: "Iterator_ProperClose_NoLeak",
			run: func(t *testing.T) {
				// This test ensures that closing the iterator releases resources immediately (no leak).
				db := testdb.NewMemDB()
				defer db.Close()

				// Insert some data.
				db.Set([]byte("a"), []byte("value-a"))
				db.Set([]byte("b"), []byte("value-b"))

				// Create an iterator and then properly close it.
				iter := db.Iterator([]byte("a"), []byte("z"))
				require.NoError(t, iter.Error(), "creating iterator")
				// Consume the iterator (optional: read all items to simulate usage).
				for iter.Valid() {
					_ = iter.Key()   // access key (would be "a", then "b")
					_ = iter.Value() // access value
					iter.Next()
				}
				// Close the iterator to free its resources.
				require.NoError(t, iter.Close(), "closing iterator should succeed")

				// After proper close, a write should proceed without any delay or issue.
				db.Set([]byte("c"), []byte("value-c"))
			},
		},
		{
			name: "Cache_Release_Frees_Memory",
			run: func(t *testing.T) {
				// This test creates and releases VM caches to ensure no RAM leakage.
				// It measures memory usage to verify that releasing the cache frees most allocated memory.
				// If memory is not freed (e.g., missing ReleaseCache call or underlying leak), the test will detect it.
				// Note: This test may be sensitive to GC timing and memory fragmentation, so it allows some slack.

				// Helper to get current memory allocation.
				getAlloc := func() uint64 {
					var m runtime.MemStats
					runtime.ReadMemStats(&m)
					return m.HeapAlloc
				}

				// Use a temporary directory for the cache (simulate disk cache if needed).
				dir, err := os.MkdirTemp("", "wasmvm-cache-*")
				require.NoError(t, err, "should create temp dir for cache")
				defer os.RemoveAll(dir)

				// Baseline memory usage.
				runtime.GC()
				baseAlloc := getAlloc()

				// Number of caches to create for the test.
				const N = 5
				caches := make([]Cache, 0, N) // Cache is the interface for VM cache
				for i := 0; i < N; i++ {
					// Initialize a new cache with small cache size and memory limit for testing.
					config := types.VMConfig{
						Cache: types.CacheOptions{
							BaseDir:                  dir,
							AvailableCapabilities:    []string{},
							MemoryCacheSizeBytes:     types.NewSizeMebi(0),  // no persistent limit
							InstanceMemoryLimitBytes: types.NewSizeMebi(32), // 32 MiB instance memory limit
						},
					}
					cache, err := InitCache(config)
					require.NoError(t, err, "InitCache should succeed")
					caches = append(caches, cache)

					// (Optional) If available, store a small WASM code to force some memory allocation in Rust.
					// In case a compiled contract is needed to amplify memory usage:
					// code := []byte{...} // a small valid WASM module bytes could be inserted here
					// _, _, err = StoreCode(cache, code)
					// require.NoError(t, err, "StoreCode should succeed")
				}

				// Measure memory after creating caches (without releasing yet).
				runtime.GC()
				allocAfterCreate := getAlloc()

				// Release all caches to free their allocated memory.
				for _, c := range caches {
					ReleaseCache(c)
				}

				// Force GC to collect any freed resources.
				runtime.GC()
				allocAfterRelease := getAlloc()

				// There should be no significant increase in allocated heap memory after releasing caches
				// compared to the baseline. We allow some overhead for runtime allocations.
				require.Less(t, allocAfterRelease, baseAlloc*2,
					"Heap allocation after releasing caches is too high, indicating a memory leak in cache release. base=%d, afterRelease=%d",
					baseAlloc, allocAfterRelease,
				)
				// Also, the memory after releasing should be much lower than after creating (most allocated memory freed).
				require.Less(t, allocAfterRelease*2, allocAfterCreate,
					"Releasing caches did not free expected memory: before=%d, after=%d (possible leak in VM cache)",
					allocAfterCreate, allocAfterRelease,
				)
			},
		},
		{
			name: "MemDB_Iterator_Range_Correctness",
			run: func(t *testing.T) {
				// This test ensures the MemDB iterator respects range boundaries (logic correctness, not leak-related).
				// It populates some keys and tests iterator results for various start/end ranges.
				db := testdb.NewMemDB()
				defer db.Close()

				// Insert sample keys.
				keys := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
				for _, k := range keys {
					db.Set(k, []byte("val:"+string(k)))
				}

				// Table of sub-cases for different range queries.
				subCases := []struct {
					start, end []byte
					expKeys    [][]byte
				}{
					// Full range (nil, nil) should return all keys in ascending order.
					{nil, nil, [][]byte{[]byte("a"), []byte("b"), []byte("c")}},
					// Range [a, c) should include "a", "b" (end is exclusive).
					{[]byte("a"), []byte("c"), [][]byte{[]byte("a"), []byte("b")}},
					// Range [a, b) should include "a" only.
					{[]byte("a"), []byte("b"), [][]byte{[]byte("a")}},
					// Range [b, b) should yield nothing (start == end).
					{[]byte("b"), []byte("b"), [][]byte{}},
					// Range [b, c] where end is not strictly greater than start; MemDB expects end exclusive, so treat "c" as exclusive boundary.
					{[]byte("b"), []byte("c"), [][]byte{[]byte("b")}},
				}

				for _, sub := range subCases {
					iter := db.Iterator(sub.start, sub.end)
					require.NoError(t, iter.Error(), "Iterator(%q, %q) should not error", sub.start, sub.end)
					var gotKeys [][]byte
					for ; iter.Valid(); iter.Next() {
						// Copy the key because the underlying key is a pointer into the DB (modifying it would alter the DB).
						k := append([]byte{}, iter.Key()...)
						gotKeys = append(gotKeys, k)
					}
					require.NoError(t, iter.Close(), "closing iterator")
					require.Equal(t, sub.expKeys, gotKeys, "Iterator(%q, %q) returned unexpected keys", sub.start, sub.end)
				}
			},
		},
	}

	// Run each test case as a subtest.
	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}
