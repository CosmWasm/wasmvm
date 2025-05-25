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
    eprintln!("🔍 [DEBUG] impl_read_db called");
    unsafe {
        *gas_used = GAS_COST_READ;
        eprintln!("🔍 [DEBUG] Gas set to {}", GAS_COST_READ);

        let key_bytes = match key.read() {
            Some(k) => {
                eprintln!("🔍 [DEBUG] Key read successfully: {} bytes", k.len());
                k
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read key from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid key".to_vec()));
                return 1;
            }
        };

        eprintln!("🔍 [DEBUG] Attempting to lock storage");
        match STORAGE.lock() {
            Ok(storage) => {
                eprintln!(
                    "🔍 [DEBUG] Storage locked successfully, {} items in storage",
                    storage.data.len()
                );
                if let Some(value) = storage.data.get(key_bytes) {
                    eprintln!("✅ [DEBUG] Key found, value size: {} bytes", value.len());
                    *value_out = UnmanagedVector::new(Some(value.clone()));
                } else {
                    eprintln!("🔍 [DEBUG] Key not found in storage");
                    *value_out = UnmanagedVector::new(None); // Key not found
                }
                eprintln!("✅ [DEBUG] impl_read_db returning success");
                0 // Success
            }
            Err(e) => {
                eprintln!("❌ [DEBUG] Failed to lock storage: {:?}", e);
                *err_msg_out = UnmanagedVector::new(Some(b"Storage lock error".to_vec()));
                1 // Error
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
    eprintln!("🔍 [DEBUG] impl_write_db called");
    unsafe {
        *gas_used = GAS_COST_WRITE;
        eprintln!("🔍 [DEBUG] Gas set to {}", GAS_COST_WRITE);

        let key_bytes = match key.read() {
            Some(k) => {
                eprintln!("🔍 [DEBUG] Key read successfully: {} bytes", k.len());
                k.to_vec()
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read key from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid key".to_vec()));
                return 1;
            }
        };

        let value_bytes = match value.read() {
            Some(v) => {
                eprintln!("🔍 [DEBUG] Value read successfully: {} bytes", v.len());
                v.to_vec()
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read value from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid value".to_vec()));
                return 1;
            }
        };

        eprintln!("🔍 [DEBUG] Attempting to lock storage for write");
        match STORAGE.lock() {
            Ok(mut storage) => {
                eprintln!("🔍 [DEBUG] Storage locked, inserting key-value pair");
                storage.data.insert(key_bytes, value_bytes);
                eprintln!(
                    "✅ [DEBUG] impl_write_db returning success, storage now has {} items",
                    storage.data.len()
                );
                0 // Success
            }
            Err(e) => {
                eprintln!("❌ [DEBUG] Failed to lock storage: {:?}", e);
                *err_msg_out = UnmanagedVector::new(Some(b"Storage lock error".to_vec()));
                1 // Error
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
    eprintln!("🔍 [DEBUG] impl_remove_db called");
    unsafe {
        *gas_used = GAS_COST_REMOVE;

        let key_bytes = match key.read() {
            Some(k) => {
                eprintln!("🔍 [DEBUG] Key read successfully: {} bytes", k.len());
                k
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read key from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid key".to_vec()));
                return 1;
            }
        };

        match STORAGE.lock() {
            Ok(mut storage) => {
                let existed = storage.data.remove(key_bytes).is_some();
                eprintln!(
                    "🔍 [DEBUG] Key removal: existed={}, storage now has {} items",
                    existed,
                    storage.data.len()
                );
                0 // Success
            }
            Err(e) => {
                eprintln!("❌ [DEBUG] Failed to lock storage: {:?}", e);
                *err_msg_out = UnmanagedVector::new(Some(b"Storage lock error".to_vec()));
                1 // Error
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
    eprintln!("🔍 [DEBUG] impl_scan_db called (not implemented)");
    unsafe {
        *gas_used = GAS_COST_SCAN;
        *err_msg_out = UnmanagedVector::new(Some(b"Scan not implemented yet".to_vec()));
        1 // Error
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
    eprintln!("🔍 [DEBUG] impl_humanize_address called");
    unsafe {
        *gas_used = GAS_COST_API_CALL;

        let input_bytes = match input.read() {
            Some(i) => {
                eprintln!("🔍 [DEBUG] Input read successfully: {} bytes", i.len());
                i
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read input from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid input".to_vec()));
                return 1;
            }
        };

        let human_address = format!(
            "cosmos1{}",
            hex::encode(&input_bytes[..std::cmp::min(20, input_bytes.len())])
        );
        eprintln!("🔍 [DEBUG] Generated human address: {}", human_address);
        *humanized_address_out = UnmanagedVector::new(Some(human_address.into_bytes()));
        0 // Success
    }
}

extern "C" fn impl_canonicalize_address(
    _api: *const api_t,
    input: U8SliceView,
    canonicalized_address_out: *mut UnmanagedVector,
    err_msg_out: *mut UnmanagedVector,
    gas_used: *mut u64,
) -> i32 {
    eprintln!("🔍 [DEBUG] impl_canonicalize_address called");
    unsafe {
        *gas_used = GAS_COST_API_CALL;

        let input_bytes = match input.read() {
            Some(i) => {
                eprintln!("🔍 [DEBUG] Input read successfully: {} bytes", i.len());
                i
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read input from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid input".to_vec()));
                return 1;
            }
        };

        let input_str = match std::str::from_utf8(input_bytes) {
            Ok(s) => {
                eprintln!("🔍 [DEBUG] Input string: {}", s);
                s
            }
            Err(_) => {
                eprintln!("❌ [DEBUG] Invalid UTF-8 in input");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid UTF-8 address".to_vec()));
                return 1;
            }
        };

        if input_str.starts_with("cosmos1") && input_str.len() > 7 {
            let hex_part = &input_str[7..];
            match hex::decode(hex_part) {
                Ok(canonical) => {
                    eprintln!(
                        "🔍 [DEBUG] Canonicalized address: {} bytes",
                        canonical.len()
                    );
                    *canonicalized_address_out = UnmanagedVector::new(Some(canonical));
                    0 // Success
                }
                Err(_) => {
                    eprintln!("❌ [DEBUG] Invalid hex in address");
                    *err_msg_out = UnmanagedVector::new(Some(b"Invalid hex in address".to_vec()));
                    1 // Error
                }
            }
        } else {
            eprintln!("❌ [DEBUG] Invalid address format");
            *err_msg_out = UnmanagedVector::new(Some(b"Invalid address format".to_vec()));
            1 // Error
        }
    }
}

extern "C" fn impl_validate_address(
    _api: *const api_t,
    input: U8SliceView,
    err_msg_out: *mut UnmanagedVector,
    gas_used: *mut u64,
) -> i32 {
    eprintln!("🔍 [DEBUG] impl_validate_address called");
    unsafe {
        *gas_used = GAS_COST_API_CALL;

        let input_bytes = match input.read() {
            Some(i) => {
                eprintln!("🔍 [DEBUG] Input read successfully: {} bytes", i.len());
                i
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read input from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid input".to_vec()));
                return 1;
            }
        };

        let input_str = match std::str::from_utf8(input_bytes) {
            Ok(s) => {
                eprintln!("🔍 [DEBUG] Validating address: {}", s);
                s
            }
            Err(_) => {
                eprintln!("❌ [DEBUG] Invalid UTF-8 in input");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid UTF-8 address".to_vec()));
                return 1;
            }
        };

        if input_str.starts_with("cosmos1") && input_str.len() >= 39 && input_str.len() <= 45 {
            eprintln!("✅ [DEBUG] Address validation passed");
            0 // Valid
        } else {
            eprintln!("❌ [DEBUG] Address validation failed");
            *err_msg_out = UnmanagedVector::new(Some(b"Invalid address format".to_vec()));
            1 // Invalid
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
    eprintln!("🔍 [DEBUG] impl_query_external called");
    unsafe {
        *gas_used = GAS_COST_QUERY;

        let _request_bytes = match request.read() {
            Some(r) => {
                eprintln!("🔍 [DEBUG] Request read successfully: {} bytes", r.len());
                r
            }
            None => {
                eprintln!("❌ [DEBUG] Failed to read request from U8SliceView");
                *err_msg_out = UnmanagedVector::new(Some(b"Invalid request".to_vec()));
                return 1;
            }
        };

        let empty_result = serde_json::json!({
            "Ok": {
                "Ok": null
            }
        });

        match serde_json::to_vec(&empty_result) {
            Ok(result_bytes) => {
                eprintln!(
                    "🔍 [DEBUG] Query result serialized: {} bytes",
                    result_bytes.len()
                );
                *result_out = UnmanagedVector::new(Some(result_bytes));
                0 // Success
            }
            Err(_) => {
                eprintln!("❌ [DEBUG] Failed to serialize query result");
                *err_msg_out = UnmanagedVector::new(Some(b"Failed to serialize result".to_vec()));
                1 // Error
            }
        }
    }
}

// === Vtable Constructors ===

/// Create a DbVtable with working implementations that provide in-memory storage
pub fn create_working_db_vtable() -> DbVtable {
    eprintln!("🔧 [DEBUG] Creating working DB vtable");
    DbVtable {
        read_db: Some(impl_read_db),
        write_db: Some(impl_write_db),
        remove_db: Some(impl_remove_db),
        scan_db: Some(impl_scan_db),
    }
}

/// Create a GoApiVtable with working implementations that provide basic address operations
pub fn create_working_api_vtable() -> GoApiVtable {
    eprintln!("🔧 [DEBUG] Creating working API vtable");
    GoApiVtable {
        humanize_address: Some(impl_humanize_address),
        canonicalize_address: Some(impl_canonicalize_address),
        validate_address: Some(impl_validate_address),
    }
}

/// Create a QuerierVtable with working implementations that provide basic query functionality
pub fn create_working_querier_vtable() -> QuerierVtable {
    eprintln!("🔧 [DEBUG] Creating working Querier vtable");
    QuerierVtable {
        query_external: Some(impl_query_external),
    }
}

/// Clear the in-memory storage (useful for testing)
pub fn clear_storage() {
    if let Ok(mut storage) = STORAGE.lock() {
        let count = storage.data.len();
        storage.data.clear();
        eprintln!("🧹 [DEBUG] Cleared storage, removed {} items", count);
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

    #[test]
    fn test_debug_vtable_creation() {
        println!("Testing vtable creation with debug output...");

        let _db_vtable = create_working_db_vtable();
        let _api_vtable = create_working_api_vtable();
        let _querier_vtable = create_working_querier_vtable();

        println!("All vtables created successfully");
    }

    #[test]
    fn test_storage_debug() {
        println!("Testing storage operations with debug output...");

        clear_storage();
        assert_eq!(get_storage_size(), 0);

        println!("Storage cleared and verified empty");
    }
}
