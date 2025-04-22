#![no_main]

use std::fs;
use std::path::PathBuf;

use arbitrary::Arbitrary;
use cosmwasm_vm::{
    call_execute_raw, call_instantiate_raw, call_query_raw, capabilities_from_csv,
    testing::{mock_backend, mock_env, mock_info, MockApi, MockQuerier, MockStorage},
    to_vec, Cache, CacheOptions, Size,
};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);

#[derive(Arbitrary, Debug)]
struct GasMeteringFuzzInput {
    // Gas limit to use for the operations
    #[arbitrary(with = |u: &mut arbitrary::Unstructured| {
        // Generate a gas limit between 1 and 500_000_000_000
        u.int_in_range(1..=500_000_000_000).unwrap()
    })]
    gas_limit: u64,

    // The operation message (reused for instantiate, execute, query)
    #[arbitrary(with = |u: &mut arbitrary::Unstructured| u.bytes(100))]
    msg: Vec<u8>,

    // Should we execute after instantiate
    execute_after_instantiate: bool,

    // Should we query after instantiate
    query_after_instantiate: bool,
}

fuzz_target!(|input: GasMeteringFuzzInput| {
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

    // Load the test WASM file - using cyberpunk which has cpu_loop for testing gas limits
    let wasm_path = PathBuf::from("../../testdata/cyberpunk.wasm");
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
    let creator_info = mock_info("creator", &[]);

    // Instantiate options with fuzzed gas limit
    let options = cosmwasm_vm::InstanceOptions {
        gas_limit: input.gas_limit,
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

    let raw_info = match to_vec(&creator_info) {
        Ok(raw) => raw,
        Err(_) => return,
    };

    // Try to instantiate with the fuzzed message, might fail with out-of-gas
    let instantiate_result = call_instantiate_raw(&mut instance, &raw_env, &raw_info, &input.msg);

    // Get the gas usage after instantiation
    let _gas_after_instantiate = instance.get_gas_left();

    // If instantiate succeeded and we want to execute
    if instantiate_result.is_ok() && input.execute_after_instantiate {
        // Try to execute, might fail with out-of-gas
        let _execute_result = call_execute_raw(&mut instance, &raw_env, &raw_info, &input.msg);

        // Get gas usage after execution
        let _gas_after_execute = instance.get_gas_left();
    }

    // If instantiate succeeded and we want to query
    if instantiate_result.is_ok() && input.query_after_instantiate {
        // Try to query, might fail with out-of-gas
        let _query_result = call_query_raw(&mut instance, &raw_env, &input.msg);

        // Get gas usage after query
        let _gas_after_query = instance.get_gas_left();
    }

    // The temp directory will be cleaned up automatically when it goes out of scope
});
