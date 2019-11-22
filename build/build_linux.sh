#!/bin/bash

cargo build --release
cp target/release/deps/libgo_cosmwasm.so api
strip api/libgo_cosmwasm.so
