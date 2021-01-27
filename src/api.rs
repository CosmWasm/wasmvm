use cosmwasm_std::{CanonicalAddr, HumanAddr};
use cosmwasm_vm::{Api, BackendError, BackendResult, GasInfo};

use crate::error::GoResult;
use crate::memory::{Buffer, U8SliceView, UnmanagedVector};

// this represents something passed in from the caller side of FFI
// in this case a struct with go function pointers
#[repr(C)]
pub struct api_t {
    _private: [u8; 0],
}

// These functions should return GoResult but because we don't trust them here, we treat the return value as i32
// and then check it when converting to GoResult manually
#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi_vtable {
    pub humanize_address: extern "C" fn(
        *const api_t,
        U8SliceView,
        *mut UnmanagedVector, // human output
        *mut Buffer,          // error message output
        *mut u64,
    ) -> i32,
    pub canonicalize_address: extern "C" fn(
        *const api_t,
        U8SliceView,
        *mut UnmanagedVector, // canonical output
        *mut Buffer,          // error message output
        *mut u64,
    ) -> i32,
}

#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi {
    pub state: *const api_t,
    pub vtable: GoApi_vtable,
}

// We must declare that these are safe to Send, to use in wasm.
// The known go caller passes in immutable function pointers, but this is indeed
// unsafe for possible other callers.
//
// see: https://stackoverflow.com/questions/50258359/can-a-struct-containing-a-raw-pointer-implement-send-and-be-ffi-safe
unsafe impl Send for GoApi {}

impl Api for GoApi {
    fn canonical_address(&self, human: &HumanAddr) -> BackendResult<CanonicalAddr> {
        let mut output = UnmanagedVector::default();
        let mut error_msg = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.vtable.canonicalize_address)(
            self.state,
            U8SliceView::new(Some(human.as_bytes())),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut Buffer,
            &mut used_gas as *mut u64,
        )
        .into();
        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || format!("Failed to canonicalize the address: {}", human);
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let result = output
            .consume()
            .ok_or_else(|| BackendError::unknown("Unset output"))
            .map(CanonicalAddr::from);
        (result, gas_info)
    }

    fn human_address(&self, canonical: &CanonicalAddr) -> BackendResult<HumanAddr> {
        let mut output = UnmanagedVector::default();
        let mut error_msg = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.vtable.humanize_address)(
            self.state,
            U8SliceView::new(Some(canonical.as_slice())),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut Buffer,
            &mut used_gas as *mut u64,
        )
        .into();
        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || format!("Failed to humanize the address: {}", canonical);
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let result = output
            .consume()
            .ok_or_else(|| BackendError::unknown("Unset output"))
            .and_then(|human_data| {
                String::from_utf8(human_data)
                    .map_err(BackendError::from)
                    .map(HumanAddr)
            });
        (result, gas_info)
    }
}
