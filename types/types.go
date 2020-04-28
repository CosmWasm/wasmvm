package types

// Coin is a string representation of the sdk.Coin type (more portable than sdk.Int)
type Coin struct {
	Denom  string `json:"denom"`  // type, eg. "ATOM"
	Amount string `json:"amount"` // string encoing of decimal value, eg. "12.3456"
}

// CanoncialAddress uses standard base64 encoding, just use it as a label for developers
type CanonicalAddress = []byte
