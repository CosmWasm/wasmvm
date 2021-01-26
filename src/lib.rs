#![cfg_attr(feature = "backtraces", feature(backtrace))]

mod api;
mod args;
mod cache;
mod db;
mod error;
mod gas_meter;
mod iterator;
mod memory;
mod querier;
mod storage;
mod tests;

pub use api::GoApi;
pub use db::{db_t, DB};
pub use memory::{free_rust, Buffer, ByteSliceView};
pub use querier::GoQuerier;
pub use storage::GoStorage;

use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};

use cosmwasm_vm::{
    call_handle_raw, call_ibc_channel_close_raw, call_ibc_channel_connect_raw,
    call_ibc_channel_open_raw, call_ibc_packet_ack_raw, call_ibc_packet_receive_raw,
    call_ibc_packet_timeout_raw, call_init_raw, call_migrate_raw, call_query_raw, Backend, Cache,
    Checksum, Instance, InstanceOptions, VmResult,
};

use crate::args::{ARG1, ARG2, ARG3, CACHE_ARG, CHECKSUM_ARG, GAS_USED_ARG};
use crate::cache::{cache_t, to_cache};
use crate::error::{handle_c_error_binary, Error};

fn into_backend(db: DB, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}

#[no_mangle]
pub extern "C" fn instantiate(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    info: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_3_args(
        call_init_raw,
        cache,
        checksum,
        env,
        info,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn handle(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    info: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_3_args(
        call_handle_raw,
        cache,
        checksum,
        env,
        info,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn migrate(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_migrate_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn query(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_query_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn ibc_channel_open(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_ibc_channel_open_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn ibc_channel_connect(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_ibc_channel_connect_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn ibc_channel_close(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_ibc_channel_close_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn ibc_packet_receive(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_ibc_packet_receive_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn ibc_packet_ack(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_ibc_packet_ack_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn ibc_packet_timeout(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_2_args(
        call_ibc_packet_timeout_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_used,
        err,
    )
}

type VmFn2Args = fn(
    instance: &mut Instance<GoApi, GoStorage, GoQuerier>,
    arg1: &[u8],
    arg2: &[u8],
) -> VmResult<Vec<u8>>;

// this wraps all error handling and ffi for the 6 ibc entry points and query.
// (all of which take env and one "msg" argument).
// the only difference is which low-level function they dispatch to.
fn call_2_args(
    vm_fn: VmFn2Args,
    cache: *mut cache_t,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_call_2_args(
                vm_fn,
                c,
                checksum,
                arg1,
                arg2,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                gas_used,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, err);
    Buffer::from_vec(data)
}

// this is internal processing, same for all the 6 ibc entry points
fn do_call_2_args(
    vm_fn: VmFn2Args,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let arg1 = arg1.read().ok_or_else(|| Error::unset_arg(ARG1))?;
    let arg2 = arg2.read().ok_or_else(|| Error::unset_arg(ARG2))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&checksum, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = vm_fn(&mut instance, arg1, arg2);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

type VmFn3Args = fn(
    instance: &mut Instance<GoApi, GoStorage, GoQuerier>,
    arg1: &[u8],
    arg2: &[u8],
    arg3: &[u8],
) -> VmResult<Vec<u8>>;

// this wraps all error handling and ffi for handle, init, and migrate.
// (and anything else that takes env, info and msg arguments).
// the only difference is which low-level function they dispatch to.
fn call_3_args(
    vm_fn: VmFn3Args,
    cache: *mut cache_t,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    arg3: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_call_3_args(
                vm_fn,
                c,
                checksum,
                arg1,
                arg2,
                arg3,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                gas_used,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, err);
    Buffer::from_vec(data)
}

fn do_call_3_args(
    vm_fn: VmFn3Args,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    arg3: ByteSliceView,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let arg1 = arg1.read().ok_or_else(|| Error::unset_arg(ARG1))?;
    let arg2 = arg2.read().ok_or_else(|| Error::unset_arg(ARG2))?;
    let arg3 = arg3.read().ok_or_else(|| Error::unset_arg(ARG3))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&checksum, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = vm_fn(&mut instance, arg1, arg2, arg3);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}
