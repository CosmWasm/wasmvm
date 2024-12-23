// file: internal/runtime/wasm_runtime.go
package wazero

import "github.com/CosmWasm/wasmvm/v2/types"

type WasmRuntime interface {
	// InitCache sets up any runtime-specific cache or resources. Returns a handle.
	InitCache(config types.VMConfig) (any, error)

	// ReleaseCache frees resources created by InitCache.
	ReleaseCache(handle any)

	// Compilation and code storage
	StoreCode(code []byte, persist bool) (checksum []byte, err error)
	StoreCodeUnchecked(code []byte) ([]byte, error)
	GetCode(checksum []byte) ([]byte, error)
	RemoveCode(checksum []byte) error
	Pin(checksum []byte) error
	Unpin(checksum []byte) error
	AnalyzeCode(checksum []byte) (*types.AnalysisReport, error)

	// Execution lifecycles
	Instantiate(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Execute(checksum []byte, env []byte, info []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Migrate(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	MigrateWithInfo(checksum []byte, env []byte, msg []byte, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Sudo(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Reply(checksum []byte, env []byte, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	Query(checksum []byte, env []byte, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)

	// IBC entry points
	IBCChannelOpen(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCChannelConnect(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCChannelClose(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketReceive(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketAck(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCPacketTimeout(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCSourceCallback(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)
	IBCDestinationCallback(checksum []byte, env []byte, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error)

	// Metrics
	GetMetrics() (*types.Metrics, error)
	GetPinnedMetrics() (*types.PinnedMetrics, error)
}
