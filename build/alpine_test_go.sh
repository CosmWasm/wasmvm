#!/bin/bash

IMAGE=demo-alpine-go:latest

## FOR DEBUGGING
#docker run --rm -it -v $(pwd)/..:/code -w /code \
#  -u $(id -u):$(id -g) \
#  --mount type=volume,source="gocosmwasm_alpine_cache",target=/code/target \
#  ${IMAGE} /bin/sh

# build go code
docker run --rm -v $(pwd)/..:/code -w /code \
  -u $(id -u):$(id -g) \
  ${IMAGE} go build .

# run go tests
docker run --rm -v $(pwd)/..:/code -w /code \
  -u $(id -u):$(id -g) \
  ${IMAGE} go test ./api ./types
