mod db;
mod error;
mod memory;

pub use db::{db_t, DB};
pub use memory::{free_rust, Buffer};

use failure::{bail, format_err, Error};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::str::from_utf8;

use crate::error::{clear_error, handle_c_error, set_error};
use cosmwasm_vm::{CosmCache, call_handle_raw, call_init_raw};

#[repr(C)]
pub struct cache_t {}

fn to_cache(ptr: *mut cache_t) -> Option<&'static mut CosmCache> {
    if ptr.is_null() {
        None
    } else {
        let c: &mut CosmCache = unsafe { &mut *(ptr as *mut CosmCache) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn init_cache(data_dir: Buffer, err: Option<&mut Buffer>) -> *mut cache_t {
    let r = catch_unwind(|| do_init_cache(data_dir)).unwrap_or_else(|_| bail!("Caught panic"));
    match r {
        Ok(t) => {
            clear_error();
            t as *mut cache_t
        }
        Err(e) => {
            set_error(e.to_string(), err);
            std::ptr::null_mut()
        }
    }
}

fn do_init_cache(data_dir: Buffer) -> Result<*mut CosmCache, Error> {
    let dir = data_dir
        .read()
        .ok_or_else(|| format_err!("empty data_dir"))?;
    let dir_str = from_utf8(dir)?;
    let cache = unsafe { CosmCache::new(dir_str) };
    let out = Box::new(cache);
    let res = Ok(Box::into_raw(out));
    res
}

#[no_mangle]
pub unsafe extern "C" fn release_cache(cache: *mut cache_t) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = Box::from_raw(cache as *mut CosmCache);
    }
}

#[no_mangle]
pub extern "C" fn create(cache: *mut cache_t, wasm: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_create(c, wasm)))
            .unwrap_or_else(|_| bail!("Caught panic")),
        None => Err(format_err!("cache argument is null")),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_create(cache: &mut CosmCache, wasm: Buffer) -> Result<Vec<u8>, Error> {
    let wasm = wasm
        .read()
        .ok_or_else(|| format_err!("empty wasm argument"))?;
    cache.save_wasm(wasm)
}

#[no_mangle]
pub extern "C" fn get_code(cache: *mut cache_t, id: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_get_code(c, id)))
            .unwrap_or_else(|_| bail!("Caught panic")),
        None => Err(format_err!("cache argument is null")),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_get_code(cache: &mut CosmCache, id: Buffer) -> Result<Vec<u8>, Error> {
    let id = id.read().ok_or_else(|| format_err!("empty id argument"))?;
    cache.load_wasm(id)
}

#[no_mangle]
pub extern "C" fn instantiate(
    cache: *mut cache_t,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_init(c, contract_id, params, msg, db, gas_limit)))
            .unwrap_or_else(|_| bail!("Caught panic")),
        None => Err(format_err!("cache argument is null")),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_init(
    cache: &mut CosmCache,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    // TODO: use gas_limit
    _gas_limit: i64,
) -> Result<Vec<u8>, Error> {
    let contract_id = contract_id.read().ok_or_else(|| format_err!("empty contract_id argument"))?;
    let params = params.read().ok_or_else(|| format_err!("empty params argument"))?;
    let msg = msg.read().ok_or_else(|| format_err!("empty msg argument"))?;

    let mut instance = cache.get_instance(contract_id)?;
    call_init_raw(&mut instance, params, msg)
}

#[no_mangle]
pub extern "C" fn handle(
    cache: *mut cache_t,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_handle(c, contract_id, params, msg, db, gas_limit)))
            .unwrap_or_else(|_| bail!("Caught panic")),
        None => Err(format_err!("cache argument is null")),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_handle(
    cache: &mut CosmCache,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    // TODO: use gas_limit
    _gas_limit: i64,
) -> Result<Vec<u8>, Error> {
    let contract_id = contract_id.read().ok_or_else(|| format_err!("empty contract_id argument"))?;
    let params = params.read().ok_or_else(|| format_err!("empty params argument"))?;
    let msg = msg.read().ok_or_else(|| format_err!("empty msg argument"))?;

    let mut instance = cache.get_instance(contract_id)?;
    call_handle_raw(&mut instance, params, msg)
}

#[no_mangle]
pub extern "C" fn query(
    cache: *mut cache_t,
    contract_id: Buffer,
    path: Buffer,
    data: Buffer,
    db: DB,
    gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    // TODO
    set_error("not implemented".to_string(), err);
    Buffer::default()
}
