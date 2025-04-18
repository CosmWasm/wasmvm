package types

import (
	"encoding/json"
)

//-------- Queries --------

// QueryResult is the Go counterpart of `ContractResult<Binary>`.
// The JSON annotations are used for deserializing directly. There is a custom serializer below.
type QueryResult queryResultImpl

type queryResultImpl struct {
	Ok  []byte `json:"ok,omitempty"`
	Err string `json:"error,omitempty"`
}

// A custom serializer that allows us to map QueryResult instances to the Rust
// enum `ContractResult<Binary>`.
// MarshalJSON implements json.Marshaler
func (q QueryResult) MarshalJSON() ([]byte, error) {
	// In case both Ok and Err are empty, this is interpreted and serialized
	// as an Ok case with no data because errors must not be empty.
	if q.Ok == nil && q.Err == "" {
		return []byte(`{"ok":""}`), nil
	}
	// If Ok is an empty slice, we want to serialize it as {"ok":""}
	if q.Ok != nil && len(q.Ok) == 0 {
		return []byte(`{"ok":""}`), nil
	}
	return json.Marshal(queryResultImpl(q))
}

//-------- Querier -----------

// Querier is a thing that allows the contract to query information
// from the environment it is executed in. This is typically used to query
// a different contract or another module in a Cosmos blockchain.
//
// Queries are performed synchronously, i.e. the original caller is blocked
// until the query response is returned.
type Querier interface {
	// Query takes a query request, performs the query and returns the response.
	// It takes a gas limit measured in [CosmWasm gas] (aka. wasmvm gas) to ensure
	// the query does not consume more gas than the contract execution has left.
	//
	// [CosmWasm gas]: https://github.com/CosmWasm/cosmwasm/blob/v1.3.1/docs/GAS.md
	Query(request QueryRequest, gasLimit uint64) ([]byte, error)
	// GasConsumed returns the gas that was consumed by the querier during its entire
	// lifetime or by the context in which it was executed in. The absolute gas values
	// must not be used directly as it is undefined what is included in this value. Instead
	// wasmvm will call GasConsumed before and after the query and use the difference
	// as the query's gas usage.
	// Like the gas limit above, this is measured in [CosmWasm gas] (aka. wasmvm gas).
	//
	// [CosmWasm gas]: https://github.com/CosmWasm/cosmwasm/blob/v1.3.1/docs/GAS.md
	GasConsumed() uint64
}

// this is a thin wrapper around the desired Go API to give us types closer to Rust FFI.
// RustQuery represents a query to be executed in Rust
func RustQuery(querier Querier, binRequest []byte, gasLimit uint64) QuerierResult {
	var request QueryRequest
	err := json.Unmarshal(binRequest, &request)
	if err != nil {
		return QuerierResult{
			Err: &SystemError{
				InvalidRequest: &InvalidRequest{
					Err:     err.Error(),
					Request: binRequest,
				},
			},
		}
	}
	bz, err := querier.Query(request, gasLimit)
	return ToQuerierResult(bz, err)
}

// This is a 2-level result.
// QuerierResult represents the result of a querier operation
type QuerierResult struct {
	Ok  *QueryResult `json:"ok,omitempty"`
	Err *SystemError `json:"error,omitempty"`
}

// ToQuerierResult converts a query result to a QuerierResult
func ToQuerierResult(response []byte, err error) QuerierResult {
	if err == nil {
		return QuerierResult{
			Ok: &QueryResult{
				Ok: response,
			},
		}
	}
	syserr := ToSystemError(err)
	if syserr != nil {
		return QuerierResult{
			Err: syserr,
		}
	}
	return QuerierResult{
		Ok: &QueryResult{
			Err: err.Error(),
		},
	}
}

// QueryRequest represents a request for querying various Cosmos SDK modules.
// It can contain queries for bank, custom, IBC, staking, distribution, stargate, grpc, or wasm modules.
type QueryRequest struct {
	Bank         *BankQuery         `json:"bank,omitempty"`
	Custom       json.RawMessage    `json:"custom,omitempty"`
	IBC          *IBCQuery          `json:"ibc,omitempty"`
	Staking      *StakingQuery      `json:"staking,omitempty"`
	Distribution *DistributionQuery `json:"distribution,omitempty"`
	Stargate     *StargateQuery     `json:"stargate,omitempty"`
	Grpc         *GrpcQuery         `json:"grpc,omitempty"`
	Wasm         *WasmQuery         `json:"wasm,omitempty"`
}

