# CHANGELOG

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add Submessages and Subcall types ([#197])
- Add WasmMsg::Migrate message type ([#197])
- Add IBC and Stargate message types ([#167], [#174])
- Expose IBC entry points and AnalyzeCode ([#167], [#174])

[#167]: https://github.com/CosmWasm/wasmvm/pull/167
[#174]: https://github.com/CosmWasm/wasmvm/pull/174
[#197]: https://github.com/CosmWasm/wasmvm/pull/197

### Changed

- Renamed the Go type `CodeID` to `Checksum` to clarify the difference between
  the numeric code ID assigned by x/wasm and the hash used to identify it in the cache.
- Update required Rust version in build scripts to 1.50 ([#197])

## 0.13.0

This is a baseline... no CHANGELOG was maintained until this point
