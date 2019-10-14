mod error;
mod memory;

pub use error::{get_last_error};
pub use memory::{Buffer, free_rust};

use error::{handle_c_error, update_last_error};
use memory::{read_buffer, release_vec};
use std::panic::catch_unwind;

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

#[no_mangle]
pub extern "C" fn may_panic(guess: i32) -> Buffer {
    let r = catch_unwind(|| do_may_panic(guess));
    let p = match r {
        Ok(r2) => handle_c_error(r2).into_bytes(),
        Err(_) => { update_last_error("Caught panic".to_string()); Vec::<u8>::new()}
    };
    release_vec(p)
}

fn do_may_panic(guess: i32) -> Result<String, String> {
    if guess == 0 {
        panic!("Must be negative or positive")
    } else if guess < 17 {
        Err("Too low".to_owned())
    } else {
        Ok("You are a winner!".to_owned())
    }
}