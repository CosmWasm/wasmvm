package validation

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
)

func (w *WazeroRuntime) analyzeForValidation(compiled wazero.CompiledModule) error {
	// Check memory constraints
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return fmt.Errorf("Error during static Wasm validation: Wasm contract must contain exactly one memory")
	}

	// Ensure required exports (e.g., "allocate", "deallocate")
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
			return fmt.Errorf("Wasm contract doesn't have required export: %q", r)
		}
	}

	// Ensure interface_version_8
	var interfaceVersionCount int
	for name := range exports {
		if strings.HasPrefix(name, "interface_version_") {
			interfaceVersionCount++
			if name != "interface_version_8" {
				return fmt.Errorf("Wasm contract has unknown %q marker", name)
			}
		}
	}
	if interfaceVersionCount == 0 {
		return fmt.Errorf("Wasm contract missing required marker export: interface_version_*")
	}

	return nil
}
