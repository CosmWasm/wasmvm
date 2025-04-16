package types

//---------- Env ---------

// Env represents the execution environment for a CosmWasm contract.
// It includes information about the current block, transaction, contract, and message.
//
// Env are json encoded to a byte slice before passing to the wasm contract.
type Env struct {
	Block       BlockInfo        `json:"block"`
	Transaction *TransactionInfo `json:"transaction"`
	Contract    ContractInfo     `json:"contract"`
}

// BlockInfo represents information about the current block being processed.
// It includes the block height, time, and chain ID.
type BlockInfo struct {
	// block height this transaction is executed
	Height uint64 `json:"height"`
	// time in nanoseconds since unix epoch. Uses Uint64 to ensure JavaScript compatibility.
	Time    Uint64 `json:"time"`
	ChainID string `json:"chain_id"`
}

// ContractInfo represents information about the current contract being executed.
// It includes the contract's address and code ID.
type ContractInfo struct {
	// Bech32 encoded sdk.AccAddress of the contract, to be used when sending messages
	Address HumanAddress `json:"address"`
}

type TransactionInfo struct {
	// Position of this transaction in the block.
	// The first transaction has index 0
	//
	// Along with BlockInfo.Height, this allows you to get a unique
	// transaction identifier for the chain for future queries
	Index uint32 `json:"index"`
}

// MessageInfo represents information about the message being executed.
// It includes the sender's address and the funds being sent with the message.
type MessageInfo struct {
	// Bech32 encoded sdk.AccAddress executing the contract
	Sender HumanAddress `json:"sender"`
	// Amount of funds send to the contract along with this message
	Funds Array[Coin] `json:"funds"`
}
