.PHONY: all build build-rust build-go test docker-image docker-image-centos7 docker-image-cross

# Versioned by a simple counter that is not bound to a specific CosmWasm version
TAG_PREFIX := 0002
USER_ID := $(shell id -u)
USER_GROUP = $(shell id -g)

DLL_EXT = ""
ifeq ($(OS),Windows_NT)
	DLL_EXT = dll
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		DLL_EXT = so
	endif
	ifeq ($(UNAME_S),Darwin)
		DLL_EXT = dylib
	endif
endif

all: build test

build: build-rust build-go

# don't strip for now, for better error reporting
# build-rust: build-rust-release strip
build-rust: build-rust-release

# use debug build for quick testing
build-rust-debug:
	cargo build --features backtraces
	cp target/debug/libwasmvm.$(DLL_EXT) api

# use release build to actually ship - smaller and much faster
build-rust-release:
	cargo build --release --features backtraces
	cp target/release/libwasmvm.$(DLL_EXT) api
	@ #this pulls out ELF symbols, 80% size reduction!

# implement stripping based on os
ifeq ($(DLL_EXT),so)
strip:
	strip api/libwasmvm.so
else
# TODO: add for windows and osx
strip:
endif

build-go:
	go build ./...

test:
	RUST_BACKTRACE=1 go test -v ./api ./types .

test-safety:
	GODEBUG=cgocheck=2 go test -race -v -count 1 ./api

# we should build all the docker images locally ONCE and publish them
docker-image-centos7:
	docker build . -t cosmwasm/go-ext-builder:$(TAG_PREFIX)-centos7 -f ./Dockerfile.centos7

docker-image-cross:
	docker build . -t cosmwasm/go-ext-builder:$(TAG_PREFIX)-cross -f ./Dockerfile.cross

docker-image-alpine:
	docker build . -t cosmwasm/go-ext-builder:$(TAG_PREFIX)-alpine -f ./Dockerfile.alpine

docker-images: docker-image-centos7 docker-image-cross docker-image-alpine

docker-publish: docker-images
	docker push cosmwasm/go-ext-builder:$(TAG_PREFIX)-cross
	docker push cosmwasm/go-ext-builder:$(TAG_PREFIX)-centos7
	docker push cosmwasm/go-ext-builder:$(TAG_PREFIX)-alpine

# and use them to compile release builds
release:
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(TAG_PREFIX)-cross
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(TAG_PREFIX)-centos7

test-alpine:
	# build the muslc *.a file
	rm -rf target/release/examples
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(TAG_PREFIX)-alpine
	# try running go tests using this lib with muslc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-builder:$(TAG_PREFIX)-alpine go build -tags muslc .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-builder:$(TAG_PREFIX)-alpine go test -tags muslc ./api ./types
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-builder:$(TAG_PREFIX)-alpine go build -tags muslc -o muslc.exe ./cmd
	# run static binary in an alpine machines (not dlls)
	docker run --rm --read-only -v $(shell pwd):/code -w /code alpine:3.12 ./muslc.exe ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/code -w /code alpine:3.11 ./muslc.exe ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/code -w /code alpine:3.10 ./muslc.exe ./api/testdata/hackatom.wasm
	# run static binary locally if you are on Linux
	# ./muslc.exe ./api/testdata/hackatom.wasm
