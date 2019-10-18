use std::mem;
use std::slice;

#[derive(Copy, Clone)]
#[repr(C)]
pub struct Buffer {
    pub ptr: *mut u8,
    pub size: usize,
}

impl Buffer {
    pub fn is_empty(&self) -> bool {
        self.size == 0 || self.ptr.is_null()
    }
}

impl Default for Buffer {
    fn default() -> Self {
        Buffer{
            ptr: 0 as *mut u8,
            size: 0,
        }
    }
}

// this frees memory we released earlier
#[no_mangle]
pub extern "C" fn free_rust(buf: Buffer) {
    if !buf.is_empty() {
        unsafe {
            let _ = Vec::from_raw_parts(buf.ptr, buf.size, buf.size);
        }
    }
}

pub fn read_buffer(b: &Buffer) -> Option<&'static [u8]> {
    if b.is_empty() {
        None
    } else {
        unsafe { Some(slice::from_raw_parts(b.ptr, b.size)) }
    }
}

// this releases our memory to the caller
pub fn release_vec(mut v: Vec<u8>) -> Buffer {
    if v.len() == 0 {
        return Buffer {
            ptr: 0 as *mut u8,
            size: 0,
        };
    }
    let buf = Buffer {
        ptr: v.as_mut_ptr(),
        size: v.len(),
    };
    mem::forget(v);
    buf
}
