//! A module containing calls into smart contracts via Cache and Instance.

use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::time::SystemTime;
use time::{format_description::well_known::Rfc3339, OffsetDateTime};

use cosmwasm_std::Checksum;
use cosmwasm_vm::{
    call_execute_raw, call_ibc2_packet_receive_raw, call_ibc2_packet_timeout_raw,
    call_ibc_channel_close_raw, call_ibc_channel_connect_raw, call_ibc_channel_open_raw,
    call_ibc_destination_callback_raw, call_ibc_packet_ack_raw, call_ibc_packet_receive_raw,
    call_ibc_packet_timeout_raw, call_ibc_source_callback_raw, call_instantiate_raw,
    call_migrate_raw, call_migrate_with_info_raw, call_query_raw, call_reply_raw, call_sudo_raw,
    Backend, Cache, Instance, InstanceOptions, VmResult,
};

use crate::api::GoApi;
use crate::args::{ARG1, ARG2, ARG3, CACHE_ARG, CHECKSUM_ARG, GAS_REPORT_ARG};
use crate::cache::{cache_t, to_cache};
use crate::db::Db;
use crate::error::{handle_c_error_binary, Error};
use crate::handle_vm_panic::handle_vm_panic;
use crate::memory::{ByteSliceView, UnmanagedVector};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;
use crate::GasReport;

// Constants for gas limit validation
const MIN_GAS_LIMIT: u64 = 10_000; // Lower bound for reasonable gas limit
const MAX_GAS_LIMIT: u64 = 1_000_000_000_000; // Upper bound (1 trillion, arbitrary high number)

// Constants for message validation
const MAX_MESSAGE_SIZE: usize = 1024 * 1024; // 1MB message size limit
const MAX_JSON_DEPTH: usize = 32; // Maximum nesting depth for JSON messages
const MAX_ENV_SIZE: usize = 100 * 1024; // 100KB environment size limit
const MAX_CHAIN_ID_LENGTH: usize = 128; // Reasonable max length for chain IDs
const MAX_ADDRESS_LENGTH: usize = 128; // Maximum reasonable length for addresses

/// Validates that the gas limit is within reasonable bounds
fn validate_gas_limit(gas_limit: u64) -> Result<(), Error> {
    if gas_limit < MIN_GAS_LIMIT {
        return Err(Error::invalid_gas_limit(format!(
            "Gas limit too low: {}. Minimum allowed: {}",
            gas_limit, MIN_GAS_LIMIT
        )));
    }

    if gas_limit > MAX_GAS_LIMIT {
        return Err(Error::invalid_gas_limit(format!(
            "Gas limit too high: {}. Maximum allowed: {}",
            gas_limit, MAX_GAS_LIMIT
        )));
    }

    Ok(())
}

