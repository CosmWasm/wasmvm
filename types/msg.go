package types

import (
	"encoding/json"
	"fmt"
)

// Package types provides core types used throughout the wasmvm package.

//------- Results / Msgs -------------

// ContractResult represents the result of a contract execution.
type ContractResult struct {
	Ok  *Response `json:"ok,omitempty"`
	Err string    `json:"error,omitempty"`
}

// SubMessages returns the sub-messages of the result.
func (r *ContractResult) SubMessages() []SubMsg {
	if r.Ok != nil {
		return r.Ok.Messages
	}
	return nil
}

// Response defines the return value on a successful instantiate/execute/migrate.
// This is the counterpart of [Response](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/results/response.rs#L73-L88)
type Response struct {
	// Messages comes directly from the contract and is its request for action.
	// If the ReplyOn value matches the result, the runtime will invoke this
	// contract's `reply` entry point after execution. Otherwise, this is all
	// "fire and forget".
	Messages []SubMsg `json:"messages"`
	// base64-encoded bytes to return as ABCI.Data field
	Data []byte `json:"data"`
	// attributes for a log event to return over abci interface
	Attributes []EventAttribute `json:"attributes"`
	// custom events (separate from the main one that contains the attributes
	// above)
	Events []Event `json:"events"`
}

// Event represents an event emitted during contract execution.
type Event struct {
	Type       string                `json:"type"`
	Attributes Array[EventAttribute] `json:"attributes"`
}

