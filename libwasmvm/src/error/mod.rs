mod errno;
mod go;
mod rust;

pub use go::GoError;
pub use rust::{
    handle_c_error_binary, handle_c_error_default, handle_c_error_ptr, to_c_result,
    to_c_result_binary, to_c_result_unit, RustError as Error,
};
