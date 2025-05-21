//go:build wazero || !cgo

package wazeroimpl

import (
    "context"
    "os"
    "testing"

    "github.com/CosmWasm/wasmvm/v3/types"
)

// TestReflectInstantiate loads the reflect.wasm contract (shipped in testdata)
// and instantiates it via the wazero-backed VM implementation. This ensures
// that real-world CosmWasm contracts that compile to Wasm can be instantiated
// without panics or host‐side errors.
func TestReflectInstantiate(t *testing.T) {
    wasm, err := os.ReadFile("../../testdata/reflect.wasm")
    if err != nil {
        t.Fatalf("read wasm: %v", err)
    }

    cfg := types.VMConfig{
        Cache: types.CacheOptions{
            InstanceMemoryLimitBytes: types.NewSizeMebi(32),
        },
    }

    cache, err := InitCache(cfg)
    if err != nil {
        t.Fatalf("init cache: %v", err)
    }
    defer cache.Close(context.Background())

    checksum := types.Checksum{1, 2, 3}
    if err := cache.Compile(context.Background(), checksum, wasm); err != nil {
        t.Fatalf("compile: %v", err)
    }

    if err := cache.Instantiate(
        context.Background(),
        checksum,
        []byte("{}"), []byte("{}"), []byte("{}"),
        memStore{},
        &types.GoAPI{},
        (*types.Querier)(nil),
        types.GasMeter(nil),
    ); err != nil {
        t.Fatalf("instantiate: %v", err)
    }
}
