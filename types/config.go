package types

import (
	"github.com/vmihailenco/msgpack/v5"
)

type Config struct {
	WasmLimits WasmLimits   `msgpack:"wasm_limits"`
	Cache      CacheOptions `msgpack:"cache"`
}

type WasmLimits struct {
	MemoryPageLimit        *uint32 `msgpack:"memory_page_limit"`
	TableSizeLimit         *uint32 `msgpack:"table_size_limit"`
	MaxImports             *uint32 `msgpack:"max_imports"`
	MaxFunctions           *uint32 `msgpack:"max_functions"`
	MaxFunctionParams      *uint32 `msgpack:"max_function_params"`
	MaxTotalFunctionParams *uint32 `msgpack:"max_total_function_params"`
	MaxFunctionResults     *uint32 `msgpack:"max_function_results"`
}

type CacheOptions struct {
	BaseDir               string   `msgpack:"base_dir"`
	AvailableCapabilities []string `msgpack:"available_capabilities"`
	MemoryCacheSize       Size     `msgpack:"memory_cache_size"`
	InstanceMemoryLimit   Size     `msgpack:"instance_memory_limit"`
}

type Size struct{ uint32 }

func (s Size) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeUint(uint64(s.uint32))
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
