use std::mem;
use std::slice;

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
pub extern "C" fn greet(name: Option<&Buffer>) -> *mut Buffer {
    let v = match name {
        Some(buf) => {
            let rname = unsafe { slice::from_raw_parts(buf.ptr, buf.size) };
            let mut res = b"Hello, ".to_vec();
            res.extend_from_slice(rname);
            res
        },
        None => b"Hi, <nil>".to_vec()
    };
    release_vec(v)
}

// this frees memory we released earlier
#[no_mangle]
pub extern "C" fn free_rust(raw: *mut Buffer) {
    unsafe {
        let buf = Box::from_raw(raw);
        let _buffer = Vec::from_raw_parts(
            buf.ptr,
            buf.size,
            buf.size,
        );
    }
}

/**** To memory module ***/

// this releases our memory to the caller
fn release_vec(mut v: Vec<u8>) -> *mut Buffer {
    let buf = Box::new(Buffer {
        ptr: v.as_mut_ptr(),
        size: v.len(),
    });
    mem::forget(v);
    Box::into_raw(buf)
}