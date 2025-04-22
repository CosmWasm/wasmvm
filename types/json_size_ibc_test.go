package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONSizeIBCEndpoint(t *testing.T) {
	specs := map[string]struct {
		input IBCEndpoint
	}{
		"default": {
			input: IBCEndpoint{},
		},
		"with channel ID": {
			input: IBCEndpoint{
				ChannelID: "foo",
				PortID:    "",
			},
		},
		"with port ID": {
			input: IBCEndpoint{
				ChannelID: "",
				PortID:    "bar",
			},
		},
		"with both": {
			input: IBCEndpoint{
				ChannelID: "hello",
				PortID:    "world",
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCChannel(t *testing.T) {
	specs := map[string]struct {
		input IBCChannel
	}{
		"default": {
			input: IBCChannel{},
		},
		"with data": {
			input: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCOpenInit(t *testing.T) {
	specs := map[string]struct {
		input IBCOpenInit
	}{
		"default": {
			input: IBCOpenInit{},
		},
		"with data": {
			input: IBCOpenInit{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCOpenTry(t *testing.T) {
	specs := map[string]struct {
		input IBCOpenTry
	}{
		"default": {
			input: IBCOpenTry{},
		},
		"with data": {
			input: IBCOpenTry{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}, CounterpartyVersion: "99.88"},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestIBCChannelOpenMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCChannelOpenMsg
	}{
		"default": {
			input: IBCChannelOpenMsg{},
		},
		"with open try": {
			input: IBCChannelOpenMsg{OpenTry: &IBCOpenTry{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}, CounterpartyVersion: "99.88"}},
		},
		"with open init": {
			input: IBCChannelOpenMsg{OpenInit: &IBCOpenInit{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}}},
		},
		"with both": {
			input: IBCChannelOpenMsg{
				OpenTry: &IBCOpenTry{Channel: IBCChannel{
					Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
					CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Order:                "UNORDERED",
					Version:              "v.1232423423",
					ConnectionID:         "the-id",
				}, CounterpartyVersion: "99.88"},
				OpenInit: &IBCOpenInit{Channel: IBCChannel{
					Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
					CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Order:                "UNORDERED",
					Version:              "v.1232423423",
					ConnectionID:         "the-id",
				}},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCOpenAck(t *testing.T) {
	specs := map[string]struct {
		input IBCOpenAck
	}{
		"default": {
			input: IBCOpenAck{},
		},
		"with data": {
			input: IBCOpenAck{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}, CounterpartyVersion: "99.88"},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCOpenConfirm(t *testing.T) {
	specs := map[string]struct {
		input IBCOpenConfirm
	}{
		"default": {
			input: IBCOpenConfirm{},
		},
		"with data": {
			input: IBCOpenConfirm{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCChannelConnectMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCChannelConnectMsg
	}{
		"default": {
			input: IBCChannelConnectMsg{},
		},
		"with open ack": {
			input: IBCChannelConnectMsg{OpenAck: &IBCOpenAck{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}, CounterpartyVersion: "99.88"}},
		},
		"with open confirm": {
			input: IBCChannelConnectMsg{OpenConfirm: &IBCOpenConfirm{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}}},
		},
		"with both": {
			input: IBCChannelConnectMsg{
				OpenAck: &IBCOpenAck{Channel: IBCChannel{
					Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
					CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Order:                "UNORDERED",
					Version:              "v.1232423423",
					ConnectionID:         "the-id",
				}, CounterpartyVersion: "99.88"},
				OpenConfirm: &IBCOpenConfirm{Channel: IBCChannel{
					Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
					CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Order:                "UNORDERED",
					Version:              "v.1232423423",
					ConnectionID:         "the-id",
				}},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCCloseInit(t *testing.T) {
	specs := map[string]struct {
		input IBCCloseInit
	}{
		"default": {
			input: IBCCloseInit{},
		},
		"with data": {
			input: IBCCloseInit{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCCloseConfirm(t *testing.T) {
	specs := map[string]struct {
		input IBCCloseConfirm
	}{
		"default": {
			input: IBCCloseConfirm{},
		},
		"with data": {
			input: IBCCloseConfirm{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCChannelCloseMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCChannelCloseMsg
	}{
		"default": {
			input: IBCChannelCloseMsg{},
		},
		"with close init": {
			input: IBCChannelCloseMsg{CloseInit: &IBCCloseInit{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}}},
		},
		"with close confirm": {
			input: IBCChannelCloseMsg{CloseConfirm: &IBCCloseConfirm{Channel: IBCChannel{
				Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
				CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Order:                "UNORDERED",
				Version:              "v.1232423423",
				ConnectionID:         "the-id",
			}}},
		},
		"with both": {
			input: IBCChannelCloseMsg{
				CloseInit: &IBCCloseInit{Channel: IBCChannel{
					Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
					CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Order:                "UNORDERED",
					Version:              "v.1232423423",
					ConnectionID:         "the-id",
				}},
				CloseConfirm: &IBCCloseConfirm{Channel: IBCChannel{
					Endpoint:             IBCEndpoint{ChannelID: "hello", PortID: "world"},
					CounterpartyEndpoint: IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Order:                "UNORDERED",
					Version:              "v.1232423423",
					ConnectionID:         "the-id",
				}},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCTimeoutBlock(t *testing.T) {
	specs := map[string]struct {
		input IBCTimeoutBlock
	}{
		"default": {
			input: IBCTimeoutBlock{},
		},
		"with both": {
			input: IBCTimeoutBlock{Height: 21465543, Revision: 7},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCTimeout(t *testing.T) {
	specs := map[string]struct {
		input IBCTimeout
	}{
		"default": {
			input: IBCTimeout{},
		},
		"with block": {
			input: IBCTimeout{Block: &IBCTimeoutBlock{Height: 21465543, Revision: 7}},
		},
		"with timestamp": {
			input: IBCTimeout{Timestamp: 12345677775},
		},
		"with block and timestamp": {
			input: IBCTimeout{
				Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
				Timestamp: 321,
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCPacket(t *testing.T) {
	specs := map[string]struct {
		input IBCPacket
	}{
		"default": {
			input: IBCPacket{},
		},
		"with data": {
			input: IBCPacket{
				Data:     []byte{0x11, 0x22, 0x33},
				Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
				Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
				Sequence: 556677,
				Timeout: IBCTimeout{
					Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
					Timestamp: 321,
				},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCAcknowledgement(t *testing.T) {
	specs := map[string]struct {
		input IBCAcknowledgement
	}{
		"default": {
			input: IBCAcknowledgement{},
		},
		"with data": {
			input: IBCAcknowledgement{Data: []byte{0x11, 0x22, 0x33}},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCPacketReceiveMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCPacketReceiveMsg
	}{
		"default": {
			input: IBCPacketReceiveMsg{},
		},
		"with data": {
			input: IBCPacketReceiveMsg{
				Packet: IBCPacket{
					Data:     []byte{0x11, 0x22, 0x33},
					Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
					Sequence: 556677,
					Timeout: IBCTimeout{
						Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
						Timestamp: 321,
					},
				},
				Relayer: "best-operator-in-town",
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCPacketAckMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCPacketAckMsg
	}{
		"default": {
			input: IBCPacketAckMsg{},
		},
		"with data": {
			input: IBCPacketAckMsg{
				Acknowledgement: IBCAcknowledgement{Data: []byte{0x11, 0x22, 0x33}},
				OriginalPacket: IBCPacket{
					Data:     []byte{0x11, 0x22, 0x33},
					Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
					Sequence: 556677,
					Timeout: IBCTimeout{
						Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
						Timestamp: 321,
					},
				},
				Relayer: "best-operator-in-town",
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCPacketTimeoutMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCPacketTimeoutMsg
	}{
		"default": {
			input: IBCPacketTimeoutMsg{},
		},
		"with data": {
			input: IBCPacketTimeoutMsg{
				Packet: IBCPacket{
					Data:     []byte{0x11, 0x22, 0x33},
					Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
					Sequence: 556677,
					Timeout: IBCTimeout{
						Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
						Timestamp: 321,
					},
				},
				Relayer: "best-operator-in-town",
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCSourceCallbackMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCSourceCallbackMsg
	}{
		"default": {
			input: IBCSourceCallbackMsg{},
		},
		"with acknowledgement": {
			input: IBCSourceCallbackMsg{
				Acknowledgement: &IBCAckCallbackMsg{
					Acknowledgement: IBCAcknowledgement{
						Data: []byte{0x11, 0x22, 0x33, 0x44, 0x55},
					},
					OriginalPacket: IBCPacket{
						Data:     []byte{0x11, 0x22, 0x33},
						Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
						Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
						Sequence: 556677,
						Timeout: IBCTimeout{
							Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
							Timestamp: 321,
						},
					},
					Relayer: "best-operator-in-town",
				},
			},
		},
		"with timeout": {
			input: IBCSourceCallbackMsg{
				Timeout: &IBCTimeoutCallbackMsg{
					Packet: IBCPacket{
						Data:     []byte{0x11, 0x22, 0x33},
						Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
						Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
						Sequence: 556677,
						Timeout: IBCTimeout{
							Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
							Timestamp: 321,
						},
					},
					Relayer: "best-operator-in-town",
				},
			},
		},
		"both": {
			input: IBCSourceCallbackMsg{
				Acknowledgement: &IBCAckCallbackMsg{
					Acknowledgement: IBCAcknowledgement{
						Data: []byte{0x11, 0x22, 0x33, 0x44, 0x55},
					},
					OriginalPacket: IBCPacket{
						Data:     []byte{0x11, 0x22, 0x33},
						Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
						Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
						Sequence: 556677,
						Timeout: IBCTimeout{
							Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
							Timestamp: 321,
						},
					},
					Relayer: "best-operator-in-town",
				},
				Timeout: &IBCTimeoutCallbackMsg{
					Packet: IBCPacket{
						Data:     []byte{0x11, 0x22, 0x33},
						Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
						Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
						Sequence: 556677,
						Timeout: IBCTimeout{
							Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
							Timestamp: 321,
						},
					},
					Relayer: "best-operator-in-town",
				},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}

func TestJSONSizeIBCDestinationCallbackMsg(t *testing.T) {
	specs := map[string]struct {
		input IBCDestinationCallbackMsg
	}{
		"default": {
			input: IBCDestinationCallbackMsg{},
		},
		"with data": {
			input: IBCDestinationCallbackMsg{
				Ack: IBCAcknowledgement{
					Data: []byte{0x11, 0x22, 0x33, 0x44, 0x55},
				},
				Packet: IBCPacket{
					Data:     []byte{0x11, 0x22, 0x33},
					Src:      IBCEndpoint{ChannelID: "hello", PortID: "world"},
					Dest:     IBCEndpoint{ChannelID: "otherside", PortID: "otherplace"},
					Sequence: 556677,
					Timeout: IBCTimeout{
						Block:     &IBCTimeoutBlock{Height: 21465543, Revision: 7},
						Timestamp: 321,
					},
				},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			serialized, err := json.Marshal(spec.input)
			if err != nil {
				panic("Could not marshal input")
			}
			require.Equal(t, len(serialized), spec.input.ExpectedJSONSize())
		})
	}
}
