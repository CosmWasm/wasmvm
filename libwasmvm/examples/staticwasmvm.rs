// Make sure we pull in the wasmvm library.
// We don't need though symbols though. The extern "C" functions are re-exported.
#[allow(unused)]
use wasmvm::*;
