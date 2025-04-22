#![no_main]

use std::fs;
use std::path::PathBuf;

use arbitrary::Arbitrary;
use cosmwasm_vm::{
    call_instantiate_raw, call_migrate_raw, capabilities_from_csv,
    testing::{mock_backend, mock_env, mock_info, MockApi, MockQuerier, MockStorage},
    to_vec, Cache, CacheOptions, Size,
};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);
const GAS_LIMIT: u64 = 200_000_000_000; // ~0.2ms

#[derive(Arbitrary, Debug)]
struct MigrateFuzzInput {
    // The migrate message
    #[arbitrary(with = |u: &mut arbitrary::Unstructured| u.bytes(100))]
    migrate_msg: Vec<u8>,
}

fuzz_target!(|input: MigrateFuzzInput| {
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

    // Load the first contract for instantiation
    let wasm_path1 = PathBuf::from("../../testdata/hackatom.wasm");
    let wasm1 = match fs::read(&wasm_path1) {
        Ok(wasm) => wasm,
        Err(_) => return,
    };

    // Store the first contract
    let checksum1 = match cache.store_code(&wasm1, true, true) {
        Ok(checksum) => checksum,
        Err(_) => return,
    };

    // Load the second contract to migrate to (use cyberpunk or the same contract if needed)
    let wasm_path2 = PathBuf::from("../../testdata/cyberpunk.wasm");
    let wasm2 = match fs::read(&wasm_path2) {
        Ok(wasm) => wasm,
        Err(_) => wasm1.clone(), // Use the same contract if we can't load the second one
    };

    // Store the second contract
    let _checksum2 = match cache.store_code(&wasm2, true, true) {
        Ok(checksum) => checksum,
        Err(_) => return,
    };

    // Mock blockchain objects
    let backend = mock_backend(&[]);
    let env = mock_env();
    let creator_info = mock_info("creator", &[]);

    // Instantiate options
    let options = cosmwasm_vm::InstanceOptions {
        gas_limit: GAS_LIMIT,
    };

    // Create instance of the first contract
    let mut instance = match cache.get_instance(&checksum1, backend, options) {
        Ok(instance) => instance,
        Err(_) => return,
    };

    // Prepare environment
    let raw_env = match to_vec(&env) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    let raw_info = match to_vec(&creator_info) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    // First instantiate the contract with a valid message
    let instantiate_msg = br#"{"verifier": "fred", "beneficiary": "bob"}"#;
    let instantiate_result =
        call_instantiate_raw(&mut instance, &raw_env, &raw_info, instantiate_msg);

    if instantiate_result.is_err() {
        return;
    }

    // Prepare for migration
    // Admin info (typically the creator or a designated admin)
    let admin_info = mock_info("creator", &[]);
    let _raw_admin_info = match to_vec(&admin_info) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    // Now try to migrate the contract to the new code with the fuzzed message
    let _migrate_result = call_migrate_raw(&mut instance, &raw_env, &input.migrate_msg);

    // The result is ignored as we just want to test if migration crashes the VM
});
