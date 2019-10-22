mod db;
mod error;
mod memory;

pub use db::{db_t, DB};
pub use memory::{free_rust, Buffer};

use std::panic::catch_unwind;
use std::str::from_utf8;
use failure::{bail, Error, format_err};

use cosmwasm_vm::{CosmCache};
use crate::error::{handle_c_error, set_error};

#[no_mangle]
pub extern "C" fn greet(name: Buffer) -> Buffer {
    let rname = name.read().unwrap_or(b"<nil>");
    let mut v = b"Hello, ".to_vec();
    v.extend_from_slice(rname);
    Buffer::from_vec(v)
}

#[no_mangle]
pub extern "C" fn init_cache(data_dir: Buffer, err: Option<&mut Buffer>) -> *mut CosmCache {
    let r = catch_unwind(|| do_init_cache(data_dir)).unwrap_or_else(|_| bail!("Caught panic"));
    match r {
        Ok(t) => t,
        Err(e) => {
            set_error(e.to_string(), err);
            std::ptr::null_mut()
        }
    }
}

fn do_init_cache(data_dir: Buffer) -> Result<*mut CosmCache, Error> {
    let dir = data_dir.read().ok_or_else(||format_err!("empty data_dir"))?;
    let cache = unsafe { CosmCache::new(from_utf8(dir)?) };
    let out = Box::new(cache);
    Ok(Box::into_raw(out))
}

#[no_mangle]
pub extern "C" fn release_cache(cache: *mut CosmCache) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = unsafe { Box::from_raw(cache) };
    }
}

#[no_mangle]
pub extern "C" fn create(cache: Option<&mut CosmCache>, wasm: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = catch_unwind(|| do_create(cache, wasm)).unwrap_or_else(|_| bail!("Caught panic"));
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_create(cache: Option<&mut CosmCache>, wasm: Buffer) -> Result<Vec<u8>, Error> {
    match cache {
        Some(c) => {
            let wasm = wasm.read().ok_or_else(||format_err!("empty wasm argument"))?;
            c.save_wasm(wasm)
        },
        None => Err(format_err!("null cache argument")),
    }
}

#[no_mangle]
pub extern "C" fn get_code(cache: Option<&mut CosmCache>, id: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = catch_unwind(|| do_get_code(cache, id)).unwrap_or_else(|_| bail!("Caught panic"));
    let v = handle_c_error(r, err);
    Buffer::from_vec(v)
}

fn do_get_code(cache: Option<&mut CosmCache>, id: Buffer) -> Result<Vec<u8>, Error> {
        match cache {
            Some(c) => {
                let id = id.read().ok_or_else(||format_err!("empty id argument"))?;
                c.load_wasm(id)
            },
            None => Err(format_err!("null cache argument")),
        }
}


#[no_mangle]
pub extern "C" fn instantiate(
    cache: Option<&mut CosmCache>,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    // TODO
    set_error("not implemented".to_string(), err);
    Buffer::default()
}

#[no_mangle]
pub extern "C" fn handle(
    cache: Option<&mut CosmCache>,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    gas_limit: i64,
    err: Option<&mut Buffer>,
) -> Buffer {
    // TODO
    set_error("not implemented".to_string(), err);
    Buffer::default()
}

#[no_mangle]
pub extern "C" fn query(
    cache: Option<&mut CosmCache>,
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

/// divide returns the rounded (i32) result, returns a C error if div == 0
#[no_mangle]
pub extern "C" fn divide(num: i32, div: i32, err: Option<&mut Buffer>) -> i32 {
    if div == 0 {
        set_error("Cannot divide by zero".to_string(), err);
        return 0;
    }
    num / div
}

#[no_mangle]
pub extern "C" fn may_panic(guess: i32, err: Option<&mut Buffer>) -> Buffer {
    let r = catch_unwind(|| do_may_panic(guess)).unwrap_or(Err("Caught panic".to_string()));
    let v = handle_c_error(r, err).into_bytes();
    Buffer::from_vec(v)
}

fn do_may_panic(guess: i32) -> Result<String, String> {
    if guess == 0 {
        panic!("Must be negative or positive")
    } else if guess < 17 {
        Err("Too low".to_owned())
    } else {
        Ok("You are a winner!".to_owned())
    }
}

// This loads key from DB and then appends a "." and saves it
#[no_mangle]
pub extern "C" fn update_db(db: DB, key: Buffer, err: Option<&mut Buffer>) {
    let r = catch_unwind(|| do_update_db(&db, &key)).or(Err("Caught panic".to_string()));
    handle_c_error(r, err);
}

// note we need to panic inside another function, not a closure to ensure catching it
fn do_update_db(db: &DB, key: &Buffer) {
    // Note: panics on empty key for testing
    let vkey = key.read().unwrap().to_vec();
    let mut val = db.get(vkey.clone()).unwrap_or(Vec::new());
    val.extend_from_slice(b".");
    db.set(vkey, val);
}
