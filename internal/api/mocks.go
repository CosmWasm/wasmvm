// Package api provides mock implementations for testing the wasmvm API.
package api

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api/testdb"
	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	testAddress = "foobar"
)

/** helper constructors **/

// MockContractAddr is the default contract address used in mock tests.
const MockContractAddr = "contract"

// MockEnv creates a mock environment for testing.
func MockEnv() types.Env {
	return types.Env{
		Block: types.BlockInfo{
			Height:  123,
			Time:    1578939743_987654321,
			ChainID: testAddress,
		},
		Transaction: &types.TransactionInfo{
			Index: 4,
		},
		Contract: types.ContractInfo{
			Address: MockContractAddr,
		},
	}
}

// MockEnvBin creates a mock environment and returns it as JSON bytes.
func MockEnvBin(tb testing.TB) []byte {
	tb.Helper()
	bin, err := json.Marshal(MockEnv())
	require.NoError(tb, err)
	return bin
}

// MockInfo creates a mock message info with the given sender and funds.
func MockInfo(sender types.HumanAddress, funds []types.Coin) types.MessageInfo {
	return types.MessageInfo{
		Sender: sender,
		Funds:  funds,
	}
}

// MockInfoWithFunds creates a mock message info with the given sender and default funds.
func MockInfoWithFunds(sender types.HumanAddress) types.MessageInfo {
	return MockInfo(sender, []types.Coin{{
		Denom:  "ATOM",
		Amount: "100",
	}})
}

// MockInfoBin creates a mock message info and returns it as JSON bytes.
func MockInfoBin(tb testing.TB, sender types.HumanAddress) []byte {
	tb.Helper()
	bin, err := json.Marshal(MockInfoWithFunds(sender))
	require.NoError(tb, err)
	return bin
}

