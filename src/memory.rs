use std::mem;
use std::slice;

/// A view into an externally owned byte slice (Go `[]byte`).
/// Use this for the current call only. A view cannot be copied for safety reasons.
/// If you need a copy, use [`to_owned`].
///
/// Go's nil value is fully supported, such that we can differentiate between nil and an empty slice.
#[repr(C)]
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
            Some(unsafe { slice::from_raw_parts(self.ptr, self.len) })
        }
    }

    /// Created an owned copy that can safely be stored.
    #[allow(dead_code)]
    pub fn to_owned(&self) -> Option<Vec<u8>> {
        self.read().map(|slice| slice.to_owned())
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
            Some(data) => Self {
                is_none: false,
                ptr: data.as_ptr(),
                len: data.len(),
            },
            None => Self {
                is_none: true,
                ptr: std::ptr::null::<u8>(),
                len: 0,
            },
        }
    }
}

/// A Vector type that requires explicit creation and destruction and
/// can be sent via FFI.
/// It can be created from `Option<Vec<u8>>` and be converted into `Option<Vec<u8>>`.
/// This type is always created in Rust and always dropped in Rust.
/// If Go code wants to consume it's data, it must create a copy and
/// instruct Rust to destroy it.
#[repr(C)]
pub struct UnmanagedVector {
    /// True if and only if this is None/nil. If this is true, the other fields must be ignored.
    is_nil: bool,
    ptr: *mut u8,
    len: usize,
    cap: usize,
}

impl UnmanagedVector {
    /// Consumes this optional vector for manual management.
    /// This is a zero-copy operation.
    fn new(source: Option<Vec<u8>>) -> Self {
        match source {
            Some(data) => {
                let mut data = mem::ManuallyDrop::new(data);
                Self {
                    is_nil: false,
                    ptr: data.as_mut_ptr(),
                    len: data.len(),
                    cap: data.capacity(),
                }
            }
            None => Self {
                is_nil: true,
                ptr: std::ptr::null_mut::<u8>(),
                len: 0,
                cap: 0,
            },
        }
    }

    pub fn len(&self) -> Option<usize> {
        if self.is_nil {
            None
        } else {
            Some(self.len)
        }
    }

    pub fn is_none(&self) -> bool {
        self.is_nil
    }

    pub fn is_some(&self) -> bool {
        !self.is_none()
    }

    /// Takes this UnmanagedVector and turns it into a regular, managed Rust vector.
    /// Calling this on two copies of UnmanagedVector leads to double free crashes.
    pub fn consume(self) -> Option<Vec<u8>> {
        if self.is_nil {
            None
        } else {
            Some(unsafe { Vec::from_raw_parts(self.ptr, self.len, self.cap) })
        }
    }
}

impl Default for UnmanagedVector {
    fn default() -> Self {
        Self {
            is_nil: true,
            ptr: std::ptr::null_mut::<u8>(),
            len: 0,
            cap: 0,
        }
    }
}

#[no_mangle]
pub extern "C" fn new_unmanaged_vector(
    nil: bool,
    ptr: *const u8,
    length: usize,
) -> UnmanagedVector {
    if nil {
        UnmanagedVector::new(None)
    } else if length == 0 {
        UnmanagedVector::new(Some(Vec::new()))
    } else {
        let external_memory = unsafe { slice::from_raw_parts(ptr, length) };
        let copy = Vec::from(external_memory);
        UnmanagedVector::new(Some(copy))
    }
}

#[no_mangle]
pub extern "C" fn allocate_rust(ptr: *const u8, length: usize) -> Buffer {
    // Go doesn't store empty buffers the same way Rust stores empty slices (with NonNull  pointers
    // equal to the offset of the type, which would be equal to 1 in this case)
    // so when it wants to represent an empty buffer, it passes a null pointer with 0 length here.
    if length == 0 {
        Buffer::from_vec(Vec::new())
    } else {
        Buffer::from_vec(Vec::from(unsafe { slice::from_raw_parts(ptr, length) }))
    }
}

// this frees memory we released earlier
#[no_mangle]
pub extern "C" fn free_rust(buf: Buffer) {
    unsafe {
        let _ = buf.consume();
    }
}

#[repr(C)]
pub struct Buffer {
    pub ptr: *mut u8,
    pub len: usize,
    pub cap: usize,
}

impl Buffer {
    /// `read` provides a reference to the included data to be parsed or copied elsewhere
    ///
    /// # Safety
    ///
    /// The caller must make sure that the `Buffer` points to valid and initialized memory
    pub unsafe fn read(&self) -> Option<&[u8]> {
        if self.ptr.is_null() {
            None
        } else {
            Some(slice::from_raw_parts(self.ptr, self.len))
        }
    }

    /// consume must only be used on memory previously released by from_vec
    /// when the Vec is out of scope, it will deallocate the memory previously referenced by Buffer
    ///
    /// # Safety
    ///
    /// if not empty, `ptr` must be a valid memory reference, which was previously
    /// created by `from_vec`. You may not consume a slice twice.
    /// Otherwise you risk double free panics
    pub unsafe fn consume(self) -> Vec<u8> {
        if self.ptr.is_null() {
            vec![]
        } else {
            Vec::from_raw_parts(self.ptr, self.len, self.cap)
        }
    }

