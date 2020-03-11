.PHONY: all build build-rust build-go test docker-image docker-build docker-image-centos7

DOCKER_TAG := 0.7.0
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
	cp target/debug/libgo_cosmwasm.$(DLL_EXT) api

# use release build to actually ship - smaller and much faster
build-rust-release:
	RUSTFLAGS="-C target-feature=-crt-static -C link-args=-fPIC" cargo build --release --target x86_64-unknown-linux-musl
	cp target/release/libgo_cosmwasm.$(DLL_EXT) api

install-release-tools:
	sudo apt install musl-tools
	rustup target add x86_64-unknown-linux-musl
	rustup target add x86_64-unknown-linux-musl --toolchain=nightly


# implement stripping based on os
ifeq ($(DLL_EXT),so)
strip:
	@ #this pulls out ELF symbols, 80% size reduction!
	strip api/libgo_cosmwasm.so
else
# TODO: add for windows and osx
strip:
endif

build-go:
	go build ./...

test:
    # note: you must modify api/lib.go somehow after rebuilding the rust dll to relink it
	RUST_BACKTRACE=1 go test -v -count=1 ./api .

docker-image-centos7:
	docker build . -t go-cosmwasm:$(DOCKER_TAG)-centos7 -f ./Dockerfile.centos7

docker-image-cross:
	docker build . -t go-cosmwasm:$(DOCKER_TAG)-cross -f ./Dockerfile.cross

release: docker-image-cross docker-image-centos7
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code go-cosmwasm:$(DOCKER_TAG)-cross
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code go-cosmwasm:$(DOCKER_TAG)-centos7
