mod go;
mod rust;

pub use go::GoError;
pub use rust::RustError as Error;
pub use rust::{
    clear_error, handle_c_error_binary, handle_c_error_default, handle_c_error_ptr, set_error,
};
