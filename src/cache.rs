use std::panic::catch_unwind;
use std::str::from_utf8;

use cosmwasm_vm::{features_from_csv, Cache, CacheOptions, Size};

use crate::api::GoApi;
use crate::args::{DATA_DIR_ARG, FEATURES_ARG};
use crate::db::GoStorage;
use crate::error::{clear_error, set_error, Error};
use crate::memory::Buffer;
use crate::querier::GoQuerier;

const MEMORY_CACHE_SIZE: Size = Size::mebi(500); // TODO: Make configurable

#[repr(C)]
pub struct cache_t {}

pub fn to_cache(ptr: *mut cache_t) -> Option<&'static mut Cache<GoStorage, GoApi, GoQuerier>> {
    if ptr.is_null() {
        None
    } else {
        let c = unsafe { &mut *(ptr as *mut Cache<GoStorage, GoApi, GoQuerier>) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn init_cache(
    data_dir: Buffer,
    supported_features: Buffer,
    err: Option<&mut Buffer>,
) -> *mut cache_t {
    let r = catch_unwind(|| do_init_cache(data_dir, supported_features))
        .unwrap_or_else(|_| Err(Error::panic()));
    match r {
        Ok(t) => {
            clear_error();
            t as *mut cache_t
        }
        Err(e) => {
            set_error(e, err);
            std::ptr::null_mut()
        }
    }
}

pub fn do_init_cache(
    data_dir: Buffer,
    supported_features: Buffer,
) -> Result<*mut Cache<GoStorage, GoApi, GoQuerier>, Error> {
    let dir = unsafe { data_dir.read() }.ok_or_else(|| Error::empty_arg(DATA_DIR_ARG))?;
    let dir_str = String::from_utf8(dir.to_vec())?;
    // parse the supported features
    let features_bin =
        unsafe { supported_features.read() }.ok_or_else(|| Error::empty_arg(FEATURES_ARG))?;
    let features_str = from_utf8(features_bin)?;
    let features = features_from_csv(features_str);
    let options = CacheOptions {
        base_dir: dir_str.into(),
        supported_features: features,
        memory_cache_size: MEMORY_CACHE_SIZE,
    };
    let cache = unsafe { Cache::new(options) }?;
    let out = Box::new(cache);
    Ok(Box::into_raw(out))
}
