package types

// DB represents a key-value store interface
type DB interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
	Iterator(start, end []byte) Iterator
}

// Iterator represents a database iterator
type Iterator interface {
	Valid() bool
	Key() []byte
	Value() []byte
	Next()
	Close()
}

// API represents the contract API interface
type API interface {
	ValidateAddress(addr string) (uint64, error)
}

// GasMeter represents a gas meter interface
type GasMeter interface {
	ConsumeGas(amount uint64)
	GasConsumed() uint64
	GasLimit() uint64
}

// KVStore represents a key-value store interface
type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
}

// Querier represents a query interface
type Querier interface {
	Query(request []byte) ([]byte, error)
}

// GoAPI represents the Go API interface
type GoAPI interface {
	ValidateAddress(addr string) (uint64, error)
}
