use std::fmt;
use std::mem;
use std::slice;

#[derive(Copy, Clone)]
#[repr(C)]
pub struct Buffer {
    pub ptr: *mut u8,
    pub size: usize,
}

// this frees memory we released earlier
#[no_mangle]
pub extern "C" fn free_rust(buf: Buffer) {
    unsafe {
        let _v = Vec::from_raw_parts(
            buf.ptr,
            buf.size,
            buf.size,
        );
    }
}

pub fn read_buffer(b: &Buffer) -> Option<&'static [u8]> {
    if b.ptr.is_null() || b.size == 0 {
        None
    } else {
        unsafe { Some(slice::from_raw_parts(b.ptr, b.size)) }
    }
}

pub struct MemoryErr {
    required: usize,
}

impl fmt::Display for MemoryErr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "No enough memory in buffer, need {}", self.required)
    }
}

// write buffer, writes to the (caller) memory referenced by the buffer
// it returns an error if this is too big
pub fn write_buffer(b: &Buffer, data: &[u8]) -> Result<(), MemoryErr> {
    panic!("not implemented");
}


// this releases our memory to the caller
pub fn release_vec(mut v: Vec<u8>) -> Buffer {
    let buf = Buffer {
        ptr: v.as_mut_ptr(),
        size: v.len(),
    };
    mem::forget(v);
    buf
}

