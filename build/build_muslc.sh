#!/bin/sh

cargo build --release --features backtraces --example muslc
cp /code/target/release/examples/libmuslc.a /code/api/libwasmvm_muslc.a
