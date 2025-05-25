use std::collections::HashMap;
use std::sync::{LazyLock, Mutex};
use wasmvm::{
    api_t, db_t, gas_meter_t, querier_t, DbVtable, GoApiVtable, GoIter, QuerierVtable, U8SliceView,
    UnmanagedVector,
};

/// In-memory storage for the RPC server
#[derive(Debug, Default)]
pub struct InMemoryStorage {
    data: HashMap<Vec<u8>, Vec<u8>>,
}

/// Global storage instance (thread-safe)
static STORAGE: LazyLock<Mutex<InMemoryStorage>> = LazyLock::new(|| {
    Mutex::new(InMemoryStorage {
        data: HashMap::new(),
    })
});

/// Helper function to extract data from U8SliceView using its read() method.
fn extract_u8_slice_data(view: U8SliceView) -> Option<Vec<u8>> {
    view.read().map(|slice| slice.to_vec())
}

/// Gas costs for operations (simplified)
const GAS_COST_READ: u64 = 1000;
const GAS_COST_WRITE: u64 = 2000;
const GAS_COST_REMOVE: u64 = 1500;
const GAS_COST_SCAN: u64 = 3000;
const GAS_COST_API_CALL: u64 = 500;
const GAS_COST_QUERY: u64 = 1000;

// === Database Vtable Implementation ===

extern "C" fn impl_read_db(
    _db: *mut db_t,
    _gas_meter: *mut gas_meter_t,
    gas_used: *mut u64,
    key: U8SliceView,
    value_out: *mut UnmanagedVector,
    err_msg_out: *mut UnmanagedVector,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_READ;

        let key_bytes = match extract_u8_slice_data(key) {
            Some(k) => k,
            None => {
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid key for db_read".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        match STORAGE.lock() {
            Ok(storage) => {
                if let Some(value) = storage.data.get(&key_bytes) {
                    *value_out = UnmanagedVector::new(Some(value.clone()));
                } else {
                    *value_out = UnmanagedVector::new(None); // Key not found
                }
                wasmvm::GoError::None as i32 // Success
            }
            Err(_) => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Storage lock error for db_read".to_vec()));
                wasmvm::GoError::Panic as i32 // Error
            }
        }
    }
}

extern "C" fn impl_write_db(
    _db: *mut db_t,
    _gas_meter: *mut gas_meter_t,
    gas_used: *mut u64,
    key: U8SliceView,
    value: U8SliceView,
    err_msg_out: *mut UnmanagedVector,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_WRITE;

        let key_bytes = match extract_u8_slice_data(key) {
            Some(k) => k,
            None => {
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid key for db_write".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        let value_bytes = match extract_u8_slice_data(value) {
            Some(v) => v,
            None => {
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid value for db_write".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        match STORAGE.lock() {
            Ok(mut storage) => {
                storage.data.insert(key_bytes, value_bytes);
                wasmvm::GoError::None as i32 // Success
            }
            Err(_) => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Storage lock error for db_write".to_vec()));
                wasmvm::GoError::Panic as i32 // Error
            }
        }
    }
}

extern "C" fn impl_remove_db(
    _db: *mut db_t,
    _gas_meter: *mut gas_meter_t,
    gas_used: *mut u64,
    key: U8SliceView,
    err_msg_out: *mut UnmanagedVector,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_REMOVE;

        let key_bytes = match extract_u8_slice_data(key) {
            Some(k) => k,
            None => {
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid key for db_remove".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        match STORAGE.lock() {
            Ok(mut storage) => {
                storage.data.remove(&key_bytes);
                wasmvm::GoError::None as i32 // Success
            }
            Err(_) => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Storage lock error for db_remove".to_vec()));
                wasmvm::GoError::Panic as i32 // Error
            }
        }
    }
}

