package hostapi

// RuntimeEnvironment holds the execution context for host functions
type RuntimeEnvironment struct {
	Gas        GasMeter
	MemManager MemoryManager
}

// GasMeter interface for tracking gas usage
type GasMeter interface {
	ConsumeGas(amount uint64, descriptor string) error
	GasConsumed() uint64
}

// MemoryManager interface for managing WebAssembly memory
type MemoryManager interface {
	Read(offset uint32, length uint32) ([]byte, error)
	Write(offset uint32, data []byte) error
	Allocate(size uint32) (uint32, error)
}

// Context key for environment
type EnvContextKey string

const EnvironmentKey EnvContextKey = "env"
