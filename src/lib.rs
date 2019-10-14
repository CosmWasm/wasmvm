mod error;
mod memory;

pub use error::{get_last_error};
pub use memory::{Buffer, free_rust};

use error::{handle_c_error, update_last_error};
use memory::{read_buffer, release_vec};

#[no_mangle]
pub extern "C" fn add(a: i32, b: i32) -> i32 {
    a+b
}

#[no_mangle]
pub extern "C" fn greet(name: Buffer) -> Buffer {
    let rname = read_buffer(&name).unwrap_or(b"<nil>");
    let mut v = b"Hello, ".to_vec();
    v.extend_from_slice(rname);
    release_vec(v)
}

/// divide returns the rounded (i32) result, returns a C error if div == 0
#[no_mangle]
pub extern "C" fn divide(num: i32, div: i32) -> i32 {
    if div == 0 {
        update_last_error("Cannot divide by zero".to_string());
        return 0;
    }
    num / div
}
