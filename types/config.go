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

// CacheOptions represents configuration options for the Wasm contract cache.
// It controls how contracts are stored and managed in memory.
type CacheOptions struct {
	BaseDir                  string   `json:"base_dir"`
	AvailableCapabilities    []string `json:"available_capabilities"`
	MemoryCacheSizeBytes     Size     `json:"memory_cache_size_bytes"`
	InstanceMemoryLimitBytes Size     `json:"instance_memory_limit_bytes"`
}

// Size represents a size value with unit conversion capabilities.
// It is used to specify memory and cache sizes in various units.
type Size struct{ uint32 }

// MarshalJSON implements the json.Marshaler interface for Size.
// It converts the size to a string representation with appropriate unit.
func (s Size) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.uint32)
}

// NewSize creates a new Size value in bytes.
func NewSize(v uint32) Size {
	return Size{v}
}

// NewSizeKilo creates a new Size value in kilobytes (1000 bytes).
func NewSizeKilo(v uint32) Size {
	return Size{v * 1000}
}

// NewSizeKibi creates a new Size value in kibibytes (1024 bytes).
func NewSizeKibi(v uint32) Size {
	return Size{v * 1024}
}

// NewSizeMega creates a new Size value in megabytes (1000^2 bytes).
func NewSizeMega(v uint32) Size {
	return Size{v * 1000 * 1000}
}

// NewSizeMebi creates a new Size value in mebibytes (1024^2 bytes).
func NewSizeMebi(v uint32) Size {
	return Size{v * 1024 * 1024}
}

// NewSizeGiga creates a new Size value in gigabytes (1000^3 bytes).
func NewSizeGiga(v uint32) Size {
	return Size{v * 1000 * 1000 * 1000}
}

// NewSizeGibi creates a new Size value in gibibytes (1024^3 bytes).
func NewSizeGibi(v uint32) Size {
	return Size{v * 1024 * 1024 * 1024}
}
