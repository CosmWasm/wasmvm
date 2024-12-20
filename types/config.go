package types

import (
	"encoding/json"
)

// VMConfig defines the configuration for the VM.
// For full documentation see the Rust side:
// https://docs.rs/cosmwasm-vm/2.2.0-rc.1/cosmwasm_vm/struct.Config.html
type VMConfig struct {
	WasmLimits WasmLimits   `json:"wasm_limits"`
	Cache      CacheOptions `json:"cache"`
}

// WasmLimits defines limits for static validation of wasm code that is stored on chain.
// For full documentation see the Rust side:
// https://docs.rs/cosmwasm-vm/2.2.0-rc.1/cosmwasm_vm/struct.WasmLimits.html
type WasmLimits struct {
	InitialMemoryLimitPages *uint32 `json:"initial_memory_limit_pages,omitempty"`
	TableSizeLimitElements  *uint32 `json:"table_size_limit_elements,omitempty"`
	MaxImports              *uint32 `json:"max_imports,omitempty"`
	MaxFunctions            *uint32 `json:"max_functions,omitempty"`
	MaxFunctionParams       *uint32 `json:"max_function_params,omitempty"`
	MaxTotalFunctionParams  *uint32 `json:"max_total_function_params,omitempty"`
	MaxFunctionResults      *uint32 `json:"max_function_results,omitempty"`
}

type CacheOptions struct {
	BaseDir                  string   `json:"base_dir"`
	AvailableCapabilities    []string `json:"available_capabilities"`
	MemoryCacheSizeBytes     Size     `json:"memory_cache_size_bytes"`
	InstanceMemoryLimitBytes Size     `json:"instance_memory_limit_bytes"`
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
