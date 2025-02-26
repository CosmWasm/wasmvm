package cryptoapi

// CryptoVerifier defines the interface for crypto verification operations
type CryptoVerifier interface {
	// Secp256k1Verify verifies a secp256k1 signature
	Secp256k1Verify(hash, signature, publicKey []byte) (bool, error)

	// Secp256k1RecoverPubkey recovers a public key from a signature
	Secp256k1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error)

	// Ed25519Verify verifies an ed25519 signature
	Ed25519Verify(message, signature, publicKey []byte) (bool, error)

	// Ed25519BatchVerify verifies multiple ed25519 signatures
	Ed25519BatchVerify(messages, signatures, publicKeys [][]byte) (bool, error)
}

// BLS12381Operations defines operations for BLS12-381 curves
type BLS12381Operations interface {
	// BLS12381AggregateG1 aggregates multiple G1 points
	BLS12381AggregateG1(elements [][]byte) ([]byte, error)

	// BLS12381AggregateG2 aggregates multiple G2 points
	BLS12381AggregateG2(elements [][]byte) ([]byte, error)

	// BLS12381HashToG1 hashes a message to a G1 point
	BLS12381HashToG1(message, dst []byte) ([]byte, error)

	// BLS12381HashToG2 hashes a message to a G2 point
	BLS12381HashToG2(message, dst []byte) ([]byte, error)

	// BLS12381VerifyG1G2 verifies a pairing check
	BLS12381VerifyG1G2(g1Points, g2Points [][]byte) (bool, error)
}

// CryptoOperations combines all crypto operations into a single interface
type CryptoOperations interface {
	CryptoVerifier
	BLS12381Operations
	Secp256r1Verify(hash, signature, pubkey []byte) (bool, error)
	Secp256r1RecoverPubkey(hash, signature []byte, recovery byte) ([]byte, error)
}
