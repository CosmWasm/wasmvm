use std::collections::HashMap;
use std::convert::TryInto;

use cosmwasm_std::{Order, Record};
use cosmwasm_vm::{BackendError, BackendResult, GasInfo, Storage};

use crate::db::Db;
use crate::error::GoError;
use crate::iterator::GoIter;
use crate::memory::{validate_memory_size, U8SliceView, UnmanagedVector};

// Constants for DB access validation
const MAX_KEY_SIZE: usize = 64 * 1024; // 64KB max key size
const MAX_VALUE_SIZE: usize = 1024 * 1024; // 1MB max value size

pub struct GoStorage {
    db: Db,
    iterators: HashMap<u32, GoIter>,
}

impl GoStorage {
    pub fn new(db: Db) -> Self {
        GoStorage {
            db,
            iterators: HashMap::new(),
        }
    }

    // Validate database key for safety
    fn validate_db_key(&self, key: &[u8]) -> Result<(), BackendError> {
        // Check key size
        if key.is_empty() {
            return Err(BackendError::unknown("Key cannot be empty"));
        }

        if key.len() > MAX_KEY_SIZE {
            return Err(BackendError::unknown(format!(
                "Key size exceeds limit: {} > {}",
                key.len(),
                MAX_KEY_SIZE
            )));
        }

        Ok(())
    }

    // Validate database value for safety
    fn validate_db_value(&self, value: &[u8]) -> Result<(), BackendError> {
        // Check value size
        if value.len() > MAX_VALUE_SIZE {
            return Err(BackendError::unknown(format!(
                "Value size exceeds limit: {} > {}",
                value.len(),
                MAX_VALUE_SIZE
            )));
        }

        Ok(())
    }
}

impl Storage for GoStorage {
    fn get(&self, key: &[u8]) -> BackendResult<Option<Vec<u8>>> {
        // Validate key
        if let Err(e) = self.validate_db_key(key) {
            return (Err(e), GasInfo::free());
        }

        if let Err(e) = validate_memory_size(key.len()) {
            return (
                Err(BackendError::unknown(format!(
                    "Key size validation failed: {}",
                    e
                ))),
                GasInfo::free(),
            );
        }

        let mut output = UnmanagedVector::default();
        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let read_db = self
            .db
            .vtable
            .read_db
            .expect("vtable function 'read_db' not set");
        let go_error: GoError = read_db(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(Some(key)),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
        )
        .into();

        let gas_info = GasInfo::with_externally_used(used_gas);

        let default = || {
            format!(
                "Failed to read a key in the db: {}",
                String::from_utf8_lossy(key)
            )
        };

        // First check the error result using the safe wrapper
        if let Err(err) = go_error.into_result_safe(error_msg, default) {
            return (Err(err), gas_info);
        }

        // If we got here, no error occurred, so we can safely consume the output
        let output_data = output.consume();

        // Validate returned value if present
        if let Some(ref value) = output_data {
            if let Err(e) = self.validate_db_value(value) {
                return (Err(e), gas_info);
            }
        }

        (Ok(output_data), gas_info)
    }

    fn scan(
        &mut self,
        start: Option<&[u8]>,
        end: Option<&[u8]>,
        order: Order,
    ) -> BackendResult<u32> {
        // Validate start and end keys if present
        if let Some(start_key) = start {
            if let Err(e) = self.validate_db_key(start_key) {
                return (Err(e), GasInfo::free());
            }
        }

        if let Some(end_key) = end {
            if let Err(e) = self.validate_db_key(end_key) {
                return (Err(e), GasInfo::free());
            }
        }

        let mut error_msg = UnmanagedVector::default();
        let mut iter = GoIter::stub();
        let mut used_gas = 0_u64;
        let scan_db = self
            .db
            .vtable
            .scan_db
            .expect("vtable function 'scan_db' not set");
        let go_error: GoError = scan_db(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(start),
            U8SliceView::new(end),
            order.into(),
            &mut iter as *mut GoIter,
            &mut error_msg as *mut UnmanagedVector,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);

        let default = || {
            format!(
                "Failed to read the next key between {:?} and {:?}",
                start.map(String::from_utf8_lossy),
                end.map(String::from_utf8_lossy),
            )
        };

        if let Err(err) = go_error.into_result_safe(error_msg, default) {
            return (Err(err), gas_info);
        }

        let next_id: u32 = self
            .iterators
            .len()
            .try_into()
            .expect("Iterator count exceeded uint32 range. This is a bug.");
        self.iterators.insert(next_id, iter);
        (Ok(next_id), gas_info)
    }

