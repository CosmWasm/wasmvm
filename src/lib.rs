#![cfg_attr(feature = "backtraces", feature(backtrace))]

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

// Why are those symbols public? Needed for muslc static lib?
// We should only need the `extern "C"`s.
pub use api::GoApi;
pub use db::{db_t, DB};
pub use memory::{Buffer, ByteSliceView};
pub use querier::GoQuerier;
pub use storage::GoStorage;
