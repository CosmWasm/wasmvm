# wasmvm

This is a wrapper around the [CosmWasm VM](https://github.com/CosmWasm/cosmwasm/tree/main/packages/vm).
It allows you to compile, initialize and execute CosmWasm smart contracts
from Go applications, in particular from [x/wasm](https://github.com/CosmWasm/wasmd/tree/master/x/wasm).

## Structure

This repo contains both Rust and Go code. The rust code is compiled into a dll/so
to be linked via cgo and wrapped with a pleasant Go API. The full build step
involves compiling rust -> C library, and linking that library to the Go code.
For ergonomics of the user, we will include pre-compiled libraries to easily
link with, and Go developers should just be able to import this directly.

## Supported Platforms

Requires Rust 1.51+, Requires Go 1.15+

Since this package includes a rust prebuilt dll, you cannot just import the go code,
but need to be on a system that works with an existing dll. Currently this is Linux
(tested on Ubuntu, Debian, and CentOS7) and MacOS. We have a build system for Windows,
but it is [not supported][wasmer_support] by the Wasmer Singlepass backend which we rely upon.

[wasmer_support]: https://docs.wasmer.io/ecosystem/wasmer/wasmer-features

### Overview

|               | [x86]               | [x86_64]            | [ARM32]              | [ARM64]              |
| ------------- | ------------------- | ------------------- | -------------------- | -------------------- |
| Linux (glibc) | ❌‍                 | ✅                  | ❌‍ <sub>[#53]</sub> | ❌‍ <sub>[#53]</sub> |
| Linux (muslc) | ❌‍                 | ✅                  | ❌‍ <sub>[#53]</sub> | ❌‍ <sub>[#53]</sub> |
| macOS         | ❌‍                 | ✅                  | ❌‍ <sub>[#53]</sub> | ❌‍ <sub>[#53]</sub> |
| Windows       | ❌ <sub>[#28]</sub> | ❌ <sub>[#28]</sub> | ❌ <sub>[#28]</sub>  | ❌ <sub>[#28]</sub>  |

[x86]: https://en.wikipedia.org/wiki/X86
[x86_64]: https://en.wikipedia.org/wiki/X86-64
[arm32]: https://en.wikipedia.org/wiki/AArch32
[arm64]: https://en.wikipedia.org/wiki/AArch64
[#28]: https://github.com/CosmWasm/wasmvm/issues/28
[#53]: https://github.com/CosmWasm/wasmvm/issues/53

✅ Supported and activly maintained.

❌ Blocked by external dependency.

🤷‍ Not supported because nobody cares so far. Feel free to look into it.

This is all blocked on [wasmer support for singlepass backend](https://docs.wasmer.io/ecosystem/wasmer/wasmer-features#compiler-support-by-chipset).
We can only move on these wasmvm issues when the upstream has support.

## Docs

Run `(cd libwasmvm && cargo doc --no-deps --open)`.

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
`cargo` being installed with `rustc` version 1.47+. Generally, you can just use `rustup` to
install all this with no problems.

## Toolchain

We fix the Rust version in the CI and build containers, so the following should be in sync:

- `.circleci/config.yml`
- `builders/Dockerfile.*`

For development you should be able to use any reasonably up-to-date Rust stable.
