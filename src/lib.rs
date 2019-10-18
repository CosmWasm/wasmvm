mod db;
mod error;
mod memory;

pub use db::{db_t, DB};
pub use memory::{free_rust, Buffer};

use std::panic::catch_unwind;
use crate::error::{handle_c_error, set_error};

#[no_mangle]
pub extern "C" fn greet(name: Buffer) -> Buffer {
    let rname = name.read().unwrap_or(b"<nil>");
    let mut v = b"Hello, ".to_vec();
    v.extend_from_slice(rname);
    Buffer::from_vec(v)
}

#[no_mangle]
pub extern "C" fn create(data_dir: Buffer, wasm: Buffer, err: Option<&mut Buffer>) -> Buffer {
    // TODO
    set_error("not implemented".to_string(), err);
    Buffer::default()
}

#[no_mangle]
pub extern "C" fn get_code(data_dir: Buffer, id: Buffer, err: Option<&mut Buffer>) -> Buffer {
    // TODO
    set_error("not implemented".to_string(), err);
    Buffer::default()
}

#[no_mangle]
pub extern "C" fn instantiate(
    data_dir: Buffer,
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
    data_dir: Buffer,
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
    data_dir: Buffer,
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
