package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Ptr[T any](v T) *T {
	return &v
}

func TestConfigJSON(t *testing.T) {
	// see companion test "test_config_json" on the Rust side
	config := VMConfig{
		WasmLimits: WasmLimits{
			InitialMemoryLimitPages: Ptr(uint32(15)),
			TableSizeLimitElements:  Ptr(uint32(20)),
			MaxImports:              Ptr(uint32(100)),
			MaxFunctionParams:       Ptr(uint32(0)),
		},
		Cache: CacheOptions{
			BaseDir:                  "/tmp",
			AvailableCapabilities:    []string{"a", "b"},
			MemoryCacheSizeBytes:     NewSize(100),
			InstanceMemoryLimitBytes: NewSize(100),
		},
	}
	expected := `{"wasm_limits":{"initial_memory_limit_pages":15,"table_size_limit_elements":20,"max_imports":100,"max_function_params":0},"cache":{"base_dir":"/tmp","available_capabilities":["a","b"],"memory_cache_size_bytes":100,"instance_memory_limit_bytes":100}}`

	bz, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Equal(t, expected, string(bz))
}