    /// Creates a new zero length Buffer with the given capacity
    pub fn with_capacity(capacity: usize) -> Self {
        Buffer::from_vec(Vec::<u8>::with_capacity(capacity))
    }

    // this releases our memory to the caller
    pub fn from_vec(v: Vec<u8>) -> Self {
        v.into()
    }
}

impl Default for Buffer {
    fn default() -> Self {
        Buffer {
            ptr: std::ptr::null_mut::<u8>(),
            len: 0,
            cap: 0,
        }
    }
}

impl From<Vec<u8>> for Buffer {
    fn from(original: Vec<u8>) -> Self {
        let mut v = mem::ManuallyDrop::new(original);
        Buffer {
            ptr: v.as_mut_ptr(),
            len: v.len(),
            cap: v.capacity(),
        }
    }
}

impl From<&[u8]> for Buffer {
    fn from(original: &[u8]) -> Self {
        original.to_owned().into()
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
        assert_eq!(view.read().is_none(), true);
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
        assert_eq!(view.to_owned().is_none(), true);
    }

    #[test]
    fn unmanaged_vector_len_works() {
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert_eq!(x.len(), Some(2));
        let x = UnmanagedVector::new(Some(vec![]));
        assert_eq!(x.len(), Some(0));
        let x = UnmanagedVector::new(None);
        assert_eq!(x.len(), None);
    }

    #[test]
    fn unmanaged_vector_is_some_works() {
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert_eq!(x.is_some(), true);
        let x = UnmanagedVector::new(Some(vec![]));
        assert_eq!(x.is_some(), true);
        let x = UnmanagedVector::new(None);
        assert_eq!(x.is_some(), false);
    }

    #[test]
    fn unmanaged_vector_is_none_works() {
        let x = UnmanagedVector::new(Some(vec![0x11, 0x22]));
        assert_eq!(x.is_none(), false);
        let x = UnmanagedVector::new(Some(vec![]));
        assert_eq!(x.is_none(), false);
        let x = UnmanagedVector::new(None);
        assert_eq!(x.is_none(), true);
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
    fn read_works() {
        let buffer1 = Buffer::from_vec(vec![0xAA]);
        assert_eq!(unsafe { buffer1.read() }, Some(&[0xAAu8] as &[u8]));

        let buffer2 = Buffer::from_vec(vec![0xAA, 0xBB, 0xCC]);
        assert_eq!(
            unsafe { buffer2.read() },
            Some(&[0xAAu8, 0xBBu8, 0xCCu8] as &[u8])
        );

        let empty: &[u8] = b"";

        let buffer3 = Buffer::from_vec(Vec::new());
        assert_eq!(unsafe { buffer3.read() }, Some(empty));

        let buffer4 = Buffer::with_capacity(7);
        assert_eq!(unsafe { buffer4.read() }, Some(empty));

        // Cleanup
        unsafe { buffer1.consume() };
        unsafe { buffer2.consume() };
        unsafe { buffer3.consume() };
        unsafe { buffer4.consume() };
    }

    #[test]
    fn with_capacity_works() {
        let buffer = Buffer::with_capacity(7);
        assert_eq!(buffer.ptr.is_null(), false);
        assert_eq!(buffer.len, 0);
        assert_eq!(buffer.cap, 7);

        // Cleanup
        unsafe { buffer.consume() };
    }

    #[test]
    fn from_vec_and_consume_work() {
        let mut original: Vec<u8> = vec![0x00, 0xaa, 0x76];
        original.reserve_exact(2);
        let original_ptr = original.as_ptr();

        let buffer = Buffer::from_vec(original);
        assert_eq!(buffer.ptr.is_null(), false);
        assert_eq!(buffer.len, 3);
        assert_eq!(buffer.cap, 5);

        let restored = unsafe { buffer.consume() };
        assert_eq!(restored.as_ptr(), original_ptr);
        assert_eq!(restored.len(), 3);
        assert_eq!(restored.capacity(), 5);
        assert_eq!(&restored, &[0x00, 0xaa, 0x76]);
    }

    #[test]
    fn from_vec_and_consume_work_for_zero_len() {
        let mut original: Vec<u8> = vec![];
        original.reserve_exact(2);
        let original_ptr = original.as_ptr();

        let buffer = Buffer::from_vec(original);
        assert_eq!(buffer.ptr.is_null(), false);
        assert_eq!(buffer.len, 0);
        assert_eq!(buffer.cap, 2);

        let restored = unsafe { buffer.consume() };
        assert_eq!(restored.as_ptr(), original_ptr);
        assert_eq!(restored.len(), 0);
        assert_eq!(restored.capacity(), 2);
    }

    #[test]
    fn from_vec_and_consume_work_for_zero_capacity() {
        let original: Vec<u8> = vec![];
        let original_ptr = original.as_ptr();

        let buffer = Buffer::from_vec(original);
        // Skip ptr test here. Since Vec does not allocate memory when capacity is 0, this could be anything
        assert_eq!(buffer.len, 0);
        assert_eq!(buffer.cap, 0);

        let restored = unsafe { buffer.consume() };
        assert_eq!(restored.as_ptr(), original_ptr);
        assert_eq!(restored.len(), 0);
        assert_eq!(restored.capacity(), 0);
    }
}
