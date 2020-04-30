use cosmwasm_std::{generic_err, StdResult, KV};

use crate::error::GoResult;
use crate::memory::Buffer;

// this represents something passed in from the caller side of FFI
#[repr(C)]
pub struct iterator_t {
    _private: [u8; 0],
}

// These functions should return GoResult but because we don't trust them here, we treat the return value as i32
// and then check it when converting to GoResult manually
#[repr(C)]
#[derive(Default)]
pub struct Iterator_vtable {
    pub next_db: Option<extern "C" fn(*mut iterator_t, *mut Buffer, *mut Buffer) -> i32>,
}

#[repr(C)]
pub struct GoIter {
    pub state: *mut iterator_t,
    pub vtable: Iterator_vtable,
}

impl Default for GoIter {
    fn default() -> Self {
        GoIter {
            state: std::ptr::null_mut(),
            vtable: Iterator_vtable::default(),
        }
    }
}

impl Iterator for GoIter {
    type Item = StdResult<KV>;

    fn next(&mut self) -> Option<Self::Item> {
        let next_db = match self.vtable.next_db {
            Some(f) => f,
            // TODO: return None here???
            None => return Some(Err(generic_err("iterator vtable not set"))),
        };

        let mut key_buf = Buffer::default();
        let mut value_buf = Buffer::default();
        let go_result: GoResult = (next_db)(
            self.state,
            &mut key_buf as *mut Buffer,
            &mut value_buf as *mut Buffer,
        )
        .into();
        if !go_result.is_ok() {
            return Some(Err(generic_err(format!(
                "Go {}: iterator.next()",
                go_result
            ))));
        }

        // TODO: Check if key is null, return none, otherwise, make KV
        let okey = unsafe { key_buf.read() };
        match okey {
            Some(key) => {
                // TODO: return error not unwrap??
                let value = unsafe { value_buf.read().unwrap() };
                let kv = (key.to_vec(), value.to_vec());
                Some(Ok(kv))
            }
            None => None,
        }
    }
}
