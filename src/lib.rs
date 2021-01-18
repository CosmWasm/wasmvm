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
pub use memory::{free_rust, Buffer};
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

use crate::args::{
    ARG1, ARG2, CACHE_ARG, CODE_ID_ARG, CONTRACT_INFO_ARG, ENV_ARG, GAS_USED_ARG, INFO_ARG,
    MSG_ARG, WASM_ARG,
};
use crate::cache::{cache_t, to_cache};
use crate::error::{handle_c_error, Error};

#[derive(Copy, Clone)]
#[repr(C)]
/// A set of flags exposing some aspects of the contract configuration
pub struct ContractFlags {
    pub ibc_enabled: bool,
    pub stargate: bool,
    // TODO: add other flags here as needed
}

fn into_backend(db: DB, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}

/// frees a cache reference
///
/// # Safety
///
/// This must be called exactly once for any `*cache_t` returned by `init_cache`
/// and cannot be called on any other pointer.
#[no_mangle]
pub extern "C" fn release_cache(cache: *mut cache_t) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = unsafe { Box::from_raw(cache as *mut Cache<GoApi, GoStorage, GoQuerier>) };
    }
}

#[no_mangle]
pub extern "C" fn create(cache: *mut cache_t, wasm: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_create(c, wasm)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_create(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    wasm: Buffer,
) -> Result<Checksum, Error> {
    let wasm = unsafe { wasm.read() }.ok_or_else(|| Error::empty_arg(WASM_ARG))?;
    let checksum = cache.save_wasm(wasm)?;
    Ok(checksum)
}

#[no_mangle]
pub extern "C" fn get_code(
    cache: *mut cache_t,
    contract_checksum: Buffer,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_get_code(c, contract_checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_get_code(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    contract_checksum: Buffer,
) -> Result<Vec<u8>, Error> {
    let contract_checksum: Checksum = unsafe { contract_checksum.read() }
        .ok_or_else(|| Error::empty_arg(CACHE_ARG))?
        .try_into()?;
    let wasm = cache.load_wasm(&contract_checksum)?;
    Ok(wasm)
}

#[no_mangle]
pub extern "C" fn instantiate(
    cache: *mut cache_t,
    contract_checksum: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    contract_info: Option<&mut ContractFlags>,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_init(
        cache,
        contract_checksum,
        env,
        info,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        contract_info,
        gas_used,
        err,
    )
}

#[no_mangle]
pub extern "C" fn handle(
    cache: *mut cache_t,
    contract_checksum: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    call_3_args(
        call_migrate_raw,
        cache,
        contract_checksum,
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
pub extern "C" fn query(
    cache: *mut cache_t,
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    contract_checksum: Buffer,
    env: Buffer,
    msg: Buffer,
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
        contract_checksum,
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
    env: &[u8],
    msg: &[u8],
) -> VmResult<Vec<u8>>;

// this wraps all error handling and ffi for the 6 ibc entry points and query.
// (all of which take env and one "msg" argument).
// the only difference is which low-level function they dispatch to.
fn call_2_args(
    vm_fn: VmFn2Args,
    cache: *mut cache_t,
    contract_checksum: Buffer,
    arg1: Buffer,
    arg2: Buffer,
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
                contract_checksum,
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
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

// this is internal processing, same for all the 6 ibc entry points
fn do_call_2_args(
    vm_fn: VmFn2Args,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    contract_checksum: Buffer,
    arg1: Buffer,
    arg2: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let contract_checksum: Checksum = unsafe { contract_checksum.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let arg1 = unsafe { arg1.read() }.ok_or_else(|| Error::empty_arg(ARG1))?;
    let arg2 = unsafe { arg2.read() }.ok_or_else(|| Error::empty_arg(ARG2))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&contract_checksum, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = vm_fn(&mut instance, arg1, arg2);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

type VmFn3Args = fn(
    instance: &mut Instance<GoApi, GoStorage, GoQuerier>,
    env: &[u8],
    info: &[u8],
    msg: &[u8],
) -> VmResult<Vec<u8>>;

// this wraps all error handling and ffi for handle, init, and migrate.
// (and anything else that takes env, info and msg arguments).
// the only difference is which low-level function they dispatch to.
fn call_3_args(
    vm_fn: VmFn3Args,
    cache: *mut cache_t,
    contract_checksum: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
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
                contract_checksum,
                env,
                info,
                msg,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                gas_used,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_call_3_args(
    vm_fn: VmFn3Args,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    contract_checksum: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let contract_checksum: Checksum = unsafe { contract_checksum.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let env = unsafe { env.read() }.ok_or_else(|| Error::empty_arg(ENV_ARG))?;
    let info = unsafe { info.read() }.ok_or_else(|| Error::empty_arg(INFO_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&contract_checksum, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = vm_fn(&mut instance, env, info, msg);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

/// this is just for init. It is like the 3 args, but take a callback
/// to return some info on the contract
fn call_init(
    cache: *mut cache_t,
    code_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    contract_info: Option<&mut ContractFlags>,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_call_init(
                c,
                code_id,
                env,
                info,
                msg,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                contract_info,
                gas_used,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_call_init(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    code_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    contract_info: Option<&mut ContractFlags>,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let contract_info = contract_info.ok_or_else(|| Error::empty_arg(CONTRACT_INFO_ARG))?;

    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let env = unsafe { env.read() }.ok_or_else(|| Error::empty_arg(ENV_ARG))?;
    let info = unsafe { info.read() }.ok_or_else(|| Error::empty_arg(INFO_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&code_id, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_init_raw(&mut instance, env, info, msg);
    *gas_used = instance.create_gas_report().used_internally;
    // add some flags about the contract here
    contract_info.ibc_enabled = instance.has_ibc_entry_points();
    contract_info.stargate = instance.required_features.contains("stargate");
    instance.recycle();
    Ok(res?)
}