/// Validates contract environment data for safety
fn validate_environment(env_data: &[u8]) -> Result<(), Error> {
    // Check env size
    if env_data.len() > MAX_ENV_SIZE {
        return Err(Error::vm_err(format!(
            "Environment data size exceeds limit: {} > {}",
            env_data.len(),
            MAX_ENV_SIZE
        )));
    }

    // Parse and validate the environment structure
    let env: serde_json::Value = match serde_json::from_slice(env_data) {
        Ok(env) => env,
        Err(e) => {
            return Err(Error::vm_err(format!("Invalid environment JSON: {}", e)));
        }
    };

    // Must be an object
    if !env.is_object() {
        return Err(Error::vm_err("Environment must be a JSON object"));
    }

    // Validate required fields and structure
    let block = env
        .get("block")
        .ok_or_else(|| Error::vm_err("Missing 'block' field in environment"))?;
    if !block.is_object() {
        return Err(Error::vm_err("'block' must be a JSON object"));
    }

    // Validate block height is present and is an unsigned integer
    let height = block
        .get("height")
        .ok_or_else(|| Error::vm_err("Missing 'height' field in block"))?;
    if !height.is_u64() {
        return Err(Error::vm_err("Block height must be a positive integer"));
    }

    // Validate block time is present and is either an unsigned integer or a string-encoded unsigned integer
    let time = block
        .get("time")
        .ok_or_else(|| Error::vm_err("Missing 'time' field in block"))?;

    // Check if time is a direct number or a string-encoded number
    if !time.is_u64() && !time.is_string() {
        return Err(Error::vm_err(
            "Block time must be a positive integer or a string-encoded positive integer",
        ));
    }

    // If it's a string, validate it contains a valid positive integer
    if time.is_string() {
        if let Some(time_str) = time.as_str() {
            if time_str.parse::<u64>().is_err() {
                return Err(Error::vm_err(
                    "Block time string must contain a valid positive integer",
                ));
            }
        }
    }

    // Validate chain_id is present and is a string of reasonable length
    let chain_id = block
        .get("chain_id")
        .ok_or_else(|| Error::vm_err("Missing 'chain_id' field in block"))?;
    if !chain_id.is_string() {
        return Err(Error::vm_err("Chain ID must be a string"));
    }
    if let Some(chain_id_str) = chain_id.as_str() {
        if chain_id_str.len() > MAX_CHAIN_ID_LENGTH {
            return Err(Error::vm_err(format!(
                "Chain ID exceeds maximum length: {} > {}",
                chain_id_str.len(),
                MAX_CHAIN_ID_LENGTH
            )));
        }
    }

    // Validate contract field is present and is an object
    let contract = env
        .get("contract")
        .ok_or_else(|| Error::vm_err("Missing 'contract' field in environment"))?;
    if !contract.is_object() {
        return Err(Error::vm_err("'contract' must be a JSON object"));
    }

    // Validate contract address is present and is a string of reasonable length
    let address = contract
        .get("address")
        .ok_or_else(|| Error::vm_err("Missing 'address' field in contract"))?;
    if !address.is_string() {
        return Err(Error::vm_err("Contract address must be a string"));
    }
    if let Some(addr_str) = address.as_str() {
        if addr_str.len() > MAX_ADDRESS_LENGTH {
            return Err(Error::vm_err(format!(
                "Contract address exceeds maximum length: {} > {}",
                addr_str.len(),
                MAX_ADDRESS_LENGTH
            )));
        }
        // Basic character validation for addresses
        if !addr_str.chars().all(|c| {
            c.is_alphanumeric()
                || c == '1'
                || c == 'c'
                || c == 'o'
                || c == 's'
                || c == 'm'
                || c == '_'
                || c == '-'
        }) {
            return Err(Error::vm_err(
                "Contract address contains invalid characters",
            ));
        }
    }

    // Transaction is optional but must be an object if present
    if let Some(tx) = env.get("transaction") {
        if !tx.is_null() && !tx.is_object() {
            return Err(Error::vm_err(
                "'transaction' must be a JSON object if present",
            ));
        }
        // If transaction is present, validate 'index' is a non-negative integer
        if tx.is_object() {
            let index = tx
                .get("index")
                .ok_or_else(|| Error::vm_err("Missing 'index' field in transaction"))?;
            if !index.is_u64() {
                return Err(Error::vm_err(
                    "Transaction index must be a non-negative integer",
                ));
            }
        }
    }

    Ok(())
}