// EventAttribute represents an attribute of an event.
type EventAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CosmosMsg represents a message that can be sent to the Cosmos SDK.
type CosmosMsg struct {
	Bank         *BankMsg         `json:"bank,omitempty"`
	Custom       json.RawMessage  `json:"custom,omitempty"`
	Distribution *DistributionMsg `json:"distribution,omitempty"`
	Gov          *GovMsg          `json:"gov,omitempty"`
	IBC          *IBCMsg          `json:"ibc,omitempty"`
	Staking      *StakingMsg      `json:"staking,omitempty"`
	Any          *AnyMsg          `json:"any,omitempty"`
	Wasm         *WasmMsg         `json:"wasm,omitempty"`
	IBC2         *IBC2Msg         `json:"ibc2,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for CosmosMsg.
func (m *CosmosMsg) UnmarshalJSON(data []byte) error {
	// We need a custom unmarshaler to parse both the "stargate" and "any" variants
	type InternalCosmosMsg struct {
		Bank         *BankMsg         `json:"bank,omitempty"`
		Custom       json.RawMessage  `json:"custom,omitempty"`
		Distribution *DistributionMsg `json:"distribution,omitempty"`
		Gov          *GovMsg          `json:"gov,omitempty"`
		IBC          *IBCMsg          `json:"ibc,omitempty"`
		Staking      *StakingMsg      `json:"staking,omitempty"`
		Any          *AnyMsg          `json:"any,omitempty"`
		Wasm         *WasmMsg         `json:"wasm,omitempty"`
		Stargate     *AnyMsg          `json:"stargate,omitempty"`
		IBC2         *IBC2Msg         `json:"ibc2,omitempty"`
	}
	var tmp InternalCosmosMsg
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	if tmp.Any != nil && tmp.Stargate != nil {
		return fmt.Errorf("invalid CosmosMsg: both 'any' and 'stargate' fields are set")
	} else if tmp.Any == nil && tmp.Stargate != nil {
		// Use "Any" for both variants
		tmp.Any = tmp.Stargate
	}

	*m = CosmosMsg{
		Bank:         tmp.Bank,
		Custom:       tmp.Custom,
		Distribution: tmp.Distribution,
		Gov:          tmp.Gov,
		IBC:          tmp.IBC,
		Staking:      tmp.Staking,
		Any:          tmp.Any,
		Wasm:         tmp.Wasm,
		IBC2:         tmp.IBC2,
	}
	return nil
}

// BankMsg represents a message to the bank module.
type BankMsg struct {
	Send *SendMsg `json:"send,omitempty"`
	Burn *BurnMsg `json:"burn,omitempty"`
}

// SendMsg represents a message to send tokens.
type SendMsg struct {
	ToAddress string      `json:"to_address"`
	Amount    Array[Coin] `json:"amount"`
}

// BurnMsg will burn the given coins from the contract's account.
// There is no Cosmos SDK message that performs this, but it can be done by calling the bank keeper.
// Important if a contract controls significant token supply that must be retired.
type BurnMsg struct {
	Amount Array[Coin] `json:"amount"`
}

// IBCMsg represents a message to the IBC module.
type IBCMsg struct {
	Transfer             *TransferMsg             `json:"transfer,omitempty"`
	SendPacket           *SendPacketMsg           `json:"send_packet,omitempty"`
	WriteAcknowledgement *WriteAcknowledgementMsg `json:"write_acknowledgement,omitempty"`
	CloseChannel         *CloseChannelMsg         `json:"close_channel,omitempty"`
	PayPacketFee         *PayPacketFeeMsg         `json:"pay_packet_fee,omitempty"`
	PayPacketFeeAsync    *PayPacketFeeAsyncMsg    `json:"pay_packet_fee_async,omitempty"`
}

// GovMsg represents a message to the governance module.
type GovMsg struct {
	// This maps directly to [MsgVote](https://github.com/cosmos/cosmos-sdk/blob/v0.42.5/proto/cosmos/gov/v1beta1/tx.proto#L46-L56) in the Cosmos SDK with voter set to the contract address.
	Vote *VoteMsg `json:"vote,omitempty"`
	/// This maps directly to [MsgVoteWeighted](https://github.com/cosmos/cosmos-sdk/blob/v0.45.8/proto/cosmos/gov/v1beta1/tx.proto#L66-L78) in the Cosmos SDK with voter set to the contract address.
	VoteWeighted *VoteWeightedMsg `json:"vote_weighted,omitempty"`
}

type voteOption int

// VoteMsg represents a message to vote on a proposal.
type VoteMsg struct {
	ProposalId uint64 `json:"proposal_id"`
	// Option is the vote option.
	//
	// This used to be called "vote", but was changed for consistency with Cosmos SDK.
	// The old name is still supported for backwards compatibility.
	Option voteOption `json:"option"`
}

// UnmarshalJSON implements json.Unmarshaler for VoteMsg.
func (m *VoteMsg) UnmarshalJSON(data []byte) error {
	// We need a custom unmarshaler to parse both the "stargate" and "any" variants
	type InternalVoteMsg struct {
		ProposalId uint64      `json:"proposal_id"`
		Option     *voteOption `json:"option"`
		Vote       *voteOption `json:"vote"` // old version
	}
	var tmp InternalVoteMsg
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	if tmp.Option != nil && tmp.Vote != nil {
		return fmt.Errorf("invalid VoteMsg: both 'option' and 'vote' fields are set")
	} else if tmp.Option == nil && tmp.Vote != nil {
		// Use "Option" for both variants
		tmp.Option = tmp.Vote
	}

	*m = VoteMsg{
		ProposalId: tmp.ProposalId,
		Option:     *tmp.Option,
	}
	return nil
}

type VoteWeightedMsg struct {
	ProposalId uint64               `json:"proposal_id"`
	Options    []WeightedVoteOption `json:"options"`
}

type WeightedVoteOption struct {
	Option voteOption `json:"option"`
	// Weight is a Decimal string, e.g. "0.25" for 25%
	Weight string `json:"weight"`
}

const (
	UnsetVoteOption voteOption = iota // The default value. We never return this in any valid instance (see toVoteOption).
	Yes
	No
	Abstain
	NoWithVeto
)

var fromVoteOption = map[voteOption]string{
	Yes:        "yes",
	No:         "no",
	Abstain:    "abstain",
	NoWithVeto: "no_with_veto",
}

var toVoteOption = map[string]voteOption{
	"yes":          Yes,
	"no":           No,
	"abstain":      Abstain,
	"no_with_veto": NoWithVeto,
}

func (v voteOption) String() string {
	return fromVoteOption[v]
}

func (v voteOption) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

func (s *voteOption) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}

	voteOption, ok := toVoteOption[j]
	if !ok {
		return fmt.Errorf("invalid vote option '%v'", j)
	}
	*s = voteOption
	return nil
}

// StakingMsg represents a message to the staking module.
type StakingMsg struct {
	Delegate   *DelegateMsg   `json:"delegate,omitempty"`
	Undelegate *UndelegateMsg `json:"undelegate,omitempty"`
	Redelegate *RedelegateMsg `json:"redelegate,omitempty"`
}

// DelegateMsg represents a message to delegate tokens.
type DelegateMsg struct {
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

// UndelegateMsg represents a message to undelegate tokens.
type UndelegateMsg struct {
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

// RedelegateMsg represents a message to redelegate tokens.
type RedelegateMsg struct {
	SrcValidator string `json:"src_validator"`
	DstValidator string `json:"dst_validator"`
	Amount       Coin   `json:"amount"`
}

// DistributionMsg represents a message to the distribution module.
type DistributionMsg struct {
	SetWithdrawAddress      *SetWithdrawAddressMsg      `json:"set_withdraw_address,omitempty"`
	WithdrawDelegatorReward *WithdrawDelegatorRewardMsg `json:"withdraw_delegator_reward,omitempty"`
	FundCommunityPool       *FundCommunityPoolMsg       `json:"fund_community_pool,omitempty"`
}

// SetWithdrawAddressMsg represents a message to set the withdraw address.
type SetWithdrawAddressMsg struct {
	// Address contains the `delegator_address` of a MsgSetWithdrawAddress
	Address string `json:"address"`
}

// WithdrawDelegatorRewardMsg represents a message to withdraw delegator rewards.
type WithdrawDelegatorRewardMsg struct {
	// Validator contains `validator_address` of a MsgWithdrawDelegatorReward
	Validator string `json:"validator"`
}

// FundCommunityPoolMsg is translated to a [MsgFundCommunityPool](https://github.com/cosmos/cosmos-sdk/blob/v0.42.4/proto/cosmos/distribution/v1beta1/tx.proto#LL69C1-L76C2).
// `depositor` is automatically filled with the current contract's address.
type FundCommunityPoolMsg struct {
	// Amount is the list of coins to be send to the community pool
	Amount Array[Coin] `json:"amount"`
}

// AnyMsg is encoded the same way as a protobof [Any](https://github.com/protocolbuffers/protobuf/blob/master/src/google/protobuf/any.proto).
// This is the same structure as messages in `TxBody` from [ADR-020](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-020-protobuf-transaction-encoding.md)
type AnyMsg struct {
	TypeURL string `json:"type_url"`
	Value   []byte `json:"value"`
}

// WasmMsg represents a message to the wasm module.
type WasmMsg struct {
	Execute      *ExecuteMsg      `json:"execute,omitempty"`
	Instantiate  *InstantiateMsg  `json:"instantiate,omitempty"`
	Instantiate2 *Instantiate2Msg `json:"instantiate2,omitempty"`
	Migrate      *MigrateMsg      `json:"migrate,omitempty"`
	UpdateAdmin  *UpdateAdminMsg  `json:"update_admin,omitempty"`
	ClearAdmin   *ClearAdminMsg   `json:"clear_admin,omitempty"`
}

// These are messages in the IBC lifecycle using the new IBC2 approach. Only usable by IBC2-enabled contracts.
type IBC2Msg struct {
	SendPacket           *IBC2SendPacketMsg           `json:"send_packet,omitempty"`
	WriteAcknowledgement *IBC2WriteAcknowledgementMsg `json:"write_acknowledgement,omitempty"`
}

// Sends an IBC packet with given payloads over the existing channel.
type IBC2SendPacketMsg struct {
	ChannelID string        `json:"channel_id"`
	Payloads  []IBC2Payload `json:"payloads"`
	Timeout   uint64        `json:"timeout,string,omitempty"`
}

type IBC2WriteAcknowledgementMsg struct {
	// The acknowledgement to send back
	Ack IBCAcknowledgement `json:"ack"`
	// Existing channel where the packet was received
	ChannelID string `json:"channel_id"`
	// Sequence number of the packet that was received
	PacketSequence uint64 `json:"packet_sequence"`
}

// ExecuteMsg represents a message to execute a wasm contract.
type ExecuteMsg struct {
	// ContractAddr is the sdk.AccAddress of the contract, which uniquely defines
	// the contract ID and instance ID. The sdk module should maintain a reverse lookup table.
	ContractAddr string `json:"contract_addr"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Handle` on the above-defined contract
	Msg []byte `json:"msg"`
	// Send is an optional amount of coins this contract sends to the called contract
	Funds Array[Coin] `json:"funds"`
}

// InstantiateMsg represents a message to instantiate a wasm contract.
type InstantiateMsg struct {
	// CodeID is the reference to the wasm byte code as used by the Cosmos-SDK
	CodeID uint64 `json:"code_id"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Instantiate` on a new contract with the above-defined CodeID
	Msg []byte `json:"msg"`
	// Send is an optional amount of coins this contract sends to the called contract
	Funds Array[Coin] `json:"funds"`
	// Label is optional metadata to be stored with a contract instance.
	Label string `json:"label"`
	// Admin (optional) may be set here to allow future migrations from this address
	Admin string `json:"admin,omitempty"`
}

// Instantiate2Msg will create a new contract instance from a previously uploaded CodeID
// using the predictable address derivation.
type Instantiate2Msg struct {
	// CodeID is the reference to the wasm byte code as used by the Cosmos-SDK
	CodeID uint64 `json:"code_id"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Instantiate` on a new contract with the above-defined CodeID
	Msg []byte `json:"msg"`
	// Send is an optional amount of coins this contract sends to the called contract
	Funds Array[Coin] `json:"funds"`
	// Label is optional metadata to be stored with a contract instance.
	Label string `json:"label"`
	// Admin (optional) may be set here to allow future migrations from this address
	Admin string `json:"admin,omitempty"`
	Salt  []byte `json:"salt"`
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

// UpdateAdminMsg is the Go counterpart of WasmMsg::UpdateAdmin
// (https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta5/packages/std/src/results/cosmos_msg.rs#L158-L160).
type UpdateAdminMsg struct {
	// ContractAddr is the sdk.AccAddress of the target contract.
	ContractAddr string `json:"contract_addr"`
	// Admin is the sdk.AccAddress of the new admin.
	Admin string `json:"admin"`
}

// ClearAdminMsg is the Go counterpart of WasmMsg::ClearAdmin
// (https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta5/packages/std/src/results/cosmos_msg.rs#L158-L160).
type ClearAdminMsg struct {
	// ContractAddr is the sdk.AccAddress of the target contract.
	ContractAddr string `json:"contract_addr"`
}

// TransferMsg represents a message to transfer tokens through IBC.
type TransferMsg struct {
	ChannelID string     `json:"channel_id"`
	ToAddress string     `json:"to_address"`
	Amount    Coin       `json:"amount"`
	Timeout   IBCTimeout `json:"timeout"`
	Memo      string     `json:"memo,omitempty"`
}

// SendPacketMsg represents a message to send an IBC packet.
type SendPacketMsg struct {
	ChannelID string     `json:"channel_id"`
	Data      []byte     `json:"data"`
	Timeout   IBCTimeout `json:"timeout"`
}

// WriteAcknowledgementMsg represents a message to write an IBC packet acknowledgement.
type WriteAcknowledgementMsg struct {
	// The acknowledgement to send back
	Ack IBCAcknowledgement `json:"ack"`
	// Existing channel where the packet was received
	ChannelID string `json:"channel_id"`
	// Sequence number of the packet that was received
	PacketSequence uint64 `json:"packet_sequence"`
}

// CloseChannelMsg represents a message to close an IBC channel.
type CloseChannelMsg struct {
	ChannelID string `json:"channel_id"`
}

// PayPacketFeeMsg represents a message to pay fees for an IBC packet.
type PayPacketFeeMsg struct {
	// The channel id on the chain where the packet is sent from (this chain).
	ChannelID string `json:"channel_id"`
	Fee       IBCFee `json:"fee"`
	// The port id on the chain where the packet is sent from (this chain).
	PortID string `json:"port_id"`
	// Allowlist of relayer addresses that can receive the fee. This is currently not implemented and *must* be empty.
	Relayers Array[string] `json:"relayers"`
}

// PayPacketFeeAsyncMsg represents a message to pay fees for an IBC packet asynchronously.
type PayPacketFeeAsyncMsg struct {
	// The channel id on the chain where the packet is sent from (this chain).
	ChannelID string `json:"channel_id"`
	Fee       IBCFee `json:"fee"`
	// The port id on the chain where the packet is sent from (this chain).
	PortID string `json:"port_id"`
	// Allowlist of relayer addresses that can receive the fee. This is currently not implemented and *must* be empty.
	Relayers Array[string] `json:"relayers"`
	// The sequence number of the packet that should be incentivized.
	Sequence uint64 `json:"sequence"`
}

// IBCFee represents the fees for an IBC packet.
type IBCFee struct {
	AckFee     Array[Coin] `json:"ack_fee"`
	ReceiveFee Array[Coin] `json:"receive_fee"`
	TimeoutFee Array[Coin] `json:"timeout_fee"`
}
