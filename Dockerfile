# FROM rust:1.39
# We need nightly currently for compiling with singlepass
FROM rustlang/rust:nightly

# Install build dependencies
RUN apt-get update
RUN apt install -y clang gcc g++ zlib1g-dev libmpc-dev libmpfr-dev libgmp-dev
RUN apt install -y build-essential cmake

# add some llvm configs for later - how to cross-compile this in wasmer-llvm-backend???
# RUN echo deb http://deb.debian.org/debian buster-backports main >> /etc/apt/sources.list
# RUN apt-get update
# RUN apt-get install libllvm8 llvm-8 llvm-8-dev llvm-8-runtime
# ENV LLVM_SYS_80_PREFIX=/usr/lib/llvm-8

WORKDIR /root

# Add macOS Rust target
RUN rustup target add x86_64-apple-darwin

# Build osxcross
RUN git clone https://github.com/tpoechtrager/osxcross
RUN cd osxcross && \
	wget -nc https://s3.dockerproject.org/darwin/v2/MacOSX10.10.sdk.tar.xz && \
    mv MacOSX10.10.sdk.tar.xz tarballs/ && \
    UNATTENDED=yes OSX_VERSION_MIN=10.7 ./build.sh

# pre-fetch many deps
WORKDIR /scratch
COPY Cargo.toml /scratch/
COPY src /scratch/src
RUN cargo fetch

## TODO: add support for windows cross-compile

WORKDIR /code
RUN rm -rf /scratch

COPY build/build_linux.sh /root
COPY build/build_osx.sh /root
COPY build/build.sh /root

CMD ["/root/build.sh"]