/// Validates information data structure (MessageInfo)
fn validate_message_info(info_data: &[u8]) -> Result<(), Error> {
    // Check info size
    if info_data.len() > MAX_ENV_SIZE {
        return Err(Error::vm_err(format!(
            "Message info data size exceeds limit: {} > {}",
            info_data.len(),
            MAX_ENV_SIZE
        )));
    }

    // Parse and validate the info structure
    let info: serde_json::Value = match serde_json::from_slice(info_data) {
        Ok(info) => info,
        Err(e) => {
            return Err(Error::vm_err(format!("Invalid message info JSON: {}", e)));
        }
    };

    // Must be an object
    if !info.is_object() {
        return Err(Error::vm_err("Message info must be a JSON object"));
    }

    // Validate 'sender' field is present and is a string of reasonable length
    let sender = info
        .get("sender")
        .ok_or_else(|| Error::vm_err("Missing 'sender' field in message info"))?;
    if !sender.is_string() {
        return Err(Error::vm_err("Sender must be a string"));
    }
    if let Some(sender_str) = sender.as_str() {
        if sender_str.len() > MAX_ADDRESS_LENGTH {
            return Err(Error::vm_err(format!(
                "Sender address exceeds maximum length: {} > {}",
                sender_str.len(),
                MAX_ADDRESS_LENGTH
            )));
        }
        // Basic character validation for addresses
        if !sender_str.chars().all(|c| {
            c.is_alphanumeric()
                || c == '1'
                || c == 'c'
                || c == 'o'
                || c == 's'
                || c == 'm'
                || c == '_'
                || c == '-'
        }) {
            return Err(Error::vm_err("Sender address contains invalid characters"));
        }
    }

    // Validate 'funds' field is present and is an array
    let funds = info
        .get("funds")
        .ok_or_else(|| Error::vm_err("Missing 'funds' field in message info"))?;
    if !funds.is_array() {
        return Err(Error::vm_err("Funds must be an array"));
    }

    // Validate each coin in the funds
    if let Some(funds_array) = funds.as_array() {
        for (i, coin) in funds_array.iter().enumerate() {
            if !coin.is_object() {
                return Err(Error::vm_err(format!(
                    "Coin at index {} must be an object",
                    i
                )));
            }

            // Validate 'denom' field
            let denom = coin.get("denom").ok_or_else(|| {
                Error::vm_err(format!("Missing 'denom' field in coin at index {}", i))
            })?;
            if !denom.is_string() {
                return Err(Error::vm_err(format!(
                    "Denom at index {} must be a string",
                    i
                )));
            }
            if let Some(denom_str) = denom.as_str() {
                if denom_str.is_empty() {
                    return Err(Error::vm_err(format!(
                        "Denom at index {} cannot be empty",
                        i
                    )));
                }
                if denom_str.len() > 128 {
                    return Err(Error::vm_err(format!(
                        "Denom at index {} exceeds maximum length: {} > 128",
                        i,
                        denom_str.len()
                    )));
                }
                // Basic character validation for denoms
                if !denom_str
                    .chars()
                    .all(|c| c.is_alphanumeric() || c == '/' || c == ':' || c == '_' || c == '-')
                {
                    return Err(Error::vm_err(format!(
                        "Denom at index {} contains invalid characters",
                        i
                    )));
                }
            }

            // Validate 'amount' field
            let amount = coin.get("amount").ok_or_else(|| {
                Error::vm_err(format!("Missing 'amount' field in coin at index {}", i))
            })?;
            if !amount.is_string() {
                return Err(Error::vm_err(format!(
                    "Amount at index {} must be a string",
                    i
                )));
            }
            if let Some(amount_str) = amount.as_str() {
                if amount_str.is_empty() {
                    return Err(Error::vm_err(format!(
                        "Amount at index {} cannot be empty",
                        i
                    )));
                }
                if amount_str.len() > 50 {
                    return Err(Error::vm_err(format!(
                        "Amount at index {} exceeds maximum length: {} > 50",
                        i,
                        amount_str.len()
                    )));
                }
                // Verify amount is a valid numeric string
                if !amount_str.chars().all(|c| c.is_ascii_digit()) {
                    return Err(Error::vm_err(format!(
                        "Amount at index {} contains non-numeric characters",
                        i
                    )));
                }
            }
        }
    }

    Ok(())
}

