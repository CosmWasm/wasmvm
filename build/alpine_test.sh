#!/bin/bash

IMAGE=demo-alpine:latest

# FOR DEBUGGING
#docker run -it -v $(pwd)/..:/code -w /code \
#  -u $(id -u):$(id -g) \
#  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
#  ${IMAGE} /bin/sh

# ENSURE TESTS RUN
#docker run -v $(pwd)/..:/code -w /code \
#  -u $(id -u):$(id -g) \
#  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
#  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
#  ${IMAGE} cargo test

# BUILD ALPINE DLL
docker run --rm -v $(pwd)/..:/code -w /code \
  -u $(id -u):$(id -g) \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  ${IMAGE} cargo build --release --features backtraces

# VIEW THE DATA
docker run --rm -v $(pwd)/..:/code -w /code \
  -u $(id -u):$(id -g) \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  ${IMAGE} ls target/release

# COPY IT OUT
docker run --rm -v $(pwd)/..:/code -w /code \
  -u $(id -u):$(id -g) \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  ${IMAGE} cp /code/target/release//libgo_cosmwasm.a /code/api
