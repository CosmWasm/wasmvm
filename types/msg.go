package types

import (
	"encoding/json"
)

//------- Results / Msgs -------------

// ContractResult is the raw response from the instantiate/execute/migrate calls.
// This is mirrors Rust's ContractResult<Response>.
type ContractResult struct {
	Ok  *Response `json:"ok,omitempty"`
	Err string    `json:"error,omitempty"`
}

// Response defines the return value on a successful instantiate/execute/migrate.
// This is the counterpart of [Response](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/results/response.rs#L73-L88)
type Response struct {
	// Submessages are like Messages, but they guarantee a reply to the calling contract
	// after their execution, and return both success and error rather than auto-failing on error
	Submessages []SubMsg `json:"submessages"`
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg `json:"messages"`
	// base64-encoded bytes to return as ABCI.Data field
	Data []byte `json:"data"`
	// attributes for a log event to return over abci interface
	Attributes []EventAttribute `json:"attributes"`
}

// EventAttributes must encode empty array as []
type EventAttributes []EventAttribute

// MarshalJSON ensures that we get [] for empty arrays
func (a EventAttributes) MarshalJSON() ([]byte, error) {
	if len(a) == 0 {
		return []byte("[]"), nil
	}
	var raw []EventAttribute = a
	return json.Marshal(raw)
}

// UnmarshalJSON ensures that we get [] for empty arrays
func (a *EventAttributes) UnmarshalJSON(data []byte) error {
	// make sure we deserialize [] back to null
	if string(data) == "[]" || string(data) == "null" {
		return nil
	}
	var raw []EventAttribute
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = raw
	return nil
}

// EventAttribute
type EventAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CosmosMsg is an rust enum and only (exactly) one of the fields should be set
// Should we do a cleaner approach in Go? (type/data?)
type CosmosMsg struct {
	Bank     *BankMsg        `json:"bank,omitempty"`
	Custom   json.RawMessage `json:"custom,omitempty"`
	IBC      *IBCMsg         `json:"ibc,omitempty"`
	Staking  *StakingMsg     `json:"staking,omitempty"`
	Stargate *StargateMsg    `json:"stargate,omitempty"`
	Wasm     *WasmMsg        `json:"wasm,omitempty"`
}

type BankMsg struct {
	Send *SendMsg `json:"send,omitempty"`
	Burn *BurnMsg `json:"burn,omitempty"`
}

// SendMsg contains instructions for a Cosmos-SDK/SendMsg
// It has a fixed interface here and should be converted into the proper SDK format before dispatching
type SendMsg struct {
	ToAddress string `json:"to_address"`
	Amount    Coins  `json:"amount"`
}

// BurnMsg will burn the given coins from the contract's account.
// There is no Cosmos SDK message that performs this, but it can be done by calling the bank keeper.
// Important if a contract controls significant token supply that must be retired.
type BurnMsg struct {
	Amount Coins `json:"amount"`
}

type IBCMsg struct {
	Transfer     *TransferMsg     `json:"transfer,omitempty"`
	SendPacket   *SendPacketMsg   `json:"send_packet,omitempty"`
	CloseChannel *CloseChannelMsg `json:"close_channel,omitempty"`
}

type TransferMsg struct {
	ChannelID string `json:"channel_id"`
	ToAddress string `json:"to_address"`
	Amount    Coin   `json:"amount"`
	// block after which the packet times out.
	// at least one of timeout_block, timeout_timestamp is required
	TimeoutBlock *IBCTimeoutBlock `json:"timeout_block,omitempty"`
	// Nanoseconds since UNIX epoch
	// See https://golang.org/pkg/time/#Time.UnixNano
	// at least one of timeout_block, timeout_timestamp is required
	TimeoutTimestamp *uint64 `json:"timeout_timestamp,omitempty"`
}

type SendPacketMsg struct {
	ChannelID string `json:"channel_id"`
	Data      []byte `json:"data"`
	// block after which the packet times out.
	// at least one of timeout_block, timeout_timestamp is required
	TimeoutBlock *IBCTimeoutBlock `json:"timeout_block,omitempty"`
	// Nanoseconds since UNIX epoch
	// See https://golang.org/pkg/time/#Time.UnixNano
	// at least one of timeout_block, timeout_timestamp is required
	TimeoutTimestamp *uint64 `json:"timeout_timestamp,omitempty"`
}

type CloseChannelMsg struct {
	ChannelID string `json:"channel_id"`
}

type StakingMsg struct {
	Delegate   *DelegateMsg   `json:"delegate,omitempty"`
	Undelegate *UndelegateMsg `json:"undelegate,omitempty"`
	Redelegate *RedelegateMsg `json:"redelegate,omitempty"`
	Withdraw   *WithdrawMsg   `json:"withdraw,omitempty"`
}

type DelegateMsg struct {
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

type UndelegateMsg struct {
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

type RedelegateMsg struct {
	SrcValidator string `json:"src_validator"`
	DstValidator string `json:"dst_validator"`
	Amount       Coin   `json:"amount"`
}

type WithdrawMsg struct {
	Validator string `json:"validator"`
	// this is optional
	Recipient string `json:"recipient,omitempty"`
}

// StargateMsg is encoded the same way as a protobof [Any](https://github.com/protocolbuffers/protobuf/blob/master/src/google/protobuf/any.proto).
// This is the same structure as messages in `TxBody` from [ADR-020](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-020-protobuf-transaction-encoding.md)
type StargateMsg struct {
	TypeURL string `json:"type_url"`
	Value   []byte `json:"value"`
}

type WasmMsg struct {
	Execute     *ExecuteMsg     `json:"execute,omitempty"`
	Instantiate *InstantiateMsg `json:"instantiate,omitempty"`
	Migrate     *MigrateMsg     `json:"migrate,omitempty"`
}

// ExecuteMsg is used to call another defined contract on this chain.
// The calling contract requires the callee to be defined beforehand,
// and the address should have been defined in initialization.
// And we assume the developer tested the ABIs and coded them together.
//
// Since a contract is immutable once it is deployed, we don't need to transform this.
// If it was properly coded and worked once, it will continue to work throughout upgrades.
type ExecuteMsg struct {
	// ContractAddr is the sdk.AccAddress of the contract, which uniquely defines
	// the contract ID and instance ID. The sdk module should maintain a reverse lookup table.
	ContractAddr string `json:"contract_addr"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Handle` on the above-defined contract
	Msg []byte `json:"msg"`
	// Send is an optional amount of coins this contract sends to the called contract
	Send Coins `json:"send"`
}

// InstantiateMsg will create a new contract instance from a previously uploaded CodeID.
// This allows one contract to spawn "sub-contracts".
type InstantiateMsg struct {
	// CodeID is the reference to the wasm byte code as used by the Cosmos-SDK
	CodeID uint64 `json:"code_id"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Init` on a new contract with the above-defined CodeID
	Msg []byte `json:"msg"`
	// Send is an optional amount of coins this contract sends to the called contract
	Send Coins `json:"send"`
	// Label is optional metadata to be stored with a contract instance.
	Label string `json:"label"`
}

// MigrateMsg will migrate an existing contract from it's current wasm code (logic)
// to another previously uploaded wasm code. It requires the calling contract to be
// listed as "admin" of the contract to be migrated.
type MigrateMsg struct {
	// ContractAddr is the sdk.AccAddress of the target contract, to migrate.
	ContractAddr string `json:"contract_addr"`
	// NewCodeID is the reference to the wasm byte code for the new logic to migrate to
	NewCodeID uint64 `json:"new_code_id"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Migrate` on the above-defined contract
	Msg []byte `json:"msg"`
}
