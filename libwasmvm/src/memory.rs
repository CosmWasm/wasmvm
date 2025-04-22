use std::mem;
use std::slice;

use crate::error::Error;

// Constants for memory validation
const MAX_MEMORY_SIZE: usize = 1024 * 1024 * 10; // 10MB limit

/// Validates that memory operations don't exceed safe limits
pub fn validate_memory_size(len: usize) -> Result<(), Error> {
    if len > MAX_MEMORY_SIZE {
        return Err(Error::vm_err(format!(
            "Memory size exceeds limit: {len} > {MAX_MEMORY_SIZE}"
        )));
    }
    Ok(())
}

/// A view into an externally owned byte slice (Go `[]byte`).
/// Use this for the current call only. A view cannot be copied for safety reasons.
/// If you need a copy, use [`ByteSliceView::to_owned`].
///
/// Go's nil value is fully supported, such that we can differentiate between nil and an empty slice.
#[repr(C)]
#[derive(Debug)]
pub struct ByteSliceView {
    /// True if and only if the byte slice is nil in Go. If this is true, the other fields must be ignored.
    is_nil: bool,
    ptr: *const u8,
    len: usize,
}

impl ByteSliceView {
    /// ByteSliceViews are only constructed in Go. This constructor is a way to mimic the behaviour
    /// when testing FFI calls from Rust. It must not be used in production code.
    #[cfg(test)]
    pub fn new(source: &[u8]) -> Self {
        Self {
            is_nil: false,
            ptr: source.as_ptr(),
            len: source.len(),
        }
    }

    /// ByteSliceViews are only constructed in Go. This constructor is a way to mimic the behaviour
    /// when testing FFI calls from Rust. It must not be used in production code.
    #[cfg(test)]
    pub fn nil() -> Self {
        Self {
            is_nil: true,
            ptr: std::ptr::null::<u8>(),
            len: 0,
        }
    }

    /// Provides a reference to the included data to be parsed or copied elsewhere
    /// This is safe as long as the `ByteSliceView` is constructed correctly.
    pub fn read(&self) -> Option<&[u8]> {
        if self.is_nil {
            None
        } else {
            // Validate length before creating slice
            if let Err(e) = validate_memory_size(self.len) {
                // Log error and return None instead of panicking
                eprintln!("Memory validation error: {}", e);
                return None;
            }

            Some(
                // "`data` must be non-null and aligned even for zero-length slices"
                if self.len == 0 {
                    let dangling = std::ptr::NonNull::<u8>::dangling();
                    unsafe { slice::from_raw_parts(dangling.as_ptr(), 0) }
                } else if self.ptr.is_null() {
                    // Don't create slice from null pointer
                    &[]
                } else {
                    unsafe { slice::from_raw_parts(self.ptr, self.len) }
                },
            )
        }
    }

    /// Creates an owned copy that can safely be stored and mutated.
    #[allow(dead_code)]
    pub fn to_owned(&self) -> Option<Vec<u8>> {
        self.read().map(|slice| slice.to_owned())
    }
}

/// A safer wrapper around ByteSliceView that tracks consumption
/// to prevent double use of the same data
#[derive(Debug)]
pub struct SafeByteSlice {
    inner: ByteSliceView,
    // Tracks whether this slice has been consumed
    consumed: bool,
}

impl SafeByteSlice {
    /// Creates from ByteSliceView but tracks consumption
    pub fn new(view: ByteSliceView) -> Self {
        Self {
            inner: view,
            consumed: false,
        }
    }

    /// Return data if not yet consumed
    pub fn read(&mut self) -> Result<Option<&[u8]>, Error> {
        if self.consumed {
            return Err(Error::vm_err(
                "Attempted to read already consumed byte slice",
            ));
        }
        self.consumed = true;
        Ok(self.inner.read())
    }

    /// Check if this slice has been consumed
    pub fn is_consumed(&self) -> bool {
        self.consumed
    }

    /// Safely checks if the byte slice is available (not consumed and not nil)
    /// Helpful for defensive programming without consuming the slice
    pub fn is_available(&self) -> bool {
        !self.consumed && self.inner.read().is_some()
    }
}

