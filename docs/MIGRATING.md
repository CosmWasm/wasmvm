# Migrating docs for callers of the wasmvm Go project

## 1.x -> 2.0

- The `supportedCapabilities` argument in `NewVM` changed from a comma separated
  list to a list of type `[]string`.

## Renamings

This section contains renamed symbols that do not require any further
explanation. Some of the new names may be available in 1.x already in cases
where the old name was deprecated.

| Old name                 | New name                    | Note                                                        |
| ------------------------ | --------------------------- | ----------------------------------------------------------- |
| `VM.Create`              | `VM.StoreCode`              | StoreCode brings consistency with wasmd naming              |
| `SubcallResult`          | `SubMsgResult`              | Contracts do not "call" each other but send messages around |
| `SubcallResponse`        | `SubMsgResponse`            | Contracts do not "call" each other but send messages around |
| `HumanizeAddress`        | `HumanizeAddressFunc`       | Follow [best practice for naming function types][ft]        |
| `CanonicalizeAddress`    | `CanonicalizeAddressFunc`   | Follow [best practice for naming function types][ft]        |
| `GoAPI.HumanAddress`     | `GoAPI.HumanizeAddress`     | Perfer verbs for converters                                 |
| `GoAPI.CanonicalAddress` | `GoAPI.CanonicalizeAddress` | Perfer verbs for converters                                 |

[ft]: https://stackoverflow.com/a/60073310
