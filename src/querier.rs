use cosmwasm_std::{Binary, ContractResult, SystemError, SystemResult};
use cosmwasm_vm::{BackendResult, GasInfo, Querier};

use crate::error::GoResult;
use crate::memory::Buffer;

// this represents something passed in from the caller side of FFI
#[repr(C)]
#[derive(Clone)]
pub struct querier_t {
    _private: [u8; 0],
}

#[repr(C)]
#[derive(Clone)]
pub struct Querier_vtable {
    // We return errors through the return buffer, but may return non-zero error codes on panic
    pub query_external:
        extern "C" fn(*const querier_t, u64, *mut u64, Buffer, *mut Buffer, *mut Buffer) -> i32,
}

#[repr(C)]
#[derive(Clone)]
pub struct GoQuerier {
    pub state: *const querier_t,
    pub vtable: Querier_vtable,
}

// TODO: check if we can do this safer...
unsafe impl Send for GoQuerier {}

impl Querier for GoQuerier {
    fn query_raw(
        &self,
        request: &[u8],
        gas_limit: u64,
    ) -> BackendResult<SystemResult<ContractResult<Binary>>> {
        let request_buf = Buffer::from_vec(request.to_vec());
        let mut result_buf = Buffer::default();
        let mut error_msg = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.vtable.query_external)(
            self.state,
            gas_limit,
            &mut used_gas as *mut u64,
            request_buf,
            &mut result_buf as *mut Buffer,
            &mut error_msg as *mut Buffer,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);
        let _request = unsafe { request_buf.consume() };

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed to query another contract with this request: {}",
                String::from_utf8_lossy(request)
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let bin_result = unsafe { result_buf.consume() };
        let result = serde_json::from_slice(&bin_result).or_else(|e| {
            Ok(SystemResult::Err(SystemError::InvalidResponse {
                error: format!("Parsing Go response: {}", e),
                response: bin_result.into(),
            }))
        });
        (result, gas_info)
    }
}
