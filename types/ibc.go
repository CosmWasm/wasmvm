package types

// Package types provides core types used throughout the wasmvm package.

// IBCEndpoint represents an endpoint in an IBC channel.
type IBCEndpoint struct {
	PortID    string `json:"port_id"`
	ChannelID string `json:"channel_id"`
}

// IBCChannel represents an IBC channel with its endpoints and ordering.
type IBCChannel struct {
	Endpoint             IBCEndpoint `json:"endpoint"`
	CounterpartyEndpoint IBCEndpoint `json:"counterparty_endpoint"`
	Order                IBCOrder    `json:"order"`
	Version              string      `json:"version"`
	ConnectionID         string      `json:"connection_id"`
}

// IBCChannelOpenMsg represents a message to open an IBC channel.
type IBCChannelOpenMsg struct {
	OpenInit *IBCOpenInit `json:"open_init,omitempty"`
	OpenTry  *IBCOpenTry  `json:"open_try,omitempty"`
}

// GetChannel returns the IBCChannel in this message.
func (msg IBCChannelOpenMsg) GetChannel() IBCChannel {
	if msg.OpenInit != nil {
		return msg.OpenInit.Channel
	}
	return msg.OpenTry.Channel
}

// GetCounterVersion checks if the message has a counterparty version and
// returns it if so.
func (msg IBCChannelOpenMsg) GetCounterVersion() (ver string, ok bool) {
	if msg.OpenTry != nil {
		return msg.OpenTry.CounterpartyVersion, true
	}
	return "", false
}

// IBCOpenInit represents an IBC channel open initialization message.
type IBCOpenInit struct {
	Channel IBCChannel `json:"channel"`
}

// ToMsg converts an IBCOpenInit to an IBCChannelOpenMsg.
func (m *IBCOpenInit) ToMsg() IBCChannelOpenMsg {
	return IBCChannelOpenMsg{
		OpenInit: m,
	}
}

// IBCOpenTry represents an IBC channel open try message.
type IBCOpenTry struct {
	Channel             IBCChannel `json:"channel"`
	CounterpartyVersion string     `json:"counterparty_version"`
}

// ToMsg converts an IBCOpenTry to an IBCChannelOpenMsg.
func (m *IBCOpenTry) ToMsg() IBCChannelOpenMsg {
	return IBCChannelOpenMsg{
		OpenTry: m,
	}
}

// IBCChannelConnectMsg represents a message to connect an IBC channel.
type IBCChannelConnectMsg struct {
	OpenAck     *IBCOpenAck     `json:"open_ack,omitempty"`
	OpenConfirm *IBCOpenConfirm `json:"open_confirm,omitempty"`
}

// GetChannel returns the IBCChannel in this message.
func (msg IBCChannelConnectMsg) GetChannel() IBCChannel {
	if msg.OpenAck != nil {
		return msg.OpenAck.Channel
	}
	return msg.OpenConfirm.Channel
}

// GetCounterVersion checks if the message has a counterparty version and
// returns it if so.
func (msg IBCChannelConnectMsg) GetCounterVersion() (ver string, ok bool) {
	if msg.OpenAck != nil {
		return msg.OpenAck.CounterpartyVersion, true
	}
	return "", false
}

// IBCOpenAck represents an IBC channel open acknowledgment message.
type IBCOpenAck struct {
	Channel             IBCChannel `json:"channel"`
	CounterpartyVersion string     `json:"counterparty_version"`
}

// ToMsg converts an IBCOpenAck to an IBCChannelConnectMsg.
func (m *IBCOpenAck) ToMsg() IBCChannelConnectMsg {
	return IBCChannelConnectMsg{
		OpenAck: m,
	}
}

// IBCOpenConfirm represents an IBC channel open confirmation message.
type IBCOpenConfirm struct {
	Channel IBCChannel `json:"channel"`
}

// ToMsg converts an IBCOpenConfirm to an IBCChannelConnectMsg.
func (m *IBCOpenConfirm) ToMsg() IBCChannelConnectMsg {
	return IBCChannelConnectMsg{
		OpenConfirm: m,
	}
}

// IBCChannelCloseMsg represents a message to close an IBC channel.
type IBCChannelCloseMsg struct {
	CloseInit    *IBCCloseInit    `json:"close_init,omitempty"`
	CloseConfirm *IBCCloseConfirm `json:"close_confirm,omitempty"`
}

