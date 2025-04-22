use std::collections::HashMap;
use std::convert::TryInto;

use cosmwasm_std::{Order, Record};
use cosmwasm_vm::{BackendError, BackendResult, GasInfo, Storage};

use crate::db::Db;
use crate::error::GoError;
use crate::iterator::GoIter;
use crate::memory::{validate_memory_size, U8SliceView, UnmanagedVector};

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
}

impl Storage for GoStorage {
    fn get(&self, key: &[u8]) -> BackendResult<Option<Vec<u8>>> {
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

        (Ok(output_data), gas_info)
    }

    fn scan(
        &mut self,
        start: Option<&[u8]>,
        end: Option<&[u8]>,
        order: Order,
    ) -> BackendResult<u32> {
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
        iterator.next()
    }

    fn next_key(&mut self, iterator_id: u32) -> BackendResult<Option<Vec<u8>>> {
        let Some(iterator) = self.iterators.get_mut(&iterator_id) else {
            return (
                Err(BackendError::iterator_does_not_exist(iterator_id)),
                GasInfo::free(),
            );
        };

        iterator.next_key()
    }

    fn next_value(&mut self, iterator_id: u32) -> BackendResult<Option<Vec<u8>>> {
        let Some(iterator) = self.iterators.get_mut(&iterator_id) else {
            return (
                Err(BackendError::iterator_does_not_exist(iterator_id)),
                GasInfo::free(),
            );
        };

        iterator.next_value()
    }

    fn set(&mut self, key: &[u8], value: &[u8]) -> BackendResult<()> {
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
