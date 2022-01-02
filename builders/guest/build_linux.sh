#!/bin/bash
set -o errexit -o nounset -o pipefail

# See https://github.com/CosmWasm/wasmvm/issues/222#issuecomment-880616953 for two approaches to
# enable stripping through cargo (if that is desired).

cargo build --release
