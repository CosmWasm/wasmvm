# Fuzzing wasmvm

This directory contains fuzzing tests for both Go and Rust parts of wasmvm.

## Go Fuzzing

The Go fuzzing tests use Go's built-in fuzzing engine. To run them:

```bash
# Navigate to the go fuzz directory
cd fuzz/go

# Basic fuzzing (short duration, single thread)
go test -fuzz=FuzzStoreCode -fuzztime=1m

# More intensive fuzzing (longer duration, multiple threads)
go test -fuzz=FuzzStoreCode -fuzztime=10m -parallel=4

# Extended fuzzing (for thorough testing)
go test -fuzz=FuzzStoreCode -fuzztime=1h -parallel=8 -fuzzminimizetime=10s

# Run any of the other available fuzzers with similar options
go test -fuzz=FuzzContractExecution -fuzztime=1m
go test -fuzz=FuzzQuery -fuzztime=1m
go test -fuzz=FuzzPinUnpin -fuzztime=1m -parallel=4
go test -fuzz=FuzzCodeManagement -fuzztime=1m -parallel=4
go test -fuzz=FuzzGasMetering -fuzztime=1m -parallel=4
go test -fuzz=FuzzMetadata -fuzztime=1m -parallel=4
go test -fuzz=FuzzMultiContract -fuzztime=1m -parallel=4
```

You can adjust `-fuzztime` to control how long the fuzzer runs and `-parallel` to control how many worker threads to use. The `-fuzzminimizetime` flag controls how long the fuzzer spends minimizing a crash input.

### Available Go Fuzzers

Basic functionality:
- `FuzzStoreCode`: Tests storing and validating WASM code
- `FuzzContractExecution`: Tests contract instantiation and execution with varying inputs
- `FuzzQuery`: Tests contract queries with varying inputs

Advanced functionality:
- `FuzzPinUnpin`: Tests contract pinning and unpinning in the cache with complex patterns
- `FuzzCodeManagement`: Tests code storage, retrieval, and removal operations
- `FuzzGasMetering`: Tests gas measurement and limits with various sequences of operations
- `FuzzMetadata`: Tests contract analysis and metrics retrieval
- `FuzzMultiContract`: Tests complex interactions between multiple contract instances

## Rust Fuzzing

The Rust fuzzing tests use cargo-fuzz with libFuzzer. To run them, you'll first need to install cargo-fuzz:

```bash
cargo install cargo-fuzz
```

Then, you can run the fuzzers:

```bash
# Navigate to the rust fuzz directory
cd fuzz/rust

# Basic fuzzing (limited runs)
cargo fuzz run store_code -- -max_len=10240 -runs=10000

# More intensive fuzzing (more runs, multiple jobs)
cargo fuzz run store_code -- -max_len=10240 -runs=100000 -jobs=4

# Extended fuzzing (continuous, with memory limit)
cargo fuzz run store_code -- -max_len=10240 -jobs=8 -rss_limit_mb=4096

# Run any of the available fuzzers with similar options
cargo fuzz run instantiate -- -max_len=10240 -runs=10000
cargo fuzz run execute -- -max_len=10240 -runs=10000
cargo fuzz run query -- -max_len=10240 -runs=10000
cargo fuzz run pin_unpin -- -max_len=10240 -runs=10000
cargo fuzz run migrate -- -max_len=10240 -runs=10000
cargo fuzz run ibc_operations -- -max_len=10240 -runs=10000
cargo fuzz run gas_metering -- -max_len=10240 -runs=10000
cargo fuzz run multi_contract -- -max_len=10240 -runs=10000
```

You can adjust the options for more intensive fuzzing:

- `-runs=N`: Controls how many inputs to test (omit for continuous fuzzing)
- `-jobs=N`: Controls how many parallel jobs to run
- `-max_len=N`: Maximum length of generated inputs
- `-rss_limit_mb=N`: Memory limit in MB (prevents excessive memory usage)

### Available Rust Fuzzers

Basic functionality:
- `store_code`: Tests the store_code functionality in the Rust VM
- `instantiate`: Tests contract instantiation with varying inputs
- `execute`: Tests contract execution with varying inputs
- `query`: Tests contract queries with varying inputs

Advanced functionality:
- `pin_unpin`: Tests contract pinning and unpinning in the cache
- `migrate`: Tests contract migrations from one code version to another
- `ibc_operations`: Tests IBC-related operations like channel open/connect/close and packet receive
- `gas_metering`: Tests gas measurement and limits by varying gas limits and operations
- `multi_contract`: Tests interactions between multiple contract instances

## Continuous Fuzzing in CI

To integrate these fuzzers into a CI pipeline, you can set up a workflow that runs each fuzzer for a limited time (e.g., 15 minutes). This way, you can catch regressions early.

### Sample Workflow

You can add steps in your CI workflow to run the fuzzers:

```yaml
- name: Run Go fuzzers (matrix strategy)
  strategy:
    matrix:
      fuzzer: [FuzzStoreCode, FuzzContractExecution, FuzzQuery, FuzzPinUnpin, 
               FuzzCodeManagement, FuzzGasMetering, FuzzMetadata, FuzzMultiContract]
  run: |
    cd fuzz/go
    go test -fuzz=${{ matrix.fuzzer }} -fuzztime=15m -parallel=4

- name: Run Rust fuzzers (matrix strategy)
  strategy:
    matrix:
      fuzzer: [store_code, instantiate, execute, query, pin_unpin, 
               migrate, ibc_operations, gas_metering, multi_contract]
  run: |
    cd fuzz/rust
    cargo fuzz build
    cargo fuzz run ${{ matrix.fuzzer }} -- -max_len=10240 -runs=15000 -jobs=4
```

## Crash Reproduction

### Go

If a Go fuzzer finds a crash, it will save a file in the `testdata/fuzz/<FuzzerName>` directory. You can reproduce the crash with:

```bash
cd fuzz/go
go test -run=<FuzzerName>/$(basename crashfile)
```

### Rust

If a Rust fuzzer finds a crash, it will save a file in the `fuzz/artifacts/<fuzzer_name>` directory. You can reproduce the crash with:

```bash
cd fuzz/rust
cargo fuzz run <fuzzer_name> path/to/crashfile
```

## Recommended Fuzzing Strategy

For the most comprehensive fuzzing coverage:

1. **Basic Fuzzing**: Run all fuzzers with default settings during development for quick feedback
2. **Intensive Fuzzing**: Run with longer durations and parallel processing before major releases
3. **Continuous Fuzzing**: Set up CI jobs to run fuzzers regularly with reasonable time limits
4. **Targeted Fuzzing**: After code changes, focus on the specific fuzzers related to the changes 