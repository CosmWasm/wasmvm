package types

type ExpectedJSONSize interface {
	// ExpectedJSONSize returns the expected JSON size in bytes when using
	// json.Marshal with the given value.
	// Since JSON marshalling does not have a guaranteed output format,
	// this should be understood as a best guess and correct in most cases.
	// Do not use it when a precise value is required.
	ExpectedJSONSize() int
}

// ExpectedJSONSizeString returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func ExpectedJSONSizeString(s string) int {
	// 2x quote + length of string + escaping overhead
	out := quotes + len(s)
	for _, r := range s {
		if r == '"' || r == '\\' {
			out += 1
		} else if r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t' {
			// https://cs.opensource.google/go/go/+/master:src/encoding/json/encode.go;l=992-1001;drc=0909bcd9e4acb01089d588d608d669d69710e50a
			out += 1
		} else if r <= 0x1F {
			// control codes \u0000 - \u001f
			out += 5
		} else if r == '<' || r == '>' || r == '&' {
			// Go escapes HTML which is a bit pointless but legal
			// \u003c, \u003e, \u0026
			out += 5
		}
	}
	return out
}

// ExpectedJSONSizeBinary returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func ExpectedJSONSizeBinary(b []byte) int {
	if b == nil {
		return null
	}
	// padded base64 encoding (https://stackoverflow.com/questions/13378815/base64-length-calculation) plus quotes
	b64Len := ((4 * len(b) / 3) + 3) & ^3
	return b64Len + quotes
}

// ExpectedJSONSizeInt returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func ExpectedJSONSizeInt(i int) int {
	out := 0
	// minus sign or zero
	if i <= 0 {
		i = -i
		out += 1
	}
	for i > 0 {
		i /= 10
		out += 1
	}
	return out
}

// ExpectedJSONSizeUint64 returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func ExpectedJSONSizeUint64(i uint64) int {
	if i == 0 {
		return 1
	}

	out := 0
	for i > 0 {
		i /= 10
		out += 1
	}
	return out
}

// ExpectedJSONSizeBool returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func ExpectedJSONSizeBool(b bool) int {
	if b {
		return 4 // true
	} else {
		return 5 // false
	}
}