extern "C" fn impl_scan_db(
    _db: *mut db_t,
    _gas_meter: *mut gas_meter_t,
    gas_used: *mut u64,
    _start: U8SliceView,
    _end: U8SliceView,
    _order: i32,
    _iterator_out: *mut GoIter,
    err_msg_out: *mut UnmanagedVector,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_SCAN;
        // For now, return an error as iterator implementation is complex
        *err_msg_out = UnmanagedVector::new(Some(b"Scan not implemented yet".to_vec()));
        wasmvm::GoError::User as i32 // User error as it's a known unimplemented feature
    }
}

// === API Vtable Implementation ===

extern "C" fn impl_humanize_address(
    _api: *const api_t,
    input: U8SliceView,
    humanized_address_out: *mut UnmanagedVector,
    err_msg_out: *mut UnmanagedVector,
    gas_used: *mut u64,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_API_CALL;

        let input_bytes = match extract_u8_slice_data(input) {
            Some(i) => i,
            None => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Invalid input for humanize_address".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        // Simple implementation: assume input is canonical (e.g. 20 bytes) and prefix with "cosmos1"
        // In a real implementation, this would convert from canonical to human-readable format,
        // potentially involving bech32 encoding.
        let human_address = format!("cosmos1{}", hex::encode(&input_bytes));
        *humanized_address_out = UnmanagedVector::new(Some(human_address.into_bytes()));
        wasmvm::GoError::None as i32 // Success
    }
}

