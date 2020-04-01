use cosmwasm::traits::{ReadonlyStorage, Storage};

use crate::memory::Buffer;

static DB_GET_MAX_RESULT_LENGTH: usize = 2000;

// this represents something passed in from the caller side of FFI
#[repr(C)]
pub struct db_t {}

#[repr(C)]
pub struct DB_vtable {
    pub read_db: extern "C" fn(*mut db_t, Buffer, Buffer) -> i64,
    pub write_db: extern "C" fn(*mut db_t, Buffer, Buffer),
}

#[repr(C)]
pub struct DB {
    pub state: *mut db_t,
    pub vtable: DB_vtable,
}

impl ReadonlyStorage for DB {
    fn get(&self, key: &[u8]) -> Option<Vec<u8>> {
        let buf = Buffer::from_vec(key.to_vec());
        // TODO: dynamic size (https://github.com/CosmWasm/go-cosmwasm/issues/59)
        let mut result_buf = Buffer::with_capacity(DB_GET_MAX_RESULT_LENGTH);
        let res = (self.vtable.read_db)(self.state, buf, result_buf);

        // read in the number of bytes returned
        if res < 0 {
            // TODO
            panic!("val was not big enough for data");
        }
        if res == 0 {
            return None;
        }
        result_buf.len = res as usize;
        unsafe { Some(result_buf.consume()) }
    }
}

impl Storage for DB {
    fn set(&mut self, key: &[u8], value: &[u8]) {
        let buf = Buffer::from_vec(key.to_vec());
        let buf2 = Buffer::from_vec(value.to_vec());
        // caller will free input
        (self.vtable.write_db)(self.state, buf, buf2);
    }
}
