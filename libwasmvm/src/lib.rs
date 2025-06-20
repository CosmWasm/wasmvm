#![cfg_attr(feature = "backtraces", feature(backtrace))]
#![allow(clippy::not_unsafe_ptr_arg_deref, clippy::missing_safety_doc)]

mod api;
mod args;
mod cache;
mod calls;
mod db;
mod error;
mod gas_meter;
mod gas_report;
mod handle_vm_panic;
mod iterator;
mod memory;
mod querier;
mod storage;
mod test_utils;
mod tests;
mod version;
mod vtables;

// We only interact with this crate via `extern "C"` interfaces, not those public
// exports. There are no guarantees those exports are stable.
// We keep them here such that we can access them in the docs (`cargo doc`).
pub use api::{api_t, GoApi, GoApiVtable};
// FFI cache functions
pub use cache::{
    analyze_code, cache_t, get_metrics, get_pinned_metrics, init_cache, load_wasm, pin,
    remove_wasm, store_code, unpin,
};
// FFI call functions
pub use calls::{
    execute, ibc2_packet_ack, ibc2_packet_receive, ibc2_packet_send, ibc2_packet_timeout,
    ibc_channel_close, ibc_channel_connect, ibc_channel_open, ibc_destination_callback,
    ibc_packet_ack, ibc_packet_receive, ibc_packet_timeout, ibc_source_callback, instantiate,
    migrate, query, reply, sudo,
};
pub use db::{db_t, Db, DbVtable};
pub use error::GoError;
pub use gas_meter::gas_meter_t;
pub use gas_report::GasReport;
pub use iterator::{GoIter, IteratorVtable};
pub use memory::{
    destroy_unmanaged_vector, new_unmanaged_vector, ByteSliceView, U8SliceView, UnmanagedVector,
};
pub use querier::{querier_t, GoQuerier, QuerierVtable};
pub use storage::GoStorage;
pub use vtables::Vtable;
