#![cfg(test)]

use serde::{Deserialize, Serialize};
use tempfile::TempDir;

use cosmwasm_std::coins;
use cosmwasm_vm::testing::{mock_backend, mock_env, mock_info, mock_instance_with_gas_limit};
use cosmwasm_vm::{
    call_execute_raw, call_instantiate_raw, features_from_csv, to_vec, Cache, CacheOptions,
    InstanceOptions, Size,
};

static CONTRACT: &[u8] = include_bytes!("../../api/testdata/hackatom.wasm");
const PRINT_DEBUG: bool = false;
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);
const GAS_LIMIT: u64 = 200_000_000_000; // ~0.2ms

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct InstantiateMsg {
    pub verifier: String,
    pub beneficiary: String,
}

fn make_instantiate_msg() -> (InstantiateMsg, String) {
    let verifier = String::from("verifies");
    let beneficiary = String::from("benefits");
    let creator = String::from("creator");
    (
        InstantiateMsg {
            verifier,
            beneficiary,
        },
        creator,
    )
}

#[test]
fn handle_cpu_loop_with_cache() {
    let backend = mock_backend(&[]);
    let options = CacheOptions {
        base_dir: TempDir::new().unwrap().path().to_path_buf(),
        supported_features: features_from_csv("staking"),
        memory_cache_size: MEMORY_CACHE_SIZE,
        instance_memory_limit: MEMORY_LIMIT,
    };
    let cache = unsafe { Cache::new(options) }.unwrap();

    let options = InstanceOptions {
        gas_limit: GAS_LIMIT,
        print_debug: PRINT_DEBUG,
    };

    // store code
    let checksum = cache.save_wasm(CONTRACT).unwrap();

    // instantiate
    let (instantiate_msg, creator) = make_instantiate_msg();
    let env = mock_env();
    let info = mock_info(&creator, &coins(1000, "cosm"));
    let mut instance = cache.get_instance(&checksum, backend, options).unwrap();
    let raw_msg = to_vec(&instantiate_msg).unwrap();
    let raw_env = to_vec(&env).unwrap();
    let raw_info = to_vec(&info).unwrap();
    let res = call_instantiate_raw(&mut instance, &raw_env, &raw_info, &raw_msg);
    let gas_left = instance.get_gas_left();
    let gas_used = options.gas_limit - gas_left;
    println!("Init gas left: {}, used: {}", gas_left, gas_used);
    assert!(res.is_ok());
    let backend = instance.recycle().unwrap();

    // execute
    let mut instance = cache.get_instance(&checksum, backend, options).unwrap();
    let raw_msg = r#"{"cpu_loop":{}}"#;
    let res = call_execute_raw(&mut instance, &raw_env, &raw_info, raw_msg.as_bytes());
    let gas_left = instance.get_gas_left();
    let gas_used = options.gas_limit - gas_left;
    println!("Handle gas left: {}, used: {}", gas_left, gas_used);
    assert!(res.is_err());
    assert_eq!(gas_left, 0);
    let _ = instance.recycle();
}

#[test]
fn handle_cpu_loop_no_cache() {
    let gas_limit = GAS_LIMIT;
    let mut instance = mock_instance_with_gas_limit(CONTRACT, gas_limit);

    // instantiate
    let (instantiate_msg, creator) = make_instantiate_msg();
    let env = mock_env();
    let info = mock_info(&creator, &coins(1000, "cosm"));
    let raw_msg = to_vec(&instantiate_msg).unwrap();
    let raw_env = to_vec(&env).unwrap();
    let raw_info = to_vec(&info).unwrap();
    let res = call_instantiate_raw(&mut instance, &raw_env, &raw_info, &raw_msg);
    let gas_left = instance.get_gas_left();
    let gas_used = gas_limit - gas_left;
    println!("Init gas left: {}, used: {}", gas_left, gas_used);
    assert!(res.is_ok());

    // execute
    let raw_msg = r#"{"cpu_loop":{}}"#;
    let res = call_execute_raw(&mut instance, &raw_env, &raw_info, raw_msg.as_bytes());
    let gas_left = instance.get_gas_left();
    let gas_used = gas_limit - gas_left;
    println!("Handle gas left: {}, used: {}", gas_left, gas_used);
    assert!(res.is_err());
    assert_eq!(gas_left, 0);
}
