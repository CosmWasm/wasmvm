#![no_main]

use std::fs;
use std::path::PathBuf;

use arbitrary::Arbitrary;
use cosmwasm_vm::{capabilities_from_csv, Cache, CacheOptions, Size};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);

#[derive(Arbitrary, Debug)]
struct PinUnpinFuzzInput {
    pin_first: bool,
    pin_second: bool,
    unpin_first: bool,
    unpin_second: bool,
}

fuzz_target!(|input: PinUnpinFuzzInput| {
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

    // Create cache
    let cache = match unsafe { Cache::new(options) } {
        Ok(cache) => cache,
        Err(_) => return,
    };

    // Get two test WASM files
    let wasm_paths = [
        PathBuf::from("../../testdata/hackatom.wasm"),
        PathBuf::from("../../testdata/cyberpunk.wasm"),
    ];

    let mut checksums = Vec::new();

    // Load and store both contracts
    for path in &wasm_paths {
        if let Ok(wasm) = fs::read(path) {
            if let Ok(checksum) = cache.store_code(&wasm, true, true) {
                checksums.push(checksum);
            }
        }
    }

    // Skip if we don't have at least one checksum
    if checksums.is_empty() {
        return;
    }

    // Perform pin/unpin operations based on the fuzzing input
    if input.pin_first && !checksums.is_empty() {
        let _ = cache.pin(&checksums[0]);
    }

    if input.pin_second && checksums.len() > 1 {
        let _ = cache.pin(&checksums[1]);
    }

    if input.unpin_first && !checksums.is_empty() {
        let _ = cache.unpin(&checksums[0]);
    }

    if input.unpin_second && checksums.len() > 1 {
        let _ = cache.unpin(&checksums[1]);
    }

    // Get metrics to check state
    let _ = cache.get_metrics();
});