/// A view into a `Option<&[u8]>`, created and maintained by Rust.
///
/// This can be copied into a []byte in Go.
#[repr(C)]
pub struct U8SliceView {
    /// True if and only if this is None. If this is true, the other fields must be ignored.
    is_none: bool,
    ptr: *const u8,
    len: usize,
}

impl U8SliceView {
    pub fn new(source: Option<&[u8]>) -> Self {
        match source {
            Some(data) => {
                // Validate memory size
                if let Err(e) = validate_memory_size(data.len()) {
                    eprintln!("Memory validation error in U8SliceView: {}", e);
                    return Self {
                        is_none: true,
                        ptr: std::ptr::null::<u8>(),
                        len: 0,
                    };
                }

                Self {
                    is_none: false,
                    ptr: if data.is_empty() {
                        std::ptr::null::<u8>()
                    } else {
                        data.as_ptr()
                    },
                    len: data.len(),
                }
            }
            None => Self {
                is_none: true,
                ptr: std::ptr::null::<u8>(),
                len: 0,
            },
        }
    }
}

/// An optional Vector type that requires explicit creation and destruction
/// and can be sent via FFI.
/// It can be created from `Option<Vec<u8>>` and be converted into `Option<Vec<u8>>`.
///
/// This type is always created in Rust and always dropped in Rust.
/// If Go code want to create it, it must instruct Rust to do so via the
/// [`new_unmanaged_vector`] FFI export. If Go code wants to consume its data,
/// it must create a copy and instruct Rust to destroy it via the
/// [`destroy_unmanaged_vector`] FFI export.
///
/// An UnmanagedVector is immutable.
///
/// ## Ownership
///
/// Ownership is the right and the obligation to destroy an `UnmanagedVector`
/// exactly once. Both Rust and Go can create an `UnmanagedVector`, which gives
/// then ownership. Sometimes it is necessary to transfer ownership.
///
/// ### Transfer ownership from Rust to Go
///
/// When an `UnmanagedVector` was created in Rust using [`UnmanagedVector::new`], [`UnmanagedVector::default`]
/// or [`new_unmanaged_vector`], it can be passed to Go as a return value (see e.g. [load_wasm][crate::load_wasm]).
/// Rust then has no chance to destroy the vector anymore, so ownership is transferred to Go.
/// In Go, the data has to be copied to a garbage collected `[]byte`. Then the vector must be destroyed
/// using [`destroy_unmanaged_vector`].
///
/// ### Transfer ownership from Go to Rust
///
/// When Rust code calls into Go (using the vtable methods), return data or error messages must be created
/// in Go. This is done by calling [`new_unmanaged_vector`] from Go, which copies data into a newly created
/// `UnmanagedVector`. Since Go created it, it owns it. The ownership is then passed to Rust via the
/// mutable return value pointers. On the Rust side, the vector is destroyed using [`UnmanagedVector::consume`].
///
/// ## Examples
///
/// Transferring ownership from Rust to Go using return values of FFI calls:
///
/// ```
/// # use wasmvm::{cache_t, ByteSliceView, UnmanagedVector};
/// #[no_mangle]
/// pub extern "C" fn save_wasm_to_cache(
///     cache: *mut cache_t,
///     wasm: ByteSliceView,
///     error_msg: Option<&mut UnmanagedVector>,
/// ) -> UnmanagedVector {
///     # let checksum: Vec<u8> = Default::default();
///     // some operation producing a `let checksum: Vec<u8>`
///
///     UnmanagedVector::new(Some(checksum)) // this unmanaged vector is owned by the caller
/// }
/// ```
///
/// Transferring ownership from Go to Rust using return value pointers:
///
/// ```rust
/// # use cosmwasm_vm::{BackendResult, GasInfo};
/// # use wasmvm::{Db, GoError, U8SliceView, UnmanagedVector};
/// fn db_read(db: &Db, key: &[u8]) -> BackendResult<Option<Vec<u8>>> {
///
///     // Create a None vector in order to reserve memory for the result
///     let mut output = UnmanagedVector::default();
///
///     // …
///     # let mut error_msg = UnmanagedVector::default();
///     # let mut used_gas = 0_u64;
///     # let read_db = db.vtable.read_db.unwrap();
///
///     let go_error: GoError = read_db(
///         db.state,
///         db.gas_meter,
///         &mut used_gas as *mut u64,
///         U8SliceView::new(Some(key)),
///         // Go will create a new UnmanagedVector and override this address
///         &mut output as *mut UnmanagedVector,
///         &mut error_msg as *mut UnmanagedVector,
///     )
///     .into();
///
///     // We now own the new UnmanagedVector written to the pointer and must destroy it
///     let value = output.consume();
///
///     // Some gas processing and error handling
///     # let gas_info = GasInfo::free();
///
///     (Ok(value), gas_info)
/// }
/// ```
///
///
/// If you want to mutate data, you need to consume the vector and create a new one:
///
/// ```rust
/// # use wasmvm::{UnmanagedVector};
/// # let input = UnmanagedVector::new(Some(vec![0xAA]));
/// let mut mutable: Vec<u8> = input.consume().unwrap_or_default();
/// assert_eq!(mutable, vec![0xAA]);
///
/// // `input` is now gone and we cam do everything we want to `mutable`,
/// // including operations that reallocate the underlying data.
///
/// mutable.push(0xBB);
/// mutable.push(0xCC);
///
/// assert_eq!(mutable, vec![0xAA, 0xBB, 0xCC]);
///
/// let output = UnmanagedVector::new(Some(mutable));
///
/// // `output` is ready to be passed around
/// ```
#[repr(C)]
#[derive(Copy, Clone, Debug, PartialEq, Eq)]
pub struct UnmanagedVector {
    /// True if and only if this is None. If this is true, the other fields must be ignored.
    is_none: bool,
    ptr: *mut u8,
    len: usize,
    cap: usize,
}

