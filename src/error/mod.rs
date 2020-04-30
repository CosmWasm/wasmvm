mod rust;

pub use rust::{clear_error, handle_c_error, set_error, EmptyArg, Error, Panic, Utf8Err, WasmErr};

pub(crate) use rust::empty_err;
