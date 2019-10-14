.PHONY: all build build-rust build-go test

all: build test

build: build-rust build-go

build-rust:
	cargo build --release
	cp target/release/libgo_cosmwasm.so api
	# this pulls out ELF symbols, 80% size reduction!
	strip api/libgo_cosmwasm.so

build-go:
	go build .

test:
	go test -v ./api