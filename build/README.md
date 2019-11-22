# Cross Compilation Scripts

As this library is targetting go developers, we cannot assume a properly set up
rust environment on their system. Further, when importing this library, there is no
clean way to add a `libgo_cosmwasm.{so,dll,dylib}`. It needs to be committed with the
tagged (go) release in order to be easily usable.

The solution is to precompile the rust code into libraries for the major platforms 
(Linux, Windows, MacOS) and commit them to the repository at each tagged release.
This should be doable from one host machine, but is a bit tricky. This folder 
contains build scripts and a Docker image to create all dynamic libraries from one
host. In general this is set up for a Linux host, but any machine that can run Docker
can do the cross-compilation.

## Usage

1. Set `SOURCE_DIR` to the directory where `Cargo.toml` is located (likely `$(pwd)/..`).
1. Set `BUILD_DIR` to some empty directory we can use for build output (eg. `$HOME/go-out`)
1. `docker run --rm --mount type=bind,src=$SOURCE_DIR,dst=/code --mount type=bind,src=$BUILD_DIR,dst=/code/target confio/go-cosmwasm:latest`

Then look inside `$OUTDIR` and you should see:

**TODO** locate where the files are

**TODO** copy to final location

## Development and Testing

* `docker build . -t confio/go-cosmwasm:latest`
* `docker run -it --rm --mount type=bind,src=$SOURCE_DIR,dst=/code --mount type=bind,src=$BUILD_DIR,dst=/code/target confio/go-cosmwasm:latest /bin/bash`
