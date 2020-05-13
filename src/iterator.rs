use cosmwasm_std::KV;

use crate::error::GoResult;
use crate::memory::Buffer;
use cosmwasm_vm::{FfiError, FfiResult};

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
    type Item = FfiResult<KV>;

    fn next(&mut self) -> Option<Self::Item> {
        let next_db = match self.vtable.next_db {
            Some(f) => f,
            // TODO: return None here???
            None => return Some(Err(FfiError::Other)),
        };

        let mut key_buf = Buffer::default();
        let mut value_buf = Buffer::default();
        let go_result: GoResult = (next_db)(
            self.state,
            &mut key_buf as *mut Buffer,
            &mut value_buf as *mut Buffer,
        )
        .into();
        let go_result: FfiResult<()> = go_result.into();
        if let Err(err) = go_result {
            return Some(Err(err));
        }

        let okey = unsafe { key_buf.read() };
        match okey {
            Some(key) => {
                let value = unsafe { value_buf.read() };
                if let Some(value) = value {
                    let kv = (key.to_vec(), value.to_vec());
                    Some(Ok(kv))
                } else {
                    Some(Err(FfiError::Other))
                }
            }
            None => None,
        }
    }
}
