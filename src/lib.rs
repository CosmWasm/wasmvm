mod db;
mod error;
mod memory;

pub use db::{db_t, DB};
pub use memory::{free_rust, Buffer};

use std::panic::{catch_unwind, AssertUnwindSafe};
use std::str::from_utf8;
use snafu::ResultExt;

use cosmwasm_vm::{CosmCache, call_handle_raw, call_init_raw};
use crate::error::{EmptyArg, Error, Panic, Utf8Err, WasmErr};
use crate::error::{clear_error, handle_c_error, set_error};

#[repr(C)]
pub struct cache_t {}

fn to_cache(ptr: *mut cache_t) -> Option<&'static mut CosmCache<DB>> {
    if ptr.is_null() {
        None
    } else {
        let c = unsafe { &mut *(ptr as *mut CosmCache<DB>) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn init_cache(data_dir: Buffer, cache_size: usize, err: Option<&mut Buffer>) -> *mut cache_t {
    let r = catch_unwind(|| do_init_cache(data_dir, cache_size)).
        unwrap_or_else(|_| Panic{}.fail());
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

// store some common string for argument names
static DATA_DIR_ARG: &str = "data_dir";
static CACHE_ARG: &str = "cache";
static WASM_ARG: &str = "wasm";
static CODE_ID_ARG: &str = "code_id";
static MSG_ARG: &str = "msg";
static PARAMS_ARG: &str = "params";

fn do_init_cache(data_dir: Buffer, cache_size: usize) -> Result<*mut CosmCache<DB>, Error> {
    let dir = data_dir
        .read()
        .ok_or_else(|| EmptyArg{name: DATA_DIR_ARG}.fail::<()>().unwrap_err() )?;
    let dir_str = from_utf8(dir).context(Utf8Err{})?;
    let cache = unsafe { CosmCache::new(dir_str, cache_size).context(WasmErr{})? };
    let out = Box::new(cache);
    let res = Ok(Box::into_raw(out));
    res
}

#[no_mangle]
pub unsafe extern "C" fn release_cache(cache: *mut cache_t) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = Box::from_raw(cache as *mut CosmCache<DB>);
    }
}

#[no_mangle]
pub extern "C" fn create(cache: *mut cache_t, wasm: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_create(c, wasm))).
            unwrap_or_else(|_| Panic{}.fail()),
        None => EmptyArg{name: CACHE_ARG}.fail(),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_create(cache: &mut CosmCache<DB>, wasm: Buffer) -> Result<Vec<u8>, Error> {
    let wasm = wasm
        .read()
        .ok_or_else(|| EmptyArg{name: WASM_ARG}.fail::<()>().unwrap_err())?;
    cache.save_wasm(wasm).context(WasmErr{})
}

#[no_mangle]
pub extern "C" fn get_code(cache: *mut cache_t, id: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_get_code(c, id))).
            unwrap_or_else(|_| Panic{}.fail()),
        None => EmptyArg{name: CACHE_ARG}.fail(),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_get_code(cache: &mut CosmCache<DB>, id: Buffer) -> Result<Vec<u8>, Error> {
    let id = id.read().ok_or_else(|| EmptyArg{name: CACHE_ARG}.fail::<()>().unwrap_err())?;
    cache.load_wasm(id).context(WasmErr{})
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
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_init(c, contract_id, params, msg, db, gas_limit))).
            unwrap_or_else(|_| Panic{}.fail()),
        None => EmptyArg{name: CACHE_ARG}.fail(),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_init(
    cache: &mut CosmCache<DB>,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    // TODO: use gas_limit
    _gas_limit: i64,
) -> Result<Vec<u8>, Error> {
    let code_id = code_id.read().ok_or_else(|| EmptyArg{name: CODE_ID_ARG}.fail::<()>().unwrap_err())?;
    let params = params.read().ok_or_else(|| EmptyArg{name: PARAMS_ARG}.fail::<()>().unwrap_err())?;
    let msg = msg.read().ok_or_else(|| EmptyArg{name: MSG_ARG}.fail::<()>().unwrap_err())?;

    let mut instance = cache.get_instance(code_id, db).context(WasmErr {})?;
    let res = call_init_raw(&mut instance, params, msg).context(WasmErr {})?;
    cache.store_instance(code_id, instance);
    Ok(res)
}

#[no_mangle]
pub extern "C" fn handle(
    cache: *mut cache_t,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_handle(c, code_id, params, msg, db, gas_limit))).
            unwrap_or_else(|_| Panic{}.fail()),
        None => EmptyArg{name: CACHE_ARG}.fail(),
    };
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_handle(
    cache: &mut CosmCache<DB>,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    // TODO: use gas_limit
    _gas_limit: i64,
) -> Result<Vec<u8>, Error> {
    let code_id = code_id.read().ok_or_else(|| EmptyArg{name: CODE_ID_ARG}.fail::<()>().unwrap_err())?;
    let params = params.read().ok_or_else(|| EmptyArg{name: PARAMS_ARG}.fail::<()>().unwrap_err())?;
    let msg = msg.read().ok_or_else(|| EmptyArg{name: MSG_ARG}.fail::<()>().unwrap_err())?;

    let mut instance = cache.get_instance(code_id, db).context(WasmErr {})?;
    let res = call_handle_raw(&mut instance, params, msg).context(WasmErr {})?;
    cache.store_instance(code_id, instance);
    Ok(res)
}

#[no_mangle]
pub extern "C" fn query(
    _cache: *mut cache_t,
    _code_id: Buffer,
    _path: Buffer,
    _data: Buffer,
    _db: DB,
    _gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    // TODO
    set_error("not implemented".to_string(), err);
    Buffer::default()
}
