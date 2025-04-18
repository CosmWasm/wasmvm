package types

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Checksum represents a unique identifier for a Wasm contract.
// It is typically a SHA-256 hash of the contract's bytecode.
type Checksum [ChecksumLen]byte

func (cs Checksum) String() string {
	return hex.EncodeToString(cs[:])
}

// MarshalJSON implements the json.Marshaler interface for Checksum.
// It converts the checksum to a hex-encoded string.
func (cs Checksum) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(cs[:]))
}

// UnmarshalJSON implements the json.Unmarshaler interface for Checksum.
// It parses a hex-encoded string into a checksum.
func (cs *Checksum) UnmarshalJSON(input []byte) error {
	var hexString string
	err := json.Unmarshal(input, &hexString)
	if err != nil {
		return err
	}

	data, err := hex.DecodeString(hexString)
	if err != nil {
		return err
	}
	if len(data) != ChecksumLen {
		return fmt.Errorf("got wrong number of bytes for checksum")
	}
	copy(cs[:], data)
	return nil
}

// ChecksumLen is the length of a checksum in bytes.
const ChecksumLen = 32

// ForceNewChecksum creates a Checksum instance from a hex string.
// It panics in case the input is invalid.
func ForceNewChecksum(input string) Checksum {
	data, err := hex.DecodeString(input)
	if err != nil {
		panic("could not decode hex bytes")
	}
	if len(data) != ChecksumLen {
		panic("got wrong number of bytes")
	}
	var cs Checksum
	copy(cs[:], data)
	return cs
}

// Bytes returns the checksum as a byte slice.
func (cs Checksum) Bytes() []byte {
	return cs[:]
}

// NewChecksum creates a new Checksum from a byte slice.
// Returns an error if the slice length is not ChecksumLen.
func NewChecksum(b []byte) (Checksum, error) {
	if len(b) != ChecksumLen {
		return Checksum{}, errors.New("got wrong number of bytes for checksum")
	}
	var cs Checksum
	copy(cs[:], b)
	return cs, nil
}
