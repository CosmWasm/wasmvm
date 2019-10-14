use std::mem;
use std::slice;

#[derive(Copy, Clone)]
#[repr(C)]
pub struct Buffer {
    ptr: *mut u8,
    size: usize,
}

#[no_mangle]
pub extern "C" fn add(a: i32, b: i32) -> i32 {
    a+b
}

#[no_mangle]
pub extern "C" fn greet(name: Buffer) -> Buffer {
    if name.ptr.is_null() || name.size == 0 {
        return release_vec(b"Hi, <nil>".to_vec());
    }
    let rname = unsafe { slice::from_raw_parts(name.ptr, name.size) };
    let mut v = b"Hello, ".to_vec();
    v.extend_from_slice(rname);
    release_vec(v)
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

/**** To memory module ***/

// this releases our memory to the caller
fn release_vec(mut v: Vec<u8>) -> Buffer {
    let buf = Buffer {
        ptr: v.as_mut_ptr(),
        size: v.len(),
    };
    mem::forget(v);
    buf
}