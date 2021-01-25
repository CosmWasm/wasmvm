use std::convert::TryInto;
use std::panic::catch_unwind;
use std::str::from_utf8;

use cosmwasm_vm::{features_from_csv, Cache, CacheOptions, Size};

use crate::api::GoApi;
use crate::args::{DATA_DIR_ARG, FEATURES_ARG};
use crate::error::{clear_error, set_error, Error};
use crate::memory::Buffer;
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

#[repr(C)]
pub struct cache_t {}

pub fn to_cache(ptr: *mut cache_t) -> Option<&'static mut Cache<GoApi, GoStorage, GoQuerier>> {
    if ptr.is_null() {
        None
    } else {
        let c = unsafe { &mut *(ptr as *mut Cache<GoApi, GoStorage, GoQuerier>) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn init_cache(
    data_dir: Buffer,
    supported_features: Buffer,
    cache_size: u32,
    instance_memory_limit: u32,
    err: Option<&mut Buffer>,
) -> *mut cache_t {
    let r = catch_unwind(|| {
        do_init_cache(
            data_dir,
            supported_features,
            cache_size,
            instance_memory_limit,
        )
    })
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

fn do_init_cache(
    data_dir: Buffer,
    supported_features: Buffer,
    cache_size: u32,
    instance_memory_limit: u32, // in MiB
) -> Result<*mut Cache<GoApi, GoStorage, GoQuerier>, Error> {
    let dir = unsafe { data_dir.read() }.ok_or_else(|| Error::empty_arg(DATA_DIR_ARG))?;
    let dir_str = String::from_utf8(dir.to_vec())?;
    // parse the supported features
    let features_bin =
        unsafe { supported_features.read() }.ok_or_else(|| Error::empty_arg(FEATURES_ARG))?;
    let features_str = from_utf8(features_bin)?;
    let features = features_from_csv(features_str);
    let memory_cache_size = Size::mebi(
        cache_size
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let instance_memory_limit = Size::mebi(
        instance_memory_limit
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let options = CacheOptions {
        base_dir: dir_str.into(),
        supported_features: features,
        memory_cache_size,
        instance_memory_limit,
    };
    let cache = unsafe { Cache::new(options) }?;
    let out = Box::new(cache);
    Ok(Box::into_raw(out))
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

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn init_cache_and_release_cache_work() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let mut err = Buffer::default();
        let features: &[u8] = b"staking";
        let cache_ptr = init_cache(
            dir.as_bytes().into(),
            features.into(),
            512,
            32,
            Some(&mut err),
        );
        assert_eq!(err.len, 0);
        release_cache(cache_ptr);
    }
}
