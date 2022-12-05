use cosmwasm_vm::VmError;
#[cfg(feature = "backtraces")]
use std::backtrace::Backtrace;
use thiserror::Error;

use crate::memory::UnmanagedVector;

#[derive(Error, Debug)]
pub enum RustError {
    #[error("Empty argument: {}", name)]
    EmptyArg {
        name: String,
        #[cfg(feature = "backtraces")]
        backtrace: Backtrace,
    },
    /// Whenever UTF-8 bytes cannot be decoded into a unicode string, e.g. in String::from_utf8 or str::from_utf8.
    #[error("Cannot decode UTF8 bytes into string: {}", msg)]
    InvalidUtf8 {
        msg: String,
        #[cfg(feature = "backtraces")]
        backtrace: Backtrace,
    },
    #[error("Ran out of gas")]
    OutOfGas {
        #[cfg(feature = "backtraces")]
        backtrace: Backtrace,
    },
    #[error("Caught panic")]
    Panic {
        #[cfg(feature = "backtraces")]
        backtrace: Backtrace,
    },
    #[error("Null/Nil argument: {}", name)]
    UnsetArg {
        name: String,
        #[cfg(feature = "backtraces")]
        backtrace: Backtrace,
    },
    #[error("Error calling the VM: {}", msg)]
    VmErr {
        msg: String,
        #[cfg(feature = "backtraces")]
        backtrace: Backtrace,
    },
}

impl RustError {
    pub fn empty_arg<T: Into<String>>(name: T) -> Self {
        RustError::EmptyArg {
            name: name.into(),
            #[cfg(feature = "backtraces")]
            backtrace: Backtrace::capture(),
        }
    }

    pub fn invalid_utf8<S: ToString>(msg: S) -> Self {
        RustError::InvalidUtf8 {
            msg: msg.to_string(),
            #[cfg(feature = "backtraces")]
            backtrace: Backtrace::capture(),
        }
    }

    pub fn panic() -> Self {
        RustError::Panic {
            #[cfg(feature = "backtraces")]
            backtrace: Backtrace::capture(),
        }
    }

    pub fn unset_arg<T: Into<String>>(name: T) -> Self {
        RustError::UnsetArg {
            name: name.into(),
            #[cfg(feature = "backtraces")]
            backtrace: Backtrace::capture(),
        }
    }

    pub fn vm_err<S: ToString>(msg: S) -> Self {
        RustError::VmErr {
            msg: msg.to_string(),
            #[cfg(feature = "backtraces")]
            backtrace: Backtrace::capture(),
        }
    }

    pub fn out_of_gas() -> Self {
        RustError::OutOfGas {
            #[cfg(feature = "backtraces")]
            backtrace: Backtrace::capture(),
        }
    }
}

impl From<VmError> for RustError {
    fn from(source: VmError) -> Self {
        match source {
            VmError::GasDepletion { .. } => RustError::out_of_gas(),
            _ => RustError::vm_err(source),
        }
    }
}

impl From<std::str::Utf8Error> for RustError {
    fn from(source: std::str::Utf8Error) -> Self {
        RustError::invalid_utf8(source)
    }
}

impl From<std::string::FromUtf8Error> for RustError {
    fn from(source: std::string::FromUtf8Error) -> Self {
        RustError::invalid_utf8(source)
    }
}

/// An error code used to communicate the errors of FFI calls.
/// Similar to shell codes and errno, 0 means no error.
/// cbindgen:prefix-with-name
#[repr(i32)]
#[derive(Debug, Clone, Copy)]
pub enum ErrorCode {
    Success = 0,
    Other = 1,
    OutOfGas = 2,
}

impl ErrorCode {
    pub fn to_int(self) -> i32 {
        self as i32
    }
}

pub fn set_error(err: RustError, error_msg: Option<&mut UnmanagedVector>) -> ErrorCode {
    if let Some(error_msg) = error_msg {
        if error_msg.is_some() {
            panic!(
                "There is an old error message in the given pointer that has not been \
                cleaned up. Error message pointers should not be reused for multiple calls."
            )
        }

        let msg: Vec<u8> = err.to_string().into();
        *error_msg = UnmanagedVector::new(Some(msg));
    } else {
        // The caller provided a nil pointer for the error message.
        // That's not nice but we can live with it.
    }

    let errno: ErrorCode = match err {
        RustError::OutOfGas { .. } => ErrorCode::OutOfGas,
        _ => ErrorCode::Other,
    };
    errno
}

