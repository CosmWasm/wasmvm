#![cfg_attr(feature = "backtraces", feature(backtrace))]
#![allow(clippy::not_unsafe_ptr_arg_deref)]

mod api;
mod args;
mod cache;
mod calls;
mod db;
mod error;
mod gas_meter;
mod iterator;
mod memory;
mod querier;
mod storage;
mod tests;

// We only interact with this crate via `extern "C"` interfaces, not those public
// exports. There are no guarantees those exports are stable.
// We keep them here such that we can access them in the docs (`cargo doc`).
pub use api::GoApi;
pub use cache::load_wasm;
pub use db::{db_t, DB};
pub use memory::{
    destroy_unmanaged_vector, new_unmanaged_vector, ByteSliceView, U8SliceView, UnmanagedVector,
};
pub use querier::GoQuerier;
pub use storage::GoStorage;
