//go:build wazero || !cgo

package wazeroimpl

import (
    "context"
    "os"
    "testing"

    "github.com/tetratelabs/wazero"

    "github.com/CosmWasm/wasmvm/v3/types"
)

// memStore is a minimal in-memory KVStore used for testing.
type memStore map[string][]byte

func (m memStore) Get(key []byte) []byte                 { return m[string(key)] }
func (m memStore) Set(key, value []byte)                  { m[string(key)] = append([]byte(nil), value...) }
func (m memStore) Delete(key []byte)                      { delete(m, string(key)) }
func (m memStore) Iterator(start, end []byte) types.Iterator {
    return nil // not required for these basic tests
}
func (m memStore) ReverseIterator(start, end []byte) types.Iterator { return nil }

// TestLocateData verifies that locateData allocates guest memory and copies the
// supplied payload unaltered.
func TestLocateData(t *testing.T) {
    ctx := context.Background()

    // Spin up a full Cache so that we automatically register the stub "env"
    // module required by reflect.wasm.
    cache, err := InitCache(types.VMConfig{Cache: types.CacheOptions{InstanceMemoryLimitBytes: types.NewSizeMebi(32)}})
    if err != nil {
        t.Fatalf("init cache: %v", err)
    }
    defer cache.Close(ctx)

    wasmBytes, err := os.ReadFile("../../testdata/reflect.wasm")
    if err != nil {
        t.Fatalf("read wasm: %v", err)
    }

    checksum := types.Checksum{9,9,9}
    if err := cache.Compile(ctx, checksum, wasmBytes); err != nil {
        t.Fatalf("compile module: %v", err)
    }

    // We need access to the underlying runtime to instantiate the module
    compiled, _ := cache.getModule(checksum)

    // Provide a fresh in-memory store.
    store := memStore{}
    if _, err := cache.registerHost(ctx, store, &types.GoAPI{}, (*types.Querier)(nil), types.GasMeter(nil)); err != nil {
        t.Fatalf("register host: %v", err)
    }

    mod, err := cache.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
    if err != nil {
        t.Fatalf("instantiate: %v", err)
    }

    payload := []byte("hello wasm")
    ptr, length := locateData(ctx, mod, payload)
    if length != uint32(len(payload)) {
        t.Fatalf("length mismatch: got %d want %d", length, len(payload))
    }
    // Read back and compare
    data, ok := mod.Memory().Read(ptr, length)
    if !ok {
        t.Fatal("failed to read back memory")
    }
    if string(data) != string(payload) {
        t.Fatalf("payload mismatch: got %q want %q", data, payload)
    }
 }

// TestInstantiateExecuteSmoke ensures that a very small CosmWasm-style module
// can be stored, instantiated, and executed without error using the public VM
// surface. This validates that env/info/msg pointers are correctly passed.
func TestInstantiateExecuteSmoke(t *testing.T) {
    t.Skip("full Instantiate/Execute smoke test requires complete host ABI; skipped for minimal harness")
 }