pub fn set_out<T>(value: T, out_ptr: Option<&mut T>) {
    if let Some(out_ref) = out_ptr {
        *out_ref = value;
    } else {
        // The caller provided a nil pointer for the output message.
        // That's not nice but we can live with it.
    }
}

/// If `result` is Ok, this writes the Ok value to `out` and returns 0.
/// Otherwise it writes the error message to `error_msg` and returns the error code.
pub fn to_c_result_binary(
    result: Result<UnmanagedVector, RustError>,
    error_msg_ptr: Option<&mut UnmanagedVector>,
    out_ptr: Option<&mut UnmanagedVector>,
) -> i32 {
    let code = match result {
        Ok(value) => {
            set_out(value, out_ptr);
            ErrorCode::Success
        }
        Err(error) => set_error(error, error_msg_ptr),
    };
    code.to_int()
}

/// If `result` is Ok, this writes the Ok value to `out` and returns 0.
/// Otherwise it writes the error message to `error_msg` and returns the error code.
pub fn to_c_result<T>(
    result: Result<T, RustError>,
    error_msg_ptr: Option<&mut UnmanagedVector>,
    out_ptr: Option<&mut T>,
) -> i32 {
    let code = match result {
        Ok(value) => {
            set_out(value, out_ptr);
            ErrorCode::Success
        }
        Err(error) => set_error(error, error_msg_ptr),
    };
    code.to_int()
}

/// If `result` is Ok, this writes the Ok value to `out` and returns 0.
/// Otherwise it writes the error message to `error_msg` and returns the error code.
pub fn to_c_result_unit(
    result: Result<(), RustError>,
    error_msg_ptr: Option<&mut UnmanagedVector>,
) -> i32 {
    let code = match result {
        Ok(_) => ErrorCode::Success,
        Err(error) => set_error(error, error_msg_ptr),
    };
    code.to_int()
}

#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm_vm::BackendError;
    use std::str;

    #[test]
    fn empty_arg_works() {
        let error = RustError::empty_arg("gas");
        match error {
            RustError::EmptyArg { name, .. } => {
                assert_eq!(name, "gas");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn invalid_utf8_works_for_strings() {
        let error = RustError::invalid_utf8("my text");
        match error {
            RustError::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "my text");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn invalid_utf8_works_for_errors() {
        let original = String::from_utf8(vec![0x80]).unwrap_err();
        let error = RustError::invalid_utf8(original);
        match error {
            RustError::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 1 bytes from index 0");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn panic_works() {
        let error = RustError::panic();
        match error {
            RustError::Panic { .. } => {}
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn unset_arg_works() {
        let error = RustError::unset_arg("gas");
        match error {
            RustError::UnsetArg { name, .. } => {
                assert_eq!(name, "gas");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn vm_err_works_for_strings() {
        let error = RustError::vm_err("my text");
        match error {
            RustError::VmErr { msg, .. } => {
                assert_eq!(msg, "my text");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn vm_err_works_for_errors() {
        // No public interface exists to generate a VmError directly
        let original: VmError = BackendError::out_of_gas().into();
        let error = RustError::vm_err(original);
        match error {
            RustError::VmErr { msg, .. } => {
                assert_eq!(msg, "Ran out of gas during contract execution");
            }
            _ => panic!("expect different error"),
        }
    }

    // Tests of `impl From<X> for RustError` converters

    #[test]
    fn from_std_str_utf8error_works() {
        let error: RustError = str::from_utf8(b"Hello \xF0\x90\x80World")
            .unwrap_err()
            .into();
        match error {
            RustError::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 3 bytes from index 6")
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn from_std_string_fromutf8error_works() {
        let error: RustError = String::from_utf8(b"Hello \xF0\x90\x80World".to_vec())
            .unwrap_err()
            .into();
        match error {
            RustError::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 3 bytes from index 6")
            }
            _ => panic!("expect different error"),
        }
    }
}
