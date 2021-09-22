use std::collections::HashSet;
use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::str::from_utf8;

use cosmwasm_vm::{features_from_csv, Cache, CacheOptions, Checksum, Size};

use crate::api::GoApi;
use crate::args::{CACHE_ARG, CHECKSUM_ARG, DATA_DIR_ARG, FEATURES_ARG, WASM_ARG};
use crate::error::{clear_error, handle_c_error_binary, handle_c_error_default, set_error, Error};
use crate::memory::{ByteSliceView, UnmanagedVector};
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
    data_dir: ByteSliceView,
    supported_features: ByteSliceView,
    cache_size: u32,            // in MiB
    instance_memory_limit: u32, // in MiB
    error_msg: Option<&mut UnmanagedVector>,
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
            set_error(e, error_msg);
            std::ptr::null_mut()
        }
    }
}

fn do_init_cache(
    data_dir: ByteSliceView,
    supported_features: ByteSliceView,
    cache_size: u32,            // in MiB
    instance_memory_limit: u32, // in MiB
) -> Result<*mut Cache<GoApi, GoStorage, GoQuerier>, Error> {
    let dir = data_dir
        .read()
        .ok_or_else(|| Error::unset_arg(DATA_DIR_ARG))?;
    let dir_str = String::from_utf8(dir.to_vec())?;
    // parse the supported features
    let features_bin = supported_features
        .read()
        .ok_or_else(|| Error::unset_arg(FEATURES_ARG))?;
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

#[no_mangle]
pub extern "C" fn save_wasm(
    cache: *mut cache_t,
    wasm: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_save_wasm(c, wasm)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let checksum = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(checksum))
}

fn do_save_wasm(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    wasm: ByteSliceView,
) -> Result<Checksum, Error> {
    let wasm = wasm.read().ok_or_else(|| Error::unset_arg(WASM_ARG))?;
    let checksum = cache.save_wasm(wasm)?;
    Ok(checksum)
}

#[no_mangle]
pub extern "C" fn load_wasm(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_load_wasm(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(data))
}

fn do_load_wasm(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<Vec<u8>, Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let wasm = cache.load_wasm(&checksum)?;
    Ok(wasm)
}

#[no_mangle]
pub extern "C" fn pin(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_pin(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg);
}

fn do_pin(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    cache.pin(&checksum)?;
    Ok(())
}

#[no_mangle]
pub extern "C" fn unpin(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_unpin(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg);
}

fn do_unpin(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    cache.unpin(&checksum)?;
    Ok(())
}

/// The result type of the FFI function analyze_code.
///
/// Please note that the unmanaged vector in `required_features`
/// has to be destroyed exactly once. When calling `analyze_code`
/// from Go this is done via `C.destroy_unmanaged_vector`.
#[repr(C)]
#[derive(Copy, Clone, Default, Debug, PartialEq)]
pub struct AnalysisReport {
    pub has_ibc_entry_points: bool,
    /// An UTF-8 encoded comma separated list of reqired features.
    /// This is never None/nil.
    pub required_features: UnmanagedVector,
}

impl From<cosmwasm_vm::AnalysisReport> for AnalysisReport {
    fn from(report: cosmwasm_vm::AnalysisReport) -> Self {
        let cosmwasm_vm::AnalysisReport {
            has_ibc_entry_points,
            required_features,
        } = report;

        let required_features_utf8 = set_to_csv(required_features).into_bytes();
        AnalysisReport {
            has_ibc_entry_points,
            required_features: UnmanagedVector::new(Some(required_features_utf8)),
        }
    }
}

fn set_to_csv(set: HashSet<String>) -> String {
    let mut list: Vec<String> = set.into_iter().collect();
    list.sort_unstable();
    list.join(",")
}

