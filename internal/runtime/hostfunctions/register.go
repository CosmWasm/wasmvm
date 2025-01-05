package hostfunctions

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	envKey contextKey = "env"
)

// RegisterHostFunctions registers all host functions with the wazero runtime
func RegisterHostFunctions(runtime wazero.Runtime, env *runtime.RuntimeEnvironment) (wazero.CompiledModule, error) {
	builder := runtime.NewHostModuleBuilder("env")

	// Register abort function
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, code uint32) {
			ctx = context.WithValue(ctx, envKey, env)
			panic(fmt.Sprintf("Wasm contract aborted with code: %d (0x%x)", code, code))
		}).
		WithParameterNames("code").
		Export("abort")

	// Register BLS12-381 functions
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, g1sPtr, outPtr uint32) uint32 {
			ctx = context.WithValue(ctx, envKey, env)
			ptr, err := env.Crypto.BLS12381AggregateG1Host(g1sPtr)
			if err != nil {
				panic(err)
			}
			return ptr
		}).
		WithParameterNames("g1s_ptr", "out_ptr").
		WithResultNames("result").
		Export("bls12_381_aggregate_g1")

	// Add other host functions here...

	return builder.Compile(context.Background())
}