// BankQuery represents a query to the bank module.
// It can contain queries for supply, balance, all balances, denom metadata, or all denom metadata.
type BankQuery struct {
	Supply           *SupplyQuery           `json:"supply,omitempty"`
	Balance          *BalanceQuery          `json:"balance,omitempty"`
	AllBalances      *AllBalancesQuery      `json:"all_balances,omitempty"`
	DenomMetadata    *DenomMetadataQuery    `json:"denom_metadata,omitempty"`
	AllDenomMetadata *AllDenomMetadataQuery `json:"all_denom_metadata,omitempty"`
}

// SupplyQuery represents a query for the total supply of a specific denomination.
type SupplyQuery struct {
	Denom string `json:"denom"`
}

// SupplyResponse is the expected response to SupplyQuery.
type SupplyResponse struct {
	Amount Coin `json:"amount"`
}

// BalanceQuery represents a query for the balance of a specific denomination for a given address.
type BalanceQuery struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
}

// BalanceResponse is the expected response to BalanceQuery.
type BalanceResponse struct {
	Amount Coin `json:"amount"`
}

// AllBalancesQuery represents a query for all balances of a given address.
type AllBalancesQuery struct {
	Address string `json:"address"`
}

// AllBalancesResponse is the expected response to AllBalancesQuery.
type AllBalancesResponse struct {
	Amount Array[Coin] `json:"amount"`
}

// DenomMetadataQuery represents a query for metadata of a specific denomination.
type DenomMetadataQuery struct {
	Denom string `json:"denom"`
}

// DenomMetadataResponse represents the response containing metadata for a denomination.
type DenomMetadataResponse struct {
	Metadata DenomMetadata `json:"metadata"`
}

// AllDenomMetadataQuery represents a query for metadata of all denominations.
type AllDenomMetadataQuery struct {
	// Pagination is an optional argument.
	// Default pagination will be used if this is omitted
	Pagination *PageRequest `json:"pagination,omitempty"`
}

// AllDenomMetadataResponse represents the response for all denomination metadata
type AllDenomMetadataResponse struct {
	Metadata []DenomMetadata `json:"metadata"`
	// NextKey is the key to be passed to PageRequest.key to
	// query the next page most efficiently. It will be empty if
	// there are no more results.
	NextKey []byte `json:"next_key,omitempty"`
}

// IBCQuery defines a query request from the contract into the chain.
// This is the counterpart of [IbcQuery](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/ibc.rs#L61-L83).
type IBCQuery struct {
	PortID            *PortIDQuery            `json:"port_id,omitempty"`
	ListChannels      *ListChannelsQuery      `json:"list_channels,omitempty"`
	Channel           *ChannelQuery           `json:"channel,omitempty"`
	FeeEnabledChannel *FeeEnabledChannelQuery `json:"fee_enabled_channel,omitempty"`
}

// FeeEnabledChannelQuery represents a query for fee-enabled channels
type FeeEnabledChannelQuery struct {
	ChannelID string `json:"channel_id"`
	PortID    string `json:"port_id,omitempty"`
}

// FeeEnabledChannelResponse represents the response for fee-enabled channels
type FeeEnabledChannelResponse struct {
	FeeEnabled bool `json:"fee_enabled"`
}

// PortIDQuery represents a query for a port ID
type PortIDQuery struct{}

// PortIDResponse represents the response for a port ID
type PortIDResponse struct {
	PortID string `json:"port_id"`
}

// ListChannelsQuery is an IBCQuery that lists all channels that are bound to a given port.
// If `PortID` is unset, this list all channels bound to the contract's port.
// Returns a `ListChannelsResponse`.
// This is the counterpart of [IbcQuery::ListChannels](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/ibc.rs#L70-L73).
type ListChannelsQuery struct {
	// optional argument
	PortID string `json:"port_id,omitempty"`
}

// ListChannelsResponse represents the response for listing channels
type ListChannelsResponse struct {
	Channels Array[IBCChannel] `json:"channels"`
}

// ChannelQuery represents a query for a channel
type ChannelQuery struct {
	// optional argument
	PortID    string `json:"port_id,omitempty"`
	ChannelID string `json:"channel_id"`
}

// ChannelResponse represents the response for a channel
type ChannelResponse struct {
	// may be empty if there is no matching channel
	Channel *IBCChannel `json:"channel,omitempty"`
}

