package api

// Common numeric constants
const (
	Zero                  = 0
	One                   = 1
	Two                   = 2
	Three                 = 3
	Four                  = 4
	Ten                   = 10
	Fifteen               = 15
	Seventeen             = 17
	Twenty                = 20
	TwentyFive            = 25
	Thirty                = 30
	FortyTwo              = 42
	Seventy               = 70
	SeventyFive           = 75
	OneHundred            = 100
	TwoHundredFifty       = 250
	FourHundredFortyFour  = 444
	SevenHundred          = 700
	OneThousand           = 1000
	OneThousandTwentyFour = 1024
	FortyMillion          = 40_000_000
	MaxInt32              = 2147483647
	MaxInt32PlusOne       = 2147483648
	MaxUint64             = ^uint64(0)
)

// Common string constants
const (
	EmptyJSON               = `{}`
	ReleaseJSON             = `{"release":{}}`
	ClaimJSON               = `{"claim":{}}`
	MyInstance              = "my instance"
	Creator                 = "creator"
	Fred                    = "fred"
	ATOM                    = "ATOM"
	TimeFormat              = "Time (%d gas): %s\n"
	ContractErrorEmpty      = "Contract error should be empty"
	ResponseBytesNotNil     = "Response bytes should not be nil"
	ContractResultNotNil    = "Contract result should not be nil"
	NoMessagesReturned      = "No messages should be returned"
	FailedToMarshalEnv      = "Failed to marshal env"
	FailedToMarshalInfo     = "Failed to marshal info"
	UnmanagedVectorOverride = "Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one."
	EmptyArray              = `[]`
	EmptyClaim              = `{"claim":{}}`
	FormatVerbose           = "%#v"
)

// Common hex constants
const (
	Hex0x00      = 0x00
	Hex0x0B      = 0x0B
	Hex0x3f      = 0x3f
	Hex0x4f      = 0x4f
	Hex0x72      = 0x72
	Hex0x84      = 0x84
	Hex0xaa      = 0xaa
	Hex0xbb      = 0xbb
	Hex0xcd      = 0xcd
	Hex0xf0      = 0xf0
	Hex0x895c33  = 0x895c33
	Hex0xd2189c  = 0xd2189c
	Hex0xd2ce86  = 0xd2ce86
	Hex0xbe8534  = 0xbe8534
	Hex0x15fce67 = 0x15fce67
	Hex0x160131d = 0x160131d
)

// Common gas constants
const (
	GasD35950  = 0xd35950
	GasD2189c  = 0xd2189c
	GasD2ce86  = 0xd2ce86
	GasBe8534  = 0xbe8534
	Gas15fce67 = 0x15fce67
	Gas160131d = 0x160131d
	Gas16057d3 = 0x16057d3
	Gas895c33  = 0x895c33
)

// File permissions
const (
	FileMode755 = 0o755
	FileMode666 = 0o666
	FileMode750 = 0o750
)

// Test data paths
const (
	HackatomWasmPath  = "../../testdata/hackatom.wasm"
	CyberpunkWasmPath = "../../testdata/cyberpunk.wasm"
	QueueWasmPath     = "../../testdata/queue.wasm"
	ReflectWasmPath   = "../../testdata/reflect.wasm"
	Floaty2WasmPath   = "../../testdata/floaty_2.0.wasm"
)

// Test data constants
const (
	TestBlockHeight  = 1578939743_987654321
	TestBlockHeight2 = 1955939743_123456789
	TestGasUsed      = 12345678
	TestGasPrice     = 0.01
	TestSequence     = 1234
	TestPortID       = 7897
	TestChannelID    = 4312324
	TestAmount       = 321
	TestLargeNumber  = 777777777
	TestLargeNumber2 = 888888888
	TestSmallNumber  = 123
	TestMediumNumber = 123456
	TestLargeAmount  = 700
	TestSmallAmount  = 1
	TestMediumAmount = 100
	TestLargeHeight  = 1955939743_123456789
	TestSmallHeight  = 1578939743_987654321
	TestLargeGas     = 40_000_000
	TestSmallGas     = 1000
	TestLargeMemory  = 32
	TestSmallMemory  = 1
	TestLargeCache   = 100
	TestSmallCache   = 1
)