    fn next(&mut self, iterator_id: u32) -> BackendResult<Option<Record>> {
        let Some(iterator) = self.iterators.get_mut(&iterator_id) else {
            return (
                Err(BackendError::iterator_does_not_exist(iterator_id)),
                GasInfo::free(),
            );
        };

        let result = iterator.next();

        // Validate the returned record if present
        if let Ok(Some((key, value))) = &result.0 {
            if let Err(e) = self.validate_db_key(key) {
                return (Err(e), result.1);
            }

            if let Err(e) = self.validate_db_value(value) {
                return (Err(e), result.1);
            }
        }

        result
    }

    fn next_key(&mut self, iterator_id: u32) -> BackendResult<Option<Vec<u8>>> {
        let Some(iterator) = self.iterators.get_mut(&iterator_id) else {
            return (
                Err(BackendError::iterator_does_not_exist(iterator_id)),
                GasInfo::free(),
            );
        };

        let result = iterator.next_key();

        // Validate the returned key if present
        if let Ok(Some(ref key)) = &result.0 {
            if let Err(e) = self.validate_db_key(key) {
                return (Err(e), result.1);
            }
        }

        result
    }

    fn next_value(&mut self, iterator_id: u32) -> BackendResult<Option<Vec<u8>>> {
        let Some(iterator) = self.iterators.get_mut(&iterator_id) else {
            return (
                Err(BackendError::iterator_does_not_exist(iterator_id)),
                GasInfo::free(),
            );
        };

        let result = iterator.next_value();

        // Validate the returned value if present
        if let Ok(Some(ref value)) = &result.0 {
            if let Err(e) = self.validate_db_value(value) {
                return (Err(e), result.1);
            }
        }

        result
    }

    fn set(&mut self, key: &[u8], value: &[u8]) -> BackendResult<()> {
        // Validate key and value
        if let Err(e) = self.validate_db_key(key) {
            return (Err(e), GasInfo::free());
        }

        if let Err(e) = self.validate_db_value(value) {
            return (Err(e), GasInfo::free());
        }

        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let write_db = self
            .db
            .vtable
            .write_db
            .expect("vtable function 'write_db' not set");
        let go_error: GoError = write_db(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(Some(key)),
            U8SliceView::new(Some(value)),
            &mut error_msg as *mut UnmanagedVector,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);
        let default = || {
            format!(
                "Failed to set a key in the db: {}",
                String::from_utf8_lossy(key),
            )
        };

        if let Err(err) = go_error.into_result_safe(error_msg, default) {
            return (Err(err), gas_info);
        }

        (Ok(()), gas_info)
    }

    fn remove(&mut self, key: &[u8]) -> BackendResult<()> {
        // Validate key
        if let Err(e) = self.validate_db_key(key) {
            return (Err(e), GasInfo::free());
        }

        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let remove_db = self
            .db
            .vtable
            .remove_db
            .expect("vtable function 'remove_db' not set");
        let go_error: GoError = remove_db(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(Some(key)),
            &mut error_msg as *mut UnmanagedVector,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);
        let default = || {
            format!(
                "Failed to delete a key in the db: {}",
                String::from_utf8_lossy(key),
            )
        };

        if let Err(err) = go_error.into_result_safe(error_msg, default) {
            return (Err(err), gas_info);
        }

        (Ok(()), gas_info)
    }
}
