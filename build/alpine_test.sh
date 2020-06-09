#!/bin/bash

#IMAGE=rust:alpine3.11
IMAGE=demo-alpine:latest

# FOR DEBUGGING
#docker run -it -v $(pwd)/..:/code -w /code \
#  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
#  ${IMAGE} /bin/sh

# ENSURE TESTS RUN
#docker run -v $(pwd)/..:/code -w /code \
#  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
#  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
#  ${IMAGE} cargo test

# BUILD ALPINE DLL
docker run -v $(pwd)/..:/code -w /code \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  ${IMAGE} cargo build --release --features backtraces

# VIEW THE DATA
docker run -v $(pwd)/..:/code -w /code \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  ${IMAGE} ls target/release

# COPY IT OUT
docker run -v $(pwd)/..:/code -w /code \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  ${IMAGE} cp /code/target/release//libgo_cosmwasm.a /code/api


## for wasmer
#docker run -v $(pwd):/code -w /code \
#  --mount type=volume,source="wasmer_alpine_cache",target=/code/target \
#  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
#  demo-alpine:latest cargo build
#
## for cosmwasm
#docker run -v $(pwd):/code -w /code/packages/std \
#  --env RUSTFLAGS="-C target-feature=-crt-static" \
#  --mount type=volume,source="cosmwasm_alpine_cache",target=/code/target \
#  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
#  demo-alpine:latest cargo build


#docker run -v $(pwd):/code
