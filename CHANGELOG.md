# CHANGELOG

The format is based on [Keep a Changelog], and this project adheres to [Semantic Versioning].

## [Unreleased]

- Recover the CHANGELOG ([#708])
- Add workflow dispatch in CI ([#703])
- Improved tests ([#699])
- Update setup-go to v6 ([#697])
- Unified security policy ([#693])

[#708]: https://github.com/CosmWasm/wasmvm/issues/708
[#703]: https://github.com/CosmWasm/wasmvm/pull/703
[#699]: https://github.com/CosmWasm/wasmvm/pull/699
[#697]: https://github.com/CosmWasm/wasmvm/pull/697
[#693]: https://github.com/CosmWasm/wasmvm/pull/693

## [3.0.2] - 2025-08-26

### Changed

- Upgraded CosmWasm dependencies to version **3.0.2**.

## [3.0.1] - 2025-08-26

### Changed

- ci: bump actions/checkout to v5 ([#691])
- Migrate from CircleCI to GH Actions ([#689])

### Fixed

- Fix deploy to git step in CI ([#692])
- chore: fix minor typo in comment ([#690])
- fix link memdb.go ([#686])
- Update outdated Tendermint PR link in memdb.go Close() comment ([#685])

[#692]: https://github.com/CosmWasm/wasmvm/pull/692
[#691]: https://github.com/CosmWasm/wasmvm/pull/691
[#690]: https://github.com/CosmWasm/wasmvm/pull/690
[#689]: https://github.com/CosmWasm/wasmvm/pull/689
[#686]: https://github.com/CosmWasm/wasmvm/pull/686
[#685]: https://github.com/CosmWasm/wasmvm/pull/685

## [3.0.0] - 2025-06-23

### Changed

- Use cosmwasm `main` branch ([#621])
- Add new EurekaMsg ([#617])
- Lock cargo-audit CI job ([#604])
- Update Go and golangci-lint version in CI ([#590])
- Linter pr 1: testifylint  ([#587])

### Fixed

- Docs fix spelling issues ([#618])
- Fix unchecked flag ([#612])

[#621]: https://github.com/CosmWasm/wasmvm/pull/621
[#618]: https://github.com/CosmWasm/wasmvm/pull/618
[#617]: https://github.com/CosmWasm/wasmvm/pull/617
[#612]: https://github.com/CosmWasm/wasmvm/pull/612
[#604]: https://github.com/CosmWasm/wasmvm/pull/604
[#590]: https://github.com/CosmWasm/wasmvm/pull/590
[#587]: https://github.com/CosmWasm/wasmvm/pull/587

## [2.3.1] - 2025-12-10

> [!NOTE]
> This release fixes broken _release process_ of version [2.3.0].
> No changes in code of version [2.3.0].

- Prepare release v2.3.1 ([#705])

[#705]: https://github.com/CosmWasm/wasmvm/pull/705

## [2.3.0] - 2025-12-10

### Added

- Add workflow dispatch ([#704])

### Changed

- Bump cosmwasm 2.3.0 ([#701])

### Fixed

- Minor fix in Cargo.toml ([#702])

[#704]: https://github.com/CosmWasm/wasmvm/pull/704
[#702]: https://github.com/CosmWasm/wasmvm/pull/702
[#701]: https://github.com/CosmWasm/wasmvm/pull/701

## [2.2.5] - 2025-12-19

### Changed

- Bump CW v2.2.3 ([#707])
- Backport "Replace circleci with gh actions" ([#695])

[#707]: https://github.com/CosmWasm/wasmvm/pull/707
[#695]: https://github.com/CosmWasm/wasmvm/pull/695

## [2.2.4] - 2025-04-30

### Added

- Add ExpectedJSONSize (backport #635) ([#660])
-
### Changed

- Bump min Go version to 1.22 (backport #637) ([#661])
- Backport 2.2: improve panic messages when vm panicks ([#650])

[#661]: https://github.com/CosmWasm/wasmvm/pull/661
[#660]: https://github.com/CosmWasm/wasmvm/pull/660
[#650]: https://github.com/CosmWasm/wasmvm/pull/650

## [2.2.3] - 2025-03-05

### Changed

- Bump cosmwasm ([0x985914])
- Set libwasmvm version to 2.2.3 ([0xdf886d])

### Fixed

- Fixed tests ([0x4115e4])

[0x4115e4]: https://github.com/CosmWasm/wasmvm/commit/4115e4b37ac54744f49091b8d20c506c28d1a038
[0x985914]: https://github.com/CosmWasm/wasmvm/commit/9859140917ff9d7f02b1453ca356b320bdfd5b29
[0xdf886d]: https://github.com/CosmWasm/wasmvm/commit/df886d2568e841c4a2ab3bcc96ee0a2460d4ee33

## [2.2.2] - 2025-02-04
## [2.2.1] - 2024-12-19
## [2.2.0] - 2024-12-17
## [2.1.6] - 2025-03-05
## [2.1.5] - 2025-02-04
## [2.1.4] - 2024-12-10
## [2.1.3] - 2024-09-23
## [2.1.2] - 2024-08-08
## [2.1.1] - 2024-08-08
## [2.1.0] - 2024-07-11
## [2.0.6] - 2025-02-04
## [2.0.5] - 2024-12-10
## [2.0.4] - 2024-09-23
## [2.0.3] - 2024-08-08
## [2.0.2] - 2024-08-08
## [2.0.1] - 2024-04-04
## [2.0.0] - 2024-03-12
## [1.5.9] - 2025-03-05
## [1.5.8] - 2025-02-04
## [1.5.7] - 2025-01-07
## [1.5.6] - 2024-12-10
## [1.5.5] - 2024-09-23
## [1.5.4] - 2024-08-08
## [1.5.3] - 2024-08-08
## [1.5.2] - 2024-01-18
## [1.5.1] - 2024-01-10
## [1.5.0] - 2023-10-31
## [1.4.3] - 2024-01-18
## [1.4.2] - 2024-01-10
## [1.4.1] - 2023-10-09
## [1.4.0] - 2023-09-04
## [1.3.1] - 2024-01-10
## [1.3.0] - 2023-07-17
## [1.2.6] - 2024-01-10
## [1.2.5] - 2024-01-10
## [1.2.4] - 2023-06-05
## [1.2.3] - 2023-04-18
## [1.2.2] - 2023-04-06
## [1.2.1] - 2023-03-08
## [1.2.0] - 2023-01-24
## [1.1.2] - 2023-04-18
## [1.1.1] - 2022-09-19
## [1.1.0] - 2022-09-06
## [1.0.1] - 2023-04-18
## [1.0.0] - 2022-05-16

[Unreleased]: https://github.com/CosmWasm/wasmvm/compare/v3.0.2...HEAD
[3.0.2]: https://github.com/CosmWasm/wasmvm/compare/v3.0.1...v3.0.2
[3.0.1]: https://github.com/CosmWasm/wasmvm/compare/v3.0.0...v3.0.1
[3.0.0]: https://github.com/CosmWasm/wasmvm/compare/v2.3.1...v3.0.0
[2.3.1]: https://github.com/CosmWasm/wasmvm/compare/v2.3.0...v2.3.1
[2.3.0]: https://github.com/CosmWasm/wasmvm/compare/v2.2.5...v2.3.0
[2.2.5]: https://github.com/CosmWasm/wasmvm/compare/v2.2.4...v2.2.5
[2.2.4]: https://github.com/CosmWasm/wasmvm/compare/v2.2.3...v2.2.4
[2.2.3]: https://github.com/CosmWasm/wasmvm/compare/v2.2.2...v2.2.3
[2.2.2]: https://github.com/CosmWasm/wasmvm/compare/v2.2.1...v2.2.2
[2.2.1]: https://github.com/CosmWasm/wasmvm/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/CosmWasm/wasmvm/compare/v2.1.6...v2.2.0
[2.1.6]: https://github.com/CosmWasm/wasmvm/compare/v2.1.5...v2.1.6
[2.1.5]: https://github.com/CosmWasm/wasmvm/compare/v2.1.4...v2.1.5
[2.1.4]: https://github.com/CosmWasm/wasmvm/compare/v2.1.3...v2.1.4
[2.1.3]: https://github.com/CosmWasm/wasmvm/compare/v2.1.2...v2.1.3
[2.1.2]: https://github.com/CosmWasm/wasmvm/compare/v2.1.1...v2.1.2
[2.1.1]: https://github.com/CosmWasm/wasmvm/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/CosmWasm/wasmvm/compare/v2.0.6...v2.1.0
[2.0.6]: https://github.com/CosmWasm/wasmvm/compare/v2.0.5...v2.0.6
[2.0.5]: https://github.com/CosmWasm/wasmvm/compare/v2.0.4...v2.0.5
[2.0.4]: https://github.com/CosmWasm/wasmvm/compare/v2.0.3...v2.0.4
[2.0.3]: https://github.com/CosmWasm/wasmvm/compare/v2.0.2...v2.0.3
[2.0.2]: https://github.com/CosmWasm/wasmvm/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/CosmWasm/wasmvm/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/CosmWasm/wasmvm/compare/v1.5.9...v2.0.0
[1.5.9]: https://github.com/CosmWasm/wasmvm/compare/v1.5.8...v1.5.9
[1.5.8]: https://github.com/CosmWasm/wasmvm/compare/v1.5.7...v1.5.8
[1.5.7]: https://github.com/CosmWasm/wasmvm/compare/v1.5.6...v1.5.7
[1.5.6]: https://github.com/CosmWasm/wasmvm/compare/v1.5.5...v1.5.6
[1.5.5]: https://github.com/CosmWasm/wasmvm/compare/v1.5.4...v1.5.5
[1.5.4]: https://github.com/CosmWasm/wasmvm/compare/v1.5.3...v1.5.4
[1.5.3]: https://github.com/CosmWasm/wasmvm/compare/v1.5.2...v1.5.3
[1.5.2]: https://github.com/CosmWasm/wasmvm/compare/v1.5.1...v1.5.2
[1.5.1]: https://github.com/CosmWasm/wasmvm/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/CosmWasm/wasmvm/compare/v1.4.3...v1.5.0
[1.4.3]: https://github.com/CosmWasm/wasmvm/compare/v1.4.2...v1.4.3
[1.4.2]: https://github.com/CosmWasm/wasmvm/compare/v1.4.1...v1.4.2
[1.4.1]: https://github.com/CosmWasm/wasmvm/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/CosmWasm/wasmvm/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/CosmWasm/wasmvm/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/CosmWasm/wasmvm/compare/v1.2.6...v1.3.0
[1.2.6]: https://github.com/CosmWasm/wasmvm/compare/v1.2.5...v1.2.6
[1.2.5]: https://github.com/CosmWasm/wasmvm/compare/v1.2.4...v1.2.5
[1.2.4]: https://github.com/CosmWasm/wasmvm/compare/v1.2.3...v1.2.4
[1.2.3]: https://github.com/CosmWasm/wasmvm/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/CosmWasm/wasmvm/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/CosmWasm/wasmvm/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/CosmWasm/wasmvm/compare/v1.1.2...v1.2.0
[1.1.2]: https://github.com/CosmWasm/wasmvm/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/CosmWasm/wasmvm/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/CosmWasm/wasmvm/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/CosmWasm/wasmvm/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/CosmWasm/wasmvm/compare/v0.16.7...v1.0.0

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
