#!/bin/bash

# ref: https://www.reddit.com/r/rust/comments/5k8uab/crosscompiling_from_ubuntu_to_windows_with_rustup/

cargo build --release --target=x86_64-pc-windows-gnu --verbose
cp target/x86_64-pc-windows-gnu/release/deps/libgo_cosmwasm.dll api
