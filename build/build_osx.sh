#!/bin/bash

# ref: https://wapl.es/rust/2019/02/17/rust-cross-compile-linux-to-macos.html
export PATH="/root/osxcross/target/bin:$PATH"
export LIBZ_SYS_STATIC=1
export CC=o64-clang
export CXX=o64-clang++

RUSTFLAGS="-C link-args=-Wl,-install_name,@rpath/libgo_cosmwasm.dylib" cargo build --release --target x86_64-apple-darwin --features force-osx-x86-64
cp target/x86_64-apple-darwin/release/deps/libgo_cosmwasm.dylib api
