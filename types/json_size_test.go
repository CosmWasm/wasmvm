package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectedJSONSizeString(t *testing.T) {
	specs := map[string]struct {
		input string
	}{
		"empty": {
			input: ``,
		},
		"hello": {
			input: `hello`,
		},
		"quotes": {
			input: `some "thing"`,
		},
		"backspack": {
			input: `\`,
		},
		"html": {
			input: `<html>`,
		},
		"null": {
			input: "\x00 terminated",
		},
		"control character 1": {
			input: "\x01",
		},
		"control character 2": {
			input: "\x02",
		},
		"control character 3": {
			input: "\x03",
		},
		"control character 31": {
			input: "\x1F",
		},
		"control character DEL": {
			input: "\x7F",
		},
		"emoji": {
			input: "😮😗🧝🏾‍♀️",
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), ExpectedJSONSizeString(spec.input))
		})
	}
}

func TestExpectedJSONSizeBinary(t *testing.T) {
	specs := map[string]struct {
		input []byte
	}{
		"default": {
			input: nil,
		},
		"empty": {
			input: []byte{},
		},
		"a": {
			input: []byte{'a'},
		},
		"three": {
			input: []byte{1, 2, 3},
		},
		"four": {
			input: []byte{1, 2, 3, 4},
		},
		"five": {
			input: []byte{1, 2, 3, 4, 5},
		},
		"long": {
			input: []byte{0xCD, 0x6F, 0xF9, 0xFF, 0x3F, 0xAE, 0xA9, 0xB3, 0xD0, 0x22, 0x4A, 0x7E, 0x0D, 0x11, 0x33, 0xE6, 0xEA, 0xE1, 0xB7, 0x80, 0x0E, 0x83, 0x92, 0x44, 0x12, 0x00, 0x05, 0x69, 0x86, 0xC3, 0x58, 0xA9},
		},
		"very long": {
			input: []byte{
				// 1653 bytes of a gzipped FileDescriptorProto
				0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xcc, 0x59, 0xcf, 0x6f, 0x13, 0xc7,
				0x17, 0xcf, 0x04, 0xc7, 0x71, 0x26, 0xf9, 0x7e, 0x71, 0xe6, 0x1b, 0x20, 0x18, 0xb0, 0xa3, 0x05,
				0x42, 0x08, 0xc4, 0x4b, 0xc2, 0x97, 0x46, 0xd0, 0x43, 0x15, 0x07, 0x4a, 0x40, 0x50, 0x82, 0x91,
				0x8a, 0xd4, 0xaa, 0x72, 0xc7, 0xf6, 0xc4, 0xd9, 0xd6, 0xde, 0x35, 0x3b, 0x13, 0x42, 0x14, 0x85,
				0x03, 0xa7, 0x4a, 0x3d, 0xb4, 0x55, 0x4f, 0xa5, 0x52, 0x7f, 0x48, 0x3d, 0xd0, 0xd2, 0x4a, 0x48,
				0xad, 0x54, 0x54, 0xa9, 0xf7, 0x5c, 0x2a, 0xa1, 0xf6, 0xd2, 0x93, 0xd5, 0x86, 0x4a, 0x54, 0xfc,
				0x09, 0x9c, 0xaa, 0x9d, 0x7d, 0xeb, 0x5d, 0xff, 0x18, 0xdb, 0x04, 0x1f, 0x7a, 0x31, 0xbb, 0x3b,
				0xef, 0xbd, 0xf9, 0xcc, 0xe7, 0xfd, 0x98, 0xf7, 0x02, 0xde, 0x9f, 0xb3, 0x78, 0x69, 0x95, 0xf2,
				0x92, 0x2e, 0x7f, 0x6e, 0x4e, 0xeb, 0x37, 0x56, 0x98, 0xbd, 0x96, 0x2c, 0xdb, 0x96, 0xb0, 0x48,
				0xd4, 0x5b, 0x4d, 0xca, 0x9f, 0x9b, 0xd3, 0xb1, 0x91, 0x82, 0x55, 0xb0, 0xe4, 0xa2, 0xee, 0x3c,
				0xb9, 0x72, 0xb1, 0x46, 0x2b, 0x62, 0xad, 0xcc, 0xb8, 0xb7, 0x5a, 0xb0, 0xac, 0x42, 0x91, 0xe9,
				0xb4, 0x6c, 0xe8, 0xd4, 0x34, 0x2d, 0x41, 0x85, 0x61, 0x99, 0xde, 0xea, 0xa4, 0xa3, 0x6b, 0x71,
				0x3d, 0x4b, 0x39, 0x73, 0x37, 0xd7, 0x6f, 0x4e, 0x67, 0x99, 0xa0, 0xd3, 0x7a, 0x99, 0x16, 0x0c,
				0x53, 0x0a, 0x83, 0xec, 0x3e, 0x90, 0xf5, 0xc4, 0x82, 0x60, 0x63, 0xc3, 0xb4, 0x64, 0x98, 0x96,
				0x2e, 0x7f, 0xe1, 0xd3, 0x5e, 0x57, 0x3e, 0xe3, 0x02, 0x76, 0x5f, 0xdc, 0x25, 0xed, 0x35, 0x3c,
				0x7a, 0xd5, 0x51, 0x9e, 0xb7, 0x4c, 0x61, 0xd3, 0x9c, 0xb8, 0x60, 0x2e, 0x59, 0x69, 0x76, 0x63,
				0x85, 0x71, 0x41, 0x66, 0x70, 0x3f, 0xcd, 0xe7, 0x6d, 0xc6, 0xf9, 0x28, 0x1a, 0x43, 0x13, 0x03,
				0xa9, 0xd1, 0x5f, 0x7f, 0x98, 0x1a, 0x01, 0xf5, 0x39, 0x77, 0xe5, 0x9a, 0xb0, 0x0d, 0xb3, 0x90,
				0xf6, 0x04, 0xb5, 0xef, 0x10, 0xde, 0xdb, 0xc4, 0x20, 0x2f, 0x5b, 0x26, 0x67, 0xdb, 0xb1, 0x48,
				0x5e, 0xc7, 0xff, 0xc9, 0x81, 0xad, 0x8c, 0x61, 0x2e, 0x59, 0xa3, 0xbd, 0x63, 0x68, 0x62, 0x70,
				0x26, 0x9e, 0xac, 0x77, 0x4a, 0x32, 0xb8, 0x65, 0x6a, 0x78, 0xb3, 0x92, 0xe8, 0x79, 0x54, 0x49,
				0xa0, 0xa7, 0x95, 0x44, 0xcf, 0xbd, 0x27, 0x0f, 0x26, 0x51, 0x7a, 0x28, 0x17, 0x10, 0x38, 0x13,
				0xfa, 0xfb, 0x8b, 0x04, 0xd2, 0x3e, 0x41, 0x78, 0x5f, 0x0d, 0xde, 0x05, 0x83, 0x0b, 0xcb, 0x5e,
				0x7b, 0x01, 0x0e, 0xc8, 0xab, 0x18, 0xfb, 0x2e, 0x03, 0xb8, 0xe3, 0x49, 0xd0, 0x71, 0xfc, 0x9b,
				0x74, 0xfd, 0x05, 0xfe, 0x4d, 0x2e, 0xd2, 0x02, 0x83, 0xfd, 0xd2, 0x01, 0x4d, 0xed, 0x21, 0xc2,
				0xfb, 0x9b, 0x63, 0x03, 0x3a, 0xaf, 0xe0, 0x7e, 0x66, 0x0a, 0xdb, 0x60, 0x0e, 0xb8, 0x1d, 0x13,
				0x83, 0x33, 0x93, 0x6a, 0x52, 0xe6, 0xad, 0x3c, 0x03, 0xfd, 0x73, 0xa6, 0xb0, 0xd7, 0x52, 0x03,
				0x9b, 0x55, 0x62, 0x3c, 0x2b, 0xe4, 0x7c, 0x13, 0xe4, 0x47, 0xda, 0x22, 0x77, 0xd1, 0xd4, 0x40,
				0xbf, 0x5d, 0xc7, 0x2a, 0x4f, 0xad, 0x39, 0x00, 0x3c, 0x56, 0xf7, 0xe0, 0xfe, 0x9c, 0x95, 0x67,
				0x19, 0x23, 0x2f, 0x59, 0x0d, 0xa5, 0xc3, 0xce, 0xeb, 0x85, 0x7c, 0xd7, 0xa8, 0xfb, 0xbc, 0x9e,
				0xba, 0x2a, 0x00, 0xa0, 0xee, 0x25, 0x3c, 0xe0, 0x45, 0x83, 0x4b, 0x5e, 0x2b, 0xcf, 0xfa, 0xa2,
				0xdd, 0x63, 0xe8, 0xae, 0x87, 0x70, 0xae, 0x58, 0xf4, 0x40, 0x5e, 0x13, 0x54, 0xb0, 0x7f, 0x43,
				0xe4, 0x7d, 0x85, 0xf0, 0x01, 0x05, 0x38, 0xe0, 0xef, 0x0c, 0x0e, 0x97, 0xac, 0x3c, 0x2b, 0x7a,
				0x91, 0xb7, 0xa7, 0x31, 0xf2, 0x2e, 0x3b, 0xeb, 0xc1, 0x30, 0x03, 0x8d, 0xee, 0x71, 0x78, 0x03,
				0x28, 0x4c, 0xd3, 0xd5, 0xae, 0x51, 0x78, 0x00, 0x63, 0xb9, 0x7b, 0x26, 0x4f, 0x05, 0x95, 0xe0,
				0x86, 0xd2, 0x03, 0xf2, 0xcb, 0x59, 0x2a, 0xa8, 0x76, 0x12, 0x88, 0x69, 0xdc, 0x12, 0x88, 0x21,
				0x38, 0x24, 0x35, 0x91, 0xd4, 0x94, 0xcf, 0xda, 0xa7, 0x08, 0xc7, 0xa5, 0xd6, 0xb5, 0x12, 0xb5,
				0x45, 0xd7, 0xa0, 0x9e, 0x6b, 0x84, 0x9a, 0x1a, 0x7f, 0x56, 0x49, 0x90, 0x00, 0xb8, 0xcb, 0x8c,
				0x73, 0x5a, 0x60, 0x77, 0x9f, 0x3c, 0x98, 0x1c, 0x34, 0xcc, 0xa2, 0x61, 0xb2, 0xcc, 0x3b, 0xdc,
				0x32, 0x83, 0x47, 0x7a, 0x0b, 0x27, 0x94, 0xe0, 0xaa, 0xde, 0x0e, 0x1c, 0xaa, 0xe3, 0x3d, 0xdc,
				0xc3, 0x1f, 0xc3, 0x51, 0xc8, 0xc4, 0xf6, 0xf9, 0xaf, 0xe9, 0x78, 0xa4, 0x2a, 0x1c, 0xbc, 0x8a,
				0x94, 0x0a, 0xdf, 0xf4, 0xe2, 0x5d, 0x75, 0x1a, 0x80, 0xf9, 0x60, 0x9d, 0x4a, 0x0a, 0x6f, 0x55,
				0x12, 0x61, 0x29, 0x76, 0xb6, 0x5a, 0x6f, 0x66, 0x70, 0x7f, 0xce, 0x66, 0x54, 0x58, 0xb6, 0xe4,
				0xaf, 0x25, 0xed, 0x20, 0x48, 0x16, 0x71, 0x24, 0xb7, 0xcc, 0x72, 0xef, 0xf2, 0x95, 0xd2, 0xe8,
				0x0e, 0x49, 0xc8, 0xff, 0x9f, 0x55, 0x12, 0x27, 0x0a, 0x86, 0x58, 0x5e, 0xc9, 0x26, 0x73, 0x56,
				0x49, 0xcf, 0x59, 0x25, 0x26, 0xb2, 0x4b, 0xc2, 0x7f, 0x28, 0x1a, 0x59, 0xae, 0x67, 0xd7, 0x04,
				0xe3, 0xc9, 0x05, 0x76, 0x2b, 0xe5, 0x3c, 0xa4, 0xab, 0x56, 0xc8, 0xdb, 0x78, 0xb7, 0x61, 0x72,
				0x41, 0x4d, 0x61, 0x50, 0xc1, 0x32, 0x65, 0x66, 0x97, 0x0c, 0xce, 0x9d, 0xe4, 0x08, 0xa9, 0xee,
				0xba, 0xb9, 0x5c, 0x8e, 0x71, 0x3e, 0x6f, 0x99, 0x4b, 0x46, 0x21, 0x98, 0x63, 0xbb, 0x02, 0x86,
				0x16, 0xab, 0x76, 0xe0, 0xb2, 0x7b, 0xd8, 0x8b, 0xa3, 0x0d, 0x3c, 0x1d, 0xad, 0xe7, 0x29, 0xea,
				0xf3, 0xf4, 0xb4, 0x92, 0xe8, 0x35, 0xf2, 0x2f, 0xc4, 0xd6, 0x55, 0x3c, 0xe0, 0x84, 0x41, 0x66,
				0x99, 0xf2, 0xe5, 0x17, 0xa3, 0xcb, 0x31, 0xb3, 0x40, 0xf9, 0x72, 0x0b, 0xba, 0xc2, 0xdd, 0xa4,
				0xeb, 0x62, 0x28, 0x12, 0x8a, 0xf6, 0x5d, 0x0c, 0x45, 0xfa, 0xa2, 0x61, 0xed, 0x0e, 0xc2, 0xc3,
				0x81, 0x30, 0x06, 0xee, 0x2e, 0x38, 0xb7, 0x88, 0xc3, 0x9d, 0xd3, 0x97, 0x20, 0xb9, 0xb9, 0xd6,
				0xec, 0x0a, 0xae, 0xa5, 0x3c, 0x15, 0xf1, 0xfa, 0x92, 0x74, 0x24, 0x07, 0x6b, 0x64, 0x3f, 0xa4,
				0x98, 0x9b, 0xc6, 0x91, 0xa7, 0x95, 0x84, 0x7c, 0x77, 0x93, 0x08, 0xfc, 0xf7, 0x66, 0x00, 0x03,
				0xf7, 0x52, 0xa3, 0xb6, 0xe6, 0xa3, 0x6d, 0xd7, 0xfc, 0xfb, 0x08, 0x93, 0xa0, 0x75, 0x38, 0xe2,
				0x25, 0x8c, 0xab, 0x47, 0xf4, 0x8a, 0x7d, 0x27, 0x67, 0x0c, 0x90, 0x3c, 0xe0, 0x1d, 0xb2, 0x8b,
				0xa5, 0x9f, 0xe2, 0x3d, 0x12, 0xec, 0xa2, 0x61, 0x9a, 0x2c, 0xdf, 0x82, 0x90, 0xed, 0x5f, 0x82,
				0xef, 0x23, 0xe8, 0x8d, 0x6b, 0xf6, 0x00, 0x5a, 0xc6, 0x71, 0x04, 0xb2, 0xc6, 0x25, 0x25, 0x94,
				0x1a, 0xdc, 0xaa, 0x24, 0xfa, 0xdd, 0xb4, 0xe1, 0xe9, 0x7e, 0x37, 0x63, 0xba, 0x78, 0xe0, 0x11,
				0xf0, 0xce, 0x22, 0xb5, 0x69, 0xc9, 0x3b, 0xab, 0x96, 0xc6, 0xff, 0xab, 0xf9, 0x0a, 0xe8, 0x5e,
				0xc6, 0xe1, 0xb2, 0xfc, 0x02, 0xf1, 0x30, 0xda, 0xe8, 0x30, 0x57, 0xa3, 0xe6, 0x7a, 0x76, 0x55,
				0x9c, 0x40, 0x88, 0x37, 0xf4, 0x4e, 0x6e, 0x36, 0x7b, 0x14, 0xcf, 0xe1, 0x9d, 0x90, 0xdf, 0x99,
				0x4e, 0x6f, 0xad, 0xff, 0x82, 0xc2, 0x5c, 0x97, 0x5b, 0x95, 0xef, 0x11, 0x5c, 0x5f, 0xcd, 0xd0,
				0x02, 0x1d, 0xe7, 0x31, 0xa9, 0x8e, 0x10, 0x80, 0x97, 0xb5, 0xef, 0xfa, 0x86, 0x3d, 0x9d, 0x39,
				0x4f, 0xa5, 0x7b, 0xde, 0x8c, 0x43, 0xe7, 0x72, 0x9d, 0xf2, 0xd2, 0x25, 0xa3, 0x64, 0x08, 0xa8,
				0x4d, 0x9e, 0x5f, 0x67, 0xa1, 0xcd, 0x68, 0x5c, 0x87, 0x23, 0xed, 0xc6, 0xe1, 0x9c, 0xfc, 0xe2,
				0x12, 0x9f, 0x86, 0x37, 0xc7, 0x79, 0x6e, 0xd0, 0xa6, 0x56, 0x8c, 0x62, 0x1e, 0x90, 0x7b, 0x6e,
				0xdb, 0x07, 0xe5, 0x4a, 0xd6, 0x62, 0x57, 0x4f, 0x46, 0xb1, 0xac, 0xaa, 0x4d, 0x7c, 0xda, 0xfb,
				0x9c, 0x3e, 0x25, 0x38, 0xc4, 0x69, 0x51, 0xc8, 0x32, 0x3f, 0x90, 0x96, 0xcf, 0xce, 0x9e, 0x86,
				0x69, 0x88, 0x0c, 0xb5, 0x0b, 0x5c, 0x5e, 0x67, 0x43, 0xe9, 0x88, 0xf3, 0x61, 0xce, 0x2e, 0x70,
				0xed, 0x0a, 0x0c, 0x8b, 0xb5, 0x60, 0xb7, 0x3f, 0x2c, 0xce, 0xfc, 0x32, 0x8c, 0xfb, 0xa4, 0x45,
				0x72, 0x17, 0xe1, 0xa1, 0xe0, 0x40, 0x48, 0x9a, 0xcc, 0x46, 0xaa, 0xc9, 0x37, 0x76, 0xac, 0x23,
				0x59, 0x17, 0xa7, 0x36, 0xfd, 0x9e, 0x93, 0x3e, 0x77, 0x7e, 0xfb, 0xeb, 0xe3, 0xde, 0x71, 0x72,
				0x48, 0x6f, 0xf8, 0x1b, 0x80, 0x17, 0x46, 0xfa, 0x3a, 0xa0, 0xdc, 0x20, 0xf7, 0x11, 0xde, 0x59,
				0x37, 0xd4, 0x91, 0xa9, 0x36, 0x7b, 0xd6, 0x0e, 0xa6, 0xb1, 0x64, 0xa7, 0xe2, 0x80, 0xf2, 0xb4,
				0x8f, 0x32, 0x49, 0x8e, 0x77, 0x82, 0x52, 0x5f, 0x06, 0x64, 0x5f, 0x07, 0xd0, 0xc2, 0x1c, 0xd5,
				0x16, 0x6d, 0xed, 0xc0, 0xd7, 0x16, 0x6d, 0xdd, 0x78, 0xa6, 0xcd, 0xfa, 0x68, 0x8f, 0x93, 0xc9,
				0x66, 0x68, 0xf3, 0x4c, 0x5f, 0x87, 0x0a, 0xbc, 0xa1, 0xfb, 0xf3, 0xd9, 0xb7, 0x08, 0x47, 0xeb,
				0x87, 0x16, 0xa2, 0xda, 0x5d, 0x31, 0x7a, 0xc5, 0xf4, 0x8e, 0xe5, 0x3b, 0x86, 0xdb, 0x40, 0x2e,
				0x97, 0xc8, 0x7e, 0x44, 0x38, 0x5a, 0x3f, 0x4a, 0x28, 0xe1, 0x2a, 0xc6, 0x1c, 0x25, 0x5c, 0xd5,
				0x8c, 0xa2, 0xa5, 0x7c, 0xb8, 0xb3, 0xe4, 0x54, 0x47, 0x70, 0x6d, 0xba, 0xaa, 0xaf, 0xfb, 0xd3,
				0xc6, 0x06, 0xf9, 0x09, 0x61, 0xd2, 0x38, 0x31, 0x90, 0x13, 0x0a, 0x2c, 0xca, 0xc9, 0x27, 0x36,
				0xfd, 0x1c, 0x1a, 0x80, 0xff, 0x15, 0x09, 0xfd, 0x34, 0x99, 0xed, 0x8c, 0x69, 0xc7, 0x50, 0x2d,
				0xf8, 0xdb, 0x38, 0x24, 0xa3, 0x58, 0x53, 0x86, 0xa5, 0x1f, 0xba, 0x07, 0x5b, 0xca, 0x00, 0xa2,
				0x29, 0x9f, 0x51, 0x8d, 0x8c, 0xb5, 0x8b, 0x57, 0xb2, 0x8a, 0xfb, 0x64, 0x3b, 0x41, 0x5a, 0x19,
				0xf7, 0xca, 0x76, 0xec, 0x50, 0x6b, 0x21, 0x80, 0x70, 0xd0, 0x87, 0x30, 0x4a, 0x76, 0x37, 0x87,
				0x40, 0x3e, 0x40, 0x38, 0xe2, 0xb5, 0x6a, 0x64, 0xbc, 0x85, 0xdd, 0x60, 0x35, 0x3c, 0xd2, 0x56,
				0x0e, 0x20, 0xcc, 0xf8, 0x10, 0x8e, 0x90, 0xc3, 0xcd, 0x21, 0x4c, 0x39, 0x8d, 0x64, 0x80, 0x8a,
				0x8f, 0x10, 0x1e, 0x0c, 0x34, 0x58, 0xe4, 0xa8, 0x62, 0xb3, 0xc6, 0x46, 0x2f, 0x36, 0xd9, 0x89,
				0x28, 0x40, 0x3b, 0xe6, 0x43, 0x1b, 0x23, 0xf1, 0xe6, 0xd0, 0xb8, 0x5e, 0x96, 0x9a, 0xe4, 0x0e,
				0xc2, 0x61, 0xb7, 0x3f, 0x22, 0x2a, 0xee, 0x6b, 0xda, 0xb0, 0xd8, 0xe1, 0x36, 0x52, 0xcf, 0x07,
				0xc2, 0xdd, 0xf9, 0x67, 0x84, 0x49, 0x63, 0x4f, 0xa3, 0x4c, 0x30, 0x65, 0xb3, 0xa6, 0x4c, 0x30,
				0x75, 0xc3, 0xd4, 0x71, 0x81, 0xe0, 0x3a, 0x74, 0x00, 0xfa, 0x7a, 0x5d, 0xef, 0xb0, 0x41, 0xbe,
				0x44, 0x38, 0x5a, 0xdf, 0xbe, 0x28, 0x4b, 0x9b, 0xa2, 0x0f, 0x52, 0x96, 0x36, 0x55, 0x5f, 0xa4,
				0x1d, 0x57, 0xdf, 0xc3, 0xce, 0xbf, 0x53, 0x45, 0xa9, 0x34, 0xe5, 0x76, 0x4b, 0xe4, 0x33, 0x84,
				0x87, 0x82, 0xbd, 0x87, 0xb2, 0x49, 0x68, 0xd2, 0x4d, 0x29, 0x9b, 0x84, 0x66, 0xcd, 0x8c, 0x76,
				0xca, 0x67, 0x74, 0x92, 0x4c, 0xb4, 0xa8, 0x5b, 0x59, 0x47, 0xdb, 0x63, 0x31, 0xb5, 0xb0, 0xf9,
				0x67, 0xbc, 0xe7, 0xde, 0x56, 0xbc, 0x67, 0x73, 0x2b, 0x8e, 0x1e, 0x6d, 0xc5, 0xd1, 0x1f, 0x5b,
				0x71, 0xf4, 0xe1, 0xe3, 0x78, 0xcf, 0xa3, 0xc7, 0xf1, 0x9e, 0xdf, 0x1f, 0xc7, 0x7b, 0xde, 0x18,
				0x0f, 0x0c, 0xd2, 0xf3, 0x16, 0x2f, 0x5d, 0xf7, 0xac, 0xe6, 0xf5, 0x5b, 0xae, 0x75, 0xf9, 0x7f,
				0x10, 0xd9, 0xb0, 0xfc, 0x7b, 0xff, 0xc9, 0x7f, 0x02, 0x00, 0x00, 0xff, 0xff, 0x3b, 0xe2, 0xae,
				0x71, 0xea, 0x18, 0x00, 0x00,
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), ExpectedJSONSizeBinary(spec.input))
		})
	}
}

func TestExpectedJSONSizeInt(t *testing.T) {
	specs := map[string]struct {
		input int
	}{
		"zero": {
			input: 0,
		},
		"three": {
			input: 3,
		},
		"negative": {
			input: -5,
		},
		"ten": {
			input: 10,
		},
		"twelve": {
			input: 12,
		},
		"big": {
			input: 165684468468,
		},
		"negative big": {
			input: -165684468468,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), ExpectedJSONSizeInt(spec.input))
		})
	}
}

func TestExpectedJSONSizeUint64(t *testing.T) {
	specs := map[string]struct {
		input uint64
	}{
		"zero": {
			input: 0,
		},
		"three": {
			input: 3,
		},
		"ten": {
			input: 10,
		},
		"twelve": {
			input: 12,
		},
		"big": {
			input: 165684468468,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), ExpectedJSONSizeUint64(spec.input))
		})
	}
}

func TestExpectedJSONSizeBool(t *testing.T) {
	specs := map[string]struct {
		input bool
	}{
		"true": {
			input: true,
		},
		"false": {
			input: false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), ExpectedJSONSizeBool(spec.input))
		})
	}
}
