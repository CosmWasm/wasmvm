#!/bin/sh

# See https://github.com/CosmWasm/wasmvm/issues/222#issuecomment-880616953 for two approaches to
# enable stripping through cargo (if that is desired).

echo "Starting aarch64-unknown-linux-musl build"
cargo build --release --target aarch64-unknown-linux-musl --example muslc
echo "Starting x86_64-unknown-linux-musl build"
cargo build --release --target x86_64-unknown-linux-musl --example muslc

cp target/aarch64-unknown-linux-musl/release/examples/libmuslc.a artifacts/libwasmvm_muslc.aarch64.a
cp target/x86_64-unknown-linux-musl/release/examples/libmuslc.a artifacts/libwasmvm_muslc.a