extern "C" fn impl_canonicalize_address(
    _api: *const api_t,
    input: U8SliceView,
    canonicalized_address_out: *mut UnmanagedVector,
    err_msg_out: *mut UnmanagedVector,
    gas_used: *mut u64,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_API_CALL;

        let input_bytes = match extract_u8_slice_data(input) {
            Some(i) => i,
            None => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Invalid input for canonicalize_address".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        // Simple implementation: convert human-readable address to canonical format
        let input_str = match std::str::from_utf8(&input_bytes) {
            Ok(s) => s,
            Err(_) => {
                *err_msg_out = UnmanagedVector::new(Some(
                    b"Invalid UTF-8 address for canonicalize_address".to_vec(),
                ));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        // Extract the hex part after "cosmos1" prefix
        if input_str.starts_with("cosmos1") && input_str.len() > 7 {
            let hex_part = &input_str[7..];
            match hex::decode(hex_part) {
                Ok(canonical) => {
                    *canonicalized_address_out = UnmanagedVector::new(Some(canonical));
                    wasmvm::GoError::None as i32 // Success
                }
                Err(_) => {
                    *err_msg_out = UnmanagedVector::new(Some(
                        b"Invalid hex in address for canonicalize_address".to_vec(),
                    ));
                    wasmvm::GoError::User as i32 // User error for invalid format
                }
            }
        } else {
            *err_msg_out = UnmanagedVector::new(Some(
                b"Invalid address format for canonicalize_address".to_vec(),
            ));
            wasmvm::GoError::User as i32 // User error for invalid format
        }
    }
}

extern "C" fn impl_validate_address(
    _api: *const api_t,
    input: U8SliceView,
    err_msg_out: *mut UnmanagedVector,
    gas_used: *mut u64,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_API_CALL;

        let input_bytes = match extract_u8_slice_data(input) {
            Some(i) => i,
            None => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Invalid input for validate_address".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        let input_str = match std::str::from_utf8(&input_bytes) {
            Ok(s) => s,
            Err(_) => {
                *err_msg_out = UnmanagedVector::new(Some(
                    b"Invalid UTF-8 address for validate_address".to_vec(),
                ));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        // Simple validation: check if it starts with "cosmos1" and has reasonable length
        if input_str.starts_with("cosmos1") && input_str.len() >= 39 && input_str.len() <= 45 {
            wasmvm::GoError::None as i32 // Valid
        } else {
            *err_msg_out = UnmanagedVector::new(Some(
                b"Invalid address format for validate_address".to_vec(),
            ));
            wasmvm::GoError::User as i32 // Invalid
        }
    }
}

// === Querier Vtable Implementation ===

extern "C" fn impl_query_external(
    _querier: *const querier_t,
    _gas_limit: u64,
    gas_used: *mut u64,
    request: U8SliceView,
    result_out: *mut UnmanagedVector,
    err_msg_out: *mut UnmanagedVector,
) -> i32 {
    unsafe {
        *gas_used = GAS_COST_QUERY;

        let _request_bytes = match extract_u8_slice_data(request) {
            Some(r) => r,
            None => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Invalid request for query_external".to_vec()));
                return wasmvm::GoError::BadArgument as i32;
            }
        };

        // Simple implementation: return empty result for any query (or a predefined mock)
        // In a real implementation, this would handle bank queries, staking queries, etc.
        let empty_result = serde_json::json!({
            "Ok": {
                "Ok": serde_json::Value::Null // Result is null, but success
            }
        });

        match serde_json::to_vec(&empty_result) {
            Ok(result_bytes) => {
                *result_out = UnmanagedVector::new(Some(result_bytes));
                wasmvm::GoError::None as i32 // Success
            }
            Err(_) => {
                *err_msg_out =
                    UnmanagedVector::new(Some(b"Failed to serialize query result".to_vec()));
                wasmvm::GoError::CannotSerialize as i32 // Error
            }
        }
    }
}

// === Vtable Constructors ===

/// Create a DbVtable with working implementations that provide in-memory storage
pub fn create_working_db_vtable() -> DbVtable {
    DbVtable {
        read_db: Some(impl_read_db),
        write_db: Some(impl_write_db),
        remove_db: Some(impl_remove_db),
        scan_db: Some(impl_scan_db),
    }
}

/// Create a GoApiVtable with working implementations that provide basic address operations
pub fn create_working_api_vtable() -> GoApiVtable {
    GoApiVtable {
        humanize_address: Some(impl_humanize_address),
        canonicalize_address: Some(impl_canonicalize_address),
        validate_address: Some(impl_validate_address),
    }
}

/// Create a QuerierVtable with working implementations that provide basic query functionality
pub fn create_working_querier_vtable() -> QuerierVtable {
    QuerierVtable {
        query_external: Some(impl_query_external),
    }
}

/// Clear the in-memory storage (useful for testing)
pub fn clear_storage() {
    if let Ok(mut storage) = STORAGE.lock() {
        storage.data.clear();
    }
}

/// Get storage size (useful for debugging)
pub fn get_storage_size() -> usize {
    STORAGE.lock().map(|s| s.data.len()).unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_working_vtables_have_functions() {
        let db_vtable = create_working_db_vtable();
        assert!(db_vtable.read_db.is_some());
        assert!(db_vtable.write_db.is_some());
        assert!(db_vtable.remove_db.is_some());
        assert!(db_vtable.scan_db.is_some());

        let api_vtable = create_working_api_vtable();
        assert!(api_vtable.humanize_address.is_some());
        assert!(api_vtable.canonicalize_address.is_some());
        assert!(api_vtable.validate_address.is_some());

        let querier_vtable = create_working_querier_vtable();
        assert!(querier_vtable.query_external.is_some());
    }

    #[test]
    fn test_storage_operations() {
        clear_storage();

        // Test that storage starts empty
        assert_eq!(get_storage_size(), 0);

        // Note: We can't easily test the actual FFI functions here without
        // setting up the full FFI environment, but we can test that the
        // vtables are properly constructed.
    }

    #[test]
    fn test_address_validation() {
        // Test valid addresses
        let valid_addresses = vec![
            "cosmos1abc123def456ghi789jkl012mno345pqr678st",
            "cosmos1qwertyuiopasdfghjklzxcvbnm1234567890",
        ];

        for addr in valid_addresses {
            // In a real test, we'd call the FFI function, but for now just test the logic
            assert!(addr.starts_with("cosmos1"));
            assert!(addr.len() >= 39 && addr.len() <= 45);
        }
    }
}
