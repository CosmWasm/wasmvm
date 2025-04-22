#![no_main]

use std::fs;
use std::path::PathBuf;

use arbitrary::Arbitrary;
use cosmwasm_vm::{
    call_execute_raw, call_instantiate_raw, capabilities_from_csv,
    testing::{mock_backend, mock_env, mock_info},
    to_vec, Cache, CacheOptions, Size,
};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);
const GAS_LIMIT: u64 = 500_000_000_000; // ~0.5ms

#[derive(Arbitrary, Debug)]
struct MultiContractFuzzInput {
    // Execute message to send to the contracts
    #[arbitrary(with = |u: &mut arbitrary::Unstructured| u.bytes(100))]
    execute_msg: Vec<u8>,

    // Which contract to execute first (0 or 1)
    execute_first_contract: bool,
}

fuzz_target!(|input: MultiContractFuzzInput| {
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

    // Load two different contracts
    let contract_paths = [
        PathBuf::from("../../testdata/hackatom.wasm"),
        PathBuf::from("../../testdata/cyberpunk.wasm"),
    ];

    let mut checksums = Vec::new();

    // Store both contracts
    for path in &contract_paths {
        let wasm = match fs::read(path) {
            Ok(wasm) => wasm,
            Err(_) => continue,
        };

        match cache.store_code(&wasm, true, true) {
            Ok(checksum) => checksums.push(checksum),
            Err(_) => continue,
        };
    }

    // Need at least two contracts
    if checksums.len() < 2 {
        return;
    }

    // Mock blockchain objects
    let backend = mock_backend(&[]);
    let env = mock_env();
    let info = mock_info("creator", &[]);

    // Instantiate options
    let options = cosmwasm_vm::InstanceOptions {
        gas_limit: GAS_LIMIT,
    };

    // Create instances of both contracts
    let mut instances = Vec::new();
    for checksum in &checksums[0..2] {
        match cache.get_instance(checksum, backend.clone(), options) {
            Ok(instance) => instances.push(instance),
            Err(_) => return,
        }
    }

    if instances.len() < 2 {
        return;
    }

    // Prepare environment
    let raw_env = match to_vec(&env) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    let raw_info = match to_vec(&info) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    // Instantiate the first contract with a valid message
    let instantiate_msg1 = br#"{"verifier": "fred", "beneficiary": "bob"}"#;
    let _instantiate_result1 =
        call_instantiate_raw(&mut instances[0], &raw_env, &raw_info, instantiate_msg1);

    // Instantiate the second contract with a valid message
    let instantiate_msg2 = br#"{}"#;
    let _instantiate_result2 =
        call_instantiate_raw(&mut instances[1], &raw_env, &raw_info, instantiate_msg2);

    // Determine which contract to execute first
    let first_idx = if input.execute_first_contract { 0 } else { 1 };
    let second_idx = if input.execute_first_contract { 1 } else { 0 };

    // Execute the first contract
    let _execute_result1 = call_execute_raw(
        &mut instances[first_idx],
        &raw_env,
        &raw_info,
        &input.execute_msg,
    );

    // Execute the second contract
    let _execute_result2 = call_execute_raw(
        &mut instances[second_idx],
        &raw_env,
        &raw_info,
        &input.execute_msg,
    );

    // The temp directory will be cleaned up automatically when it goes out of scope
});
