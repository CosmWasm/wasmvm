package types

import (
	"encoding/json"
)

// VMConfig defines the configuration for the VM.
// For full documentation see the Rust side:
// https://github.com/CosmWasm/cosmwasm/blob/main/packages/vm/src/config.rs#L27
type VMConfig struct {
	WasmLimits WasmLimits   `json:"wasm_limits"`
	Cache      CacheOptions `json:"cache"`
}

type WasmLimits struct {
	InitialMemoryLimit     *uint32 `json:"initial_memory_limit,omitempty"`
	TableSizeLimit         *uint32 `json:"table_size_limit,omitempty"`
	MaxImports             *uint32 `json:"max_imports,omitempty"`
	MaxFunctions           *uint32 `json:"max_functions,omitempty"`
	MaxFunctionParams      *uint32 `json:"max_function_params,omitempty"`
	MaxTotalFunctionParams *uint32 `json:"max_total_function_params,omitempty"`
	MaxFunctionResults     *uint32 `json:"max_function_results,omitempty"`
}

type CacheOptions struct {
	BaseDir               string   `json:"base_dir"`
	AvailableCapabilities []string `json:"available_capabilities"`
	MemoryCacheSize       Size     `json:"memory_cache_size"`
	InstanceMemoryLimit   Size     `json:"instance_memory_limit"`
}

type Size struct{ uint32 }

func (s Size) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.uint32)
}

func NewSize(v uint32) Size {
	return Size{v}
}

func NewSizeKilo(v uint32) Size {
	return Size{v * 1000}
}

func NewSizeKibi(v uint32) Size {
	return Size{v * 1024}
}

func NewSizeMega(v uint32) Size {
	return Size{v * 1000 * 1000}
}

func NewSizeMebi(v uint32) Size {
	return Size{v * 1024 * 1024}
}

func NewSizeGiga(v uint32) Size {
	return Size{v * 1000 * 1000 * 1000}
}

func NewSizeGibi(v uint32) Size {
	return Size{v * 1024 * 1024 * 1024}
}