// The size in bytes in JSON serialization
const (
	brackets int = 2 // a pair of brackets {} or []
	quotes   int = 2 // a pair of quotes ""
	comma    int = 1 // ,
	colon    int = 1 // :
	null     int = 4 // null
)

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCEndpoint) ExpectedJSONSize() int {
	// PortID    string `json:"port_id"`
	// ChannelID string `json:"channel_id"`
	return brackets +
		9 + colon + ExpectedJSONSizeString(t.PortID) + comma +
		12 + colon + ExpectedJSONSizeString(t.ChannelID)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCChannel) ExpectedJSONSize() int {
	// Endpoint             IBCEndpoint `json:"endpoint"`
	// CounterpartyEndpoint IBCEndpoint `json:"counterparty_endpoint"`
	// Order                IBCOrder    `json:"order"`
	// Version              string      `json:"version"`
	// ConnectionID         string      `json:"connection_id"`
	return brackets +
		10 + colon + t.Endpoint.ExpectedJSONSize() + comma +
		23 + colon + t.CounterpartyEndpoint.ExpectedJSONSize() + comma +
		7 + colon + ExpectedJSONSizeString(t.Order) + comma +
		9 + colon + ExpectedJSONSizeString(t.Version) + comma +
		15 + colon + ExpectedJSONSizeString(t.ConnectionID)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCOpenInit) ExpectedJSONSize() int {
	// Channel IBCChannel `json:"channel"`
	return brackets + 9 + colon + t.Channel.ExpectedJSONSize()
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCOpenTry) ExpectedJSONSize() int {
	// Channel             IBCChannel `json:"channel"`
	// CounterpartyVersion string     `json:"counterparty_version"`
	return brackets +
		9 + colon + t.Channel.ExpectedJSONSize() + comma +
		22 + colon + ExpectedJSONSizeString(t.CounterpartyVersion)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCChannelOpenMsg) ExpectedJSONSize() int {
	// OpenInit *IBCOpenInit `json:"open_init,omitempty"`
	// OpenTry  *IBCOpenTry  `json:"open_try,omitempty"`
	out := brackets
	if t.OpenInit != nil {
		out += 11 + colon + t.OpenInit.ExpectedJSONSize()
	}
	if t.OpenInit != nil && t.OpenTry != nil {
		// this case should not happen for an enum but we don't have an error return type, so just handle it
		out += comma
	}
	if t.OpenTry != nil {
		out += 10 + colon + t.OpenTry.ExpectedJSONSize()
	}
	return out
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCOpenAck) ExpectedJSONSize() int {
	// Channel             IBCChannel `json:"channel"`
	// CounterpartyVersion string     `json:"counterparty_version"`
	return brackets +
		9 + colon + t.Channel.ExpectedJSONSize() + comma +
		22 + colon + ExpectedJSONSizeString(t.CounterpartyVersion)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCOpenConfirm) ExpectedJSONSize() int {
	// Channel IBCChannel `json:"channel"`
	return brackets + 9 + colon + t.Channel.ExpectedJSONSize()
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCChannelConnectMsg) ExpectedJSONSize() int {
	// OpenAck     *IBCOpenAck     `json:"open_ack,omitempty"`
	// OpenConfirm *IBCOpenConfirm `json:"open_confirm,omitempty"`
	out := brackets
	if t.OpenAck != nil {
		out += 10 + colon + t.OpenAck.ExpectedJSONSize()
	}
	if t.OpenAck != nil && t.OpenConfirm != nil {
		// this case should not happen for an enum but we don't have an error return type, so just handle it
		out += comma
	}
	if t.OpenConfirm != nil {
		out += 14 + colon + t.OpenConfirm.ExpectedJSONSize()
	}
	return out
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCCloseInit) ExpectedJSONSize() int {
	// Channel IBCChannel `json:"channel"`
	return brackets + 9 + colon + t.Channel.ExpectedJSONSize()
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCCloseConfirm) ExpectedJSONSize() int {
	// Channel IBCChannel `json:"channel"`
	return brackets + 9 + colon + t.Channel.ExpectedJSONSize()
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCChannelCloseMsg) ExpectedJSONSize() int {
	// CloseInit    *IBCCloseInit    `json:"close_init,omitempty"`
	// CloseConfirm *IBCCloseConfirm `json:"close_confirm,omitempty"`
	out := brackets
	if t.CloseInit != nil {
		out += 12 + colon + t.CloseInit.ExpectedJSONSize()
	}
	if t.CloseInit != nil && t.CloseConfirm != nil {
		// this case should not happen for an enum but we don't have an error return type, so just handle it
		out += comma
	}
	if t.CloseConfirm != nil {
		out += 15 + colon + t.CloseConfirm.ExpectedJSONSize()
	}
	return out
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCTimeoutBlock) ExpectedJSONSize() int {
	// Revision uint64 `json:"revision"`
	// Height uint64 `json:"height"`
	return brackets +
		10 + colon + ExpectedJSONSizeUint64(t.Revision) + comma +
		8 + colon + ExpectedJSONSizeUint64(t.Height)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCTimeout) ExpectedJSONSize() int {
	// Block *IBCTimeoutBlock `json:"block"`
	// Timestamp uint64 `json:"timestamp,string,omitempty"`
	out := brackets

	if t.Block != nil {
		out += 7 + colon + t.Block.ExpectedJSONSize()
	} else {
		out += 7 + colon + null // Block never omitted
	}

	if t.Timestamp != 0 {
		out += comma + 11 + colon + ExpectedJSONSizeUint64(t.Timestamp) + quotes
	}

	return out
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCPacket) ExpectedJSONSize() int {
	// Data     []byte      `json:"data"`
	// Src      IBCEndpoint `json:"src"`
	// Dest     IBCEndpoint `json:"dest"`
	// Sequence uint64      `json:"sequence"`
	// Timeout  IBCTimeout  `json:"timeout"`
	return brackets +
		6 + colon + ExpectedJSONSizeBinary(t.Data) + comma +
		5 + colon + t.Src.ExpectedJSONSize() + comma +
		6 + colon + t.Dest.ExpectedJSONSize() + comma +
		10 + colon + ExpectedJSONSizeUint64(t.Sequence) + comma +
		9 + colon + t.Timeout.ExpectedJSONSize()
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCAcknowledgement) ExpectedJSONSize() int {
	// Data []byte `json:"data"`
	return brackets +
		6 + colon + ExpectedJSONSizeBinary(t.Data)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCPacketReceiveMsg) ExpectedJSONSize() int {
	// Packet  IBCPacket `json:"packet"`
	// Relayer string    `json:"relayer"`
	return brackets +
		8 + colon + t.Packet.ExpectedJSONSize() + comma +
		9 + colon + ExpectedJSONSizeString(t.Relayer)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCPacketAckMsg) ExpectedJSONSize() int {
	// Acknowledgement IBCAcknowledgement `json:"acknowledgement"`
	// OriginalPacket  IBCPacket          `json:"original_packet"`
	// Relayer         string             `json:"relayer"`
	return brackets +
		17 + colon + t.Acknowledgement.ExpectedJSONSize() + comma +
		17 + colon + t.OriginalPacket.ExpectedJSONSize() + comma +
		9 + colon + ExpectedJSONSizeString(t.Relayer)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCPacketTimeoutMsg) ExpectedJSONSize() int {
	// Packet  IBCPacket `json:"packet"`
	// Relayer string    `json:"relayer"`
	return brackets +
		8 + colon + t.Packet.ExpectedJSONSize() + comma +
		9 + colon + ExpectedJSONSizeString(t.Relayer)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCAckCallbackMsg) ExpectedJSONSize() int {
	// Acknowledgement IBCAcknowledgement `json:"acknowledgement"`
	// OriginalPacket  IBCPacket          `json:"original_packet"`
	// Relayer         string             `json:"relayer"`
	return brackets +
		17 + colon + t.Acknowledgement.ExpectedJSONSize() + comma +
		17 + colon + t.OriginalPacket.ExpectedJSONSize() + comma +
		9 + colon + ExpectedJSONSizeString(t.Relayer)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCTimeoutCallbackMsg) ExpectedJSONSize() int {
	// Packet  IBCPacket `json:"packet"`
	// Relayer string    `json:"relayer"`
	return brackets +
		8 + colon + t.Packet.ExpectedJSONSize() + comma +
		9 + colon + ExpectedJSONSizeString(t.Relayer)
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCSourceCallbackMsg) ExpectedJSONSize() int {
	// Acknowledgement *IBCAckCallbackMsg     `json:"acknowledgement,omitempty"`
	// Timeout         *IBCTimeoutCallbackMsg `json:"timeout,omitempty"`
	out := brackets

	if t.Acknowledgement != nil {
		out += 17 + colon + t.Acknowledgement.ExpectedJSONSize()
	}

	if t.Acknowledgement != nil && t.Timeout != nil {
		out += comma
	}

	if t.Timeout != nil {
		out += 9 + colon + t.Timeout.ExpectedJSONSize()
	}

	return out
}

// ExpectedJSONSize returns the expected JSON size in bytes when using
// json.Marshal with the given value.
// Since JSON marshalling does not have a guaranteed output format,
// this should be understood as a best guess and correct in most cases.
// Do not use it when a precise value is required.
func (t IBCDestinationCallbackMsg) ExpectedJSONSize() int {
	// Ack    IBCAcknowledgement `json:"ack"`
	// Packet IBCPacket          `json:"packet"`
	return brackets +
		5 + colon + t.Ack.ExpectedJSONSize() + comma +
		8 + colon + t.Packet.ExpectedJSONSize()
}
