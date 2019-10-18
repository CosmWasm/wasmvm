# Go-CosmWasm

This provides go bindings to the [cosmwasm](https://github.com/confio/cosmwasm) smart
contract framework. In particular, it allows you to easily compile, initialize,
and execute these contracts from Go.

## Structure

This repo contains both Rust and Go code. The rust code is compiled into a dll/so
to be linked via cgo and wrapped with a pleasant Go API. The full build step
involves compiling rust -> C library, and linking that library to the Go code.
For ergonomics of the user, we will include pre-compiled libraries to easily
link with, and Go developers should just be able to import this directly.

## Design

Please read the [Documentation](./spec/Index.md) to understand both the general
[Architecture](./spec/Architecture.md), as well as the more detailed 
[Specification](./spec/Specification.md) of the parameters and entry points.