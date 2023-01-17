#!/bin/bash
set -o errexit -o nounset -o pipefail

cargo build --release --target x86_64-pc-windows-gnu --example staticwasmvm
