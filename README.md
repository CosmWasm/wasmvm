# wasmvm

This is a wrapper around the [CosmWasm VM](https://github.com/CosmWasm/cosmwasm/tree/master/packages/vm).
It allows you to compile, initialize and execute CosmWasm smart contracts
from Go applications, in particular from [x/wasm](https://github.com/CosmWasm/wasmd/tree/master/x/wasm).

## Structure

This repo contains both Rust and Go code. The rust code is compiled into a dll/so
to be linked via cgo and wrapped with a pleasant Go API. The full build step
involves compiling rust -> C library, and linking that library to the Go code.
For ergonomics of the user, we will include pre-compiled libraries to easily
link with, and Go developers should just be able to import this directly.

## Supported Platforms

Since this package includes a rust prebuilt dll, you cannot just import the go code,
but need to be on a system that works with an existing dll. Currently this is Linux
(tested on Ubuntu, Debian, and CentOS7) and MacOS. We have a build system for Windows,
but it is not supported by the wasmer singlepass backend which we rely upon for gas
metering.

*Note: CentOS support is currently disabled due to work on CD tooling. We require Linux with glibc 2.18+*

*Note: Windows is not supported currently*

*Note: We only currently support i686/amd64 architectures, although AMD support is an open issue*

## Design

Please read the [Documentation](./spec/Index.md) to understand both the general
[Architecture](./spec/Architecture.md), as well as the more detailed
[Specification](./spec/Specification.md) of the parameters and entry points.

## Development

There are two halfs to this code - go and rust. The first step is to ensure that there is
a proper dll built for your platform. This should be `api/libwasmvm.X`, where X is:

- `so` for Linux systems
- `dylib` for MacOS
- `dll` for Windows - Not currently supported due to upstream dependency

If this is present, then `make test` will run the Go test suite and you can import this code freely.
If it is not present you will have to build it for your system, and ideally add it to this repo
with a PR (on your fork). We will set up a proper CI system for building these binaries,
but we are not there yet.

To build the rust side, try `make build-rust` and wait for it to compile. This depends on
`cargo` being installed with `rustc` version 1.39+. Generally, you can just use `rustup` to
install all this with no problems.

## Toolchain

We fix the Rust version in the CI and build containers, so the following should be in sync:

- `.circleci/config.yml`
- `builders/Dockerfile.*`

For development you should be able to use any reasonably up-to-date Rust stable.
