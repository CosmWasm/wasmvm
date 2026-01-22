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

### Changed

- Set libwasmvm version to 2.1.6 ([0xba537a6])
- Bump cosmwasm ([0x43ebaaa])

### Fixed

- Fix tests ([0x0e23081])

[0xba537a6]: https://github.com/CosmWasm/wasmvm/commit/ba537a6675377c0e0aa40b8957fc50f6edff97d8
[0x43ebaaa]: https://github.com/CosmWasm/wasmvm/commit/43ebaaaa0e3d7b1c603a97d24ed1275030119ad9
[0x0e23081]: https://github.com/CosmWasm/wasmvm/commit/0e230813db97422546ff3d8232293aec8b10a947

## [2.1.5] - 2025-02-04

### Changed

- Lock cargo-audit CI job (backport #604) ([#607])
- Bump cosmwasm ([0x888a468])
- Set libwasmvm version to 2.1.5 ([0x0c208b3])

### Fixed

- Fix unchecked flag (backport #612) ([#616])
- Fix ([0x8d44a28])
- Fix tests ([0x58424a5])

[#616]: https://github.com/CosmWasm/wasmvm/pull/616
[#607]: https://github.com/CosmWasm/wasmvm/pull/607
[0x8d44a28]: https://github.com/CosmWasm/wasmvm/commit/8d44a286fabc793a2fba93752e58cd0fd5b88a2d
[0x58424a5]: https://github.com/CosmWasm/wasmvm/commit/58424a517e62bdb14c6c058f9e677ac77eedf8b3
[0x888a468]: https://github.com/CosmWasm/wasmvm/commit/888a468dcf28f05e4d26cfd227cc09935127747b
[0x0c208b3]: https://github.com/CosmWasm/wasmvm/commit/0c208b39afa10146e519d2452f267d39fd249110

## [2.1.4] - 2024-12-10

### Added

- Add SimulateStoreCode function ([0x319a1be])

### Changed

- Update to cosmwasm 2.1.5 ([0x095b849])
- Bump CI Rust version ([0x11b5867])
- Set libwasmvm version to 2.1.4 ([0x19f01b5])

### Fixed

- Fix pointer problem in UnmanagedVector (backport #571) ([#574])

[#574]: https://github.com/CosmWasm/wasmvm/pull/574
[0x095b849]: https://github.com/CosmWasm/wasmvm/commit/095b849b4fd5098d43888647819c977ae3e48b79
[0x319a1be]: https://github.com/CosmWasm/wasmvm/commit/319a1be2c6f67a97311523708f57216ee0fa63b3
[0x11b5867]: https://github.com/CosmWasm/wasmvm/commit/11b5867b1cc123a77656cbe0dfde3af2bcc28482
[0x19f01b5]: https://github.com/CosmWasm/wasmvm/commit/19f01b50424fb5bf763cc60f482ba8a8fca518b6

## [2.1.3] - 2024-09-23

### Changed

- Set libwasmvm version to 2.1.3 ([0xcd297b3])
- Bump cosmwasm ([0x186d1df])

[0xcd297b3]: https://github.com/CosmWasm/wasmvm/commit/cd297b3f70a57649bdbbc6a152a3f102ec93d48b
[0x186d1df]: https://github.com/CosmWasm/wasmvm/commit/186d1df83584d3c4262a9a0a67ca6dc562e05e39

## [2.1.2] - 2024-08-08

### Changed
- Update cosmwasm to 2.1.3 ([0x5a99735])
- Bump wasmvm version ([0x7180b79])

[0x5a99735]: https://github.com/CosmWasm/wasmvm/commit/5a997355a42bbb24c85f7e9f8c117b40cc26f382
[0x7180b79]: https://github.com/CosmWasm/wasmvm/commit/7180b79dfee9fb8f7ac32a1ea46555504f0ef823

## [2.1.1] - 2024-08-08

### Added

- Create Debian builder image and use for GNU linux .so files ([#439])

### Changed

- Upgrade clippy to 1.80.0 ([#547])
- Update comments on stripping ([#546])
- Update to cosmwasm 2.1.2 ([0x1343023])
- Bump wasmvm version ([0x539d83a])
- Update calling convention of builders to 0100 ([0xf3df522])

### Removed

- Remove unused Dockerfile.centos7 ([0xb9a3dd9])

### Fixed

- Fixup builders 0100 ([0x23be57a])

[#547]: https://github.com/CosmWasm/wasmvm/pull/547
[#546]: https://github.com/CosmWasm/wasmvm/pull/546
[#439]: https://github.com/CosmWasm/wasmvm/pull/439
[0xb9a3dd9]: https://github.com/CosmWasm/wasmvm/commit/b9a3dd9c1c225c487ee2c18e14f8e540db5f3b55
[0x1343023]: https://github.com/CosmWasm/wasmvm/commit/1343023b07f3b502d1e249dc4fbc9808e8b4183d
[0x539d83a]: https://github.com/CosmWasm/wasmvm/commit/539d83a0261c46c3936652a27f838a8bb66c5315
[0xf3df522]: https://github.com/CosmWasm/wasmvm/commit/f3df522ea6d0d04252668ac0926e8bd3b6e645cf
[0x23be57a]: https://github.com/CosmWasm/wasmvm/commit/23be57a0fc19beb528b2320c8fc50b49af6b1e53

## [2.1.0] - 2024-07-11

### Added

- Add bindings for migrate version ([#539])
- Add compile time type assertions for `hasSubMessages` interface ([#537])
- Add bindings for the pinned metrics ([#526])
- Add migrating entry for static linking ([#522])
- IBC Callbacks ([#520])
- Add SimulateStoreCode function ([0xde68126])

### Changed

- Merge 2.0.1 ([#535])
- Migrate to Rust 1.74.0+ in cross compiler ([#513])
- Make test-alpine work on ARM as well ([#483])
- Lock cargo-audit CI job (backport #604) ([#606])
- Expose pinned metrics through vm ([#544])
- Async Ack message type ([#542])
- Avoid checking `errOut.is_none` for unused errOut ([#541])
- Contract Migrate Version ([#540])
- Increase min rust version to 1.74.0 ([#538])
- Bump OSX_VERSION_MIN to 10.15 and Rust to 1.77.0 (builders 0019) ([#529])
- Document ​libwasmvmstatic_darwin.a support ([#528])
- imp: allow cgo while disabling libwasmvm linking ([#527])
- Merge 2.0 ([#525])
- Check goimports in golangci-lint ([#521])
- Updated README link ([#518])
- Refactor IteratorReference ([#501])
- Bump cosmwasm ([0xd62c3b8])
- Set libwasmvm version to 2.1.0-rc.1 ([0x6f5c9c9])
- Bump cosmwasm ([0xf092b7c])
- Set libwasmvm version to 2.0.5 ([0xc6dad83])
- Set libwasmvm version to 2.0.6 ([0x54521e5])
- Set libwasmvm version to 2.1.0-rc.2 ([0xe1c2e4e])
- Set libwasmvm version to 2.1.0 ([0xccf6865])
- Update time crate ([0xd7cb567])
- Bump wasmvm version ([0xdebc2dd])
- Ignore .DS_Store ([0x57bba20])
- Update to cosmwasm 2.0.5 ([0xe4d9884])
- Update cosmwasm to 2.0.6 ([0x0cb1ef2])
- Update to cosmwasm 2.0.8 ([0x68f94f2])
- Bump wasmvm version ([0x98ba855])
- Set libwasmvm version to 2.0.4 ([0xff1eb7c])
- Merge branch 'release/2.0 ([0x4094e65])

### Removed

- Remove x86 requirement for test-alpine job ([0x0a69b95])

### Fixed

- Broken optimised wasmd build ([#536])
- Crash "SIGABRT: abort"/"signal arrived during cgo execution" during store code on Alpine 3.19 ([#523])
- Fix unchecked flag (backport #612) ([#615])
- chore: fix duplicate word repetition in CreateChecksum error return ([#531])
- Fix `errorWithMessage` ([#543])
- Fix tests ([0x22e6892])
- Fix pointer problem in UnmanagedVector (backport #571) ([#573])
- Fix ([0xd4ff2ad])
- Fix tests ([0x956daaa])

[#539]: https://github.com/CosmWasm/wasmvm/issues/539
[#536]: https://github.com/CosmWasm/wasmvm/issues/536
[#523]: https://github.com/CosmWasm/wasmvm/issues/523
[#513]: https://github.com/CosmWasm/wasmvm/issues/513
[#483]: https://github.com/CosmWasm/wasmvm/issues/483
[#615]: https://github.com/CosmWasm/wasmvm/pull/615
[#606]: https://github.com/CosmWasm/wasmvm/pull/606
[#573]: https://github.com/CosmWasm/wasmvm/pull/573
[#544]: https://github.com/CosmWasm/wasmvm/pull/544
[#543]: https://github.com/CosmWasm/wasmvm/pull/543
[#542]: https://github.com/CosmWasm/wasmvm/pull/542
[#541]: https://github.com/CosmWasm/wasmvm/pull/541
[#540]: https://github.com/CosmWasm/wasmvm/pull/540
[#538]: https://github.com/CosmWasm/wasmvm/pull/538
[#537]: https://github.com/CosmWasm/wasmvm/pull/537
[#535]: https://github.com/CosmWasm/wasmvm/pull/535
[#531]: https://github.com/CosmWasm/wasmvm/pull/531
[#529]: https://github.com/CosmWasm/wasmvm/pull/529
[#528]: https://github.com/CosmWasm/wasmvm/pull/528
[#527]: https://github.com/CosmWasm/wasmvm/pull/527
[#526]: https://github.com/CosmWasm/wasmvm/pull/526
[#525]: https://github.com/CosmWasm/wasmvm/pull/525
[#522]: https://github.com/CosmWasm/wasmvm/pull/522
[#521]: https://github.com/CosmWasm/wasmvm/pull/521
[#520]: https://github.com/CosmWasm/wasmvm/pull/520
[#518]: https://github.com/CosmWasm/wasmvm/pull/518
[#501]: https://github.com/CosmWasm/wasmvm/pull/501
[0xd62c3b8]: https://github.com/CosmWasm/wasmvm/commit/d62c3b826a9d5a279149951b20f9ee9b5c8550a6
[0x22e6892]: https://github.com/CosmWasm/wasmvm/commit/22e689281084876bf6f7e12ea55e9f6c80391e9d
[0x6f5c9c9]: https://github.com/CosmWasm/wasmvm/commit/6f5c9c9f920c726853da5d1692bbfa6f4a9e0778
[0xf092b7c]: https://github.com/CosmWasm/wasmvm/commit/f092b7c336979972f5fe2f27a50750a974eeccc7
[0xde68126]: https://github.com/CosmWasm/wasmvm/commit/de68126114c2511c6b55b1030153cc95f10a146a
[0xd4ff2ad]: https://github.com/CosmWasm/wasmvm/commit/d4ff2adee44e6b9f7415a5dfbb3de745ab9b7678
[0x68f94f2]: https://github.com/CosmWasm/wasmvm/commit/68f94f25e6ddee2f11784332a2a24f2713ceeda2
[0xc6dad83]: https://github.com/CosmWasm/wasmvm/commit/c6dad83162c367366a3b23cc6371a944b1abc9e2
[0xccf6865]: https://github.com/CosmWasm/wasmvm/commit/ccf6865975db35ee5fc4c90380c39a5cfc4ba338
[0xd7cb567]: https://github.com/CosmWasm/wasmvm/commit/d7cb567a498e0ee438a04beeb5bfff64d493deea
[0xdebc2dd]: https://github.com/CosmWasm/wasmvm/commit/debc2ddb675e689fe1277ab533ba2c2b1016bbe6
[0x57bba20]: https://github.com/CosmWasm/wasmvm/commit/57bba20d18ac88424294c166ddde1b1025de514f
[0x0a69b95]: https://github.com/CosmWasm/wasmvm/commit/0a69b9511bcf21c901acbd5e81feeb5bbcf24b0c
[0x0cb1ef2]: https://github.com/CosmWasm/wasmvm/commit/0cb1ef22129cbd95478ca1df43ea4fa722b560e9
[0xe1c2e4e]: https://github.com/CosmWasm/wasmvm/commit/e1c2e4e0ae23adc6f36958834751fabdb6587eef
[0x98ba855]: https://github.com/CosmWasm/wasmvm/commit/98ba855efe8d59c08b1d5b0f5b09fd5bc6e221fc
[0x4094e65]: https://github.com/CosmWasm/wasmvm/commit/4094e656b0b7fdb6125824825347d5278f097cba
[0x956daaa]: https://github.com/CosmWasm/wasmvm/commit/956daaa918b54028f538e2c1f67b8ebdaf322acf
[0xff1eb7c]: https://github.com/CosmWasm/wasmvm/commit/ff1eb7c15196e0ade223081d92d883d2add131ec
[0xe4d9884]: https://github.com/CosmWasm/wasmvm/commit/e4d9884a12dc68ac96c0a5c6ff0f580ed4fe6b2f
[0x54521e5]: https://github.com/CosmWasm/wasmvm/commit/54521e52855db832f188b52f9ddc0f7681862354

## [2.0.6] - 2025-02-04

### Added

- Add test for StoreCodeUnchecked ([0x16a4a03])
- Use locked dependencies for cargo-audit install ([0xad91cb6])

### Changed

- Merge pull request #615 from CosmWasm/mergify/bp/release/2.0/pr-612 ([0x2d8c291])
- Merge pull request #606 from CosmWasm/mergify/bp/release/2.0/pr-604 ([0x8f8edd9])
- Update cargo-audit ([0xe20fc50])
- Bump cosmwasm ([0xd62c3b8])
- Set libwasmvm version to 2.0.6 ([0x54521e5])

### Fixed

- Fix unchecked flag ([0x0caab74])
- Fix import ([0xc6985ed])
- Fix tests ([0x22e6892])
- Fix ([0xd4ff2ad])

[0x2d8c291]: https://github.com/CosmWasm/wasmvm/commit/2d8c29175686a389bd6801ba945583cabc7360ec
[0x16a4a03]: https://github.com/CosmWasm/wasmvm/commit/16a4a03d42b5361f72a0702bbbb0d9a66d03df18
[0x0caab74]: https://github.com/CosmWasm/wasmvm/commit/0caab74f6ad21b3c5c77453046d116a315e01234
[0xe20fc50]: https://github.com/CosmWasm/wasmvm/commit/e20fc505d699ea878286b952c5e9b5110958d103
[0xad91cb6]: https://github.com/CosmWasm/wasmvm/commit/ad91cb66623d80bd8803437412f3ed3877b1005a
[0x8f8edd9]: https://github.com/CosmWasm/wasmvm/commit/8f8edd9e3afd691abca8c37feeb83c7a3b595812
[0xc6985ed]: https://github.com/CosmWasm/wasmvm/commit/c6985ed2b3b63c50fd3182b80ad42635c74393fa
[0x54521e5]: https://github.com/CosmWasm/wasmvm/commit/54521e52855db832f188b52f9ddc0f7681862354
[0x22e6892]: https://github.com/CosmWasm/wasmvm/commit/22e689281084876bf6f7e12ea55e9f6c80391e9d
[0xd62c3b8]: https://github.com/CosmWasm/wasmvm/commit/d62c3b826a9d5a279149951b20f9ee9b5c8550a6
[0xd4ff2ad]: https://github.com/CosmWasm/wasmvm/commit/d4ff2adee44e6b9f7415a5dfbb3de745ab9b7678

## [2.0.5] - 2024-12-10

### Added

- Add comments ([0xd0ac6ca])
- Add SimulateStoreCode function ([0xde68126])

### Changed

- Merge pull request #573 from CosmWasm/mergify/bp/release/2.0/pr-571 ([0xc404ec2])
- Update to cosmwasm 2.0.8 ([0x68f94f2])
- Set libwasmvm version to 2.0.5 ([0xc6dad83])

### Fixed

- Fix ([0x36a5c7f])

[0xc404ec2]: https://github.com/CosmWasm/wasmvm/commit/c404ec2ad0f287de02d281872130fd5663b05a57
[0xc6dad83]: https://github.com/CosmWasm/wasmvm/commit/c6dad83162c367366a3b23cc6371a944b1abc9e2
[0xd0ac6ca]: https://github.com/CosmWasm/wasmvm/commit/d0ac6ca5434e13d16fc60962e80dc67482a767c1
[0xde68126]: https://github.com/CosmWasm/wasmvm/commit/de68126114c2511c6b55b1030153cc95f10a146a
[0x68f94f2]: https://github.com/CosmWasm/wasmvm/commit/68f94f25e6ddee2f11784332a2a24f2713ceeda2
[0x36a5c7f]: https://github.com/CosmWasm/wasmvm/commit/36a5c7f9781570f933f170b1205f653de3cd2ffe

## [2.0.4] - 2024-09-23

### Changed

- Bump cosmwasm ([0xf092b7c])
- Set libwasmvm version to 2.0.4 ([0xff1eb7c])

### Fixed

- Fix tests ([0x956daaa])

[0xf092b7c]: https://github.com/CosmWasm/wasmvm/commit/f092b7c336979972f5fe2f27a50750a974eeccc7
[0x956daaa]: https://github.com/CosmWasm/wasmvm/commit/956daaa918b54028f538e2c1f67b8ebdaf322acf
[0xff1eb7c]: https://github.com/CosmWasm/wasmvm/commit/ff1eb7c15196e0ade223081d92d883d2add131ec

## [2.0.3] - 2024-08-08

### Changed

- Update cosmwasm to 2.0.6 ([0x0cb1ef2])
- Bump wasmvm version ([0xdebc2dd])

[0xdebc2dd]: https://github.com/CosmWasm/wasmvm/commit/debc2ddb675e689fe1277ab533ba2c2b1016bbe6
[0x0cb1ef2]: https://github.com/CosmWasm/wasmvm/commit/0cb1ef22129cbd95478ca1df43ea4fa722b560e9

## [2.0.2] - 2024-08-08

### Changed

- Update time crate ([0xd7cb567])
- Update to cosmwasm 2.0.5 ([0xe4d9884])
- Bump wasmvm version ([0x98ba855])

[0xd7cb567]: https://github.com/CosmWasm/wasmvm/commit/d7cb567a498e0ee438a04beeb5bfff64d493deea
[0x98ba855]: https://github.com/CosmWasm/wasmvm/commit/98ba855efe8d59c08b1d5b0f5b09fd5bc6e221fc
[0xe4d9884]: https://github.com/CosmWasm/wasmvm/commit/e4d9884a12dc68ac96c0a5c6ff0f580ed4fe6b2f

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

### Added

- Add query for the total supply of a coin ([#337])
- Add libwasmvm check to demo binary ([0x828d1a2])

### Changed

- Upgrade to cosmwasm 1.1 ([#346])
- Rename `features` to `capabilities` ([#339])
- Make "api" package internal ([#333])
- Make memory size tests less strict ([#351])
- Create builders version 0013 ([#350])
- Upgrade cbindgen to 0.24.3 ([#348])
- Upgrade to CosmWasm to 1.1 ([#347])
- Check and polish bindings.h ([#344])
- Upgrade cargo-audit ([#343])
- Move api to internal ([#342])
- support linking against wasmvm library in system path ([#335])
- Upgrade tempfile ([0x93f8bf5])
- Upgrade go and pin version of ghr ([0x02e4020])
- Merge pull request #424 from CosmWasm/demo-version-1.0 ([0xe8607ce])
- Upgrade cosmwasm to 1.0.1 ([0x75d2c3a])
- Set libwasmvm version to 1.0.1 ([0xa410575])
- Build shared libraries on release branches ([0x84529d1])
- Bump libwasmvm version to 1.1.0 ([0x3c8c6f6])
- Merge pull request #423 from CosmWasm/upgrade-tempfile-1.0 ([0xb9b7163])

### Fixed

- Fix JSON serialization of Transaction in  Env ([#345])
- Fix JSON serialization of Env.block.transaction ([#341])

[#346]: https://github.com/CosmWasm/wasmvm/issues/346
[#341]: https://github.com/CosmWasm/wasmvm/issues/341
[#339]: https://github.com/CosmWasm/wasmvm/issues/339
[#333]: https://github.com/CosmWasm/wasmvm/issues/333
[#351]: https://github.com/CosmWasm/wasmvm/pull/351
[#350]: https://github.com/CosmWasm/wasmvm/pull/350
[#348]: https://github.com/CosmWasm/wasmvm/pull/348
[#347]: https://github.com/CosmWasm/wasmvm/pull/347
[#345]: https://github.com/CosmWasm/wasmvm/pull/345
[#344]: https://github.com/CosmWasm/wasmvm/pull/344
[#343]: https://github.com/CosmWasm/wasmvm/pull/343
[#342]: https://github.com/CosmWasm/wasmvm/pull/342
[#337]: https://github.com/CosmWasm/wasmvm/pull/337
[#335]: https://github.com/CosmWasm/wasmvm/pull/335
[0x93f8bf5]: https://github.com/CosmWasm/wasmvm/commit/93f8bf5c6af549bbdddb191c49991647fbc9bfc5
[0x828d1a2]: https://github.com/CosmWasm/wasmvm/commit/828d1a2d00fd7a1714c401a7d365629e5e71926b
[0x02e4020]: https://github.com/CosmWasm/wasmvm/commit/02e4020feccd1abc2fef42dfec61628eb8c8dc40
[0xe8607ce]: https://github.com/CosmWasm/wasmvm/commit/e8607ce40054eeb1a1299f3e5ec319c9d2eae16f
[0x75d2c3a]: https://github.com/CosmWasm/wasmvm/commit/75d2c3a0dad2e1fae0b6af7b8dce0fd601ea1bc5
[0xa410575]: https://github.com/CosmWasm/wasmvm/commit/a4105758289dd15c30d8a54ffe99619a62c328b0
[0x84529d1]: https://github.com/CosmWasm/wasmvm/commit/84529d1bceadc08fc0f3d21b1360a8f1c54a0f53
[0x3c8c6f6]: https://github.com/CosmWasm/wasmvm/commit/3c8c6f621ca8847305a251b57934ca96d51213ac
[0xb9b7163]: https://github.com/CosmWasm/wasmvm/commit/b9b7163a6c22266038f501a0625eda6cacd96f32

## [1.0.1] - 2023-04-18

### Added

- Add libwasmvm check to demo binary (1.0) ([#424])

### Changed

- Upgrade tempfile (1.0) ([#423])
- Set libwasmvm version to 1.0.1 ([0xa410575])
- Upgrade cosmwasm to 1.0.1 ([0x75d2c3a])
- Build shared libraries on release branches ([0x84529d1])
- Upgrade go and pin version of ghr ([0x02e4020])

[#424]: https://github.com/CosmWasm/wasmvm/pull/424
[#423]: https://github.com/CosmWasm/wasmvm/pull/423
[0xa410575]: https://github.com/CosmWasm/wasmvm/commit/a4105758289dd15c30d8a54ffe99619a62c328b0
[0x75d2c3a]: https://github.com/CosmWasm/wasmvm/commit/75d2c3a0dad2e1fae0b6af7b8dce0fd601ea1bc5
[0x84529d1]: https://github.com/CosmWasm/wasmvm/commit/84529d1bceadc08fc0f3d21b1360a8f1c54a0f53
[0x02e4020]: https://github.com/CosmWasm/wasmvm/commit/02e4020feccd1abc2fef42dfec61628eb8c8dc40

## [1.0.0] - 2022-05-16

### Added

- Pass complete errors through FFI ([#73])
- Improve Go to Rust memory ownership transfer ([#66])
- Build system support for .so on GNU Linux for both x86_64 and aarch64 ([#303])
- Push GasMultiplier into cosmos-sdk ([#122])
- Add ibc v3 support ([#332])
- Add build system for musl Linux static libraries for aarch64 ([#305])
- Add folder libwasmvm/artifacts/ ([0x9a10c3c])
- Add Vec::into_raw_parts explanation ([0x5ad99f8])
- Add test unmanaged_vector_new_works ([0xf6b35a1])

### Changed

- Various cleanup items ([#130])
- Final touches for 1.0.0 ([#334])
- Create ARM build for glibc Linux ([#330])
- Consume outputs early to ensure destruction of the UnmanagedVector's ([#327])
- Unconditionally destruct value's UnmanagedVector ([#326])
- Improve clarity on GoResult (now GoError) ([#325])
- Run linter on all targets ([#324])
- Format codebase using gofumpt ([#321])
- Create frame limit for iterator frames ([#320])
- Upgrade testify to 1.7.1 ([#319])
- Refactor iterator stack code ([#315])
- Expose libwasmvm version number at runtime ([#314])
- Upgrade CosmWasm to 1.0.0-beta8 ([#311])
- Upgrade to CosmWasm to 0.16.7 ([#310])
- Upgrade builders and upgrade cosmwasm to 0.16.6 ([#309])
- Update Rust to 1.59.0 ([#307])
- Upgrade cosmwasm to 1.0.0-beta7 ([#304])
- Bump Rust and cargo-audit in libwasmvm_audit CI job (0.16) ([#300])
- bump tm-db for rocksdb support ([#297])
- Upgrade Wasmer to 2.2 (CosmWasm v1.0.0-beta6) ([#296])
- Bump Rust and cargo-audit in libwasmvm_audit CI job ([#295])
- Let human_address/canonical_address return correct error type to report back to contracts ([#124])
- Create universal library with for ARM and Intel ([#294])
- Prepare ~~0.16.4~~ 0.16.5 release ([#292])
- Upgrade cosmwasm to 1.0.0 rc.0 ([#329])
- Debug demo binary ([#291])
- Update cosmwasm to v1.0.0-beta5 ([#290])
- Upgrade wasmvm 0.16 to Go 1.17 ([#287])
- Add tidy-go CI job (0.16) ([#286])
- Rename SubcallResponse/SubcallResult to SubMsgResponse/SubMsgResult ([#301])
- Run test job in Go 1.17 ([#285])
- Set libwasmvm version to 1.0.0-rc.0 ([#331])
- Use stronger machines for long running CI jobs (0.16) ([#284])
- Use stronger machines for long running jobs ([#283])
- Upgrade to cosmwasm 1.0.0-beta3 ([#279])
- Upgrade builders and build setup ([#278])
- Return full result on IBCPacketRecv ([#276])
- Update CI images to Go 1.17 ([#274])
- Check tidyness in CI ([#273])
- tm-db version bump ([#272])
- Go 1.17 ([#271])
- Upgrade shfmt and pin version (backport to 0.16 branch) ([#267])
- Upgrade cosmwasm to 1.0.0-beta ([#263])
- Upgrade shfmt and pin version ([#262])
- Upgrade to CosmWasm to 1.0.0-soon2 ([#261])
- Update to CosmWasm 1.0.0-soon ([#260])
- Bump CosmWasm to 1.0.0-beta2 ([0xda602fa])
- Run deploy_to_git on 0.16 branch ([0xbcb820c])
- Update gas values in tests ([0x712cc31])
- Upgrade const-oid, crossbeam-utils and sha2 ([0x30b07b4])
- Simplify UnmanagedVector::default implementation ([0x39f915d])
- Use handle_c_error_default/handle_c_error_ptr consistently ([0x16e8e0a])

### Removed

- Remove unused type StargateResponse ([#316])
- Remove stacktrace from runtime error (1.0) ([#281])
- Remove stacktrace from runtime error (0.16) ([#280])
- Remove StargateResponse and rename to SubMsgResponse/SubMsgResult ([#317])

### Fixed
- Fix go test commands ([0x4ff2a3c])
- Fix none handling in copyAndDestroyUnmanagedVector ([0xceaebca])
- Fix go test commands ([0x18fea2a])
- Fix none handling in copyAndDestroyUnmanagedVector ([0x790cafa])
- Fix omitempty spelling ([#275])
- Fix cosmwasm beta4 upgrade ([#289])

[#73]: https://github.com/CosmWasm/wasmvm/issues/73
[#66]: https://github.com/CosmWasm/wasmvm/issues/66
[#316]: https://github.com/CosmWasm/wasmvm/issues/316
[#303]: https://github.com/CosmWasm/wasmvm/issues/303
[#301]: https://github.com/CosmWasm/wasmvm/issues/301
[#130]: https://github.com/CosmWasm/wasmvm/issues/130
[#124]: https://github.com/CosmWasm/wasmvm/issues/124
[#122]: https://github.com/CosmWasm/wasmvm/issues/122
[#334]: https://github.com/CosmWasm/wasmvm/pull/334
[#332]: https://github.com/CosmWasm/wasmvm/pull/332
[#331]: https://github.com/CosmWasm/wasmvm/pull/331
[#330]: https://github.com/CosmWasm/wasmvm/pull/330
[#329]: https://github.com/CosmWasm/wasmvm/pull/329
[#327]: https://github.com/CosmWasm/wasmvm/pull/327
[#326]: https://github.com/CosmWasm/wasmvm/pull/326
[#325]: https://github.com/CosmWasm/wasmvm/pull/325
[#324]: https://github.com/CosmWasm/wasmvm/pull/324
[#321]: https://github.com/CosmWasm/wasmvm/pull/321
[#320]: https://github.com/CosmWasm/wasmvm/pull/320
[#319]: https://github.com/CosmWasm/wasmvm/pull/319
[#317]: https://github.com/CosmWasm/wasmvm/pull/317
[#315]: https://github.com/CosmWasm/wasmvm/pull/315
[#314]: https://github.com/CosmWasm/wasmvm/pull/314
[#311]: https://github.com/CosmWasm/wasmvm/pull/311
[#310]: https://github.com/CosmWasm/wasmvm/pull/310
[#309]: https://github.com/CosmWasm/wasmvm/pull/309
[#307]: https://github.com/CosmWasm/wasmvm/pull/307
[#305]: https://github.com/CosmWasm/wasmvm/pull/305
[#304]: https://github.com/CosmWasm/wasmvm/pull/304
[#300]: https://github.com/CosmWasm/wasmvm/pull/300
[#297]: https://github.com/CosmWasm/wasmvm/pull/297
[#296]: https://github.com/CosmWasm/wasmvm/pull/296
[#295]: https://github.com/CosmWasm/wasmvm/pull/295
[#294]: https://github.com/CosmWasm/wasmvm/pull/294
[#292]: https://github.com/CosmWasm/wasmvm/pull/292
[#291]: https://github.com/CosmWasm/wasmvm/pull/291
[#290]: https://github.com/CosmWasm/wasmvm/pull/290
[#289]: https://github.com/CosmWasm/wasmvm/pull/289
[#287]: https://github.com/CosmWasm/wasmvm/pull/287
[#286]: https://github.com/CosmWasm/wasmvm/pull/286
[#285]: https://github.com/CosmWasm/wasmvm/pull/285
[#284]: https://github.com/CosmWasm/wasmvm/pull/284
[#283]: https://github.com/CosmWasm/wasmvm/pull/283
[#281]: https://github.com/CosmWasm/wasmvm/pull/281
[#280]: https://github.com/CosmWasm/wasmvm/pull/280
[#279]: https://github.com/CosmWasm/wasmvm/pull/279
[#278]: https://github.com/CosmWasm/wasmvm/pull/278
[#276]: https://github.com/CosmWasm/wasmvm/pull/276
[#275]: https://github.com/CosmWasm/wasmvm/pull/275
[#274]: https://github.com/CosmWasm/wasmvm/pull/274
[#273]: https://github.com/CosmWasm/wasmvm/pull/273
[#272]: https://github.com/CosmWasm/wasmvm/pull/272
[#271]: https://github.com/CosmWasm/wasmvm/pull/271
[#267]: https://github.com/CosmWasm/wasmvm/pull/267
[#263]: https://github.com/CosmWasm/wasmvm/pull/263
[#262]: https://github.com/CosmWasm/wasmvm/pull/262
[#261]: https://github.com/CosmWasm/wasmvm/pull/261
[#260]: https://github.com/CosmWasm/wasmvm/pull/260
[0xda602fa]: https://github.com/CosmWasm/wasmvm/commit/da602fad79ed534318444bfd54383d0b3ccb3c84
[0x790cafa]: https://github.com/CosmWasm/wasmvm/commit/790cafa0e1625d0d232472de41c1341967e3ba2a
[0xbcb820c]: https://github.com/CosmWasm/wasmvm/commit/bcb820c081054f9ac54d7963f93cc060fb2766b8
[0x712cc31]: https://github.com/CosmWasm/wasmvm/commit/712cc31e4f42adb72f0e899342532a20bb933239
[0x30b07b4]: https://github.com/CosmWasm/wasmvm/commit/30b07b4492852a6aadf72159d99ba7d2529e982d
[0x9a10c3c]: https://github.com/CosmWasm/wasmvm/commit/9a10c3c28aa6ab5fbdd471369659519522d571f4
[0x18fea2a]: https://github.com/CosmWasm/wasmvm/commit/18fea2a2158b766aafa336ac046bc75e97606f59
[0x39f915d]: https://github.com/CosmWasm/wasmvm/commit/39f915d3b7268c2be4c9727a843613aadd0827a9
[0x5ad99f8]: https://github.com/CosmWasm/wasmvm/commit/5ad99f8173f23457fd9e20cdf2e96c5755f430eb
[0xf6b35a1]: https://github.com/CosmWasm/wasmvm/commit/f6b35a12e4f6c8fa7f832ce2b3cb6c0d7369289f
[0xceaebca]: https://github.com/CosmWasm/wasmvm/commit/ceaebca68ca2ddbda8cff6bcf2b89316e90121b1
[0x16e8e0a]: https://github.com/CosmWasm/wasmvm/commit/16e8e0a7648823ab0c060aadf60f75236af168e5
[0x4ff2a3c]: https://github.com/CosmWasm/wasmvm/commit/4ff2a3cadfd01b8bd245e82dc9a1d964d2315f88

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
