package types

import (
	"sync"

	"github.com/CosmWasm/wasmvm/v2/internal/runtime/memory"
)

// GasOperation represents different types of gas operations
type GasOperation int

const (
	GasOperationMemoryRead GasOperation = iota
	GasOperationMemoryWrite
	GasOperationDBRead
	GasOperationDBWrite
	GasOperationDBDelete
	GasOperationCompile
)

// OutOfGasError represents an out of gas error
type OutOfGasError struct {
	Descriptor string
}

func (e OutOfGasError) Error() string {
	return "out of gas: " + e.Descriptor
}

// GasCost represents a gas cost with base and per-unit components
type GasCost struct {
	BaseCost uint64
	PerUnit  uint64
}

// TotalCost calculates total gas cost for an operation
func (g GasCost) TotalCost(units uint64) uint64 {
	return g.BaseCost + (g.PerUnit * units)
}

// GasMeter interface defines gas consumption methods
type GasMeter interface {
	GasConsumed() uint64
	ConsumeGas(amount uint64, descriptor string) error
}

// Add missing interfaces
type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
	Iterator(start, end []byte) Iterator
	ReverseIterator(start, end []byte) Iterator
}

type Iterator interface {
	Valid() bool
	Next()
	Key() []byte
	Value() []byte
	Close() error
	Domain() (start, end []byte)
	Error() error
}

type GoAPI interface {
	HumanizeAddress([]byte) (string, uint64, error)
	CanonicalizeAddress(string) ([]byte, uint64, error)
	ValidateAddress(string) (uint64, error)
	Secp256k1Verify(message, signature, pubkey []byte) (bool, uint64, error)
	Secp256k1RecoverPubkey(message, signature []byte, recovery uint8) ([]byte, uint64, error)
	Ed25519Verify(message, signature, pubkey []byte) (bool, uint64, error)
	Ed25519BatchVerify(messages [][]byte, signatures [][]byte, pubkeys [][]byte) (bool, uint64, error)
	// Add other required methods
}

type Querier interface {
	Query(request []byte) ([]byte, error)
}

// Add missing types
type Env struct {
	Block       BlockInfo
	Contract    ContractInfo
	Transaction TransactionInfo
}

type MessageInfo struct {
	Sender string
	Funds  []Coin
}

type ContractResult struct {
	Data   []byte
	Events []Event
}

type Reply struct {
	ID     uint64
	Result SubMsgResult
}

type UFraction struct {
	Numerator   uint64
	Denominator uint64
}

// Add these type definitions
type BlockInfo struct {
	Height  int64
	Time    int64
	ChainID string
}

type ContractInfo struct {
	Address string
	CodeID  uint64
}

type TransactionInfo struct {
	Index uint32
}

type Coin struct {
	Denom  string
	Amount uint64
}

type Event struct {
	Type       string
	Attributes []EventAttribute
}

type EventAttribute struct {
	Key   string
	Value string
}

type SubMsgResult struct {
	Ok  *SubMsgResponse
	Err string
}

type SubMsgResponse struct {
	Events []Event
	Data   []byte
}

// Add after other type definitions

// RuntimeEnvironment holds the execution context for host functions
type RuntimeEnvironment struct {
	DB         KVStore
	API        GoAPI
	Querier    Querier
	Gas        GasMeter
	GasConfig  GasConfig
	MemManager *memory.MemoryManager
	GasUsed    uint64 // Track gas usage

	// Iterator management
	iterators      map[uint64]map[uint64]Iterator
	iteratorsMutex sync.RWMutex
	nextCallID     uint64
	nextIterID     uint64
}

// Add methods to RuntimeEnvironment
func (e *RuntimeEnvironment) StartCall() uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextCallID++
	e.iterators[e.nextCallID] = make(map[uint64]Iterator)
	return e.nextCallID
}

func (e *RuntimeEnvironment) StoreIterator(callID uint64, iter Iterator) uint64 {
	e.iteratorsMutex.Lock()
	defer e.iteratorsMutex.Unlock()
	e.nextIterID++
	e.iterators[callID][e.nextIterID] = iter
	return e.nextIterID
}

func (e *RuntimeEnvironment) GetIterator(callID, iterID uint64) Iterator {
	e.iteratorsMutex.RLock()
	defer e.iteratorsMutex.RUnlock()
	if callMap, ok := e.iterators[callID]; ok {
		return callMap[iterID]
	}
	return nil
}
