package api

import (
	"reflect"
	"testing"

	"github.com/CosmWasm/wasmvm/v2/internal/ffi"
)

// Test U8SliceView conversions (using the local type definitions for testing)
type testU8SliceView struct {
	IsNone uint8
	Ptr    *uint8
	Len    uintptr
}

type testUnmanagedVector struct {
	IsNone uint8
	Ptr    *uint8
	Len    uintptr
	Cap    uintptr
}

func TestMakeView(t *testing.T) {
	// Test nil slice
	view := makeView(nil)
	if view.IsNil == 0 {
		t.Errorf("Expected IsNil to be 1 for nil slice, got %d", view.IsNil)
	}

	// Test empty slice
	emptySlice := []byte{}
	view = makeView(emptySlice)
	if view.IsNil != 0 {
		t.Errorf("Expected IsNil to be 0 for empty slice, got %d", view.IsNil)
	}
	if view.Ptr != nil {
		t.Errorf("Expected Ptr to be nil for empty slice, got %p", view.Ptr)
	}
	if view.Len != 0 {
		t.Errorf("Expected Len to be 0 for empty slice, got %d", view.Len)
	}

	// Test non-empty slice
	data := []byte{1, 2, 3}
	view = makeView(data)
	if view.IsNil != 0 {
		t.Errorf("Expected IsNil to be 0 for non-empty slice, got %d", view.IsNil)
	}
	if view.Ptr == nil {
		t.Errorf("Expected Ptr to be non-nil for non-empty slice")
	}
	if view.Len != 3 {
		t.Errorf("Expected Len to be 3, got %d", view.Len)
	}
}

func TestCopyU8Slice(t *testing.T) {
	// Test nil view (represented by IsNil = true in ffi.ByteSliceView)
	noneView := ffi.ByteSliceView{IsNil: true}
	result := copyU8Slice(noneView)
	if result != nil {
		t.Errorf("Expected nil for nil view, got %v", result)
	}

	// Test empty view
	emptyView := ffi.ByteSliceView{IsNil: false, Ptr: nil, Len: 0}
	result = copyU8Slice(emptyView)
	if len(result) != 0 {
		t.Errorf("Expected empty slice for empty view, got %v", result)
	}

	// Test non-empty view
	data := []byte{1, 2, 3}
	ptr := &data[0]
	view := ffi.ByteSliceView{IsNil: false, Ptr: ptr, Len: 3}
	result = copyU8Slice(view)
	if !reflect.DeepEqual(result, data) {
		t.Errorf("Expected %v, got %v", data, result)
	}
}

/* func TestUninitializedUnmanagedVector(t *testing.T) {
	vec := uninitializedUnmanagedVector()
	if vec.IsNone == 0 {
		t.Errorf("Expected IsNone to be 1 for uninitialized vector, got %d", vec.IsNone)
	}
} */

func TestNewUnmanagedVector(t *testing.T) {
	// Test nil data
	vec := newUnmanagedVector(nil)
	if !vec.IsNone { // Compare IsNone (bool) with false
		t.Errorf("Expected IsNone to be true for nil data, got %v", vec.IsNone)
	}

	// Test empty data
	emptyData := []byte{}
	vec = newUnmanagedVector(emptyData)
	if vec.IsNone { // Compare IsNone (bool) with false
		t.Errorf("Expected IsNone to be false for empty data, got %v", vec.IsNone)
	}
	if vec.Ptr != nil {
		t.Errorf("Expected Ptr to be nil for empty data, got %p", vec.Ptr)
	}
	if vec.Len != 0 {
		t.Errorf("Expected Len to be 0 for empty data, got %d", vec.Len)
	}

	// Test non-empty data
	data := []byte{1, 2, 3}
	vec = newUnmanagedVector(data)
	if vec.IsNone { // Compare IsNone (bool) with false
		t.Errorf("Expected IsNone to be false for non-empty data, got %v", vec.IsNone)
	}
	if vec.Ptr == nil {
		t.Errorf("Expected Ptr to be non-nil for non-empty data")
	}
	if vec.Len != 3 {
		t.Errorf("Expected Len to be 3, got %d", vec.Len)
	}
	// Check if Ptr points to the correct data
	if *vec.Ptr != 1 {
		t.Errorf("Expected first element to be 1, got %d", *vec.Ptr)
	}
}

/* func TestCopyAndDestroyUnmanagedVector(t *testing.T) {
	// Test None vector
	noneVec := ffi.UnmanagedVector{IsNone: true}
	result := copyAndDestroyUnmanagedVector(noneVec)
	if result != nil {
		t.Errorf("Expected nil for None vector, got %v", result)
	}

	// Test empty vector
	emptyVec := ffi.UnmanagedVector{IsNone: false, Ptr: nil, Len: 0, Cap: 0}
	result = copyAndDestroyUnmanagedVector(emptyVec)
	if len(result) != 0 {
		t.Errorf("Expected empty slice for empty vector, got %v", result)
	}

	// Test non-empty vector
	data := []byte{1, 2, 3}
	ptr := (*uint8)(unsafe.Pointer(&data[0]))
	vec := ffi.UnmanagedVector{IsNone: false, Ptr: ptr, Len: 3, Cap: 3}
	result = copyAndDestroyUnmanagedVector(vec)
	if !reflect.DeepEqual(result, data) {
		t.Errorf("Expected %v, got %v", data, result)
	}
} */

func TestOptionalU64ToPtr(t *testing.T) {
	// Test Some value
	optSome := OptionalU64{IsSome: 1, Value: 123}
	ptr := optionalU64ToPtr(optSome)
	if ptr == nil || *ptr != 123 {
		t.Errorf("Expected pointer to 123 for Some value, got %v", ptr)
	}

	// Test None value
	optNone := OptionalU64{IsSome: 0}
	ptr = optionalU64ToPtr(optNone)
	if ptr != nil {
		t.Errorf("Expected nil pointer for None value, got %v", ptr)
	}
}
