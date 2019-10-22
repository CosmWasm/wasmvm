.PHONY: all build build-rust build-go test

all: build test

build: build-rust build-go

build-rust: build-rust-debug

# use debug build for quick testing
build-rust-debug:
	cargo build
	cp target/debug/libgo_cosmwasm.so api

# use release build to actually ship - smaller and much faster
build-rust-release:
	cargo build --release
	cp target/release/libgo_cosmwasm.so api
	@ #this pulls out ELF symbols, 80% size reduction!
	strip api/libgo_cosmwasm.so

build-go:
	go build .

test:
	go test -v ./api