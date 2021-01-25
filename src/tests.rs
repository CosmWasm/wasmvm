#![cfg(test)]

use serde::{Deserialize, Serialize};
use tempfile::TempDir;

use cosmwasm_std::{coins, HumanAddr};
use cosmwasm_vm::testing::{mock_backend, mock_env, mock_info, mock_instance_with_gas_limit};
use cosmwasm_vm::{
    call_handle_raw, call_init_raw, features_from_csv, to_vec, Cache, CacheOptions,
    InstanceOptions, Size,
};

static CONTRACT: &[u8] = include_bytes!("../api/testdata/hackatom.wasm");
const PRINT_DEBUG: bool = false;
const MEMORY_CACHE_SIZE: Size = Size::mebi(200);
const MEMORY_LIMIT: Size = Size::mebi(32);

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct InitMsg {
    pub verifier: HumanAddr,
    pub beneficiary: HumanAddr,
}

fn make_init_msg() -> (InitMsg, HumanAddr) {
    let verifier = HumanAddr::from("verifies");
    let beneficiary = HumanAddr::from("benefits");
    let creator = HumanAddr::from("creator");
    (
        InitMsg {
            verifier: verifier.clone(),
            beneficiary: beneficiary.clone(),
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
    let mut cache = unsafe { Cache::new(options) }.unwrap();

    let options = InstanceOptions {
        gas_limit: 2_000_000,
        print_debug: PRINT_DEBUG,
    };

    // store code
    let checksum = cache.save_wasm(CONTRACT).unwrap();

    // init
    let (init_msg, creator) = make_init_msg();
    let env = mock_env();
    let info = mock_info(creator, &coins(1000, "cosm"));
    let mut instance = cache.get_instance(&checksum, backend, options).unwrap();
    let raw_msg = to_vec(&init_msg).unwrap();
    let raw_env = to_vec(&env).unwrap();
    let raw_info = to_vec(&info).unwrap();
    let res = call_init_raw(&mut instance, &raw_env, &raw_info, &raw_msg);
    let gas_used = options.gas_limit - instance.get_gas_left();
    println!("Init used gas: {}", gas_used);
    res.unwrap();
    let backend = instance.recycle().unwrap();

    // handle
    let mut instance = cache.get_instance(&checksum, backend, options).unwrap();
    let raw_msg = r#"{"cpu_loop":{}}"#;
    let res = call_handle_raw(&mut instance, &raw_env, &raw_info, raw_msg.as_bytes());
    let gas_used = options.gas_limit - instance.get_gas_left();
    println!("Handle used gas: {}", gas_used);
    assert!(res.is_err());
    assert_eq!(instance.get_gas_left(), 0);
    instance.recycle();
}

#[test]
fn handle_cpu_loop_no_cache() {
    let gas_limit = 2_000_000u64;
    let mut instance = mock_instance_with_gas_limit(CONTRACT, gas_limit);

    // init
    let (init_msg, creator) = make_init_msg();
    let env = mock_env();
    let info = mock_info(creator, &coins(1000, "cosm"));
    let raw_msg = to_vec(&init_msg).unwrap();
    let raw_env = to_vec(&env).unwrap();
    let raw_info = to_vec(&info).unwrap();
    let res = call_init_raw(&mut instance, &raw_env, &raw_info, &raw_msg);
    let gas_used = gas_limit - instance.get_gas_left();
    println!("Init used gas: {}", gas_used);
    res.unwrap();

    // handle
    let raw_msg = r#"{"cpu_loop":{}}"#;
    let res = call_handle_raw(&mut instance, &raw_env, &raw_info, raw_msg.as_bytes());
    let gas_used = gas_limit - instance.get_gas_left();
    println!("Handle used gas: {}", gas_used);
    assert!(res.is_err());
    assert_eq!(instance.get_gas_left(), 0);
}
