use std::collections::HashMap;
use std::convert::TryInto;

use cosmwasm_std::{Order, KV};
use cosmwasm_vm::{BackendError, BackendResult, GasInfo, Storage};

use crate::db::DB;
use crate::error::GoResult;
use crate::iterator::GoIter;
use crate::memory::{Buffer, U8SliceView, UnmanagedVector};

pub struct GoStorage {
    db: DB,
    iterators: HashMap<u32, GoIter>,
}

impl GoStorage {
    pub fn new(db: DB) -> Self {
        GoStorage {
            db,
            iterators: HashMap::new(),
        }
    }
}

impl Storage for GoStorage {
    fn get(&self, key: &[u8]) -> BackendResult<Option<Vec<u8>>> {
        let mut result = UnmanagedVector::default();
        let mut error_msg = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.db.vtable.read_db)(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(Some(key)),
            &mut result as *mut UnmanagedVector,
            &mut error_msg as *mut Buffer,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed to read a key in the db: {}",
                String::from_utf8_lossy(key)
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let value = result.consume();
        (Ok(value), gas_info)
    }

    fn scan(
        &mut self,
        start: Option<&[u8]>,
        end: Option<&[u8]>,
        order: Order,
    ) -> BackendResult<u32> {
        let mut error_msg = Buffer::default();
        let mut iter = GoIter::new(self.db.gas_meter);
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.db.vtable.scan_db)(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(start),
            U8SliceView::new(end),
            order.into(),
            &mut iter as *mut GoIter,
            &mut error_msg as *mut Buffer,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed to read the next key between {:?} and {:?}",
                start.map(String::from_utf8_lossy),
                end.map(String::from_utf8_lossy),
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let next_id: u32 = self
            .iterators
            .len()
            .try_into()
            .expect("Iterator count exceeded uint32 range. This is a bug.");
        self.iterators.insert(next_id, iter); // This moves iter. Is this okay?
        (Ok(next_id), gas_info)
    }

    fn next(&mut self, iterator_id: u32) -> BackendResult<Option<KV>> {
        let iterator = match self.iterators.get_mut(&iterator_id) {
            Some(i) => i,
            None => {
                return (
                    Err(BackendError::iterator_does_not_exist(iterator_id)),
                    GasInfo::free(),
                )
            }
        };
        iterator.next()
    }

    fn set(&mut self, key: &[u8], value: &[u8]) -> BackendResult<()> {
        let mut error_msg = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.db.vtable.write_db)(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(Some(key)),
            U8SliceView::new(Some(value)),
            &mut error_msg as *mut Buffer,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);
        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed to set a key in the db: {}",
                String::from_utf8_lossy(key),
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }
        (Ok(()), gas_info)
    }

    fn remove(&mut self, key: &[u8]) -> BackendResult<()> {
        let mut error_msg = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.db.vtable.remove_db)(
            self.db.state,
            self.db.gas_meter,
            &mut used_gas as *mut u64,
            U8SliceView::new(Some(key)),
            &mut error_msg as *mut Buffer,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);
        let default = || {
            format!(
                "Failed to delete a key in the db: {}",
                String::from_utf8_lossy(key),
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }
        (Ok(()), gas_info)
    }
}
