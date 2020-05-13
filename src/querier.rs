use cosmwasm_std::SystemError;
use cosmwasm_vm::{FfiResult, Querier, QuerierResult};

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
    pub query_external: extern "C" fn(*const querier_t, Buffer, *mut Buffer) -> i32,
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
    fn raw_query(&self, request: &[u8]) -> QuerierResult {
        let request = Buffer::from_vec(request.to_vec());
        let mut result_buf = Buffer::default();
        let go_result: GoResult =
            (self.vtable.query_external)(self.state, request, &mut result_buf as *mut Buffer)
                .into();
        let _request = unsafe { request.consume() };
        let go_result: FfiResult<()> = go_result.into();
        go_result?;

        let bin_result = unsafe { result_buf.consume() };
        match serde_json::from_slice(&bin_result) {
            Ok(system_result) => Ok(system_result),
            Err(_) => Ok(Err(SystemError::InvalidResponse { msg: bin_result })),
        }
    }
}
