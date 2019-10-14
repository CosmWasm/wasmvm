use errno;
use errno::{set_errno, Errno};
use std::cell::RefCell;
use std::fmt::Display;

use crate::memory::{release_vec, Buffer};

thread_local! {
    static LAST_ERROR: RefCell<Option<String>> = RefCell::new(None);
}

#[no_mangle]
pub extern "C" fn get_last_error() -> Buffer {
    let msg = take_last_error().unwrap_or_default();
    release_vec(msg.into_bytes())
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
where
    T: Default,
    E: Display,
{
    match r {
        Ok(t) => t,
        Err(e) => {
            update_last_error(e.to_string());
            T::default()
        }
    }
}
