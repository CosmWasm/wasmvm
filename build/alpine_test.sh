#!/bin/bash

#IMAGE=rust:alpine3.11
IMAGE=demo-alpine:latest

#docker run -it -v $(pwd)/..:/code -w /code \
#  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
#  ${IMAGE} /bin/sh

docker run -v $(pwd)/..:/code -w /code \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  ${IMAGE} cargo build --release --features backtraces

docker run -v $(pwd)/..:/code -w /code \
  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
  ${IMAGE} ls target target/release




#docker run -v $(pwd):/code
