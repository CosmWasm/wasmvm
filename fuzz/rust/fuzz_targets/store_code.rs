#![no_main]

use cosmwasm_vm::{
    capabilities_from_csv,
    testing::{MockApi, MockQuerier, MockStorage},
    Cache, CacheOptions, Size,
};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);

fuzz_target!(|data: &[u8]| {
    // Create a temp directory for the cache
    let temp_dir = match tempfile::tempdir() {
        Ok(dir) => dir,
        Err(_) => return,
    };

    // Create a cache with standard test capabilities
    let options = CacheOptions::new(
        temp_dir.path().to_path_buf(),
        capabilities_from_csv("staking"),
        MEMORY_CACHE_SIZE,
        MEMORY_LIMIT,
    );

    // Create cache with explicit type annotation
    let cache: Cache<MockApi, MockStorage, MockQuerier> = match unsafe { Cache::new(options) } {
        Ok(cache) => cache,
        Err(_) => return,
    };

    // Skip empty data
    if data.is_empty() {
        return;
    }

    // Test storing the code
    let _result = cache.store_code(data, true, true);

    // No need to check the result - fuzzing will identify any panics or crashes
    // The temp directory will be cleaned up automatically when it goes out of scope
});
