# Migrating docs for callers of the wasmvm Go project

## 1.x -> 2.0

- `VM.Create` was removed. Use `VM.StoreCode` instead.
- The `supportedCapabilities` argument in `NewVM` changed from a comma separated
  list to a list of type `[]string`.