// StakingQuery represents a query to the staking module.
// It can contain queries for all validators, a specific validator, all delegations, a specific delegation, or the bonded denom.
type StakingQuery struct {
	AllValidators  *AllValidatorsQuery  `json:"all_validators,omitempty"`
	Validator      *ValidatorQuery      `json:"validator,omitempty"`
	AllDelegations *AllDelegationsQuery `json:"all_delegations,omitempty"`
	Delegation     *DelegationQuery     `json:"delegation,omitempty"`
	BondedDenom    *struct{}            `json:"bonded_denom,omitempty"`
}

// AllValidatorsQuery represents a query for all validators in the network.
type AllValidatorsQuery struct{}

// AllValidatorsResponse is the expected response to AllValidatorsQuery.
type AllValidatorsResponse struct {
	Validators Array[Validator] `json:"validators"`
}

// ValidatorQuery represents a query for a specific validator by address.
type ValidatorQuery struct {
	/// Address is the validator's address (e.g. cosmosvaloper1...)
	Address string `json:"address"`
}

// ValidatorResponse is the expected response to ValidatorQuery.
type ValidatorResponse struct {
	Validator *Validator `json:"validator"` // serializes to `null` when unset which matches Rust's Option::None serialization
}

// Validator represents a validator in the network.
type Validator struct {
	Address string `json:"address"`
	// decimal string, eg "0.02"
	Commission string `json:"commission"`
	// decimal string, eg "0.02"
	MaxCommission string `json:"max_commission"`
	// decimal string, eg "0.02"
	MaxChangeRate string `json:"max_change_rate"`
}

// AllDelegationsQuery represents a query for all delegations of a specific delegator.
type AllDelegationsQuery struct {
	Delegator string `json:"delegator"`
}

// DelegationQuery represents a query for a specific delegation between a delegator and validator.
type DelegationQuery struct {
	Delegator string `json:"delegator"`
	Validator string `json:"validator"`
}

// AllDelegationsResponse is the expected response to AllDelegationsQuery.
type AllDelegationsResponse struct {
	Delegations Array[Delegation] `json:"delegations"`
}

