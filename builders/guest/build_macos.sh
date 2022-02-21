#!/bin/bash
set -o errexit -o nounset -o pipefail

# ref: https://wapl.es/rust/2019/02/17/rust-cross-compile-linux-to-macos.html
export PATH="/opt/osxcross/target/bin:$PATH"
export LIBZ_SYS_STATIC=1
export CC=o64-clang
export CXX=o64-clang++

# See https://github.com/CosmWasm/wasmvm/issues/222#issuecomment-880616953 for two approaches to
# enable stripping through cargo (if that is desired).

cargo build --release --target x86_64-apple-darwin
