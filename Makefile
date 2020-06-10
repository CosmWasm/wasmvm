.PHONY: all build build-rust build-go test docker-image docker-image-centos7 docker-image-cross

DOCKER_TAG := 0.8.1
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
	cargo build --release --features backtraces
	cp target/release/libgo_cosmwasm.$(DLL_EXT) api
	@ #this pulls out ELF symbols, 80% size reduction!

# implement stripping based on os
ifeq ($(DLL_EXT),so)
strip:
	strip api/libgo_cosmwasm.so
else
# TODO: add for windows and osx
strip:
endif

build-go:
	go build ./...

test:
	RUST_BACKTRACE=1 go test -v ./api ./types .

# we should build all the docker images locally ONCE and publish them
docker-image-centos7:
	docker build . -t cosmwasm/go-ext-builder:$(DOCKER_TAG)-centos7 -f ./Dockerfile.centos7

docker-image-cross:
	docker build . -t cosmwasm/go-ext-builder:$(DOCKER_TAG)-cross -f ./Dockerfile.cross

docker-image-alpine:
	docker build . -t cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine -f ./Dockerfile.alpine-rust
	docker build . -t cosmwasm/go-ext-tester:$(DOCKER_TAG)-alpine -f ./Dockerfile.alpine-go

docker-images: docker-image-centos7 docker-image-cross docker-image-alpine

docker-publish: docker-images
	docker push cosmwasm/go-ext-builder:$(DOCKER_TAG)-cross
	docker push cosmwasm/go-ext-builder:$(DOCKER_TAG)-centos7
	docker push cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine
	docker push cosmwasm/go-ext-tester:$(DOCKER_TAG)-alpine

# and use them to compile release builds
release:
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(DOCKER_TAG)-cross
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(DOCKER_TAG)-centos7

test-alpine:
	# build the muslc *.a file
	rm -rf target/release/examples
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine
	# try running go tests using this lib with muslc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-tester:$(DOCKER_TAG)-alpine go build -tags muslc .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-tester:$(DOCKER_TAG)-alpine go test -tags muslc ./api ./types
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-tester:$(DOCKER_TAG)-alpine go build -tags muslc -o muslc.exe ./cmd
	# run this binary in a simple alpine image
	docker run --rm -it -v $(shell pwd):/code -w /code alpine:3.12 ./muslc.exe ./api/testdata/hackatom.wasm
	# run static binary locally (not dlls)
	./muslc.exe ./api/testdata/hackatom.wasm
