#!/bin/bash

cargo build --release --features force-linux-x86-64
cp target/release/deps/libgo_cosmwasm.so api
strip api/libgo_cosmwasm.so
