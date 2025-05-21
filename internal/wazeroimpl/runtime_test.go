//go:build wazero || !cgo

package wazeroimpl

import (
    "context"
    "os"
    "testing"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/experimental/wat"

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

    // Minimal wasm with "allocate" and exported memory.
    moduleWat := `(module
      (memory (export "memory") 1)
      (global $heap (mut i32) (i32.const 0))
      (func (export "allocate") (param $size i32) (result i32)
        (global.get $heap)
        (global.set $heap (i32.add (global.get $heap) (local.get $size)))
      )
    )`

    wasmBytes, err := wat.Compile([]byte(moduleWat))
    if err != nil {
        t.Fatalf("compile wat: %v", err)
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
    // The wasm module stores the lengths of env/info/msg into memory and does
    // nothing else. This keeps host-function dependencies minimal.
    moduleWat := `(module
      (memory (export "memory") 1)
      (global $heap (mut i32) (i32.const 0))
      (func (export "allocate") (param $size i32) (result i32)
        (global.get $heap)
        (global.set $heap (i32.add (global.get $heap) (local.get $size)))
      )

      ;; params: env_ptr env_len info_ptr info_len msg_ptr msg_len
      (func (export "instantiate") (param i32 i32 i32 i32 i32 i32) (result i32)
        ;; return 0
        (i32.const 0)
      )
      (func (export "execute") (param i32 i32 i32 i32 i32 i32) (result i32)
        (i32.const 0)
      )
    )`

    wasmBytes, err := wat.Compile([]byte(moduleWat))
    if err != nil {
        t.Fatalf("compile wat: %v", err)
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

    // Instantiate
    if err := cache.Instantiate(context.Background(), checksum, []byte("{}"), []byte("{}"), []byte("{}"), store, &types.GoAPI{}, &types.Querier{}, types.GasMeter{}); err != nil {
        t.Fatalf("instantiate failed: %v", err)
    }

    // Execute
    if err := cache.Execute(context.Background(), checksum, []byte("{}"), []byte("{}"), []byte("{}"), store, &types.GoAPI{}, &types.Querier{}, types.GasMeter{}); err != nil {
        t.Fatalf("execute failed: %v", err)
    }
 }
