package runtime

import (
	"encoding/hex"
	"fmt"
	"runtime"
	"time"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/constants"
	"github.com/tetratelabs/wazero/api"
)

// TraceConfig controls tracing behavior
type TraceConfig struct {
	Enabled     bool
	ShowMemory  bool
	ShowParams  bool
	ShowStack   bool
	MaxDataSize uint32 // Maximum bytes of data to print
}

// Global trace configuration - can be modified at runtime
var TraceConf = TraceConfig{
	Enabled:     true,
	ShowMemory:  true,
	ShowParams:  true,
	ShowStack:   true,
	MaxDataSize: 256,
}

// TraceFn wraps a function with tracing
func TraceFn(name string) func() {
	if !TraceConf.Enabled {
		return func() {}
	}

	start := time.Now()

	// Get caller information
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)

	// Print entry trace
	fmt.Printf("\n=== ENTER: %s ===\n", name)
	fmt.Printf("Location: %s:%d\n", file, line)
	fmt.Printf("Function: %s\n", fn.Name())

	if TraceConf.ShowStack {
		// Capture and print stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		fmt.Printf("Stack:\n%s\n", string(buf[:n]))
	}

	// Return function to be deferred
	return func() {
		duration := time.Since(start)
		fmt.Printf("=== EXIT: %s (took %v) ===\n\n", name, duration)
	}
}

// TraceMemory prints memory state if enabled
func TraceMemory(memory api.Memory, msg string) {
	if !TraceConf.Enabled || !TraceConf.ShowMemory {
		return
	}

	fmt.Printf("\n=== Memory State: %s ===\n", msg)
	fmt.Printf("Size: %d bytes (%d pages)\n", memory.Size(), memory.Size()/constants.WasmPageSize)

	// Print first page contents
	if data, ok := memory.Read(0, TraceConf.MaxDataSize); ok {
		fmt.Printf("First %d bytes:\n%s\n", TraceConf.MaxDataSize, hex.Dump(data))
	}
}

// TraceParams prints parameter values if enabled
func TraceParams(params ...interface{}) {
	if !TraceConf.Enabled || !TraceConf.ShowParams {
		return
	}

	fmt.Printf("Parameters:\n")
	for i, p := range params {
		// Handle different parameter types appropriately
		switch v := p.(type) {
		case []byte:
			if uint32(len(v)) > TraceConf.MaxDataSize {
				fmt.Printf("  %d: []byte len=%d (truncated)\n", i, len(v))
				fmt.Printf("     %x...\n", v[:int(TraceConf.MaxDataSize)])
			} else {
				fmt.Printf("  %d: []byte %x\n", i, v)
			}
		default:
			fmt.Printf("  %d: %v\n", i, p)
		}
	}
}
