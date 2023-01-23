# Why we are not using errno anymore

wasmvm up until 1.1.1 used errno to communicate errors over the FFI that
happened in the Rust code. The [errno](https://crates.io/crates/errno) crate was
used to set error values in Rust and
[cgo automatically read them](https://utcc.utoronto.ca/~cks/space/blog/programming/GoCgoErrorReturns)
and made them available in the error return value of
`ptr, err := C.init_cache(...)`.

Unfortunately errno does not work well for us when trying to support Windows. On
Windows, `errno`
[uses a Windows API](https://github.com/lambda-fairy/rust-errno/blob/v0.2.8/src/windows.rs#L60-L70)
which then is not what cgo expects. Also Rust's
[libc does not help us](https://github.com/rust-lang/libc/issues/1644) solving
the issue. Using the C errno variable via
[rust-errno](https://github.com/lambda-fairy/rust-errno) caused linker errors
using the Go "internal" linker.

In order to avoid wasting more time and remaining flexible when it comes to the
linker we use, we let all FFI functions return their error as an integer. The
result then goes into an output pointer.
