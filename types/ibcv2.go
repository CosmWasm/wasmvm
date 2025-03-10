package types

// IBCv2Payload defines a single packet sent in the IBCv2SendPacketMsg.
//
// Payload value should be encoded using the method specified in the Encoding field,
// and the module on the other side should know how to parse this.
type IBCv2Payload struct {
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

type IBCv2PacketReceiveMsg struct {
	Payload      IBCv2Payload `json:"payload"`
	Relayer      string       `json:"relayer"`
	SourceClient string       `json:"source_client"`
}