// Delegation represents a delegation between a delegator and validator.
type Delegation struct {
	Delegator string `json:"delegator"`
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

// DistributionQuery represents a query for distribution
type DistributionQuery struct {
	// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L222-L230>
	DelegatorWithdrawAddress *DelegatorWithdrawAddressQuery `json:"delegator_withdraw_address,omitempty"`
	// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L157-L167>
	DelegationRewards *DelegationRewardsQuery `json:"delegation_rewards,omitempty"`
	// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L180-L187>
	DelegationTotalRewards *DelegationTotalRewardsQuery `json:"delegation_total_rewards,omitempty"`
	// See <https://github.com/cosmos/cosmos-sdk/blob/b0acf60e6c39f7ab023841841fc0b751a12c13ff/proto/cosmos/distribution/v1beta1/query.proto#L202-L210>
	DelegatorValidators *DelegatorValidatorsQuery `json:"delegator_validators,omitempty"`
}

// DelegatorWithdrawAddressQuery represents a query for a delegator's withdraw address
type DelegatorWithdrawAddressQuery struct {
	DelegatorAddress string `json:"delegator_address"`
}

// DelegatorWithdrawAddressResponse represents the response for a delegator's withdraw address
type DelegatorWithdrawAddressResponse struct {
	WithdrawAddress string `json:"withdraw_address"`
}

// DelegationRewardsQuery represents a query for delegation rewards
type DelegationRewardsQuery struct {
	DelegatorAddress string `json:"delegator_address"`
	ValidatorAddress string `json:"validator_address"`
}

// DelegationRewardsResponse represents the response for delegation rewards
type DelegationRewardsResponse struct {
	Rewards []DecCoin `json:"rewards"`
}

// DelegationTotalRewardsQuery represents a query for total delegation rewards
type DelegationTotalRewardsQuery struct {
	DelegatorAddress string `json:"delegator_address"`
}

// DelegationTotalRewardsResponse represents the response for total delegation rewards
type DelegationTotalRewardsResponse struct {
	Rewards []DelegatorReward `json:"rewards"`
	Total   []DecCoin         `json:"total"`
}

// DelegatorReward represents rewards for a delegator from a specific validator.
type DelegatorReward struct {
	Reward           []DecCoin `json:"reward"`
	ValidatorAddress string    `json:"validator_address"`
}

// DelegatorValidatorsQuery represents a query for all validators a delegator has delegated to.
type DelegatorValidatorsQuery struct {
	DelegatorAddress string `json:"delegator_address"`
}

// DelegatorValidatorsResponse represents the response containing all validators a delegator has delegated to.
type DelegatorValidatorsResponse struct {
	Validators []string `json:"validators"`
}

// DelegationResponse is the expected response to Array[Delegation]Query.
type DelegationResponse struct {
	Delegation *FullDelegation `json:"delegation,omitempty"`
}

// FullDelegation represents a complete delegation including accumulated rewards and redelegation information.
type FullDelegation struct {
	Delegator          string      `json:"delegator"`
	Validator          string      `json:"validator"`
	Amount             Coin        `json:"amount"`
	AccumulatedRewards Array[Coin] `json:"accumulated_rewards"`
	CanRedelegate      Coin        `json:"can_redelegate"`
}

// BondedDenomResponse represents the response containing the bonded denomination.
type BondedDenomResponse struct {
	Denom string `json:"denom"`
}

// StargateQuery represents a query using the Stargate protocol.
// It is encoded the same way as abci_query, with path and protobuf encoded request data.
// The format is defined in [ADR-21](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-021-protobuf-query-encoding.md).
type StargateQuery struct {
	// The expected protobuf message type (not [Any](https://protobuf.dev/programming-guides/proto3/#any)), binary encoded
	Data []byte `json:"data"`
	// The fully qualified endpoint path used for routing.
	// It follows the format `/service_path/method_name`,
	// eg. "/cosmos.authz.v1beta1.Query/Grants"
	Path string `json:"path"`
}

// GrpcQuery represents a query using gRPC protocol.
// This allows querying information that is not exposed in the standard API.
// The chain needs to allowlist the supported queries.
type GrpcQuery struct {
	// The expected protobuf message type (not [Any](https://protobuf.dev/programming-guides/proto3/#any)), binary encoded
	Data []byte `json:"data"`
	// The fully qualified endpoint path used for routing.
	// It follows the format `/service_path/method_name`,
	// eg. "/cosmos.authz.v1beta1.Query/Grants"
	Path string `json:"path"`
}

// WasmQuery represents a query to the WASM module.
// It can contain smart queries, raw queries, contract info queries, or code info queries.
type WasmQuery struct {
	Smart        *SmartQuery        `json:"smart,omitempty"`
	Raw          *RawQuery          `json:"raw,omitempty"`
	ContractInfo *ContractInfoQuery `json:"contract_info,omitempty"`
	CodeInfo     *CodeInfoQuery     `json:"code_info,omitempty"`
}

// SmartQuery represents a smart contract query.
// The response is raw bytes ([]byte).
type SmartQuery struct {
	// Bech32 encoded sdk.AccAddress of the contract
	ContractAddr string `json:"contract_addr"`
	Msg          []byte `json:"msg"`
}

// RawQuery represents a raw contract query.
// The response is raw bytes ([]byte).
type RawQuery struct {
	// Bech32 encoded sdk.AccAddress of the contract
	ContractAddr string `json:"contract_addr"`
	Key          []byte `json:"key"`
}

// ContractInfoQuery represents a query for contract information.
type ContractInfoQuery struct {
	// Bech32 encoded sdk.AccAddress of the contract
	ContractAddr string `json:"contract_addr"`
}

// ContractInfoResponse represents the response containing contract information.
type ContractInfoResponse struct {
	CodeID  uint64 `json:"code_id"`
	Creator string `json:"creator"`
	// Set to the admin who can migrate contract, if any
	Admin  string `json:"admin,omitempty"`
	Pinned bool   `json:"pinned"`
	// Set if the contract is IBC enabled
	IBCPort string `json:"ibc_port,omitempty"`
	// Set if the contract is IBC2 enabled
	IBC2Port string `json:"ibc2_port,omitempty"`
}

// CodeInfoQuery represents a query for code information.
type CodeInfoQuery struct {
	CodeID uint64 `json:"code_id"`
}

// CodeInfoResponse represents the response containing code information.
type CodeInfoResponse struct {
	CodeID  uint64 `json:"code_id"`
	Creator string `json:"creator"`
	// Checksum is the hash of the Wasm blob. This field must always be set to a 32 byte value.
	// Everything else is considered a bug.
	Checksum Checksum `json:"checksum"`
}
