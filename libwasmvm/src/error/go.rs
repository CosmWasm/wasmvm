use cosmwasm_vm::BackendError;

use crate::memory::UnmanagedVector;

/// This enum gives names to the status codes returned from Go callbacks to Rust.
/// The Go code will return one of these variants when returning.
///
/// 0 means no error, all the other cases are some sort of error.
///
/// cbindgen:prefix-with-name
// NOTE TO DEVS: If you change the values assigned to the variants of this enum, You must also
//               update the match statement in the From conversion below.
//               Otherwise all hell may break loose.
//               You have been warned.
//
#[repr(i32)] // This makes it so the enum looks like a simple i32 to Go
#[derive(PartialEq)]
pub enum GoError {
    None = 0,
    /// Go panicked for an unexpected reason.
    Panic = 1,
    /// Go received a bad argument from Rust
    BadArgument = 2,
    /// Ran out of gas while using the SDK (e.g. storage)
    OutOfGas = 3,
    /// An error happened during normal operation of a Go callback, which should abort the contract
    Other = 4,
    /// An error happened during normal operation of a Go callback, which should be fed back to the contract
    User = 5,
}

impl From<i32> for GoError {
    fn from(n: i32) -> Self {
        // This conversion treats any number that is not otherwise an expected value as `GoError::Other`
        match n {
            0 => GoError::None,
            1 => GoError::Panic,
            2 => GoError::BadArgument,
            3 => GoError::OutOfGas,
            5 => GoError::User,
            _ => GoError::Other,
        }
    }
}

impl GoError {
    /// This converts a GoError to a `Result<(), BackendError>`, using a fallback error message for some cases.
    /// If it is GoError::User the error message will be returned to the contract.
    /// Otherwise, the returned error will trigger a trap in the VM and abort contract execution immediately.
    ///
    /// Safety: this reads data from an externally provided buffer and assumes valid utf-8 encoding
    /// Only call if you trust the code that provides `error_msg` to be correct.
    pub unsafe fn into_ffi_result<F>(
        self,
        error_msg: UnmanagedVector,
        default: F,
    ) -> Result<(), BackendError>
    where
        F: Fn() -> String,
    {
        let read_error_msg = || -> String {
            match error_msg.consume() {
                Some(data) => String::from_utf8_lossy(&data).into(),
                None => default(),
            }
        };

        match self {
            GoError::None => Ok(()),
            GoError::Panic => Err(BackendError::foreign_panic()),
            GoError::BadArgument => Err(BackendError::bad_argument()),
            GoError::OutOfGas => Err(BackendError::out_of_gas()),
            GoError::Other => Err(BackendError::unknown(read_error_msg())),
            GoError::User => Err(BackendError::user_err(read_error_msg())),
        }
    }
}
