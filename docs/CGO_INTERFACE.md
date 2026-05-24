# CGO interface

This document summarises the functions exported by the `libwasmvm` C library and their
wrappers implemented in the Go package `internal/api`. It also lists the Go data
structures from `types/` that cross the cgo boundary.

## Exported functions and Go wrappers

| C function (bindings.h) | Go wrapper |
| --- | --- |
| `init_cache` | `api.InitCache` |
| `store_code` | `api.StoreCode`/`api.StoreCodeUnchecked` |
| `remove_wasm` | `api.RemoveCode` |
| `load_wasm` | `api.GetCode` |
| `pin` | `api.Pin` |
| `unpin` | `api.Unpin` |
| `analyze_code` | `api.AnalyzeCode` |
| `get_metrics` | `api.GetMetrics` |
| `get_pinned_metrics` | `api.GetPinnedMetrics` |
| `release_cache` | `api.ReleaseCache` |
| `instantiate` | `api.Instantiate` |
| `execute` | `api.Execute` |
| `migrate` | `api.Migrate` |
| `migrate_with_info` | `api.MigrateWithInfo` |
| `sudo` | `api.Sudo` |
| `reply` | `api.Reply` |
| `query` | `api.Query` |
| `ibc_channel_open` | `api.IBCChannelOpen` |
| `ibc_channel_connect` | `api.IBCChannelConnect` |
| `ibc_channel_close` | `api.IBCChannelClose` |
| `ibc_packet_receive` | `api.IBCPacketReceive` |
| `ibc_packet_ack` | `api.IBCPacketAck` |
| `ibc_packet_timeout` | `api.IBCPacketTimeout` |
| `ibc_source_callback` | `api.IBCSourceCallback` |
| `ibc_destination_callback` | `api.IBCDestinationCallback` |
| `ibc2_packet_receive` | `api.IBC2PacketReceive` |
| `ibc2_packet_ack` | `api.IBC2PacketAck` |
| `ibc2_packet_timeout` | `api.IBC2PacketTimeout` |
| `ibc2_packet_send` | `api.IBC2PacketSend` |
| `new_unmanaged_vector` | internal helpers in `memory.go` |
| `destroy_unmanaged_vector` | internal helpers in `memory.go` |
| `version_str` | `api.LibwasmvmVersion` |

## Data structures crossing the boundary

Several types defined in the `types` package are used when calling into
`libwasmvm` or when reading its results:

- `VMConfig` – configuration passed to `InitCache`.
- `GasMeter` – interface supplying gas information for execution.
- `KVStore` and `Iterator` – database interface used by the VM.
- `GoAPI` – set of callbacks for address handling.
- `Querier` – interface for custom queries.
- `AnalysisReport` – returned from `AnalyzeCode`.
- `Metrics` and `PinnedMetrics` – returned from metric queries.
- `GasReport` – returned from execution functions.

All other data (such as contract messages, environment and info values) is
passed as byte slices and therefore does not rely on additional exported Go
structures.

