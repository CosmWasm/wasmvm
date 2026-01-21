# CHANGELOG

The format is based on [Keep a Changelog], and this project adheres to [Semantic Versioning].

## [Unreleased]

- Recover the CHANGELOG ([#708])
- Add workflow dispatch in CI ([#703])
- Improved tests ([#699])
- Update setup-go to v6 ([#697])
- Unified security policy ([#693])
- Updated year in NOTICE file. ([0xf2566c3])

[#708]: https://github.com/CosmWasm/wasmvm/issues/708
[#703]: https://github.com/CosmWasm/wasmvm/pull/703
[#699]: https://github.com/CosmWasm/wasmvm/pull/699
[#697]: https://github.com/CosmWasm/wasmvm/pull/697
[#693]: https://github.com/CosmWasm/wasmvm/pull/693
[0xf2566c3]: https://github.com/CosmWasm/wasmvm/commit/f2566c3ef91848d3bc29f184514797d43901c8c6

## [3.0.2] - 2025-08-26

### Changed

- Prepare for release v3.0.2 ([0x4da6e4d])
- trigger gh actions on tag ([0x96dbd73])

[0x4da6e4d]: https://github.com/CosmWasm/wasmvm/commit/4da6e4d9ea35fc5d6725dd2c076c6a87fb32ee0b
[0x96dbd73]: https://github.com/CosmWasm/wasmvm/commit/96dbd737cb9ac2bf9d617ce9b8f2a05b2db90bbc

## [3.0.1] - 2025-08-26

### Changed

- ci: bump actions/checkout to v5 ([#691])
- Migrate from CircleCI to GH Actions ([#689])
- Prepared version for v3.0.1 release ([0x99c5859])
- Add tag to ci flow ([0x53d26d4])

### Fixed

- Fix deploy to git step in CI ([#692])
- chore: fix minor typo in comment ([#690])
- fix link memdb.go ([#686])
- Update outdated Tendermint PR link in memdb.go Close() comment ([#685])
- fix ci ([0x06738d6])

[#692]: https://github.com/CosmWasm/wasmvm/pull/692
[#691]: https://github.com/CosmWasm/wasmvm/pull/691
[#690]: https://github.com/CosmWasm/wasmvm/pull/690
[#689]: https://github.com/CosmWasm/wasmvm/pull/689
[#686]: https://github.com/CosmWasm/wasmvm/pull/686
[#685]: https://github.com/CosmWasm/wasmvm/pull/685
[0x99c5859]: https://github.com/CosmWasm/wasmvm/commit/99c5859d9ddff5487b18ac3b3b4a1846f3fa17b7
[0x53d26d4]: https://github.com/CosmWasm/wasmvm/commit/53d26d43c566121205094497a4c67afe998ebded
[0x06738d6]: https://github.com/CosmWasm/wasmvm/commit/06738d68e3cd6a9c5dfc28da68ae03516af801b4

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
- Publish wasmvm v2.2.5-rc.1 ([0x52a38ad])
- Publish wasmvm v2.2.5-rc.2 ([0xc8f3ae1])
- Publish wasmvm v2.2.5 ([0xe5fc4d9])

[#707]: https://github.com/CosmWasm/wasmvm/pull/707
[#695]: https://github.com/CosmWasm/wasmvm/pull/695
[0xe5fc4d9]: https://github.com/CosmWasm/wasmvm/commit/e5fc4d9d957eebe021c53a9d1ac556ced9b22d5a
[0x52a38ad]: https://github.com/CosmWasm/wasmvm/commit/52a38ad99824953c6f27e0e07722353df349963b
[0xc8f3ae1]: https://github.com/CosmWasm/wasmvm/commit/c8f3ae1f2d7f7dcd675c08f79ee5165a6267ff82

## [2.2.4] - 2025-04-30

### Added

- Add ExpectedJSONSize (backport #635) ([#660])

### Changed

- Bump min Go version to 1.22 (backport #637) ([#661])
- Backport 2.2: improve panic messages when vm panicks ([#650])
- Set libwasmvm version to 2.2.4 ([0x67b6dc7])

[#661]: https://github.com/CosmWasm/wasmvm/pull/661
[#660]: https://github.com/CosmWasm/wasmvm/pull/660
[#650]: https://github.com/CosmWasm/wasmvm/pull/650
[0x67b6dc7]: https://github.com/CosmWasm/wasmvm/commit/67b6dc70f9d7364be3bd27de1bc0ece05263ef24

## [2.2.3] - 2025-03-05

### Changed

- Bump cosmwasm ([0x9859140])
- Set libwasmvm version to 2.2.3 ([0xdf886d2])

### Fixed

- Fixed tests ([0x4115e4b])

[0x4115e4b]: https://github.com/CosmWasm/wasmvm/commit/4115e4b37ac54744f49091b8d20c506c28d1a038
[0x9859140]: https://github.com/CosmWasm/wasmvm/commit/9859140917ff9d7f02b1453ca356b320bdfd5b29
[0xdf886d2]: https://github.com/CosmWasm/wasmvm/commit/df886d2568e841c4a2ab3bcc96ee0a2460d4ee33

## [2.2.2] - 2025-02-04

### Added

- Add typo check ([#581])

### Changed

- Lock cargo-audit CI job (backport #604) ([#608])
- Bump cosmwasm ([0x9b7e998])
- Set libwasmvm version to 2.2.2 ([0x6b8f8f4])

### Fixed

- Fix unchecked flag (backport #612) ([#613])
- Fix ([0x0aefa4c])
- Fix tests ([0x4fe4ee6])

[#613]: https://github.com/CosmWasm/wasmvm/pull/613
[#608]: https://github.com/CosmWasm/wasmvm/pull/608
[#581]: https://github.com/CosmWasm/wasmvm/pull/581
[0x9b7e998]: https://github.com/CosmWasm/wasmvm/commit/9b7e9983797d22219ed0ca1adfb37ace060b8e93
[0x0aefa4c]: https://github.com/CosmWasm/wasmvm/commit/0aefa4c378457aeb3c07e7975b875be38872c56d
[0x4fe4ee6]: https://github.com/CosmWasm/wasmvm/commit/4fe4ee62c7dcbe1ea8b668a12bae69dcbea929d6
[0x6b8f8f4]: https://github.com/CosmWasm/wasmvm/commit/6b8f8f43ea7d6b438ce5c904e7483c17c888f84f

## [2.2.1] - 2024-12-19

### Added

- Add SimulateStoreCode function on main ([#580])

### Changed

- Set libwasmvm version to 2.2.1 ([0xaf3791a])

[#580]: https://github.com/CosmWasm/wasmvm/pull/580
[0xaf3791a]: https://github.com/CosmWasm/wasmvm/commit/af3791a232a3bb3ccbe8d15830dd39f5637aef24

## [2.2.0] - 2024-12-17

### Added

- Conditional migrate calling ([#556])
- IBC Fees ([#545])
- Add SimulateStoreCode function ([0x319a1be])
- Add test for StoreCodeUnchecked ([0xb3c1e13])
- Add cosmwasm-vm config ([0xbc859c9])
- Add panic handler function ([0x7dd9fa5])
- Add comments ([0xca5f3d3])
- Add comments ([0xdec4c2b])

### Changed

- Finalize, push and use builders version 0101 ([#551])
- Update CosmWasm to 2.2.0 ([#549])
- Builders: bump debian image from Bullseye to Bookworm ([#533])
- Rename builders image go-ext-builder ([#364])
- Future of glibc support (aka. migrate from CentOS to Debian builders) ([#293])
- Bump to 2.2.0-rc.2 ([#562])
- Update to cosmwasm 2.2-rc.1 ([#561])
- Upgrade bytes to 1.7.1 ([#557])
- Merge updates from 2.1 branch into main ([#555])
- Bump to Rust 1.80 and other build system cleanups ([#552])
- Cleanup build commands in Makefile ([#550])
- Upgrade cbindgen to 0.27.0 ([#548])
- Bump CI Rust version ([0x11b5867])
- Bump cosmwasm ([0x888a468])
- Bump cosmwasm ([0x186d1df])
- Set builders version 0101 ([0x87b5cdf])
- Update builder to Rust 1.81 ([0xb7c1cbf])
- Improve Size type ([0x740a8c0])
- Bump cosmwasm rc ([0x010196c])
- Expose VMConfig constructor ([0xec3a0a7])
- Rename config fields ([0x8d3938d])
- Set libwasmvm version to 2.1.3 ([0xcd297b3])
- Set libwasmvm version to 2.1.4 ([0x19f01b5])
- Set libwasmvm version to 2.1.5 ([0x0c208b3])
- Set libwasmvm version to 2.1.6 ([0xba537a6])
- Set libwasmvm version to 2.2.0-rc.1 ([0x6fced70])
- Set libwasmvm version to 2.2.0-rc.2 ([0x0a2eab2])
- Set libwasmvm version to 2.2.0-rc.3 ([0x03abf89])
- Set libwasmvm version to 2.2.0 ([0x2fa12a9])
- Update cargo-audit ([0xf7f283e])
- Update to cosmwasm 2.1.2 ([0x1343023])
- Update cosmwasm to 2.1.3 ([0x5a99735])
- Update to cosmwasm 2.1.5 ([0x095b849])
- Update to cosmwasm 2.2 ([0x1ea7305])
- docs: Move `spec` to cosmwasm documentation ([0x0661bee])
- Update naming ([0x5510ac3])
- Bump wasmvm version ([0x7180b79])
- Set builder version to 0102 ([0x5bd7543])
- Update builder rust version to 1.82 ([0x484d39b])
- Bump cosmwasm ([0x43ebaaa])
- Bump wasmvm version ([0x539d83a])
- Remove unused import ([0xda987f6])
- Use locked dependencies for cargo-audit install ([0x755c1f3])
- Simplify NewVM ([0x7b88fd4])
- Move called function and error into handle_vm_panic ([0xb785826])
- Update CI libwasmvm_audit Rust version ([0xa5c3a79])
- Update test names ([0x48d8494])
- Improve docs ([0xd455a68])
- Use JSON for VMConfig ([0xbdc225e])
- Use JSON for VMConfig ([0x9846576])

### Fixed

- Fix ([0x8d44a28])
- Fix ([0x10f9281])
- Fix ([0x4c0d2ea])
- Fix unchecked flag ([0x577076b])
- Fix pinned metrics ([0x7250c10])
- Fix tests ([0x0e23081])
- Fix lints ([0xf5160b2])
- Fix tests ([0x58424a5])

[#551]: https://github.com/CosmWasm/wasmvm/issues/551
[#549]: https://github.com/CosmWasm/wasmvm/issues/549
[#533]: https://github.com/CosmWasm/wasmvm/issues/533
[#364]: https://github.com/CosmWasm/wasmvm/issues/364
[#293]: https://github.com/CosmWasm/wasmvm/issues/293
[#562]: https://github.com/CosmWasm/wasmvm/pull/562
[#561]: https://github.com/CosmWasm/wasmvm/pull/561
[#557]: https://github.com/CosmWasm/wasmvm/pull/557
[#556]: https://github.com/CosmWasm/wasmvm/pull/556
[#555]: https://github.com/CosmWasm/wasmvm/pull/555
[#552]: https://github.com/CosmWasm/wasmvm/pull/552
[#550]: https://github.com/CosmWasm/wasmvm/pull/550
[#548]: https://github.com/CosmWasm/wasmvm/pull/548
[#545]: https://github.com/CosmWasm/wasmvm/pull/545
[0x11b5867]: https://github.com/CosmWasm/wasmvm/commit/11b5867b1cc123a77656cbe0dfde3af2bcc28482
[0x888a468]: https://github.com/CosmWasm/wasmvm/commit/888a468dcf28f05e4d26cfd227cc09935127747b
[0x87b5cdf]: https://github.com/CosmWasm/wasmvm/commit/87b5cdf8c2297c8e03014baef9d6abf3b51d1d5f
[0x8d44a28]: https://github.com/CosmWasm/wasmvm/commit/8d44a286fabc793a2fba93752e58cd0fd5b88a2d
[0xb7c1cbf]: https://github.com/CosmWasm/wasmvm/commit/b7c1cbfb13e52a38e330cb186727d51a38077e23
[0x740a8c0]: https://github.com/CosmWasm/wasmvm/commit/740a8c079f48326007a090512c05275b43f9c76f
[0x319a1be]: https://github.com/CosmWasm/wasmvm/commit/319a1be2c6f67a97311523708f57216ee0fa63b3
[0x010196c]: https://github.com/CosmWasm/wasmvm/commit/010196c770bbebeceee15b0de09a109481a289ab
[0xec3a0a7]: https://github.com/CosmWasm/wasmvm/commit/ec3a0a79b4ab3709721fadb8f069b8e5094c6616
[0x10f9281]: https://github.com/CosmWasm/wasmvm/commit/10f9281db93f00eb3a0ca1de4013562979fe26f2
[0xb3c1e13]: https://github.com/CosmWasm/wasmvm/commit/b3c1e13c4c7a095e86094d2717364d571c1f8cc0
[0x8d3938d]: https://github.com/CosmWasm/wasmvm/commit/8d3938daeb45ffbadda30c35b1881123dbca8aae
[0x0c208b3]: https://github.com/CosmWasm/wasmvm/commit/0c208b39afa10146e519d2452f267d39fd249110
[0xf7f283e]: https://github.com/CosmWasm/wasmvm/commit/f7f283e073d467eb19044cf6b75b37ea69febffb
[0x6fced70]: https://github.com/CosmWasm/wasmvm/commit/6fced70305e07133bf3c49d32398d897273d27fc
[0x0a2eab2]: https://github.com/CosmWasm/wasmvm/commit/0a2eab23b6718c52f48514864f9450109ffa454b
[0x4c0d2ea]: https://github.com/CosmWasm/wasmvm/commit/4c0d2eab981c13a3cdb2489bca4b806947709012
[0xbc859c9]: https://github.com/CosmWasm/wasmvm/commit/bc859c9bff3be0073ac6014d52e900a8015f6cfa
[0x7dd9fa5]: https://github.com/CosmWasm/wasmvm/commit/7dd9fa5413eac6fdf7c545e3003b2426d0da9262
[0x19f01b5]: https://github.com/CosmWasm/wasmvm/commit/19f01b50424fb5bf763cc60f482ba8a8fca518b6
[0x186d1df]: https://github.com/CosmWasm/wasmvm/commit/186d1df83584d3c4262a9a0a67ca6dc562e05e39
[0x577076b]: https://github.com/CosmWasm/wasmvm/commit/577076b41e488b12f64a512d2badd5c0254142b1
[0x095b849]: https://github.com/CosmWasm/wasmvm/commit/095b849b4fd5098d43888647819c977ae3e48b79
[0xca5f3d3]: https://github.com/CosmWasm/wasmvm/commit/ca5f3d35a5543ed5ee83d8574dc5340c59f0413d
[0xba537a6]: https://github.com/CosmWasm/wasmvm/commit/ba537a6675377c0e0aa40b8957fc50f6edff97d8
[0x0661bee]: https://github.com/CosmWasm/wasmvm/commit/0661beec783268a7afe6cd1b8f7180a86e3426c1
[0x7250c10]: https://github.com/CosmWasm/wasmvm/commit/7250c10d739a42da0952d4af0cdaefdc8c66609d
[0x1ea7305]: https://github.com/CosmWasm/wasmvm/commit/1ea73050837f3fefa0f840e45e4fd2a7a7ad379c
[0x5510ac3]: https://github.com/CosmWasm/wasmvm/commit/5510ac3667814b2aa13d7c9657fe86dc89659d0b
[0xdec4c2b]: https://github.com/CosmWasm/wasmvm/commit/dec4c2bc6c2c765170d0c407af44bda23230d2d8
[0x03abf89]: https://github.com/CosmWasm/wasmvm/commit/03abf893710c2825d64325b9722cb0677fa1c6f2
[0x7180b79]: https://github.com/CosmWasm/wasmvm/commit/7180b79dfee9fb8f7ac32a1ea46555504f0ef823
[0x5bd7543]: https://github.com/CosmWasm/wasmvm/commit/5bd75430ec62baa2646224af61d24ba81d9c7340
[0x484d39b]: https://github.com/CosmWasm/wasmvm/commit/484d39b1193113c6b85db76169db6e7437e08a91
[0x43ebaaa]: https://github.com/CosmWasm/wasmvm/commit/43ebaaaa0e3d7b1c603a97d24ed1275030119ad9
[0x2fa12a9]: https://github.com/CosmWasm/wasmvm/commit/2fa12a984e8350e6c6a3612d44150b904a5926f0
[0x539d83a]: https://github.com/CosmWasm/wasmvm/commit/539d83a0261c46c3936652a27f838a8bb66c5315
[0xda987f6]: https://github.com/CosmWasm/wasmvm/commit/da987f606d38fbea4fdd508de884354e55e33198
[0x5a99735]: https://github.com/CosmWasm/wasmvm/commit/5a997355a42bbb24c85f7e9f8c117b40cc26f382
[0x755c1f3]: https://github.com/CosmWasm/wasmvm/commit/755c1f35b43fe189548f685a61bff08747d7aca3
[0xcd297b3]: https://github.com/CosmWasm/wasmvm/commit/cd297b3f70a57649bdbbc6a152a3f102ec93d48b
[0x0e23081]: https://github.com/CosmWasm/wasmvm/commit/0e230813db97422546ff3d8232293aec8b10a947
[0x7b88fd4]: https://github.com/CosmWasm/wasmvm/commit/7b88fd40101c4a78d52b8183aadf7b7bc38407fa
[0xb785826]: https://github.com/CosmWasm/wasmvm/commit/b785826785433c3d1537851b4e66996fa40915c3
[0xa5c3a79]: https://github.com/CosmWasm/wasmvm/commit/a5c3a795158daad6d015b5cb1fa6f473f1ac4ead
[0xf5160b2]: https://github.com/CosmWasm/wasmvm/commit/f5160b2884f306fb7263b7ff15a7b59168fd6c46
[0x48d8494]: https://github.com/CosmWasm/wasmvm/commit/48d849416e70f3c5f8df8691a845eb0999ff05b1
[0xd455a68]: https://github.com/CosmWasm/wasmvm/commit/d455a683c0619c170f35a3a020b3edfdc1710907
[0xbdc225e]: https://github.com/CosmWasm/wasmvm/commit/bdc225e1d3810aa6d4f3805838aaa29dd49de6c3
[0x58424a5]: https://github.com/CosmWasm/wasmvm/commit/58424a517e62bdb14c6c058f9e677ac77eedf8b3
[0x1343023]: https://github.com/CosmWasm/wasmvm/commit/1343023b07f3b502d1e249dc4fbc9808e8b4183d
[0x9846576]: https://github.com/CosmWasm/wasmvm/commit/9846576d00ad9d5168cf956a9fec887d7ad89788

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
