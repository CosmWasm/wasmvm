// file: internal/runtime/wazero_runtime.go
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type WazeroRuntime struct {
	mu      sync.Mutex
	runtime wazero.Runtime

	codeCache       map[string][]byte
	compiledModules map[string]wazero.CompiledModule
}

func NewWazeroRuntime() (*WazeroRuntime, error) {
	r := wazero.NewRuntime(ctxWithCloseOnDone())
	return &WazeroRuntime{
		runtime:         r,
		codeCache:       make(map[string][]byte),
		compiledModules: make(map[string]wazero.CompiledModule),
	}, nil
}

func ctxWithCloseOnDone() context.Context {
	return context.Background()
}

func (w *WazeroRuntime) InitCache(config types.VMConfig) (any, error) {
	// In a cgo-based runtime, we might need to initialize something.
	// For wazero, we currently have no special cache init logic.
	// Just return a handle (this runtime itself)
	return w, nil
}

func (w *WazeroRuntime) ReleaseCache(handle any) {
	// Close the wazero runtime to free resources.
	// After this call, the runtime cannot be used again.
	w.runtime.Close(context.Background())
}

func (w *WazeroRuntime) StoreCode(code []byte) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	checksum := sha256.Sum256(code)
	csHex := hex.EncodeToString(checksum[:])

	if _, exists := w.compiledModules[csHex]; exists {
		// already stored
		return checksum[:], nil
	}

	compiled, err := w.runtime.CompileModule(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to compile module: %w", err)
	}

	w.codeCache[csHex] = code
	w.compiledModules[csHex] = compiled
	return checksum[:], nil
}

func (w *WazeroRuntime) GetCode(checksum []byte) ([]byte, error) {
	csHex := hex.EncodeToString(checksum)
	w.mu.Lock()
	defer w.mu.Unlock()

	code, ok := w.codeCache[csHex]
	if !ok {
		return nil, errors.New("code not found")
	}
	return code, nil
}

func (w *WazeroRuntime) RemoveCode(checksum []byte) error {
	csHex := hex.EncodeToString(checksum)
	w.mu.Lock()
	defer w.mu.Unlock()

	mod, ok := w.compiledModules[csHex]
	if !ok {
		return errors.New("wasm file does not exist")
	}
	mod.Close(context.Background())
	delete(w.compiledModules, csHex)
	delete(w.codeCache, csHex)
	return nil
}

func (w *WazeroRuntime) Pin(checksum []byte) error {
	// no-op for wazero
	return nil
}

func (w *WazeroRuntime) Unpin(checksum []byte) error {
	// no-op for wazero
	return nil
}

func (w *WazeroRuntime) AnalyzeCode(checksum []byte) (*types.AnalysisReport, error) {
	// no actual analysis in wazero
	return &types.AnalysisReport{
		HasIBCEntryPoints:      false,
		RequiredCapabilities:   "",
		Entrypoints:            []string{},
		ContractMigrateVersion: nil,
	}, nil
}

func (w *WazeroRuntime) Instantiate(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("instantiate", checksum, env, info, msg)
}

func (w *WazeroRuntime) Execute(checksum, env, info, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("execute", checksum, env, info, msg)
}

func (w *WazeroRuntime) Migrate(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("migrate", checksum, env, nil, msg)
}

// If you don't have a distinct MigrateWithInfo in your wasm code, you can just call Migrate or handle migrateInfo differently
func (w *WazeroRuntime) MigrateWithInfo(checksum, env, msg, migrateInfo []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	// For now, just call migrate. A real implementation might store migrateInfo in memory and call a special function.
	return w.Migrate(checksum, env, msg, otherParams...)
}

func (w *WazeroRuntime) Sudo(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("sudo", checksum, env, nil, msg)
}

func (w *WazeroRuntime) Reply(checksum, env, reply []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("reply", checksum, env, nil, reply)
}

func (w *WazeroRuntime) Query(checksum, env, query []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("query", checksum, env, nil, query)
}

func (w *WazeroRuntime) IBCChannelOpen(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_channel_open", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCChannelConnect(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_channel_connect", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCChannelClose(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_channel_close", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketReceive(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_packet_receive", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketAck(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_packet_ack", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCPacketTimeout(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_packet_timeout", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCSourceCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_source_callback", checksum, env, nil, msg)
}

func (w *WazeroRuntime) IBCDestinationCallback(checksum, env, msg []byte, otherParams ...interface{}) ([]byte, types.GasReport, error) {
	return w.callContractFn("ibc_destination_callback", checksum, env, nil, msg)
}

func (w *WazeroRuntime) GetMetrics() (*types.Metrics, error) {
	return &types.Metrics{}, nil
}

func (w *WazeroRuntime) GetPinnedMetrics() (*types.PinnedMetrics, error) {
	return &types.PinnedMetrics{}, nil
}

func (w *WazeroRuntime) callContractFn(fnName string, checksum, env, info, msg []byte) ([]byte, types.GasReport, error) {
	csHex := hex.EncodeToString(checksum)
	w.mu.Lock()
	compiled, ok := w.compiledModules[csHex]
	w.mu.Unlock()
	if !ok {
		return nil, types.GasReport{}, errors.New("unknown code checksum")
	}

	modConfig := wazero.NewModuleConfig().WithName("contract")
	ctx := context.Background()
	module, err := w.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer module.Close(ctx)

	envPtr, envLen, err := writeToWasmMemory(module, env)
	if err != nil {
		return nil, types.GasReport{}, err
	}
	infoPtr, infoLen, err := writeToWasmMemory(module, info)
	if err != nil {
		return nil, types.GasReport{}, err
	}
	msgPtr, msgLen, err := writeToWasmMemory(module, msg)
	if err != nil {
		return nil, types.GasReport{}, err
	}

	fn := module.ExportedFunction(fnName)
	if fn == nil {
		return nil, types.GasReport{}, fmt.Errorf("function %s not found", fnName)
	}

	results, err := fn.Call(ctx,
		uint64(envPtr), uint64(envLen),
		uint64(infoPtr), uint64(infoLen),
		uint64(msgPtr), uint64(msgLen),
	)
	if err != nil {
		return nil, types.GasReport{}, fmt.Errorf("call failed: %w", err)
	}

	if len(results) < 2 {
		return nil, types.GasReport{}, fmt.Errorf("function %s returned too few results", fnName)
	}

	dataPtr := uint32(results[0])
	dataLen := uint32(results[1])

	data, ok2 := module.Memory().Read(dataPtr, dataLen)
	if !ok2 {
		return nil, types.GasReport{}, fmt.Errorf("failed to read return data")
	}

	gr := types.GasReport{
		Limit:          1_000_000_000,
		Remaining:      500_000_000,
		UsedExternally: 0,
		UsedInternally: 500_000_000,
	}

	return data, gr, nil
}

func writeToWasmMemory(module api.Module, data []byte) (uint32, uint32, error) {
	if len(data) == 0 {
		return 0, 0, nil
	}
	offset := uint32(1024)
	mem := module.Memory()
	if !mem.Write(offset, data) {
		return 0, 0, fmt.Errorf("failed to write data to memory")
	}
	return offset, uint32(len(data)), nil
}
