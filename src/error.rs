use std::cell::RefCell;
use std::fmt::Display;
use errno;
use errno::{set_errno, Errno};

use crate::memory::{Buffer, write_buffer};

thread_local! {
    static LAST_ERROR: RefCell<Option<String>> = RefCell::new(None);
}

#[no_mangle]
pub extern "C" fn get_last_error(buf: &Buffer) {
    let msg = take_last_error().unwrap_or_default();
    // return another error if we are out of memory
    handle_c_error(write_buffer(buf, msg.as_bytes()));
}

pub fn update_last_error(msg: String) {
    LAST_ERROR.with(|prev| {
        *prev.borrow_mut() = Some(msg);
    });
    // Question: should we set errno to something besides generic 1 always?
    set_errno(Errno(1));
}

/// Retrieve the most recent error, clearing it in the process.
pub fn take_last_error() -> Option<String> {
    LAST_ERROR.with(|prev| prev.borrow_mut().take())
}


pub fn handle_c_error<T, E>(r: Result<T, E>) -> T
    where T: Default, E: Display {
    match r {
        Ok(t) => t,
        Err(e) => {
            update_last_error(e.to_string());
            T::default()
        }
    }
}
