package types

// IBCv2Payload defines a single packet sent in the IBCv2SendPacketMsg.
//
// Payload value should be encoded using the method specified in the Encoding field,
// and the module on the other side should know how to parse this.
type IBCv2Payload struct {
	// The port id on the chain where the packet is sent to (external chain).
	DestinationPort string `json:"destination_port"`
	// Version of the receiving contract.
	Version string `json:"version"`
	// Encoding used to serialize the Value.
	Encoding string `json:"encoding"`
	// Encoded payload data
	Value []byte `json:"value"`
}

type IBCv2PacketReceiveMsg struct {
	Packet  IBCv2Payload `json:"packet"`
	Relayer string       `json:"relayer"`
}
