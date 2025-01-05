package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
)

// Verifier handles cryptographic verification operations
type Verifier struct{}

// New creates a new crypto verifier
func New() *Verifier {
	return &Verifier{}
}

// VerifySecp256k1Signature verifies a secp256k1 signature
func (v *Verifier) VerifySecp256k1Signature(hash, signature, pubkey []byte) (bool, error) {
	if len(hash) != sha256.Size {
		return false, ErrInvalidHashFormat
	}
	if len(signature) != 64 {
		return false, ErrInvalidSignatureFormat
	}
	if len(pubkey) != 33 && len(pubkey) != 65 {
		return false, ErrInvalidPubkeyFormat
	}

	// Parse public key
	pk, err := btcec.ParsePubKey(pubkey)
	if err != nil {
		return false, err
	}

	// Parse signature
	r := new(btcec.ModNScalar)
	s := new(btcec.ModNScalar)
	if !r.SetByteSlice(signature[:32]) || !s.SetByteSlice(signature[32:]) {
		return false, ErrInvalidSignatureFormat
	}
	sig := ecdsa.NewSignature(r, s)

	return sig.Verify(hash, pk), nil
}

// VerifyEd25519Signature verifies an ed25519 signature
func (v *Verifier) VerifyEd25519Signature(message, signature, pubkey []byte) (bool, error) {
	if len(signature) != ed25519.SignatureSize {
		return false, ErrInvalidSignatureFormat
	}
	if len(pubkey) != ed25519.PublicKeySize {
		return false, ErrInvalidPubkeyFormat
	}

	return ed25519.Verify(ed25519.PublicKey(pubkey), message, signature), nil
}

// VerifyEd25519Signatures verifies multiple ed25519 signatures in batch
func (v *Verifier) VerifyEd25519Signatures(messages [][]byte, signatures [][]byte, pubkeys [][]byte) (bool, error) {
	if len(messages) != len(signatures) || len(signatures) != len(pubkeys) {
		return false, ErrInvalidBatchFormat
	}

	for i := range messages {
		ok, err := v.VerifyEd25519Signature(messages[i], signatures[i], pubkeys[i])
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// SHA256 computes the SHA256 hash of data
func (v *Verifier) SHA256(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// SHA512 computes the SHA512 hash of data
func (v *Verifier) SHA512(data []byte) []byte {
	sum := sha512.Sum512(data)
	return sum[:]
}
