package api

/*
#include "bindings.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"
)

// makeView creates a view into the given byte slice what allows Rust code to read it.
// The byte slice is managed by Go and will be garbage collected. Use runtime.KeepAlive
// to ensure the byte slice lives long enough.
func makeView(s []byte) C.ByteSliceView {
	if s == nil {
		return C.ByteSliceView{is_nil: true, ptr: cu8_ptr(nil), len: cusize(0)}
	}

	// In Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
	// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
	// below with `&data[0]`. https://play.golang.org/p/xvDY3g9OqUk
	if len(s) == 0 {
		return C.ByteSliceView{is_nil: false, ptr: cu8_ptr(nil), len: cusize(0)}
	}

	return C.ByteSliceView{
		is_nil: false,
		ptr:    cu8_ptr(unsafe.Pointer(&s[0])),
		len:    cusize(len(s)),
	}
}

// Creates a C.UnmanagedVector, which cannot be done in test files directly
func constructUnmanagedVector(is_none cbool, ptr cu8_ptr, length cusize, capacity cusize) C.UnmanagedVector {
	return C.UnmanagedVector{
		is_none: is_none,
		ptr:     ptr,
		len:     length,
		cap:     capacity,
	}
}

// uninitializedUnmanagedVector returns an invalid C.UnmanagedVector
// instance. Only use then after someone wrote an instance to it.
func uninitializedUnmanagedVector() C.UnmanagedVector {
	return C.UnmanagedVector{}
}

func newUnmanagedVector(data []byte) C.UnmanagedVector {
	switch {
	case data == nil:
		return C.new_unmanaged_vector(cbool(true), cu8_ptr(nil), cusize(0))
	case len(data) == 0:
		// in Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
		// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
		// below with `&data[0]`.
		// https://play.golang.org/p/xvDY3g9OqUk
		return C.new_unmanaged_vector(cbool(false), cu8_ptr(nil), cusize(0))
	default:
		// This will allocate a proper vector with content and return a description of it
		return C.new_unmanaged_vector(cbool(false), cu8_ptr(unsafe.Pointer(&data[0])), cusize(len(data)))
	}
}

// NOTE: The Rust code provides safer alternatives to UnmanagedVector through functions like:
// - new_safe_unmanaged_vector: Creates a SafeUnmanagedVector that tracks consumption
// - destroy_safe_unmanaged_vector: Safely destroys a SafeUnmanagedVector, preventing double-free
// - store_code_safe and load_wasm_safe: Safer variants of store_code and load_wasm
//
// These functions return opaque pointers to SafeUnmanagedVector structures that need
// specialized functions for accessing their data. To use these in Go, additional
// wrapper functions would need to be created.

// SafeUnmanagedVector is a Go wrapper for the Rust SafeUnmanagedVector
// It provides a safer interface for working with data returned from FFI calls
type SafeUnmanagedVector struct {
	ptr      *C.SafeUnmanagedVector
	consumed bool // Track consumption state in Go to prevent multiple FFI calls
	// Store debug info
	createdAt    string   // Record where vector was created (if debug enabled)
	consumeTrace []string // Stack traces of consumption attempts
}

// Debug flag to enable detailed tracking
var debugSafeVectors = false // Set to true to enable detailed vector debugging

// Stack depth for debug tracing
const debugStackDepth = 10

// captureStack returns a simplified stack trace for debugging
func captureStack() []string {
	if !debugSafeVectors {
		return nil
	}

	stack := make([]uintptr, debugStackDepth)
	length := runtime.Callers(2, stack)
	frames := runtime.CallersFrames(stack[:length])

	var trace []string
	for {
		frame, more := frames.Next()
		// Skip runtime frames
		if !strings.HasPrefix(frame.Function, "runtime.") {
			trace = append(trace, frame.Function)
		}
		if !more {
			break
		}
		if len(trace) >= 5 {
			break // Limit to 5 frames for brevity
		}
	}

	return trace
}

// For tracking total vectors
var (
	totalVectorsCreated  uint64
	totalVectorsConsumed uint64
)

// NewSafeUnmanagedVector creates a new SafeUnmanagedVector from a Go byte slice
// It provides a safer alternative to newUnmanagedVector that tracks consumption
// to prevent double-free issues
func NewSafeUnmanagedVector(data []byte) *SafeUnmanagedVector {
	var ptr *C.SafeUnmanagedVector
	switch {
	case data == nil:
		ptr = C.new_safe_unmanaged_vector(cbool(true), cu8_ptr(nil), cusize(0))
	case len(data) == 0:
		ptr = C.new_safe_unmanaged_vector(cbool(false), cu8_ptr(nil), cusize(0))
	default:
		ptr = C.new_safe_unmanaged_vector(cbool(false), cu8_ptr(unsafe.Pointer(&data[0])), cusize(len(data)))
	}

	atomic.AddUint64(&totalVectorsCreated, 1)

	createdInfo := ""
	if debugSafeVectors {
		if stack := captureStack(); len(stack) > 0 {
			createdInfo = strings.Join(stack, " <- ")
		}
	}

	result := &SafeUnmanagedVector{
		ptr:          ptr,
		consumed:     false,
		createdAt:    createdInfo,
		consumeTrace: nil,
	}
	runtime.SetFinalizer(result, finalizeSafeUnmanagedVector)
	return result
}

// finalizeSafeUnmanagedVector ensures that the Rust SafeUnmanagedVector is properly destroyed
// when the Go wrapper is garbage collected
func finalizeSafeUnmanagedVector(v *SafeUnmanagedVector) {
	// Use atomic operation to check consumed state
	if v.ptr != nil && !v.consumed {
		if debugSafeVectors {
			v.consumeTrace = append(v.consumeTrace, "finalizer")
		}
		C.destroy_safe_unmanaged_vector(v.ptr)
		v.ptr = nil
		v.consumed = true
		atomic.AddUint64(&totalVectorsConsumed, 1)
	} else if debugSafeVectors && v.consumed {
		// Log attempted double consumption in finalizer
		fmt.Printf("WARNING: Finalizer called on already consumed vector created at: %s\n", v.createdAt)
		if len(v.consumeTrace) > 0 {
			fmt.Printf("  Previous consumption(s): %v\n", v.consumeTrace)
		}
	}
}

// IsNone returns true if the SafeUnmanagedVector represents a None value
func (v *SafeUnmanagedVector) IsNone() bool {
	if v.ptr == nil || v.consumed {
		return true
	}
	return bool(C.safe_unmanaged_vector_is_none(v.ptr))
}

// Length returns the length of the data in the SafeUnmanagedVector
// Returns 0 if the vector is None or has been consumed
func (v *SafeUnmanagedVector) Length() int {
	if v.ptr == nil || v.consumed {
		return 0
	}
	return int(C.safe_unmanaged_vector_length(v.ptr))
}

// ToBytesAndDestroy consumes the SafeUnmanagedVector and returns its content as a Go byte slice
// This function destroys the SafeUnmanagedVector, so it can only be called once
func (v *SafeUnmanagedVector) ToBytesAndDestroy() []byte {
	if v.ptr == nil {
		if debugSafeVectors {
			fmt.Printf("WARNING: ToBytesAndDestroy called on nil vector pointer\n")
			if stack := captureStack(); len(stack) > 0 {
				fmt.Printf("  Called from: %v\n", strings.Join(stack, " <- "))
			}
		}
		return nil
	}

	if v.consumed {
		if debugSafeVectors {
			fmt.Printf("WARNING: ToBytesAndDestroy called on already consumed vector created at: %s\n", v.createdAt)
			if len(v.consumeTrace) > 0 {
				fmt.Printf("  Previous consumption(s): %v\n", v.consumeTrace)
			}
			if stack := captureStack(); len(stack) > 0 {
				fmt.Printf("  Called from: %v\n", strings.Join(stack, " <- "))
			}
		}
		return nil
	}

	// Track consumption attempt
	if debugSafeVectors {
		if stack := captureStack(); len(stack) > 0 {
			v.consumeTrace = append(v.consumeTrace, strings.Join(stack, " <- "))
		} else {
			v.consumeTrace = append(v.consumeTrace, "ToBytesAndDestroy")
		}
	}

	// Remove the finalizer first to prevent double destruction
	runtime.SetFinalizer(v, nil)

	// Mark as consumed before any FFI calls to prevent double-free
	v.consumed = true
	atomic.AddUint64(&totalVectorsConsumed, 1)

	var dataPtr *C.uchar
	var dataLen C.uintptr_t

	success := C.safe_unmanaged_vector_to_bytes(v.ptr, &dataPtr, &dataLen)
	if !bool(success) {
		// Error occurred, likely already consumed on Rust side
		return nil
	}

	defer func() {
		v.ptr = nil
	}()

	if dataPtr == nil {
		if bool(C.safe_unmanaged_vector_is_none(v.ptr)) {
			// Was a None value
			C.destroy_safe_unmanaged_vector(v.ptr)
			return nil
		}
		// Was an empty slice
		C.destroy_safe_unmanaged_vector(v.ptr)
		return []byte{}
	}

	// Copy data to Go memory
	bytes := C.GoBytes(unsafe.Pointer(dataPtr), C.int(dataLen))

	// Free the C memory allocated by safe_unmanaged_vector_to_bytes
	C.free(unsafe.Pointer(dataPtr))

	// Destroy the SafeUnmanagedVector
	C.destroy_safe_unmanaged_vector(v.ptr)

	return bytes
}

// SafeStoreCode is a safer version of store_code that uses SafeUnmanagedVector
func SafeStoreCode(cache *C.cache_t, wasm []byte, checked, persist bool, errorMsg *C.UnmanagedVector) *SafeUnmanagedVector {
	view := makeView(wasm)
	ptr := C.store_code_safe(cache, view, cbool(checked), cbool(persist), errorMsg)

	atomic.AddUint64(&totalVectorsCreated, 1)

	createdInfo := ""
	if debugSafeVectors {
		if stack := captureStack(); len(stack) > 0 {
			createdInfo = strings.Join(stack, " <- ")
		}
	}

	result := &SafeUnmanagedVector{
		ptr:          ptr,
		consumed:     false,
		createdAt:    createdInfo,
		consumeTrace: nil,
	}
	runtime.SetFinalizer(result, finalizeSafeUnmanagedVector)
	return result
}

// SafeLoadWasm is a safer version of load_wasm that uses SafeUnmanagedVector
func SafeLoadWasm(cache *C.cache_t, checksum []byte, errorMsg *C.UnmanagedVector) *SafeUnmanagedVector {
	view := makeView(checksum)
	ptr := C.load_wasm_safe(cache, view, errorMsg)

	atomic.AddUint64(&totalVectorsCreated, 1)

	createdInfo := ""
	if debugSafeVectors {
		if stack := captureStack(); len(stack) > 0 {
			createdInfo = strings.Join(stack, " <- ")
		}
	}

	result := &SafeUnmanagedVector{
		ptr:          ptr,
		consumed:     false,
		createdAt:    createdInfo,
		consumeTrace: nil,
	}
	runtime.SetFinalizer(result, finalizeSafeUnmanagedVector)
	return result
}

func copyAndDestroyUnmanagedVector(v C.UnmanagedVector) []byte {
	var out []byte
	switch {
	case bool(v.is_none):
		out = nil
	case v.cap == cusize(0):
		// There is no allocation we can copy
		out = []byte{}
	default:
		// C.GoBytes create a copy (https://stackoverflow.com/a/40950744/2013738)
		out = C.GoBytes(unsafe.Pointer(v.ptr), C.int(v.len))
	}
	C.destroy_unmanaged_vector(v)
	return out
}

func optionalU64ToPtr(val C.OptionalU64) *uint64 {
	if val.is_some {
		return (*uint64)(&val.value)
	}
	return nil
}

// copyU8Slice copies the contents of an Option<&[u8]> that was allocated on the Rust side.
// Returns nil if and only if the source is None.
func copyU8Slice(view C.U8SliceView) []byte {
	if view.is_none {
		return nil
	}
	if view.len == 0 {
		// In this case, we don't want to look into the ptr
		return []byte{}
	}
	// C.GoBytes create a copy (https://stackoverflow.com/a/40950744/2013738)
	res := C.GoBytes(unsafe.Pointer(view.ptr), C.int(view.len))
	return res
}

// GetVectorStats returns information about vector creation and consumption
// This can be helpful for debugging leaks or understanding usage patterns
func GetVectorStats() (created, consumed uint64) {
	return atomic.LoadUint64(&totalVectorsCreated), atomic.LoadUint64(&totalVectorsConsumed)
}

// EnableVectorDebug toggles detailed debugging for safe vectors
func EnableVectorDebug(enable bool) {
	debugSafeVectors = enable
}
