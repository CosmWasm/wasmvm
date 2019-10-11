use std::mem;
use std::slice;
use std::str::from_utf8;

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
pub extern "C" fn greet(name: *mut Buffer) -> *mut Buffer {
    let b = unsafe { Box::from_raw(name) };
    let rname = unsafe { slice::from_raw_parts(b.ptr, b.size) };
    let mut res = String::from("Hello, ");
    res.push_str(from_utf8(rname).unwrap());
    let mut vec = res.into_bytes();

    // this releases our memory to the caller
    let buf = Box::new(Buffer {
        ptr: vec.as_mut_ptr(),
        size: vec.len(),
    });
    mem::forget(vec);
    Box::into_raw(buf)
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

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        assert_eq!(add(2, 2), 4);
    }
}
