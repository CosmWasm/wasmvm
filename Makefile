.PHONY: all build build-rust build-go test

# Builds the Rust library libwasmvm
BUILDERS_PREFIX := cosmwasm/go-ext-builder:0008
# Contains a full Go dev environment in order to run Go tests on the built library
ALPINE_TESTER := cosmwasm/go-ext-builder:0008-alpine

USER_ID := $(shell id -u)
USER_GROUP = $(shell id -g)

SHARED_LIB_EXT = "" # File extension of the shared library
ifeq ($(OS),Windows_NT)
	SHARED_LIB_EXT = dll
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		SHARED_LIB_EXT = so
	endif
	ifeq ($(UNAME_S),Darwin)
		SHARED_LIB_EXT = dylib
	endif
endif

all: build test

build: build-rust build-go

build-rust: build-rust-release

# Use debug build for quick testing.
# In order to use "--features backtraces" here we need a Rust nightly toolchain, which we don't have by default
build-rust-debug:
	(cd libwasmvm && cargo build)
	cp libwasmvm/target/debug/libwasmvm.$(SHARED_LIB_EXT) api
	make update-bindings

# use release build to actually ship - smaller and much faster
#
# See https://github.com/CosmWasm/wasmvm/issues/222#issuecomment-880616953 for two approaches to
# enable stripping through cargo (if that is desired).
build-rust-release:
	(cd libwasmvm && cargo build --release)
	cp libwasmvm/target/release/libwasmvm.$(SHARED_LIB_EXT) api
	make update-bindings
	@ #this pulls out ELF symbols, 80% size reduction!

build-go:
	go build ./...

test:
	RUST_BACKTRACE=1 go test -v ./api ./types .

test-safety:
	GODEBUG=cgocheck=2 go test -race -v -count 1 ./api

# Creates a release build in a containerized build environment of the static library for Alpine Linux (.a)
release-build-alpine:
	rm -rf libwasmvm/target/release
	# build the muslc *.a file
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX)-alpine
	cp libwasmvm/target/release/examples/libmuslc.a api/libwasmvm_muslc.a
	make update-bindings
	# try running go tests using this lib with muslc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/mnt/testrun -w /mnt/testrun $(ALPINE_TESTER) go build -tags muslc .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/mnt/testrun -w /mnt/testrun $(ALPINE_TESTER) go test -tags muslc ./api ./types

# Creates a release build in a containerized build environment of the shared library for glibc Linux (.so)
release-build-linux:
	rm -rf libwasmvm/target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX)-centos7
	cp libwasmvm/target/release/deps/libwasmvm.so api
	make update-bindings

# Creates a release build in a containerized build environment of the shared library for macOS (.dylib)
release-build-macos:
	rm -rf libwasmvm/target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX)-cross
	cp libwasmvm/target/x86_64-apple-darwin/release/deps/libwasmvm.dylib api
	cp libwasmvm/bindings.h api
	make update-bindings

update-bindings:
	# After we build libwasmvm, we have to copy the generated bindings for Go code to use.
	# We cannot use symlinks as those are not reliably resolved by `go get` (https://github.com/CosmWasm/wasmvm/pull/235).
	cp libwasmvm/bindings.h api

release-build:
	# Write like this because those must not run in parallel
	make release-build-alpine
	make release-build-linux
	make release-build-macos

test-alpine: release-build-alpine
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/mnt/testrun -w /mnt/testrun $(BUILDERS_PREFIX)-alpine go build -tags muslc -o demo ./cmd
	# run static binary in an alpine machines (not dlls)
	# See https://de.wikipedia.org/wiki/Alpine_Linux#Versionen for supported versions
	docker run --rm --read-only -v $(shell pwd):/mnt/testrun -w /mnt/testrun alpine:3.15 ./demo ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/mnt/testrun -w /mnt/testrun alpine:3.14 ./demo ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/mnt/testrun -w /mnt/testrun alpine:3.13 ./demo ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/mnt/testrun -w /mnt/testrun alpine:3.12 ./demo ./api/testdata/hackatom.wasm
	# run static binary locally if you are on Linux
	# ./muslc.exe ./api/testdata/hackatom.wasm
