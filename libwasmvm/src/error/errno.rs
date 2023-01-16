// See https://github.com/rust-lang/libc/issues/1644 for why we have to implement that ourselves.
// https://github.com/lambda-fairy/rust-errno does not work with cgo
// on Windows as cgo does not use the SetLastError/GetLastError Windows APIs.

use errno_sys::errno_location;

#[allow(unused)] // used in tests
pub fn errno() -> i32 {
    unsafe { *errno_location() }
}

pub fn set_errno(errno: i32) {
    unsafe {
        *errno_location() = errno;
    }
}