// GetChannel returns the IBCChannel in this message.
func (msg IBCChannelCloseMsg) GetChannel() IBCChannel {
	if msg.CloseInit != nil {
		return msg.CloseInit.Channel
	}
	return msg.CloseConfirm.Channel
}

// IBCCloseInit represents an IBC channel close initialization message.
type IBCCloseInit struct {
	Channel IBCChannel `json:"channel"`
}

// ToMsg converts an IBCCloseInit to an IBCChannelCloseMsg.
func (m *IBCCloseInit) ToMsg() IBCChannelCloseMsg {
	return IBCChannelCloseMsg{
		CloseInit: m,
	}
}

// IBCCloseConfirm represents an IBC channel close confirmation message.
type IBCCloseConfirm struct {
	Channel IBCChannel `json:"channel"`
}

// ToMsg converts an IBCCloseConfirm to an IBCChannelCloseMsg.
func (m *IBCCloseConfirm) ToMsg() IBCChannelCloseMsg {
	return IBCChannelCloseMsg{
		CloseConfirm: m,
	}
}

// IBCPacketReceiveMsg represents a message to receive an IBC packet.
type IBCPacketReceiveMsg struct {
	Packet  IBCPacket `json:"packet"`
	Relayer string    `json:"relayer"`
}

// IBCPacketAckMsg represents a message to acknowledge an IBC packet.
type IBCPacketAckMsg struct {
	Acknowledgement IBCAcknowledgement `json:"acknowledgement"`
	OriginalPacket  IBCPacket          `json:"original_packet"`
	Relayer         string             `json:"relayer"`
}

// IBCPacketTimeoutMsg represents a message to handle an IBC packet timeout.
type IBCPacketTimeoutMsg struct {
	Packet  IBCPacket `json:"packet"`
	Relayer string    `json:"relayer"`
}

// IBCSourceCallbackMsg represents a message for IBC source chain callbacks
type IBCSourceCallbackMsg struct {
	Acknowledgement *IBCAckCallbackMsg     `json:"acknowledgement,omitempty"`
	Timeout         *IBCTimeoutCallbackMsg `json:"timeout,omitempty"`
}

// IBCAckCallbackMsg represents a message for an IBC acknowledgment callback.
type IBCAckCallbackMsg struct {
	Acknowledgement IBCAcknowledgement `json:"acknowledgement"`
	OriginalPacket  IBCPacket          `json:"original_packet"`
	Relayer         string             `json:"relayer"`
}

// IBCTimeoutCallbackMsg represents a message for an IBC timeout callback.
type IBCTimeoutCallbackMsg struct {
	Packet  IBCPacket `json:"packet"`
	Relayer string    `json:"relayer"`
}

// IBCDestinationCallbackMsg represents a message for IBC destination chain callbacks
type IBCDestinationCallbackMsg struct {
	Ack    IBCAcknowledgement `json:"ack"`
	Packet IBCPacket          `json:"packet"`
}

// IBCOrder represents the order of an IBC channel
type IBCOrder = string

// These are the only two valid values for IbcOrder.
const (
	Unordered = "ORDER_UNORDERED"
	Ordered   = "ORDER_ORDERED"
)

// IBCTimeoutBlock Height is a monotonically increasing data type
// that can be compared against another Height for the purposes of updating and
// freezing clients.
// Ordering is (revision_number, timeout_height).
// IBCTimeoutBlock represents a timeout block for an IBC packet.
type IBCTimeoutBlock struct {
	// the version that the client is currently on
	// (eg. after resetting the chain this could increment 1 as height drops to 0)
	Revision uint64 `json:"revision"`
	// block height after which the packet times out.
	// the height within the given revision
	Height uint64 `json:"height"`
}

// IsZero returns true if the timeout block is zero.
func (t IBCTimeoutBlock) IsZero() bool {
	return t.Revision == 0 && t.Height == 0
}

// IBCTimeout is the timeout for an IBC packet. At least one of block and timestamp is required.
type IBCTimeout struct {
	Block *IBCTimeoutBlock `json:"block"`
	// Nanoseconds since UNIX epoch
	Timestamp uint64 `json:"timestamp,string,omitempty"`
}

// IBCAcknowledgement represents an IBC packet acknowledgment.
type IBCAcknowledgement struct {
	Data []byte `json:"data"`
}