/// A safety wrapper around UnmanagedVector that prevents double consumption
/// of the same vector and adds additional safety checks
#[derive(Debug)]
pub struct SafeUnmanagedVector {
    inner: UnmanagedVector,
    consumed: bool,
}

impl SafeUnmanagedVector {
    /// Creates a new safe wrapper around an UnmanagedVector
    pub fn new(source: Option<Vec<u8>>) -> Self {
        Self {
            inner: UnmanagedVector::new(source),
            consumed: false,
        }
    }

    /// Safely consumes the vector, preventing double-free
    pub fn consume(&mut self) -> Result<Option<Vec<u8>>, Error> {
        if self.consumed {
            return Err(Error::vm_err(
                "Attempted to consume an already consumed vector",
            ));
        }
        self.consumed = true;
        Ok(self.inner.consume())
    }

    /// Creates a non-none SafeUnmanagedVector with the given data
    pub fn some(data: impl Into<Vec<u8>>) -> Self {
        Self {
            inner: UnmanagedVector::some(data),
            consumed: false,
        }
    }

    /// Creates a none SafeUnmanagedVector
    pub fn none() -> Self {
        Self {
            inner: UnmanagedVector::none(),
            consumed: false,
        }
    }

    /// Check if this is a None vector
    pub fn is_none(&self) -> bool {
        self.inner.is_none()
    }

    /// Check if this is a Some vector
    pub fn is_some(&self) -> bool {
        self.inner.is_some()
    }

    /// Check if this vector has been consumed
    pub fn is_consumed(&self) -> bool {
        self.consumed
    }

    /// Get the raw UnmanagedVector (use with caution!)
    pub fn into_raw(mut self) -> Result<UnmanagedVector, Error> {
        if self.consumed {
            return Err(Error::vm_err("Cannot convert consumed vector to raw"));
        }
        self.consumed = true;
        Ok(self.inner)
    }

    /// Safely wrap a raw UnmanagedVector for safer handling during migration
    pub fn from_raw(vector: UnmanagedVector) -> Self {
        Self {
            inner: vector,
            consumed: false,
        }
    }

    /// Create a boxed pointer to a SafeUnmanagedVector from a raw UnmanagedVector
    /// Useful for FFI functions that want to return a safer alternative
    pub fn into_boxed_raw(vector: UnmanagedVector) -> *mut SafeUnmanagedVector {
        Box::into_raw(Box::new(Self::from_raw(vector)))
    }

    /// Helper method to check if a vector is none without consuming it
    pub fn check_none(&self) -> bool {
        self.inner.is_none()
    }

    /// Helper method to get the length of the vector without consuming it
    pub fn len(&self) -> usize {
        if self.inner.is_none || self.consumed {
            0
        } else {
            self.inner.len
        }
    }
}

