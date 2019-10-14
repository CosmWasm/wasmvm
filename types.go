package cosmwasm

// Params defines the state of the blockchain environment this contract is
// running in. This must contain only trusted data - nothing from the Tx itself
// that has not been verfied (like Signer).
//
// Params are json encoded to a byte slice before passing to the wasm contract.
type Params struct {
	Block BlockInfo `json:"block"`
	Message MessageInfo `json:"message"`
	Contract ContractInfo `json:"contract"`
}

type BlockInfo struct {
	Height int64 `json:"height"`
	Time int64 `json:"time"`
	ChainID string `json:"chain_id"`
}

type MessageInfo struct {
    Signer string `json:"signer"`
    SentFunds []SendAmount `json:"sent_funds"`
}

type ContractInfo struct {
    Address string `json:"address"`
    Balance: []SendAmount `json:"send_amount"`
}

type SendAmount struct {
    Denom string `json:"denom"`
    Amount string `json:"amount"`
}


// Result defines the return value on a successful
type Result struct {
	// GasUsed is what is calculated from the VM, assuming it didn't run out of gas
	GasUsed int64
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg
}

// CosmosMsg is an rust enum and only (exactly) one of the fields should be set
// Should we do a cleaner approach in Go?
type CosmosMsg struct {
	Send SendMsg `json:"send"`
}

// SendMsg contains instructions for a Cosmos-SDK/SendMsg
// It has a fixed interface here and should be converted into the proper SDK format before dispatching
type SendMsg struct {
	FromAddress string `json:from_address`
	ToAddress string `json:to_address`
	Amount []SendAmount `json:amount`
}