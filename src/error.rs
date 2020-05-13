use errno::{set_errno, Errno};
use std::fmt;

use cosmwasm_vm::VmError;
use snafu::{ResultExt, Snafu};

use crate::memory::Buffer;

#[derive(Debug, Snafu)]
#[snafu(visibility = "pub")]
pub enum Error {
    #[snafu(display("Wasm Error: {}", source))]
    WasmErr {
        source: VmError,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Caught Panic"))]
    Panic {
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Null/Empty argument passed: {}", name))]
    EmptyArg {
        name: &'static str,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Invalid string format: {}", source))]
    Utf8Err {
        source: std::str::Utf8Error,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
}

// FIXME: simplify this (and more) when refactoring the errors
impl From<VmError> for Error {
    fn from(source: VmError) -> Self {
        let r: Result<(), VmError> = Err(source);
        r.context(WasmErr {}).unwrap_err()
    }
}

/// empty_err returns an error with stack trace.
/// helper to construct Error::EmptyArg over and over.
pub(crate) fn empty_err(name: &'static str) -> Error {
    EmptyArg { name }.fail::<()>().unwrap_err()
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
    E: fmt::Display,
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

/// This enum gives names to the status codes returned from Go callbacks to Rust.
///
/// The go code will return one of these variants when returning.
///
/// cbindgen:prefix-with-name
// NOTE TO DEVS: If you change the values assigned to the variants of this enum, You must also
//               update the match statement in the From conversion below.
//               Otherwise all hell may break loose.
//               You have been warned.
//
#[repr(i32)] // This makes it so the enum looks like a simple i32 to Go
pub enum GoResult {
    Ok = 0,
    /// Go panicked for an unexpected reason.
    Panic = 1,
    /// Go received a bad argument from Rust
    BadArgument = 2,
    /// Ran out of gas while using the SDK (e.g. storage)
    OutOfGas = 3,
    /// An error happened during normal operation of a Go callback
    Other = 4,
}

impl std::convert::From<i32> for GoResult {
    fn from(n: i32) -> Self {
        use GoResult::*;
        // This conversion treats any number that is not otherwise an expected value as `GoError::Other`
        match n {
            0 => Ok,
            1 => Panic,
            2 => BadArgument,
            3 => OutOfGas,
            _ => Other,
        }
    }
}

impl fmt::Display for GoResult {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            GoResult::Ok => write!(f, "Ok"),
            GoResult::Panic => write!(f, "Panic"),
            GoResult::BadArgument => write!(f, "BadArgument"),
            GoResult::OutOfGas => write!(f, "OutOfGas"),
            GoResult::Other => write!(f, "Other Error"),
        }
    }
}

impl GoResult {
    pub fn is_ok(&self) -> bool {
        match self {
            GoResult::Ok => true,
            _ => false,
        }
    }
}