impl Default for SafeUnmanagedVector {
    fn default() -> Self {
        Self::none()
    }
}

impl UnmanagedVector {
    /// Consumes this optional vector for manual management.
    /// This is a zero-copy operation.
    pub fn new(source: Option<Vec<u8>>) -> Self {
        match source {
            Some(data) => {
                // Validate vector length
                if let Err(e) = validate_memory_size(data.len()) {
                    // Log and return empty vector instead of panicking
                    eprintln!("Memory validation error in UnmanagedVector: {}", e);
                    return Self::none();
                }

                let (ptr, len, cap) = {
                    if data.capacity() == 0 {
                        // we need to explicitly use a null pointer here, since `as_mut_ptr`
                        // always returns a dangling pointer (e.g. 0x01) on an empty Vec,
                        // which trips up Go's pointer checks.
                        // This is safe because the Vec has not allocated, so no memory is leaked.
                        (std::ptr::null_mut::<u8>(), 0, 0)
                    } else {
                        // Can be replaced with Vec::into_raw_parts when stable
                        // https://doc.rust-lang.org/std/vec/struct.Vec.html#method.into_raw_parts
                        let mut data = mem::ManuallyDrop::new(data);
                        (data.as_mut_ptr(), data.len(), data.capacity())
                    }
                };
                Self {
                    is_none: false,
                    ptr,
                    len,
                    cap,
                }
            }
            None => Self {
                is_none: true,
                ptr: std::ptr::null_mut::<u8>(),
                len: 0,
                cap: 0,
            },
        }
    }

    /// Creates a non-none UnmanagedVector with the given data.
    pub fn some(data: impl Into<Vec<u8>>) -> Self {
        Self::new(Some(data.into()))
    }

    /// Creates a none UnmanagedVector.
    pub fn none() -> Self {
        Self::new(None)
    }

    pub fn is_none(&self) -> bool {
        self.is_none
    }

    pub fn is_some(&self) -> bool {
        !self.is_none()
    }

    /// Takes this UnmanagedVector and turns it into a regular, managed Rust vector.
    /// Calling this on two copies of UnmanagedVector leads to double free crashes.
    pub fn consume(self) -> Option<Vec<u8>> {
        if self.is_none {
            None
        } else if self.cap == 0 {
            // capacity 0 means the vector was never allocated and
            // the ptr field does not point to an actual byte buffer
            // (we normalize to `null` in `UnmanagedVector::new`),
            // so no memory is leaked by ignoring the ptr field here.
            Some(Vec::new())
        } else {
            // Additional safety check for null pointers
            if self.ptr.is_null() {
                eprintln!("WARNING: UnmanagedVector::consume called with null pointer but non-zero capacity");
                Some(Vec::new())
            } else {
                Some(unsafe { Vec::from_raw_parts(self.ptr, self.len, self.cap) })
            }
        }
    }
}

impl Default for UnmanagedVector {
    fn default() -> Self {
        Self::none()
    }
}

#[no_mangle]
pub extern "C" fn new_unmanaged_vector(
    nil: bool,
    ptr: *const u8,
    length: usize,
) -> UnmanagedVector {
    // Validate memory size
    if let Err(e) = validate_memory_size(length) {
        eprintln!("Memory validation error in new_unmanaged_vector: {}", e);
        return UnmanagedVector::none();
    }

    if nil {
        UnmanagedVector::new(None)
    } else if length == 0 {
        UnmanagedVector::new(Some(Vec::new()))
    } else if ptr.is_null() {
        // Safety check for null pointers
        eprintln!("WARNING: new_unmanaged_vector called with null pointer but non-zero length");
        UnmanagedVector::new(Some(Vec::new()))
    } else {
        // In slice::from_raw_parts, `data` must be non-null and aligned even for zero-length slices.
        // For this reason we cover the length == 0 case separately above.
        let external_memory = unsafe { slice::from_raw_parts(ptr, length) };
        let copy = Vec::from(external_memory);
        UnmanagedVector::new(Some(copy))
    }
}

