# Migrating docs for callers of the wasmvm Go project

## 1.x -> 2.0

- The `supportedCapabilities` argument in `NewVM` changed from a comma separated
  list to a list of type `[]string`.
- The field `CodeInfoResponse.Checksum` is now explicitly marked as
  non-optional. It has always been set to a 32 byte value in the past.
- All entrypoint functions now return the full result with an `Ok` or `Err`
  variant instead of just the data inside the `Ok`. This was previously only the
  case for `IBCPacketReceive`. It is important to note that this means contract
  errors are no longer returned in the `error` return value. Instead, the `Err`
  field should be checked for errors.
- The field `BlockInfo.Time` now uses a wrapper type `Uint64` instead of
  `uint64` to ensure string serialization. You can use `uint64(u)` to get the
  underlying value.
- The field `IBCReceiveResponse.Acknowledgement` can now be `nil`. In this case,
  no acknowledgement must be written. Callers need to handle this case
  separately from empty data.
- CosmWasm gas values were reduced by a factor of 1000, so each instruction now
  consumes 150 CosmWasm gas instead of 150000. This should be taken into account
  when converting between CosmWasm gas and Cosmos SDK gas.
- A new lockfile called `exclusive.lock` in the base directory ensures that no
  two `VM` instances operate on the same directory in parallel. This was
  unsupported before already but now leads to an error early on. When doing
  parallel testing, use a different directory for each instance.
- `QueryRequest.Grpc` was added. It is similar to `QueryRequest.Stargate` but
  unlike that, it should always return protobuf encoded responses on all chains.
- `VM.StoreCode` now returns a `uint64` containing the gas cost in CosmWasm gas
  and takes a gas limit as argument. This was previously calculated in wasmd.
  The change brings consistency with the other functions that cause gas usage.
- `GoAPI` now requires an additional `ValidateAddress` function that validates
  whether the given string is a valid address. This was previously done
  internally using separate calls to `CanonicalizeAddress` and `HumanizeAddress`
  but can be done more efficiently using a single call.

## Renamings

This section contains renamed symbols that do not require any further
explanation. Some of the new names may be available in 1.x already in cases
where the old name was deprecated.

| Old name                          | New name                              | Note                                                         |
| --------------------------------- | ------------------------------------- | ------------------------------------------------------------ |
| `VM.Create`                       | `VM.StoreCode`                        | StoreCode brings consistency with wasmd naming               |
| `AnalysisReport.RequiredFeatures` | `AnalysisReport.RequiredCapabilities` | Renamed for a long time, but now the old version was removed |
| `SubcallResult`                   | `SubMsgResult`                        | Contracts do not "call" each other but send messages around  |
| `SubcallResponse`                 | `SubMsgResponse`                      | Contracts do not "call" each other but send messages around  |
| `HumanizeAddress`                 | `HumanizeAddressFunc`                 | Follow [best practice for naming function types][ft]         |
| `CanonicalizeAddress`             | `CanonicalizeAddressFunc`             | Follow [best practice for naming function types][ft]         |
| `GoAPI.HumanAddress`              | `GoAPI.HumanizeAddress`               | Perfer verbs for converters                                  |
| `GoAPI.CanonicalAddress`          | `GoAPI.CanonicalizeAddress`           | Perfer verbs for converters                                  |
| `CosmosMsg.Stargate`              | `CosmosMsg.Any`                       | The message has nothing to do with Stargate                  |
| `StargateMsg`                     | `AnyMsg`                              | The message has nothing to do with Stargate                  |
| `QueryResponse`                   | `QueryResult`                         | Brings consistency with the naming of the other results      |
| `VoteMsg.Vote`                    | `VoteMsg.Option`                      | Brings consistency with Cosmos SDK naming                    |

[ft]: https://stackoverflow.com/a/60073310
