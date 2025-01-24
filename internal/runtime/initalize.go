package runtime

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/CosmWasm/wasmvm/v2/types"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// initializeModules prepares and instantiates both the host and contract modules
func (w *WazeroRuntime) initializeModules(ctx context.Context, checksum []byte, hostModule wazero.CompiledModule) (api.Module, error) {
	// First instantiate the environment module (host module)
	moduleConfig := wazero.NewModuleConfig().
		WithName("env").
		WithStartFunctions() // Don't auto-start

	envModule, err := w.runtime.InstantiateModule(ctx, hostModule, moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate env module: %w", err)
	}
	defer envModule.Close(ctx)

	// Get the contract module from cache using checksum
	w.mu.Lock()
	csHex := hex.EncodeToString(checksum)
	compiledModule, ok := w.compiledModules[csHex]
	if !ok {
		w.mu.Unlock()
		return nil, fmt.Errorf("module not found for checksum: %x", checksum)
	}
	w.mu.Unlock()

	// Analyze the code to ensure it has required exports
	analysis, err := w.AnalyzeCode(checksum)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze code: %w", err)
	}

	// Validate required capabilities
	if err := w.validateCapabilities(analysis); err != nil {
		return nil, err
	}

	// Create contract module config
	contractConfig := wazero.NewModuleConfig().
		WithName("contract").
		WithStartFunctions()

	// Instantiate the contract module
	contractModule, err := w.runtime.InstantiateModule(ctx, compiledModule, contractConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate contract module: %w", err)
	}

	// Verify contract has required exports
	if err := w.verifyContractExports(contractModule); err != nil {
		contractModule.Close(ctx)
		return nil, err
	}

	return contractModule, nil
}

// validateCapabilities checks if the contract's required capabilities are supported
func (w *WazeroRuntime) validateCapabilities(analysis *types.AnalysisReport) error {
	if analysis.RequiredCapabilities == "" {
		return nil // No special capabilities required
	}

	// Split capabilities string and check each one
	capabilities := strings.Split(analysis.RequiredCapabilities, ",")
	for _, cap := range capabilities {
		cap = strings.TrimSpace(cap)
		// Add your capability validation logic here
		// For example:
		switch cap {
		case "iterator":
			// Iterator capability is supported
		case "staking":
			// Staking capability is supported
		default:
			return fmt.Errorf("unsupported capability required by contract: %s", cap)
		}
	}

	return nil
}

// verifyContractExports checks that the contract module exports required functions
func (w *WazeroRuntime) verifyContractExports(contractModule api.Module) error {
	requiredExports := []string{
		"allocate",
		"deallocate",
		"instantiate",
	}

	for _, required := range requiredExports {
		if contractModule.ExportedFunction(required) == nil {
			return fmt.Errorf("contract missing required export: %s", required)
		}
	}

	// Verify memory export
	if contractModule.Memory() == nil {
		return fmt.Errorf("contract must export memory")
	}

	return nil
}