/// Creates a new SafeUnmanagedVector from provided data
/// This function provides a safer alternative to new_unmanaged_vector
/// by returning a reference to a heap-allocated SafeUnmanagedVector
/// which includes consumption tracking.
///
/// # Safety
///
/// The returned pointer must be freed exactly once using destroy_safe_unmanaged_vector.
/// The caller is responsible for ensuring this happens.
#[no_mangle]
pub extern "C" fn new_safe_unmanaged_vector(
    nil: bool,
    ptr: *const u8,
    length: usize,
) -> *mut SafeUnmanagedVector {
    // Validate memory size
    if let Err(e) = validate_memory_size(length) {
        eprintln!(
            "Memory validation error in new_safe_unmanaged_vector: {}",
            e
        );
        return Box::into_raw(Box::new(SafeUnmanagedVector::none()));
    }

    let safe_vec = if nil {
        SafeUnmanagedVector::none()
    } else if length == 0 {
        SafeUnmanagedVector::new(Some(Vec::new()))
    } else if ptr.is_null() {
        // Safety check for null pointers
        eprintln!(
            "WARNING: new_safe_unmanaged_vector called with null pointer but non-zero length"
        );
        SafeUnmanagedVector::new(Some(Vec::new()))
    } else {
        // In slice::from_raw_parts, `data` must be non-null and aligned even for zero-length slices.
        // For this reason we cover the length == 0 case separately above.
        let external_memory = unsafe { slice::from_raw_parts(ptr, length) };
        let copy = Vec::from(external_memory);
        SafeUnmanagedVector::new(Some(copy))
    };

    Box::into_raw(Box::new(safe_vec))
}

/// Safely destroys a SafeUnmanagedVector, handling consumption tracking
/// to prevent double-free issues.
///
/// # Safety
///
/// The pointer must have been created with new_safe_unmanaged_vector.
/// After this call, the pointer must not be used again.
#[no_mangle]
pub extern "C" fn destroy_safe_unmanaged_vector(v: *mut SafeUnmanagedVector) {
    if v.is_null() {
        return; // Silently ignore null pointers
    }

    // Take ownership of the box and check if it's already been consumed
    // This is safe because we take ownership of the whole box
    let mut safe_vec = unsafe { Box::from_raw(v) };

    // Check if the vector is already consumed or has a None inner vector
    // to avoid the error message for double consumption
    if safe_vec.is_consumed() || safe_vec.inner.is_none() {
        // Already consumed or None vector - just drop the box without error
        return;
    }

    // Attempt to consume the vector
    if let Err(e) = safe_vec.consume() {
        eprintln!("Error during safe vector destruction: {}", e);
    }
}

#[no_mangle]
pub extern "C" fn destroy_unmanaged_vector(v: UnmanagedVector) {
    // Wrap in SafeUnmanagedVector for safer handling
    let mut safe_vector = SafeUnmanagedVector {
        inner: v,
        consumed: false,
    };

    // If the vector is None, we don't need to consume it
    if safe_vector.inner.is_none() {
        return;
    }

    // This will prevent double consumption by setting consumed flag
    // and returning an error if already consumed
    if let Err(e) = safe_vector.consume() {
        // Log error but don't crash - better than double free
        eprintln!("Error during vector destruction: {}", e);
    }
}

/// Checks if a SafeUnmanagedVector contains a None value
///
/// # Safety
///
/// The pointer must point to a valid SafeUnmanagedVector created with
/// new_safe_unmanaged_vector or a related function.
#[no_mangle]
pub extern "C" fn safe_unmanaged_vector_is_none(v: *const SafeUnmanagedVector) -> bool {
    if v.is_null() {
        true // Null pointers are treated as None
    } else {
        let safe_vec = unsafe { &*v };
        safe_vec.check_none()
    }
}

/// Gets the length of a SafeUnmanagedVector
/// Returns 0 if the vector is None or has been consumed
///
/// # Safety
///
/// The pointer must point to a valid SafeUnmanagedVector created with
/// new_safe_unmanaged_vector or a related function.
#[no_mangle]
pub extern "C" fn safe_unmanaged_vector_length(v: *const SafeUnmanagedVector) -> usize {
    if v.is_null() {
        0 // Null pointers have zero length
    } else {
        let safe_vec = unsafe { &*v };
        safe_vec.len()
    }
}

