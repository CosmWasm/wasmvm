package types

// IBC2Payload defines a single packet sent in the IBC2SendPacketMsg.
//
// Payload value should be encoded using the method specified in the Encoding field,
// and the module on the other side should know how to parse this.
type IBC2Payload struct {
	// The port id on the chain where the packet is sent from.
	SourcePort string `json:"source_port"`
	// The port id on the chain where the packet is sent to.
	DestinationPort string `json:"destination_port"`
	// Version of the receiving contract.
	Version string `json:"version"`
	// Encoding used to serialize the Value.
	Encoding string `json:"encoding"`
	// Encoded payload data
	Value []byte `json:"value"`
}

// IBC2PacketReceiveMsg represents a message received via the IBC2 protocol.
// It contains the payload data along with metadata about the source and relayer.
type IBC2PacketReceiveMsg struct {
	// The actual data being transmitted via IBC2.
	Payload IBC2Payload `json:"payload"`
	// The address of the entity that relayed the packet.
	Relayer string `json:"relayer"`
	// The identifier of the source IBC client.
	SourceClient string `json:"source_client"`
	// The unique sequence number of the received packet.
	PacketSequence uint64 `json:"packet_sequence"`
}

// IBC2PacketTimeoutMsg represents a timeout event for a packet that was not
// successfully delivered within the expected timeframe in the IBC2 protocol.
// It includes details about the source and destination clients, and the sequence
// number of the timed-out packet.
type IBC2PacketTimeoutMsg struct {
	// The data associated with the timed-out packet.
	Payload IBC2Payload `json:"payload"`
	// The identifier of the client that originally sent the packet.
	SourceClient string `json:"source_client"`
	// The identifier of the client that was the intended recipient.
	DestinationClient string `json:"destination_client"`
	// The sequence number of the timed-out packet.
	PacketSequence uint64 `json:"packet_sequence"`
	// The address of the relayer responsible for the packet.
	Relayer string `json:"relayer"`
}

type IBC2AcknowledgeMsg struct {
	SourceClient      string      `json:"source_client"`
	DestinationClient string      `json:"destination_client"`
	Data              IBC2Payload `json:"data"`
	Acknowledgement   []byte      `json:"acknowledgement"`
	Relayer           string      `json:"relayer"`
}

// IBC2PacketSendMsg represents a payload sent event in the IBC2 protocol.
// Since sending IBCv2 packet is permissionless, the IBC protocol indtroduces
// an extra entry point, in which the application can verify the message sent from
// a port ID belonging to the contract.
//
// It includes details about the source and destination clients, the sequence
// number of the packet and the signer that sent the message.
type IBC2PacketSendMsg struct {
	// The payload to be sent.
	Payload IBC2Payload `json:"payload"`
	// The identifier of the client that originally sent the packet.
	SourceClient string `json:"source_client"`
	// The identifier of the client that was the intended recipient.
	DestinationClient string `json:"destination_client"`
	// The sequence number of the sent packet.
	PacketSequence uint64 `json:"packet_sequence"`
	// The address of the signer that sent the packet.
	Signer string `json:"signer"`
}