/// Validates a contract message to ensure it's safe to process
/// Checks include size limits and basic JSON structure validation
fn validate_message(message: &[u8]) -> Result<(), Error> {
    // Check message size
    if message.len() > MAX_MESSAGE_SIZE {
        return Err(Error::vm_err(format!(
            "Message size exceeds limit: {} > {}",
            message.len(),
            MAX_MESSAGE_SIZE
        )));
    }

    // Verify it's valid JSON (if it looks like JSON)
    if !message.is_empty() && (message[0] == b'{' || message[0] == b'[') {
        // It looks like JSON, so validate it
        match serde_json::from_slice::<serde_json::Value>(message) {
            Ok(value) => {
                // Check JSON nesting depth
                if json_depth(&value) > MAX_JSON_DEPTH {
                    return Err(Error::vm_err(format!(
                        "JSON exceeds maximum allowed depth of {}",
                        MAX_JSON_DEPTH
                    )));
                }
            }
            Err(e) => {
                return Err(Error::vm_err(format!("Invalid JSON: {}", e)));
            }
        }
    }

    Ok(())
}

/// Helper function to measure the depth of a JSON structure
fn json_depth(value: &serde_json::Value) -> usize {
    match value {
        serde_json::Value::Object(map) => {
            let mut max_depth = 1;
            for (_, v) in map {
                let depth = 1 + json_depth(v);
                if depth > max_depth {
                    max_depth = depth;
                }
            }
            max_depth
        }
        serde_json::Value::Array(array) => {
            let mut max_depth = 1;
            for v in array {
                let depth = 1 + json_depth(v);
                if depth > max_depth {
                    max_depth = depth;
                }
            }
            max_depth
        }
        _ => 1, // Simple values have depth 1
    }
}

fn into_backend(db: Db, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}

