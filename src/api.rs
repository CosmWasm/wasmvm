//use cosmwasm::traits::{Api};
use cosmwasm::mock::MockApi;

//#[repr(C)]
//pub struct Precompiles {
//    pub c_human_address: extern "C" fn(*mut db_t, Buffer, Buffer) -> i64,
//    pub c_canonical_address: extern "C" fn(*mut db_t, Buffer, Buffer),
//}

// This is a stub with mock implementation until we support
// passing in functions from go
pub type Precompiles = MockApi;

pub fn mock_api() -> Precompiles {
    Precompiles::new(42)
}