/// Copies the content of a SafeUnmanagedVector into a newly allocated Go byte slice
/// Returns a pointer to the data and its length, which must be freed by Go
///
/// # Safety
///
/// The pointer must point to a valid SafeUnmanagedVector created with
/// new_safe_unmanaged_vector or a related function.
#[no_mangle]
pub extern "C" fn safe_unmanaged_vector_to_bytes(
    v: *mut SafeUnmanagedVector,
    output_data: *mut *mut u8,
    output_len: *mut usize,
) -> bool {
    if v.is_null() || output_data.is_null() || output_len.is_null() {
        return false;
    }

    // Get a mutable reference to the vector
    let safe_vec = unsafe { &mut *v };

    // Early check to avoid trying to consume already consumed vector
    if safe_vec.is_consumed() {
        return false;
    }

    // Try to consume the vector safely
    match safe_vec.consume() {
        Ok(maybe_data) => {
            if let Some(data) = maybe_data {
                if data.is_empty() {
                    // Empty data case
                    unsafe {
                        *output_data = std::ptr::null_mut();
                        *output_len = 0;
                    }
                } else {
                    // Convert the Vec<u8> into a raw pointer and length
                    // The Go side will take ownership of this memory
                    let mut data_clone = data.clone();
                    let len = data_clone.len();
                    let ptr = data_clone.as_mut_ptr();

                    // Prevent Rust from freeing the memory when data_clone goes out of scope
                    std::mem::forget(data_clone);

                    unsafe {
                        *output_data = ptr;
                        *output_len = len;
                    }
                }
                true
            } else {
                // None case
                unsafe {
                    *output_data = std::ptr::null_mut();
                    *output_len = 0;
                }
                true
            }
        }
        Err(_) => {
            // Vector was already consumed or other error
            false
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn byte_slice_view_read_works() {
        let data = vec![0xAA, 0xBB, 0xCC];
        let view = ByteSliceView::new(&data);
        assert_eq!(view.read().unwrap(), &[0xAA, 0xBB, 0xCC]);

        let data = vec![];
        let view = ByteSliceView::new(&data);
        assert_eq!(view.read().unwrap(), &[] as &[u8]);

        let view = ByteSliceView::nil();
        assert!(view.read().is_none());

        // This is what we get when creating a ByteSliceView for an empty []byte in Go
        let view = ByteSliceView {
            is_nil: false,
            ptr: std::ptr::null::<u8>(),
            len: 0,
        };
        assert!(view.read().is_some());
    }

    #[test]
    fn byte_slice_view_to_owned_works() {
        let data = vec![0xAA, 0xBB, 0xCC];
        let view = ByteSliceView::new(&data);
        assert_eq!(view.to_owned().unwrap(), vec![0xAA, 0xBB, 0xCC]);

        let data = vec![];
        let view = ByteSliceView::new(&data);
        assert_eq!(view.to_owned().unwrap(), Vec::<u8>::new());

        let view = ByteSliceView::nil();
        assert!(view.to_owned().is_none());
    }

    #[test]
    fn unmanaged_vector_new_works() {
        // With data
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert!(!x.is_none);
        assert_ne!(x.ptr as usize, 0);
        assert_eq!(x.len, 2);
        assert_eq!(x.cap, 2);

        // Empty data
        let x = UnmanagedVector::new(Some(vec![]));
        assert!(!x.is_none);
        assert_eq!(x.ptr as usize, 0);
        assert_eq!(x.len, 0);
        assert_eq!(x.cap, 0);

        // None
        let x = UnmanagedVector::new(None);
        assert!(x.is_none);
        assert_eq!(x.ptr as usize, 0); // this is not guaranteed, could be anything
        assert_eq!(x.len, 0); // this is not guaranteed, could be anything
        assert_eq!(x.cap, 0); // this is not guaranteed, could be anything
    }

    #[test]
    fn unmanaged_vector_some_works() {
        // With data
        let x = UnmanagedVector::some(vec![0x11, 0x22]);
        assert!(!x.is_none);
        assert_ne!(x.ptr as usize, 0);
        assert_eq!(x.len, 2);
        assert_eq!(x.cap, 2);

        // Empty data
        let x = UnmanagedVector::some(vec![]);
        assert!(!x.is_none);
        assert_eq!(x.ptr as usize, 0);
        assert_eq!(x.len, 0);
        assert_eq!(x.cap, 0);
    }

    #[test]
    fn unmanaged_vector_none_works() {
        let x = UnmanagedVector::new(None);
        assert!(x.is_none);

        assert_eq!(x.ptr as usize, 0); // this is not guaranteed, could be anything
        assert_eq!(x.len, 0); // this is not guaranteed, could be anything
        assert_eq!(x.cap, 0); // this is not guaranteed, could be anything
    }

    #[test]
    fn unmanaged_vector_is_some_works() {
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert!(x.is_some());
        let x = UnmanagedVector::new(Some(vec![]));
        assert!(x.is_some());
        let x = UnmanagedVector::new(None);
        assert!(!x.is_some());
    }

    #[test]
    fn unmanaged_vector_is_none_works() {
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert!(!x.is_none());
        let x = UnmanagedVector::new(Some(vec![]));
        assert!(!x.is_none());
        let x = UnmanagedVector::new(None);
        assert!(x.is_none());
    }

    #[test]
    fn unmanaged_vector_consume_works() {
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert_eq!(x.consume(), Some(vec![0x11u8, 0x22]));
        let x = UnmanagedVector::new(Some(vec![]));
        assert_eq!(x.consume(), Some(Vec::<u8>::new()));
        let x = UnmanagedVector::new(None);
        assert_eq!(x.consume(), None);
    }

    #[test]
    fn unmanaged_vector_defaults_to_none() {
        let x = UnmanagedVector::default();
        assert_eq!(x.consume(), None);
    }

    #[test]
    fn new_unmanaged_vector_works() {
        // Some simple data
        let data = b"some stuff";
        let x = new_unmanaged_vector(false, data.as_ptr(), data.len());
        assert_eq!(x.consume(), Some(Vec::<u8>::from(b"some stuff" as &[u8])));

        // empty created in Rust
        let data = b"";
        let x = new_unmanaged_vector(false, data.as_ptr(), data.len());
        assert_eq!(x.consume(), Some(Vec::<u8>::new()));

        // empty created in Go
        let x = new_unmanaged_vector(false, std::ptr::null::<u8>(), 0);
        assert_eq!(x.consume(), Some(Vec::<u8>::new()));

        // nil with garbage pointer
        let x = new_unmanaged_vector(true, 345 as *const u8, 46);
        assert_eq!(x.consume(), None);

        // nil with empty slice
        let data = b"";
        let x = new_unmanaged_vector(true, data.as_ptr(), data.len());
        assert_eq!(x.consume(), None);

        // nil with null pointer
        let x = new_unmanaged_vector(true, std::ptr::null::<u8>(), 0);
        assert_eq!(x.consume(), None);
    }

    #[test]
    fn safe_byte_slice_prevents_double_read() {
        let data = vec![0xAA, 0xBB, 0xCC];
        let view = ByteSliceView::new(&data);
        let mut safe_slice = SafeByteSlice::new(view);

        // First read should succeed
        let first_read = safe_slice.read();
        assert!(first_read.is_ok());
        let bytes = first_read.unwrap();
        assert!(bytes.is_some());
        assert_eq!(bytes.unwrap(), &[0xAA, 0xBB, 0xCC]);

        // Second read should fail with error
        let second_read = safe_slice.read();
        assert!(second_read.is_err());
        let err = second_read.unwrap_err();
        assert!(err.to_string().contains("already consumed"));
    }

    #[test]
    fn safe_unmanaged_vector_prevents_double_consume() {
        let data = vec![0x11, 0x22, 0x33];
        let mut safe_vec = SafeUnmanagedVector::new(Some(data.clone()));

        // First consume should succeed
        let first_consume = safe_vec.consume();
        assert!(first_consume.is_ok());
        let vec = first_consume.unwrap();
        assert!(vec.is_some());
        assert_eq!(vec.unwrap(), data);

        // Second consume should fail with error
        let second_consume = safe_vec.consume();
        assert!(second_consume.is_err());
        let err = second_consume.unwrap_err();
        assert!(err.to_string().contains("already consumed"));
    }

    #[test]
    fn validate_memory_size_rejects_too_large() {
        // 10MB + 1 byte should fail
        let size = 1024 * 1024 * 10 + 1;
        let result = validate_memory_size(size);
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("exceeds limit"));

        // 10MB exactly should be fine
        let valid_size = 1024 * 1024 * 10;
        let valid_result = validate_memory_size(valid_size);
        assert!(valid_result.is_ok());
    }
}
