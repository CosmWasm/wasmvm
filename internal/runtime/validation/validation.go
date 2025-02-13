package validation

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
)

// AnalyzeForValidation validates a compiled module to ensure it meets the CosmWasm requirements.
// It ensures the module has exactly one exported memory, that the required exports ("allocate", "deallocate")
// are present, and that the contract's interface marker export is exactly "interface_version_8".
func AnalyzeForValidation(compiled wazero.CompiledModule) error {
	// Check memory constraints: exactly one memory export is required.
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return fmt.Errorf("static Wasm validation error: contract must contain exactly one memory (found %d)", memoryCount)
	}

	// Ensure required exports (e.g., "allocate" and "deallocate") are present.
	requiredExports := []string{"allocate", "deallocate"}
	exports := compiled.ExportedFunctions()
	for _, r := range requiredExports {
		found := false
		for name := range exports {
			if name == r {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("static Wasm validation error: contract missing required export %q", r)
		}
	}

	// Ensure the interface version marker is present.
	var interfaceVersionCount int
	for name := range exports {
		if strings.HasPrefix(name, "interface_version_") {
			interfaceVersionCount++
			if name != "interface_version_8" {
				return fmt.Errorf("static Wasm validation error: unknown interface version marker %q", name)
			}
		}
	}
	if interfaceVersionCount == 0 {
		return fmt.Errorf("static Wasm validation error: contract missing required interface version marker (interface_version_*)")
	}

	return nil
}
