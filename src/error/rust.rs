use errno::{set_errno, Errno};
use std::fmt::Display;

use cosmwasm_vm::VmError;
use snafu::Snafu;

use crate::memory::Buffer;

#[derive(Debug, Snafu)]
#[snafu(visibility = "pub")]
pub enum Error {
    #[snafu(display("Null/Empty argument: {}", name))]
    EmptyArg {
        name: String,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    /// Whenever UTF-8 bytes cannot be decoded into a unicode string, e.g. in String::from_utf8 or str::from_utf8.
    #[snafu(display("Cannot decode UTF8 bytes into string: {}", msg))]
    InvalidUtf8 {
        msg: String,
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Ran out of gas"))]
    OutOfGas {
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Caught Panic"))]
    Panic {
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Error calling the VM: {}", msg))]
    VmErr {
        msg: String,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
}

impl Error {
    pub fn make_empty_arg<T: Into<String>>(name: T) -> Error {
        EmptyArg { name: name.into() }.build()
    }

    pub fn make_invalid_utf8<S: Display>(msg: S) -> Error {
        InvalidUtf8 {
            msg: msg.to_string(),
        }
        .build()
    }

    pub fn make_panic() -> Error {
        Panic {}.build()
    }

    pub fn make_vm_err<S: Display>(msg: S) -> Error {
        VmErr {
            msg: msg.to_string(),
        }
        .build()
    }
}

impl From<VmError> for Error {
    fn from(source: VmError) -> Self {
        Error::make_vm_err(source)
    }
}

pub fn clear_error() {
    set_errno(Errno(0));
}

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
        Ok(t) => {
            clear_error();
            t
        }
        Err(e) => {
            set_error(e.to_string(), errout);
            T::default()
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm_vm::make_ffi_out_of_gas;

    #[test]
    fn make_empty_arg_works() {
        let error = Error::make_empty_arg("gas");
        match error {
            Error::EmptyArg { name, .. } => {
                assert_eq!(name, "gas");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn make_invalid_utf8_works_for_strings() {
        let error = Error::make_invalid_utf8("my text");
        match error {
            Error::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "my text");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn make_invalid_utf8_works_for_errors() {
        let original = String::from_utf8(vec![0x80]).unwrap_err();
        let error = Error::make_invalid_utf8(original);
        match error {
            Error::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 1 bytes from index 0");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn make_panic_works() {
        let error = Error::make_panic();
        match error {
            Error::Panic { .. } => {}
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn make_vm_err_works_for_strings() {
        let error = Error::make_vm_err("my text");
        match error {
            Error::VmErr { msg, .. } => {
                assert_eq!(msg, "my text");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn make_vm_err_works_for_errors() {
        let original: VmError = make_ffi_out_of_gas().into();
        let error = Error::make_vm_err(original);
        match error {
            Error::VmErr { msg, .. } => {
                assert_eq!(msg, "Ran out of gas during contract execution");
            }
            _ => panic!("expect different error"),
        }
    }
}
