use errno::{set_errno, Errno};
use std::fmt::Display;

use crate::memory::Buffer;

pub fn set_error(msg: String, errout: Option<&mut Buffer>) {
    if let Some(mb) = errout {
        *mb = Buffer::from_vec(msg.into_bytes());
    }
    // Question: should we set errno to something besides generic 1 always?
    set_errno(Errno(1));
}

pub fn handle_c_error<T, E>(r: Result<T, E>, errout: Option<&mut Buffer>) -> T
where
    T: Default,
    E: Display,
{
    match r {
        Ok(t) => t,
        Err(e) => {
            set_error(e.to_string(), errout);
            T::default()
        }
    }
}
