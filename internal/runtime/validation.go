package runtime

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
)

func (w *WazeroRuntime) analyzeForValidation(compiled wazero.CompiledModule) error {
	// 1) Check memory constraints
	memoryCount := 0
	for _, exp := range compiled.ExportedMemories() {
		if exp != nil {
			memoryCount++
		}
	}
	if memoryCount != 1 {
		return fmt.Errorf("Error during static Wasm validation: Wasm contract must contain exactly one memory")
	}

	// 2) Gather exported function names
	exports := compiled.ExportedFunctions()
	var exportNames []string
	for name := range exports {
		exportNames = append(exportNames, name)
	}

	// 3) Ensure interface_version_8
	var interfaceVersionCount int
	for _, name := range exportNames {
		if strings.HasPrefix(name, "interface_version_") {
			interfaceVersionCount++
			if name != "interface_version_8" {
				return fmt.Errorf("Wasm contract has unknown %q marker (expect interface_version_8)", name)
			}
		}
	}
	if interfaceVersionCount == 0 {
		return fmt.Errorf("Wasm contract missing a required marker export: interface_version_* (expected interface_version_8)")
	}
	if interfaceVersionCount > 1 {
		return fmt.Errorf("Wasm contract contains more than one marker export: interface_version_*")
	}

	// 4) Ensure allocate + deallocate
	//    (Rust's check_wasm_exports)
	requiredExports := []string{"allocate", "deallocate"}
	for _, r := range requiredExports {
		found := false
		for _, expName := range exportNames {
			if expName == r {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Wasm contract doesn't have required export: %q", r)
		}
	}

	// 5) Possibly check function import constraints
	//    (like "db_read", "db_write", etc.)
	//    But note Wazero doesn't give a direct function to list imports from the compiled module.
	//    You might parse your Wasm differently (like using wasmer/wasmparser).
	//    Or skip if you don't need strict import checks.

	// 6) Check for "requires_*" exports (capabilities)
	//    e.g. "requires_iter", "requires_stargate", etc.
	var requiredCaps []string
	prefix := "requires_"
	for _, expName := range exportNames {
		if strings.HasPrefix(expName, prefix) && len(expName) > len(prefix) {
			capName := expName[len(prefix):]             // everything after "requires_"
			requiredCaps = append(requiredCaps, capName) //nolint:staticcheck
		}
	}

	// Compare requiredCaps to your chain's available capabilities
	// For example:
	//   chainCaps := ... // from config, or from capabilities_from_csv
	//   for _, c := range requiredCaps {
	//       if !chainCaps.Contains(c) {
	//           return fmt.Errorf("Wasm contract requires unavailable capability: %s", c)
	//       }
	//   }

	// 7) If you want function count or param-limits, you'd need a deeper parse. Wazero alone
	//    doesn't expose param counts of every function. You might do a custom parser.

	return nil
}
