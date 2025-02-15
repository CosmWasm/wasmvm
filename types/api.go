package types

type (
	// HumanizeAddressFunc is a type for functions that convert a canonical address (bytes)
	// to a human readable address (typically bech32).
	HumanizeAddressFunc func([]byte) (string, uint64, error)
	// CanonicalizeAddressFunc is a type for functions that convert a human readable address (typically bech32)
	// to a canonical address (bytes).
	CanonicalizeAddressFunc func(string) ([]byte, uint64, error)
	// ValidateAddressFunc is a type for functions that validate a human readable address (typically bech32).
	ValidateAddressFunc func(string) (uint64, error)
	// Secp256k1VerifyFunc verifies a signature given a message and public key
	Secp256k1VerifyFunc func(message, signature, pubkey []byte) (bool, uint64, error)
	// Secp256k1RecoverPubkeyFunc recovers a public key from a message hash, signature, and recovery ID
	Secp256k1RecoverPubkeyFunc func(hash, signature []byte, recovery_id uint8) ([]byte, uint64, error)
	// Ed25519VerifyFunc verifies an ed25519 signature
	Ed25519VerifyFunc func(message, signature, pubkey []byte) (bool, uint64, error)
	// Ed25519BatchVerifyFunc verifies multiple ed25519 signatures in a batch
	Ed25519BatchVerifyFunc func(messages [][]byte, signatures [][]byte, pubkeys [][]byte) (bool, uint64, error)
	// BLS12381AggregateG1Func aggregates multiple G1 points into a single compressed G1 point
	Bls12381AggregateG1Func func(elements []byte) ([]byte, error)
	// BLS12381AggregateG2Func aggregates multiple G2 points into a single compressed G2 point
	Bls12381AggregateG2Func func(elements []byte) ([]byte, error)
	// BLS12381HashToG1Func hashes arbitrary bytes to a compressed G1 point
	Bls12381HashToG1Func func(message, dst []byte) ([]byte, error)
	// BLS12381HashToG2Func hashes arbitrary bytes to a compressed G2 point
	Bls12381HashToG2Func func(message, dst []byte) ([]byte, error)
	// BLS12381PairingCheckFunc checks if e(a1, a2) == e(b1, b2) in the BLS12-381 pairing
	Bls12381PairingCheckFunc func(pairs []byte) (bool, error)
)

type GoAPI struct {
	HumanizeAddress        HumanizeAddressFunc
	CanonicalizeAddress    CanonicalizeAddressFunc
	ValidateAddress        ValidateAddressFunc
	Secp256k1Verify        Secp256k1VerifyFunc
	Secp256k1RecoverPubkey Secp256k1RecoverPubkeyFunc
	Ed25519Verify          Ed25519VerifyFunc
	Ed25519BatchVerify     Ed25519BatchVerifyFunc
	Bls12381AggregateG1    Bls12381AggregateG1Func
	Bls12381AggregateG2    Bls12381AggregateG2Func
	Bls12381HashToG1       Bls12381HashToG1Func
	Bls12381HashToG2       Bls12381HashToG2Func
	Bls12381PairingCheck   Bls12381PairingCheckFunc
}
