#![no_main]

use std::fs;
use std::path::PathBuf;

use arbitrary::Arbitrary;
use cosmwasm_vm::{
    call_ibc_channel_close_raw, call_ibc_channel_connect_raw, call_ibc_channel_open_raw,
    call_ibc_packet_receive_raw, call_instantiate_raw, capabilities_from_csv,
    testing::{mock_backend, mock_env, mock_info},
    to_vec, Cache, CacheOptions, Size,
};
use libfuzzer_sys::fuzz_target;

// Define constants for the fuzzing
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);
const GAS_LIMIT: u64 = 200_000_000_000; // ~0.2ms

// Enum to determine which IBC operation to test
#[derive(Arbitrary, Debug, Clone, Copy)]
enum IbcOperationType {
    ChannelOpen,
    ChannelConnect,
    ChannelClose,
    PacketReceive,
}

#[derive(Arbitrary, Debug)]
struct IbcFuzzInput {
    // The IBC message
    #[arbitrary(with = |u: &mut arbitrary::Unstructured| u.bytes(100))]
    ibc_msg: Vec<u8>,

    // Type of IBC operation to perform
    operation_type: IbcOperationType,
}

fuzz_target!(|input: IbcFuzzInput| {
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

    // Load an IBC-enabled contract
    // For this test, we'll use the ibc_reflect contract which supports IBC
    let wasm_path = PathBuf::from("../../testdata/ibc_reflect.wasm");
    let wasm = match fs::read(&wasm_path) {
        Ok(wasm) => wasm,
        Err(_) => return, // Skip if we can't find the IBC contract
    };

    // Store the contract
    let checksum = match cache.store_code(&wasm, true, true) {
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

    // Instantiate the contract with a valid message for an IBC contract
    let instantiate_msg = br#"{"reflect_code_id":234}"#; // simplified init message
    let instantiate_result =
        call_instantiate_raw(&mut instance, &raw_env, &raw_info, instantiate_msg);

    if instantiate_result.is_err() {
        return;
    }

    // Now try different IBC operations based on the fuzz input
    match input.operation_type {
        IbcOperationType::ChannelOpen => {
            let _result = call_ibc_channel_open_raw(&mut instance, &raw_env, &input.ibc_msg);
        }
        IbcOperationType::ChannelConnect => {
            let _result = call_ibc_channel_connect_raw(&mut instance, &raw_env, &input.ibc_msg);
        }
        IbcOperationType::ChannelClose => {
            let _result = call_ibc_channel_close_raw(&mut instance, &raw_env, &input.ibc_msg);
        }
        IbcOperationType::PacketReceive => {
            let _result = call_ibc_packet_receive_raw(&mut instance, &raw_env, &input.ibc_msg);
        }
    }

    // The results are ignored as we just want to test if the operations crash the VM
});
