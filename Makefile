.PHONY: all build build-rust build-go test

all: build test

build: build-rust build-go

build-rust:
	cargo build --release
	cp target/release/libgo_cosmwasm.so api

build-go:
	go build .

test:
	go test -v ./api