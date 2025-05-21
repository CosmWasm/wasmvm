package wazeroimpl

import (
    "context"
    "os"
    "testing"

    "github.com/CosmWasm/wasmvm/v3/types"
)

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
    cs := types.Checksum{1,2,3}
    if err := cache.Compile(context.Background(), cs, wasm); err != nil {
        t.Fatalf("compile failed: %v", err)
    }
    err = cache.Instantiate(context.Background(), cs, []byte("{}"), []byte("{}"), []byte("{}"), memStore{}, &types.GoAPI{}, &types.Querier{}, types.GasMeter{})
    if err != nil {
        t.Fatalf("instantiate: %v", err)
    }
}

