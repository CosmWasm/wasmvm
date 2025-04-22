#![no_main]

use std::fs;
use std::path::PathBuf;

use arbitrary::Arbitrary;
use cosmwasm_vm::{
    call_instantiate_raw, capabilities_from_csv,
    testing::{mock_backend, mock_env, mock_info},
    to_vec, Cache, CacheOptions, Size,
};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);
const GAS_LIMIT: u64 = 200_000_000_000; // ~0.2ms

// Define a structure for our fuzzing input
#[derive(Arbitrary, Debug)]
struct InstantiateFuzzInput {
    #[arbitrary(with = |u: &mut arbitrary::Unstructured| u.bytes(100))]
    instantiate_msg: Vec<u8>,
}

fuzz_target!(|input: InstantiateFuzzInput| {
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

    // Get the test WASM file
    let wasm_path = PathBuf::from("../../testdata/hackatom.wasm");

    // Load the test WASM file
    let wasm = match fs::read(&wasm_path) {
        Ok(wasm) => wasm,
        Err(_) => return,
    };

    // Store the code
    let checksum = match cache.store_code(&wasm, true, true) {
        Ok(checksum) => checksum,
        Err(_) => return,
    };

    // Mock blockchain objects
    let backend = mock_backend(&[]);
    let env = mock_env();
    let info = mock_info("creator", &[]);

    // Instantiate options
    let options = cosmwasm_vm::InstanceOptions {
        gas_limit: GAS_LIMIT,
    };

    // Create instance
    let mut instance = match cache.get_instance(&checksum, backend, options) {
        Ok(instance) => instance,
        Err(_) => return,
    };

    // Prepare environment
    let raw_env = match to_vec(&env) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    let raw_info = match to_vec(&info) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    // Call instantiate - we don't care about the result, just that it doesn't crash
    let _result = call_instantiate_raw(&mut instance, &raw_env, &raw_info, &input.instantiate_msg);
});
