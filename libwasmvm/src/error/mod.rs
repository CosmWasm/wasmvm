mod errno;
mod go;
mod rust;

pub use go::GoError;
pub use rust::{to_c_result, to_c_result_binary, to_c_result_unit, RustError as Error};
