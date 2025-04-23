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

// IBC2PacketReceiveMsg represents a message to receive an IBC2 packet.
type IBC2PacketReceiveMsg struct {
	Packet  IBCPacket `json:"packet"`
	Relayer string    `json:"relayer"`
}
