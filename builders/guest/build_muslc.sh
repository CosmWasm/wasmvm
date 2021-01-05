#!/bin/sh

cargo build --release --example muslc
cp /code/target/release/examples/libmuslc.a /code/api/libwasmvm_muslc.a
