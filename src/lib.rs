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
    call_handle_raw, call_init_raw, call_migrate_raw, call_query_raw, Backend, Cache, Checksum,
    InstanceOptions, Size,
};

use crate::args::{CACHE_ARG, CODE_ID_ARG, ENV_ARG, GAS_USED_ARG, INFO_ARG, MSG_ARG, WASM_ARG};
use crate::cache::{cache_t, to_cache};
use crate::error::{handle_c_error, Error};

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
pub extern "C" fn get_code(cache: *mut cache_t, id: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_get_code(c, id)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_get_code(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    id: Buffer,
) -> Result<Vec<u8>, Error> {
    let id: Checksum = unsafe { id.read() }
        .ok_or_else(|| Error::empty_arg(CACHE_ARG))?
        .try_into()?;
    let wasm = cache.load_wasm(&id)?;
    Ok(wasm)
}

#[no_mangle]
pub extern "C" fn instantiate(
    cache: *mut cache_t,
    contract_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: u32, // in MiB
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let memory_limit = Size::mebi(
        memory_limit
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_init(
                c,
                contract_id,
                env,
                info,
                msg,
                db,
                api,
                querier,
                gas_limit,
                memory_limit,
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

fn do_init(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    code_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: Size,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let env = unsafe { env.read() }.ok_or_else(|| Error::empty_arg(ENV_ARG))?;
    let info = unsafe { info.read() }.ok_or_else(|| Error::empty_arg(INFO_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        memory_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&code_id, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_init_raw(&mut instance, env, info, msg);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn handle(
    cache: *mut cache_t,
    code_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: u32, // in MiB
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let memory_limit = Size::mebi(
        memory_limit
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_handle(
                c,
                code_id,
                env,
                info,
                msg,
                db,
                api,
                querier,
                gas_limit,
                memory_limit,
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

fn do_handle(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    code_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: Size,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let env = unsafe { env.read() }.ok_or_else(|| Error::empty_arg(ENV_ARG))?;
    let info = unsafe { info.read() }.ok_or_else(|| Error::empty_arg(INFO_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        memory_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&code_id, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_handle_raw(&mut instance, env, info, msg);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn migrate(
    cache: *mut cache_t,
    contract_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: u32, // in MiB
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let memory_limit = Size::mebi(
        memory_limit
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_migrate(
                c,
                contract_id,
                env,
                info,
                msg,
                db,
                api,
                querier,
                gas_limit,
                memory_limit,
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

fn do_migrate(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    code_id: Buffer,
    env: Buffer,
    info: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: Size,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let env = unsafe { env.read() }.ok_or_else(|| Error::empty_arg(ENV_ARG))?;
    let info = unsafe { info.read() }.ok_or_else(|| Error::empty_arg(INFO_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        memory_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&code_id, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_migrate_raw(&mut instance, env, info, msg);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn query(
    cache: *mut cache_t,
    code_id: Buffer,
    env: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: u32, // in MiB
    print_debug: bool,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let memory_limit = Size::mebi(
        memory_limit
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_query(
                c,
                code_id,
                env,
                msg,
                db,
                api,
                querier,
                gas_limit,
                memory_limit,
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

fn do_query(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    code_id: Buffer,
    env: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    memory_limit: Size,
    print_debug: bool,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let env = unsafe { env.read() }.ok_or_else(|| Error::empty_arg(ENV_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        memory_limit,
        print_debug,
    };
    let mut instance = cache.get_instance(&code_id, backend, options)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_query_raw(&mut instance, env, msg);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}