// IBCPacket represents an IBC packet.
type IBCPacket struct {
	Data     []byte      `json:"data"`
	Src      IBCEndpoint `json:"src"`
	Dest     IBCEndpoint `json:"dest"`
	Sequence uint64      `json:"sequence"`
	Timeout  IBCTimeout  `json:"timeout"`
}

// IBCChannelOpenResult is the raw response from the ibc_channel_open call.
// This is mirrors Rust's ContractResult<()>.
// Check if Err == "" to see if this is success
// On Success, IBCV3ChannelOpenResponse *may* be set if the contract is ibcv3 compatible and wishes to
// define a custom version in the handshake.
type IBCChannelOpenResult struct {
	Ok  *IBC3ChannelOpenResponse `json:"ok,omitempty"`
	Err string                   `json:"error,omitempty"`
}

// IBC3ChannelOpenResponse is version negotiation data for the handshake.
type IBC3ChannelOpenResponse struct {
	Version string `json:"version"`
}

// IBCBasicResult represents the basic result of an IBC operation
type IBCBasicResult struct {
	Ok  *IBCBasicResponse `json:"ok,omitempty"`
	Err string            `json:"error,omitempty"`
}

// SubMessages returns the sub-messages of the result.
func (r *IBCBasicResult) SubMessages() []SubMsg {
	if r.Ok != nil {
		return r.Ok.Messages
	}
	return nil
}

// IBCBasicResponse defines the return value on a successful processing.
// This is the counterpart of [IbcBasicResponse](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/ibc.rs#L194-L216).
type IBCBasicResponse struct {
	// Messages comes directly from the contract and is its request for action.
	// If the ReplyOn value matches the result, the runtime will invoke this
	// contract's `reply` entry point after execution. Otherwise, this is all
	// "fire and forget".
	Messages []SubMsg `json:"messages"`
	// attributes for a log event to return over abci interface
	Attributes []EventAttribute `json:"attributes"`
	// custom events (separate from the main one that contains the attributes
	// above)
	Events []Event `json:"events"`
}

// IBCReceiveResult represents the result of receiving an IBC packet
type IBCReceiveResult struct {
	Ok  *IBCReceiveResponse `json:"ok,omitempty"`
	Err string              `json:"error,omitempty"`
}

// SubMessages returns the sub-messages of the result.
func (r *IBCReceiveResult) SubMessages() []SubMsg {
	if r.Ok != nil {
		return r.Ok.Messages
	}
	return nil
}

// IBCReceiveResponse defines the return value on packet response processing.
// This "success" case should be returned even in application-level errors,
// Where the Acknowledgement bytes contain an encoded error message to be returned to
// the calling chain. (Returning IBCReceiveResult::Err will abort processing of this packet
// and not inform the calling chain).
// This is the counterpart of (IbcReceiveResponse)(https://github.com/CosmWasm/cosmwasm/blob/v0.15.0/packages/std/src/ibc.rs#L247-L267).
type IBCReceiveResponse struct {
	// Acknowledgement is binary encoded data to be returned to calling chain as the acknowledgement.
	// If this field is nil, no acknowledgement must be written. For contracts before CosmWasm 2.0, this
	// was always a non-nil value. See also https://github.com/CosmWasm/cosmwasm/pull/1892.
	Acknowledgement []byte `json:"acknowledgement"`
	// Messages comes directly from the contract and is it's request for action.
	// If the ReplyOn value matches the result, the runtime will invoke this
	// contract's `reply` entry point after execution. Otherwise, this is all
	// "fire and forget".
	Messages   []SubMsg         `json:"messages"`
	Attributes []EventAttribute `json:"attributes"`
	// custom events (separate from the main one that contains the attributes
	// above)
	Events []Event `json:"events"`
}

var (
	_ ExpectedJSONSize = IBCChannelOpenMsg{}
	_ ExpectedJSONSize = IBCChannelConnectMsg{}
	_ ExpectedJSONSize = IBCChannelCloseMsg{}
	_ ExpectedJSONSize = IBCPacketReceiveMsg{}
	_ ExpectedJSONSize = IBCPacketAckMsg{}
	_ ExpectedJSONSize = IBCPacketTimeoutMsg{}
	_ ExpectedJSONSize = IBCSourceCallbackMsg{}
	_ ExpectedJSONSize = IBCDestinationCallbackMsg{}
)