#[no_mangle]
pub extern "C" fn instantiate(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    info: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_3_args(
        call_instantiate_raw,
        cache,
        checksum,
        env,
        info,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn execute(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    info: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_3_args(
        call_execute_raw,
        cache,
        checksum,
        env,
        info,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn migrate(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_migrate_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn migrate_with_info(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    migrate_info: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_3_args(
        call_migrate_with_info_raw,
        cache,
        checksum,
        env,
        msg,
        migrate_info,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn sudo(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_sudo_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn reply(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_reply_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn query(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_query_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_channel_open(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_channel_open_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_channel_connect(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_channel_connect_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_channel_close(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_channel_close_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_packet_receive(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_packet_receive_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_packet_ack(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_packet_ack_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_packet_timeout(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_packet_timeout_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_source_callback(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_source_callback_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc_destination_callback(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc_destination_callback_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc2_packet_receive(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc2_packet_receive_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

#[no_mangle]
pub extern "C" fn ibc2_packet_timeout(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    env: ByteSliceView,
    msg: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    call_2_args(
        call_ibc2_packet_timeout_raw,
        cache,
        checksum,
        env,
        msg,
        db,
        api,
        querier,
        gas_limit,
        print_debug,
        gas_report,
        error_msg,
    )
}

type VmFn2Args = fn(
    instance: &mut Instance<GoApi, GoStorage, GoQuerier>,
    arg1: &[u8],
    arg2: &[u8],
) -> VmResult<Vec<u8>>;

// this wraps all error handling and ffi for the 6 ibc entry points and query.
// (all of which take env and one "msg" argument).
// the only difference is which low-level function they dispatch to.
fn call_2_args(
    vm_fn: VmFn2Args,
    cache: *mut cache_t,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_call_2_args(
                vm_fn,
                c,
                checksum,
                arg1,
                arg2,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                gas_report,
            )
        }))
        .unwrap_or_else(|err| {
            handle_vm_panic("do_call_2_args", err);
            Err(Error::panic())
        }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(data))
}

// this is internal processing, same for all the 6 ibc entry points
fn do_call_2_args(
    vm_fn: VmFn2Args,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
) -> Result<Vec<u8>, Error> {
    let gas_report = gas_report.ok_or_else(|| Error::empty_arg(GAS_REPORT_ARG))?;
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let arg1 = arg1.read().ok_or_else(|| Error::unset_arg(ARG1))?;
    let arg2 = arg2.read().ok_or_else(|| Error::unset_arg(ARG2))?;

    // Validate gas limit
    validate_gas_limit(gas_limit)?;

    // Validate environment data (arg1 is usually env in 2-args functions)
    validate_environment(arg1)?;

    // Validate message payload
    validate_message(arg2)?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions { gas_limit };
    let mut instance: Instance<GoApi, GoStorage, GoQuerier> =
        cache.get_instance(&checksum, backend, options)?;

    // If print_debug = false, use default debug handler from cosmwasm-vm, which discards messages
    if print_debug {
        instance.set_debug_handler(|msg, info| {
            let t = now_rfc3339();
            let gas = info.gas_remaining;
            eprintln!("[{t}]: {msg} (gas remaining: {gas})");
        });
    }

    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = vm_fn(&mut instance, arg1, arg2);
    *gas_report = instance.create_gas_report().into();
    Ok(res?)
}

type VmFn3Args = fn(
    instance: &mut Instance<GoApi, GoStorage, GoQuerier>,
    arg1: &[u8],
    arg2: &[u8],
    arg3: &[u8],
) -> VmResult<Vec<u8>>;

// This wraps all error handling and ffi for instantiate, execute and migrate
// (and anything else that takes env, info and msg arguments).
// The only difference is which low-level function they dispatch to.
fn call_3_args(
    vm_fn: VmFn3Args,
    cache: *mut cache_t,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    arg3: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_call_3_args(
                vm_fn,
                c,
                checksum,
                arg1,
                arg2,
                arg3,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                gas_report,
            )
        }))
        .unwrap_or_else(|err| {
            handle_vm_panic("do_call_3_args", err);
            Err(Error::panic())
        }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(data))
}

fn do_call_3_args(
    vm_fn: VmFn3Args,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
    arg1: ByteSliceView,
    arg2: ByteSliceView,
    arg3: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_report: Option<&mut GasReport>,
) -> Result<Vec<u8>, Error> {
    let gas_report = gas_report.ok_or_else(|| Error::empty_arg(GAS_REPORT_ARG))?;
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let arg1 = arg1.read().ok_or_else(|| Error::unset_arg(ARG1))?;
    let arg2 = arg2.read().ok_or_else(|| Error::unset_arg(ARG2))?;
    let arg3 = arg3.read().ok_or_else(|| Error::unset_arg(ARG3))?;

    // Validate gas limit
    validate_gas_limit(gas_limit)?;

    // Validate environment data (arg1 is usually env in 3-args functions)
    validate_environment(arg1)?;

    // Validate message info (arg2 is usually info in 3-args functions)
    validate_message_info(arg2)?;

    // Validate message payload (usually arg3 is the message in 3-arg functions)
    validate_message(arg3)?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions { gas_limit };
    let mut instance = cache.get_instance(&checksum, backend, options)?;

    // If print_debug = false, use default debug handler from cosmwasm-vm, which discards messages
    if print_debug {
        instance.set_debug_handler(|msg, info| {
            let t = now_rfc3339();
            let gas = info.gas_remaining;
            eprintln!("[{t}]: {msg} (gas remaining: {gas})");
        });
    }

    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = vm_fn(&mut instance, arg1, arg2, arg3);
    *gas_report = instance.create_gas_report().into();
    Ok(res?)
}

fn now_rfc3339() -> String {
    let dt = OffsetDateTime::from(SystemTime::now());
    dt.format(&Rfc3339).unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_validate_gas_limit() {
        // Valid gas limits
        assert!(validate_gas_limit(10_000).is_ok());
        assert!(validate_gas_limit(100_000).is_ok());
        assert!(validate_gas_limit(1_000_000).is_ok());
        assert!(validate_gas_limit(1_000_000_000).is_ok());

        // Too low
        let err = validate_gas_limit(9_999).unwrap_err();
        match err {
            Error::InvalidGasLimit { .. } => {}
            _ => panic!("Expected InvalidGasLimit error"),
        }

        // Too high
        let err = validate_gas_limit(1_000_000_000_001).unwrap_err();
        match err {
            Error::InvalidGasLimit { .. } => {}
            _ => panic!("Expected InvalidGasLimit error"),
        }
    }
}
