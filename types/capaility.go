package types

import (
	"fmt"
	"strings"
)

type Capability string

const (
	CapIterator     Capability = "iterator"
	CapStaking      Capability = "staking"
	CapStargate     Capability = "stargate"
	CapCosmwasmV1_1 Capability = "cosmwasm_1_1"
	CapCosmwasmV1_2 Capability = "cosmwasm_1_2"
)

// AllCapabilities returns all capabilities available
func AllCapabilities() []Capability {
	return []Capability{
		CapIterator,
		CapStaking,
		CapStargate,
		CapCosmwasmV1_1,
		CapCosmwasmV1_2,
	}
}

// Capabilities defines a list of capabilities
type Capabilities []Capability

// Validate ensures the list of capabilities contains only defined types and no duplicates
func (c Capabilities) Validate() error {
	idx := make(map[Capability]struct{}, len(c))
	for _, v := range c {
		if !isCapability(v) {
			return fmt.Errorf("not a capability: %q", v)
		}
		if _, exists := idx[v]; exists {
			return fmt.Errorf("duplicate: %q", v)
		}
		idx[v] = struct{}{}
	}
	return nil
}

func isCapability(c Capability) bool {
	for _, v := range AllCapabilities() {
		if v == c {
			return true
		}
	}
	return false
}

// Serialize converts the capabilities into a comma separated string representation
func (c Capabilities) Serialize() string {
	s := make([]string, len(c))
	for i, v := range c {
		s[i] = string(v)
	}
	return strings.Join(s, ",")
}
