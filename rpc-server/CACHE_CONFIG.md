# WasmVM RPC Server Cache Configuration

## Persistent Cache

The WasmVM RPC server uses a persistent cache directory that is shared across all instances. This design allows multiple chain daemons to benefit from the same compiled WASM modules, reducing memory usage and improving performance.

## Default Cache Location

By default, the cache is stored at:
```
~/.wasmvm/cache
```

## Custom Cache Location

You can customize the cache location by setting the `WASMVM_CACHE_DIR` environment variable:

```bash
export WASMVM_CACHE_DIR=/path/to/custom/cache
```

## Benefits of Persistent Cache

1. **Reused Across Restarts**: Compiled WASM modules persist between server restarts
2. **Shared Between Daemons**: Multiple chain daemons can share the same cache
3. **Reduced Memory Usage**: Only one copy of each compiled module in memory
4. **Faster Contract Loading**: Pre-compiled modules load instantly
5. **Atomic Operations**: libwasmvm handles concurrent access safely

## Usage with Multiple Chains

When running multiple chain daemons (e.g., multiple wasmd instances), they can all point to the same RPC server, which will use the shared cache:

```bash
# All chains can use the same RPC server endpoint
CHAIN1_WASMVM_RPC_ENDPOINT=localhost:9090
CHAIN2_WASMVM_RPC_ENDPOINT=localhost:9090
CHAIN3_WASMVM_RPC_ENDPOINT=localhost:9090
```

## Cache Management

The cache is managed automatically by libwasmvm. It includes:
- Compiled WASM modules indexed by checksum
- Memory and filesystem caching layers
- Automatic eviction policies for memory cache
- Persistent filesystem cache with no automatic eviction

## Security Considerations

- The cache directory should have appropriate permissions
- Each compiled module is indexed by its SHA256 checksum
- Checksums ensure integrity - the same checksum always returns the same compiled module 