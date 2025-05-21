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
    r := wazero.NewRuntime(ctx)
    defer r.Close(ctx)

    // We use a known-good CosmWasm contract binary that exports the required
    // symbols (including `allocate`) instead of generating a module from WAT
    // to avoid an extra dependency on the wazero experimental text parser.
    wasmBytes, err := os.ReadFile("../../testdata/reflect.wasm")
    if err != nil {
        t.Fatalf("read wasm: %v", err)
    }

    compiled, err := r.CompileModule(ctx, wasmBytes)
    if err != nil {
        t.Fatalf("compile module: %v", err)
    }

    mod, err := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
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
    // The reflect contract is sufficient for a smoke test of Instantiate and
    // Execute because it exposes both entrypoints as well as the required
    // allocator function.
    wasmBytes, err := os.ReadFile("../../testdata/reflect.wasm")
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

    // Store code
    checksum := types.Checksum{1, 2, 3}
    if err := cache.Compile(context.Background(), checksum, wasmBytes); err != nil {
        t.Fatalf("compile: %v", err)
    }

    store := memStore{}

    // Instantiate – we can pass nil for optional Querier and GasMeter since the
    // current wazero host implementation does not use them yet.
    if err := cache.Instantiate(context.Background(), checksum, []byte("{}"), []byte("{}"), []byte("{}"), store, &types.GoAPI{}, (*types.Querier)(nil), types.GasMeter(nil)); err != nil {
        t.Fatalf("instantiate failed: %v", err)
    }

    // Execute
    if err := cache.Execute(context.Background(), checksum, []byte("{}"), []byte("{}"), []byte("{}"), store, &types.GoAPI{}, (*types.Querier)(nil), types.GasMeter(nil)); err != nil {
        t.Fatalf("execute failed: %v", err)
    }
 }
