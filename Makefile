.PHONY: all build build-rust build-go test

BUILDERS_PREFIX := cosmwasm/go-ext-builder:0006
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

# Use debug build for quick testing.
# In order to use "--features backtraces" here we need a Rust nightly toolchain, which we don't have by default
build-rust-debug:
	cargo build
	cp target/debug/libwasmvm.$(DLL_EXT) api

# use release build to actually ship - smaller and much faster
build-rust-release:
	cargo build --release
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

# Creates a release build in a containerized build environment of the static library for Alpine Linux (.a)
release-build-alpine:
	rm -rf target/release
	# build the muslc *.a file
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code $(BUILDERS_PREFIX)-alpine
	# try running go tests using this lib with muslc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code $(BUILDERS_PREFIX)-alpine go build -tags muslc .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code $(BUILDERS_PREFIX)-alpine go test -tags muslc ./api ./types

# Creates a release build in a containerized build environment of the shared library for glibc Linux (.so)
release-build-linux:
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code $(BUILDERS_PREFIX)-centos7

# Creates a release build in a containerized build environment of the shared library for macOS (.dylib)
release-build-macos:
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code $(BUILDERS_PREFIX)-cross

release-build:
	# Write like this because those must not run in parallal
	make release-build-alpine
	make release-build-linux
	make release-build-macos

test-alpine: release-build-alpine
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code $(BUILDERS_PREFIX)-alpine go build -tags muslc -o muslc.exe ./cmd
	# run static binary in an alpine machines (not dlls)
	docker run --rm --read-only -v $(shell pwd):/code -w /code alpine:3.12 ./muslc.exe ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/code -w /code alpine:3.11 ./muslc.exe ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/code -w /code alpine:3.10 ./muslc.exe ./api/testdata/hackatom.wasm
	# run static binary locally if you are on Linux
	# ./muslc.exe ./api/testdata/hackatom.wasm