#[no_mangle]
pub extern "C" fn analyze_code(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> AnalysisReport {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_analyze_code(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    match r {
        Ok(value) => {
            clear_error();
            value
        }
        Err(error) => {
            set_error(error, error_msg);
            AnalysisReport::default()
        }
    }
}

fn do_analyze_code(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<AnalysisReport, Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let report = cache.analyze(&checksum)?;
    Ok(report.into())
}

#[repr(C)]
#[derive(Copy, Clone, Default, Debug, PartialEq)]
pub struct Metrics {
    pub hits_pinned_memory_cache: u32,
    pub hits_memory_cache: u32,
    pub hits_fs_cache: u32,
    pub misses: u32,
    pub elements_pinned_memory_cache: u64,
    pub elements_memory_cache: u64,
    pub size_pinned_memory_cache: u64,
    pub size_memory_cache: u64,
}

impl From<cosmwasm_vm::Metrics> for Metrics {
    fn from(report: cosmwasm_vm::Metrics) -> Self {
        let cosmwasm_vm::Metrics {
            stats:
                cosmwasm_vm::Stats {
                    hits_pinned_memory_cache,
                    hits_memory_cache,
                    hits_fs_cache,
                    misses,
                },
            elements_pinned_memory_cache,
            elements_memory_cache,
            size_pinned_memory_cache,
            size_memory_cache,
        } = report;

        Metrics {
            hits_pinned_memory_cache,
            hits_memory_cache,
            hits_fs_cache,
            misses,
            elements_pinned_memory_cache: elements_pinned_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
            elements_memory_cache: elements_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
            size_pinned_memory_cache: size_pinned_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
            size_memory_cache: size_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
        }
    }
}

#[no_mangle]
pub extern "C" fn get_metrics(
    cache: *mut cache_t,
    error_msg: Option<&mut UnmanagedVector>,
) -> Metrics {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_get_metrics(c)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    match r {
        Ok(value) => {
            clear_error();
            value
        }
        Err(error) => {
            set_error(error, error_msg);
            Metrics::default()
        }
    }
}

#[allow(clippy::unnecessary_wraps)] // Keep unused Result for consistent boilerplate for all fn do_*
fn do_get_metrics(cache: &mut Cache<GoApi, GoStorage, GoQuerier>) -> Result<Metrics, Error> {
    Ok(cache.metrics().into())
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
    use std::iter::FromIterator;
    use tempfile::TempDir;

    static HACKATOM: &[u8] = include_bytes!("../../api/testdata/hackatom.wasm");
    static IBC_REFLECT: &[u8] = include_bytes!("../../api/testdata/ibc_reflect.wasm");

    #[test]
    fn init_cache_and_release_cache_work() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn init_cache_writes_error() {
        let dir: String = String::from("broken\0dir"); // null bytes are valid UTF8 but not allowed in FS paths
        let features = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert!(cache_ptr.is_null());
        assert_eq!(error_msg.is_some(), true);
        let msg = String::from_utf8(error_msg.consume().unwrap()).unwrap();
        assert_eq!(msg, "Error calling the VM: Cache error: Error creating directory broken\u{0}dir/state: data provided contains a nul byte");
    }

    #[test]
    fn save_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn load_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let wasm = load_wasm(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let wasm = wasm.consume().unwrap_or_default();
        assert_eq!(wasm, HACKATOM);

        release_cache(cache_ptr);
    }

    #[test]
    fn pin_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // pinning again has no effect
        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn unpin_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // Unpinning again has no effect
        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn analyze_code_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking,stargate,iterator";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum_hackatom = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum_hackatom = checksum_hackatom.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let checksum_ibc_reflect = save_wasm(
            cache_ptr,
            ByteSliceView::new(IBC_REFLECT),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum_ibc_reflect = checksum_ibc_reflect.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let hackatom_report = analyze_code(
            cache_ptr,
            ByteSliceView::new(&checksum_hackatom),
            Some(&mut error_msg),
        );
        let _ = error_msg.consume();
        assert_eq!(hackatom_report.has_ibc_entry_points, false);
        assert_eq!(hackatom_report.required_features.consume().unwrap(), b"");

        let mut error_msg = UnmanagedVector::default();
        let ibc_reflect_report = analyze_code(
            cache_ptr,
            ByteSliceView::new(&checksum_ibc_reflect),
            Some(&mut error_msg),
        );
        let _ = error_msg.consume();
        assert_eq!(ibc_reflect_report.has_ibc_entry_points, true);
        assert_eq!(
            ibc_reflect_report.required_features.consume().unwrap(),
            b"iterator,staking,stargate"
        );

        release_cache(cache_ptr);
    }

    #[test]
    fn set_to_csv_works() {
        assert_eq!(set_to_csv(HashSet::new()), "");
        assert_eq!(
            set_to_csv(HashSet::from_iter(vec!["foo".to_string()])),
            "foo",
        );
        assert_eq!(
            set_to_csv(HashSet::from_iter(vec![
                "foo".to_string(),
                "bar".to_string(),
                "baz".to_string(),
            ])),
            "bar,baz,foo",
        );
        assert_eq!(
            set_to_csv(HashSet::from_iter(vec![
                "a".to_string(),
                "aa".to_string(),
                "b".to_string(),
                "c".to_string(),
                "A".to_string(),
                "AA".to_string(),
                "B".to_string(),
                "C".to_string(),
            ])),
            "A,AA,B,C,a,aa,b,c",
        );
    }

    #[test]
    fn get_metrics_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features = b"staking";

        // Init cache
        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // Get metrics 1
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(metrics, Metrics::default());

        // Save wasm
        let mut error_msg = UnmanagedVector::default();
        let checksum_hackatom = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum_hackatom.consume().unwrap_or_default();

        // Get metrics 2
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(metrics, Metrics::default());

        // Pin
        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // Get metrics 3
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(
            metrics,
            Metrics {
                hits_pinned_memory_cache: 0,
                hits_memory_cache: 0,
                hits_fs_cache: 1,
                misses: 0,
                elements_pinned_memory_cache: 1,
                elements_memory_cache: 0,
                size_pinned_memory_cache: 4755566,
                size_memory_cache: 0,
            }
        );

        // Unpin
        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // Get metrics 4
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(
            metrics,
            Metrics {
                hits_pinned_memory_cache: 0,
                hits_memory_cache: 0,
                hits_fs_cache: 1,
                misses: 0,
                elements_pinned_memory_cache: 0,
                elements_memory_cache: 0,
                size_pinned_memory_cache: 0,
                size_memory_cache: 0,
            }
        );

        release_cache(cache_ptr);
    }
}