// MockIBCChannel creates a mock IBC channel with the given parameters.
func MockIBCChannel(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannel {
	return types.IBCChannel{
		Endpoint: types.IBCEndpoint{
			PortID:    "my_port",
			ChannelID: channelID,
		},
		CounterpartyEndpoint: types.IBCEndpoint{
			PortID:    "their_port",
			ChannelID: "channel-7",
		},
		Order:        ordering,
		Version:      ibcVersion,
		ConnectionID: "connection-3",
	}
}

// MockIBCChannelOpenInit creates a mock IBC channel open init message.
func MockIBCChannelOpenInit(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelOpenMsg {
	return types.IBCChannelOpenMsg{
		OpenInit: &types.IBCOpenInit{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
		OpenTry: nil,
	}
}

// MockIBCChannelOpenTry creates a mock IBC channel open try message.
func MockIBCChannelOpenTry(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelOpenMsg {
	return types.IBCChannelOpenMsg{
		OpenInit: nil,
		OpenTry: &types.IBCOpenTry{
			Channel:             MockIBCChannel(channelID, ordering, ibcVersion),
			CounterpartyVersion: ibcVersion,
		},
	}
}

// MockIBCChannelConnectAck mocks IBC channel connect acknowledgement
func MockIBCChannelConnectAck(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelConnectMsg {
	return types.IBCChannelConnectMsg{
		OpenAck: &types.IBCOpenAck{
			Channel:             MockIBCChannel(channelID, ordering, ibcVersion),
			CounterpartyVersion: ibcVersion,
		},
		OpenConfirm: nil,
	}
}

// MockIBCChannelConnectConfirm mocks IBC channel connect confirmation
func MockIBCChannelConnectConfirm(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelConnectMsg {
	return types.IBCChannelConnectMsg{
		OpenAck: nil,
		OpenConfirm: &types.IBCOpenConfirm{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
	}
}

// MockIBCChannelCloseInit mocks IBC channel close initialization
func MockIBCChannelCloseInit(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelCloseMsg {
	return types.IBCChannelCloseMsg{
		CloseInit: &types.IBCCloseInit{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
		CloseConfirm: nil,
	}
}

// MockIBCChannelCloseConfirm mocks IBC channel close confirmation
func MockIBCChannelCloseConfirm(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelCloseMsg {
	return types.IBCChannelCloseMsg{
		CloseInit: nil,
		CloseConfirm: &types.IBCCloseConfirm{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
	}
}

// MockIBCPacket mocks an IBC packet
func MockIBCPacket(myChannel string, data []byte) types.IBCPacket {
	return types.IBCPacket{
		Data: data,
		Src: types.IBCEndpoint{
			PortID:    "their_port",
			ChannelID: "channel-7",
		},
		Dest: types.IBCEndpoint{
			PortID:    "my_port",
			ChannelID: myChannel,
		},
		Sequence: 15,
		Timeout: types.IBCTimeout{
			Block: &types.IBCTimeoutBlock{
				Revision: 1,
				Height:   123456,
			},
		},
	}
}

// MockIBCPacketReceive mocks receiving an IBC packet
func MockIBCPacketReceive(myChannel string, data []byte) types.IBCPacketReceiveMsg {
	return types.IBCPacketReceiveMsg{
		Packet: MockIBCPacket(myChannel, data),
	}
}

// MockIBCPacketAck mocks acknowledging an IBC packet
func MockIBCPacketAck(myChannel string, data []byte, ack types.IBCAcknowledgement) types.IBCPacketAckMsg {
	packet := MockIBCPacket(myChannel, data)

	return types.IBCPacketAckMsg{
		Acknowledgement: ack,
		OriginalPacket:  packet,
	}
}

// MockIBCPacketTimeout mocks timing out an IBC packet
func MockIBCPacketTimeout(myChannel string, data []byte) types.IBCPacketTimeoutMsg {
	packet := MockIBCPacket(myChannel, data)

	return types.IBCPacketTimeoutMsg{
		Packet: packet,
	}
}

/*** Mock GasMeter ****/
// This code is borrowed from Cosmos-SDK store/types/gas.go

// ErrorOutOfGas defines an error thrown when an action results in out of gas.
type ErrorOutOfGas struct {
	Descriptor string
}

// ErrorGasOverflow defines an error thrown when an action results gas consumption
// unsigned integer overflow.
type ErrorGasOverflow struct {
	Descriptor string
}

type MockGasMeter interface {
	types.GasMeter
	ConsumeGas(amount types.Gas, descriptor string)
}

type mockGasMeter struct {
	limit    types.Gas
	consumed types.Gas
}

// NewMockGasMeter returns a reference to a new mockGasMeter.
func NewMockGasMeter(limit types.Gas) MockGasMeter {
	return &mockGasMeter{
		limit:    limit,
		consumed: 0,
	}
}

func (g *mockGasMeter) GasConsumed() types.Gas {
	return g.consumed
}

func (g *mockGasMeter) Limit() types.Gas {
	return g.limit
}

// addUint64Overflow performs the addition operation on two uint64 integers and
// returns a boolean on whether or not the result overflows.
func addUint64Overflow(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, true
	}

	return a + b, false
}

func (g *mockGasMeter) ConsumeGas(amount types.Gas, descriptor string) {
	var overflow bool
	// TODO: Should we set the consumed field after overflow checking?
	g.consumed, overflow = addUint64Overflow(g.consumed, amount)
	if overflow {
		panic(ErrorGasOverflow{descriptor})
	}

	if g.consumed > g.limit {
		panic(ErrorOutOfGas{descriptor})
	}
}

/*** Mock types.KVStore ****/
// Much of this code is borrowed from Cosmos-SDK store/transient.go

// Note: these gas prices are all in *wasmer gas* and (sdk gas * 100)
//
// We making simple values and non-clear multiples so it is easy to see their impact in test output
// Also note we do not charge for each read on an iterator (out of simplicity and not needed for tests).
const (
	GetPrice    uint64 = 99000
	SetPrice    uint64 = 187000
	RemovePrice uint64 = 142000
	RangePrice  uint64 = 261000
)

// Lookup represents a lookup table
type Lookup struct {
	db    *testdb.MemDB
	meter MockGasMeter
}

// NewLookup creates a new lookup table
func NewLookup(meter MockGasMeter) *Lookup {
	return &Lookup{
		db:    testdb.NewMemDB(),
		meter: meter,
	}
}

// SetGasMeter sets the gas meter for the lookup
func (l *Lookup) SetGasMeter(meter MockGasMeter) {
	l.meter = meter
}

// WithGasMeter sets the gas meter for the lookup and returns the lookup
func (l *Lookup) WithGasMeter(meter MockGasMeter) *Lookup {
	return &Lookup{
		db:    l.db,
		meter: meter,
	}
}

// Get wraps the underlying DB's Get method panicking on error.
func (l Lookup) Get(key []byte) []byte {
	l.meter.ConsumeGas(GetPrice, "get")
	v, err := l.db.Get(key)
	if err != nil {
		panic(err)
	}

	return v
}

// Set wraps the underlying DB's Set method panicking on error.
func (l Lookup) Set(key, value []byte) {
	l.meter.ConsumeGas(SetPrice, "set")
	if err := l.db.Set(key, value); err != nil {
		panic(err)
	}
}

// Delete wraps the underlying DB's Delete method panicking on error.
func (l Lookup) Delete(key []byte) {
	l.meter.ConsumeGas(RemovePrice, "remove")
	if err := l.db.Delete(key); err != nil {
		panic(err)
	}
}

// Iterator wraps the underlying DB's Iterator method panicking on error.
func (l Lookup) Iterator(start, end []byte) types.Iterator {
	l.meter.ConsumeGas(RangePrice, "range")
	iter, err := l.db.Iterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

// ReverseIterator wraps the underlying DB's ReverseIterator method panicking on error.
func (l Lookup) ReverseIterator(start, end []byte) types.Iterator {
	l.meter.ConsumeGas(RangePrice, "range")
	iter, err := l.db.ReverseIterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

var _ types.KVStore = (*Lookup)(nil)

/***** Mock types.GoAPI ****/

// CanonicalLength is the length of canonical addresses.
const CanonicalLength = 32

// CostCanonical is the gas cost for canonicalizing an address.
const CostCanonical uint64 = 440

// CostHuman is the gas cost for humanizing an address.
const CostHuman uint64 = 550

// MockCanonicalizeAddress converts a human-readable address to its canonical form.
func MockCanonicalizeAddress(human string) (canonical []byte, gasCost uint64, err error) {
	if len(human) > CanonicalLength {
		return nil, 0, errors.New("human encoding too long")
	}
	res := make([]byte, CanonicalLength)
	copy(res, human)
	return res, CostCanonical, nil
}

// MockHumanizeAddress converts a canonical address to its human-readable form.
func MockHumanizeAddress(canon []byte) (human string, gasCost uint64, err error) {
	if len(canon) != CanonicalLength {
		return "", 0, errors.New("wrong canonical length")
	}
	cut := CanonicalLength
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human = string(canon[:cut])
	return human, CostHuman, nil
}

// MockValidateAddress mocks address validation
func MockValidateAddress(input string) (gasCost uint64, _ error) {
	canonicalized, gasCostCanonicalize, err := MockCanonicalizeAddress(input)
	gasCost += gasCostCanonicalize
	if err != nil {
		return gasCost, err
	}
	humanized, gasCostHumanize, err := MockHumanizeAddress(canonicalized)
	gasCost += gasCostHumanize
	if err != nil {
		return gasCost, err
	}
	if !strings.EqualFold(humanized, input) {
		return gasCost, errors.New("address validation failed")
	}

	return gasCost, nil
}

// NewMockAPI creates a new mock API
func NewMockAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockHumanizeAddress,
		CanonicalizeAddress: MockCanonicalizeAddress,
		ValidateAddress:     MockValidateAddress,
	}
}

// TestMockAPI tests the mock API implementation.
func TestMockAPI(t *testing.T) {
	canon, cost, err := MockCanonicalizeAddress(testAddress)
	require.NoError(t, err)
	require.Len(t, canon, CanonicalLength)
	require.Equal(t, CostCanonical, cost)

	human, cost, err := MockHumanizeAddress(canon)
	require.NoError(t, err)
	require.Equal(t, human, testAddress)
	require.Equal(t, CostHuman, cost)
}

/***** MockQuerier *****/

// DefaultQuerierGasLimit is the default gas limit for querier operations.
const DefaultQuerierGasLimit = 1_000_000

// MockQuerier is a mock implementation of the Querier interface for testing.
type MockQuerier struct {
	Bank    BankQuerier
	Custom  CustomQuerier
	usedGas uint64
}

// DefaultQuerier creates a new MockQuerier with the given contract address and coins.
func DefaultQuerier(contractAddr string, coins types.Array[types.Coin]) types.Querier {
	balances := map[string]types.Array[types.Coin]{
		contractAddr: coins,
	}
	return &MockQuerier{
		Bank:    NewBankQuerier(balances),
		Custom:  NoCustom{},
		usedGas: 0,
	}
}

// Query implements the Querier interface.
func (q *MockQuerier) Query(request types.QueryRequest, _ uint64) ([]byte, error) {
	marshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	q.usedGas += uint64(len(marshaled))
	if request.Bank != nil {
		return q.Bank.Query(request.Bank)
	}
	if request.Custom != nil {
		return q.Custom.Query(request.Custom)
	}
	if request.Staking != nil {
		return nil, types.UnsupportedRequest{Kind: "staking"}
	}
	if request.Wasm != nil {
		return nil, types.UnsupportedRequest{Kind: "wasm"}
	}
	return nil, types.Unknown{}
}

// GasConsumed returns the amount of gas consumed by the querier.
func (q MockQuerier) GasConsumed() uint64 {
	return q.usedGas
}

// BankQuerier is a mock implementation of bank queries.
type BankQuerier struct {
	Balances map[string]types.Array[types.Coin]
}

// NewBankQuerier creates a new BankQuerier with the given balances.
func NewBankQuerier(balances map[string]types.Array[types.Coin]) BankQuerier {
	bal := make(map[string]types.Array[types.Coin], len(balances))
	for k, v := range balances {
		dst := make([]types.Coin, len(v))
		copy(dst, v)
		bal[k] = dst
	}
	return BankQuerier{
		Balances: bal,
	}
}

// Query implements the bank query functionality.
func (q BankQuerier) Query(request *types.BankQuery) ([]byte, error) {
	if request.Balance != nil {
		denom := request.Balance.Denom
		coin := types.NewCoin(0, denom)
		for _, c := range q.Balances[request.Balance.Address] {
			if c.Denom == denom {
				coin = c
			}
		}
		resp := types.BalanceResponse{
			Amount: coin,
		}
		return json.Marshal(resp)
	}
	if request.AllBalances != nil {
		coins := q.Balances[request.AllBalances.Address]
		resp := types.AllBalancesResponse{
			Amount: coins,
		}
		return json.Marshal(resp)
	}
	return nil, types.UnsupportedRequest{Kind: "Empty BankQuery"}
}

// CustomQuerier is an interface for custom query implementations.
type CustomQuerier interface {
	Query(request json.RawMessage) ([]byte, error)
}

// NoCustom is a CustomQuerier that returns an unsupported request error.
type NoCustom struct{}

// Query implements the CustomQuerier interface.
func (NoCustom) Query(_ json.RawMessage) ([]byte, error) {
	return nil, types.UnsupportedRequest{Kind: "custom"}
}

// ReflectCustom is a CustomQuerier implementation for testing reflect contracts.
type ReflectCustom struct{}

// CustomQuery represents a query that can be handled by ReflectCustom.
type CustomQuery struct {
	Ping        *struct{}         `json:"ping,omitempty"`
	Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
}

// CapitalizedQuery represents a query to capitalize text.
type CapitalizedQuery struct {
	Text string `json:"text"`
}

// CustomResponse is the response format for CustomQuery.
type CustomResponse struct {
	Msg string `json:"msg"`
}

// Query implements the CustomQuerier interface for ReflectCustom.
func (ReflectCustom) Query(request json.RawMessage) ([]byte, error) {
	var query CustomQuery
	err := json.Unmarshal(request, &query)
	if err != nil {
		return nil, err
	}
	var resp CustomResponse
	switch {
	case query.Ping != nil:
		resp.Msg = "PONG"
	case query.Capitalized != nil:
		resp.Msg = strings.ToUpper(query.Capitalized.Text)
	default:
		return nil, errors.New("unsupported query")
	}
	return json.Marshal(resp)
}

// TestBankQuerierAllBalances tests the BankQuerier's AllBalances functionality.
func TestBankQuerierAllBalances(t *testing.T) {
	addr := "foobar"
	balance := types.Array[types.Coin]{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: addr,
			},
		},
	}
	res, err := q.Query(req, DefaultQuerierGasLimit)
	require.NoError(t, err)
	var resp types.AllBalancesResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, balance)

	// query missing account
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: "someone-else",
			},
		},
	}
	res, err = q.Query(req2, DefaultQuerierGasLimit)
	require.NoError(t, err)
	var resp2 types.AllBalancesResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Nil(t, resp2.Amount)
}

// TestBankQuerierBalance tests the BankQuerier's Balance functionality.
func TestBankQuerierBalance(t *testing.T) {
	addr := "foobar"
	balance := types.Array[types.Coin]{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account with matching denom
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "ATOM",
			},
		},
	}
	res, err := q.Query(req, DefaultQuerierGasLimit)
	require.NoError(t, err)
	var resp types.BalanceResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, types.NewCoin(12345678, "ATOM"))

	// query existing account with missing denom
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "BTC",
			},
		},
	}
	res, err = q.Query(req2, DefaultQuerierGasLimit)
	require.NoError(t, err)
	var resp2 types.BalanceResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Equal(t, resp2.Amount, types.NewCoin(0, "BTC"))

	// query missing account
	req3 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: "someone-else",
				Denom:   "ATOM",
			},
		},
	}
	res, err = q.Query(req3, DefaultQuerierGasLimit)
	require.NoError(t, err)
	var resp3 types.BalanceResponse
	err = json.Unmarshal(res, &resp3)
	require.NoError(t, err)
	assert.Equal(t, resp3.Amount, types.NewCoin(0, "ATOM"))
}

// TestReflectCustomQuerier tests the ReflectCustom querier implementation.
func TestReflectCustomQuerier(t *testing.T) {
	q := ReflectCustom{}

	// try ping
	msg, err := json.Marshal(CustomQuery{Ping: &struct{}{}})
	require.NoError(t, err)
	bz, err := q.Query(msg)
	require.NoError(t, err)
	var resp CustomResponse
	err = json.Unmarshal(bz, &resp)
	require.NoError(t, err)
	assert.Equal(t, "PONG", resp.Msg)

	// try capital
	msg2, err := json.Marshal(CustomQuery{Capitalized: &CapitalizedQuery{Text: "small."}})
	require.NoError(t, err)
	bz, err = q.Query(msg2)
	require.NoError(t, err)
	var resp2 CustomResponse
	err = json.Unmarshal(bz, &resp2)
	require.NoError(t, err)
	assert.Equal(t, "SMALL.", resp2.Msg)
}
