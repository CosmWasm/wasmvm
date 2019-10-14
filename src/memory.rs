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

// this releases our memory to the caller
pub fn release_vec(mut v: Vec<u8>) -> Buffer {
    if v.len() == 0 {
        return Buffer{ptr: 0 as *mut u8, size: 0};
    }
    let buf = Buffer {
        ptr: v.as_mut_ptr(),
        size: v.len(),
    };
    mem::forget(v);
    buf
}

