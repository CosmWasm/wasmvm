package types

type IBCEndpoint struct {
	PortId    string `json:"port_id"`
	ChannelId string `json:"channel_id"`
}

type IBCChannel struct {
	Endpoint             IBCEndpoint `json:"endpoint"`
	CounterpartyEndpoint IBCEndpoint `json:"counterparty_endpoint"`
	Order                IBCOrder    `json:"order"`
	Version              string      `json:"version"`
	// optional
	CounterpartyVersion string `json:"counterparty_version,omitempty"`
	ConnectionID        string `json:"connection_id"`
}

// TODO: test what the sdk Order.String() represents and how to parse back
// Proto files: https://github.com/cosmos/cosmos-sdk/blob/v0.40.0/proto/ibc/core/channel/v1/channel.proto#L69-L80
// Auto-gen code: https://github.com/cosmos/cosmos-sdk/blob/v0.40.0/x/ibc/core/04-channel/types/channel.pb.go#L70-L101
type IBCOrder = string

// These are the only two valid values for IbcOrder
const Unordered = "ORDER_UNORDERED"
const Ordered = "ORDER_ORDERED"

// IBCTimeoutHeight Height is a monotonically increasing data type
// that can be compared against another Height for the purposes of updating and
// freezing clients.
// Ordering is (revision_number, timeout_height)
type IBCTimeoutHeight struct {
	// the version that the client is currently on
	// (eg. after reseting the chain this could increment 1 as height drops to 0)
	RevisionNumber uint64 `json:"revision_number"`
	// block height after which the packet times out.
	// the height within the given revision
	TimeoutHeight uint64 `json:"timeout_height"`
}

type IBCPacket struct {
	Data             []byte           `json:"data"`
	Src              IBCEndpoint      `json:"src"`
	Dest             IBCEndpoint      `json:"dest"`
	Sequence         uint64           `json:"sequence"`
	TimeoutHeight    IBCTimeoutHeight `json:"timeout_height"`
	TimeoutTimestamp uint64           `json:"timeout_timestamp"`
	Version          uint64           `json:"version"`
}

type IBCAcknowledgement struct {
	Acknowledgement []byte    `json:"acknowledgement"`
	OriginalPacket  IBCPacket `json:"original_packet"`
}

// IBCChannelOpenResult is the raw response from the ibc_channel_open call.
// This is mirrors Rust's ContractResult<()>.
// We just check if Err == "" to see if this is success (no other data on success)
type IBCChannelOpenResult struct {
	Ok  *struct{} `json:"ok,omitempty"`
	Err string    `json:"error,omitempty"`
}

// This is the return value for the majority of the ibc handlers.
// That are able to dispatch messages / events on their own,
// but have no meaningful return value to the calling code.
//
// Callbacks that have return values (like ibc_receive_packet)
// or that cannot redispatch messages (like ibc_channel_open)
// will use other Response types
type IBCBasicResult struct {
	Ok  *IBCBasicResponse `json:"ok,omitempty"`
	Err string            `json:"error,omitempty"`
}

// IBCBasicResponse defines the return value on a successful processing
type IBCBasicResponse struct {
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg `json:"messages"`
	// attributes for a log event to return over abci interface
	Attributes []EventAttribute `json:"attributes"`
}

// This is the return value for the majority of the ibc handlers.
// That are able to dispatch messages / events on their own,
// but have no meaningful return value to the calling code.
//
// Callbacks that have return values (like receive_packet)
// or that cannot redispatch messages (like the handshake callbacks)
// will use other Response types
type IBCReceiveResult struct {
	Ok  *IBCReceiveResponse `json:"ok,omitempty"`
	Err string              `json:"error,omitempty"`
}

// IBCReceiveResponse defines the return value on packet response processing.
// This "success" case should be returned even in application-level errors,
// Where the Acknowledgement bytes contain an encoded error message to be returned to
// the calling chain. (Returning IBCReceiveResult::Err will abort processing of this packet
// and not inform the calling chain).
type IBCReceiveResponse struct {
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg `json:"messages"`
	// binary encoded data to be returned to calling chain as the acknowledgement
	Acknowledgement []byte `json:"acknowledgement"`
	// attributes for a log event to return over abci interface
	Attributes []EventAttribute `json:"attributes"`
}
