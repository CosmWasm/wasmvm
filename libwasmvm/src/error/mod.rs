mod go;
mod rust;

pub use go::GoResult;
pub use rust::{
    clear_error, handle_c_error_binary, handle_c_error_default, set_error, RustError as Error,
};
