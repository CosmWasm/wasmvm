use crate::memory::{Buffer, free_rust, read_buffer, release_vec};

// this represents something passed in from the caller side of FFI
#[repr(C)]
pub struct db_t { }

#[repr(C)]
pub struct DB {
    pub state: *mut db_t,
    pub c_get: extern fn(*mut db_t, Buffer, Buffer) -> i64,
    pub c_set: extern fn(*mut db_t, Buffer, Buffer),
}

impl DB {
    pub fn get(&self, key: Vec<u8>) -> Option<Vec<u8>> {
        let buf = release_vec(key);
        // TODO: dynamic size
        let buf2 = release_vec(vec![0u8; 2000]);
        let res = (self.c_get)(self.state, buf, buf2);

        // read in the number of bytes returned
        if res < 0 {
            // TODO
            panic!("val was not big enough for data");
        }
        let mut buf3 = buf2;
        buf3.size = res as usize;
        let val = read_buffer(&buf3).map(|s| s.to_vec());

        // clean up our value buffer
        // Contract: the key is cleaned up by caller
        free_rust(buf2);
        val
    }

    pub fn set(&self, key: Vec<u8>, value: Vec<u8>) {
        let buf = release_vec(key);
        let buf2 = release_vec(value);
        // caller will free input
        (self.c_set)(self.state, buf, buf2);
    }
}
