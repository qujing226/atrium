package secure

import (
	"crypto"
	"crypto/rand"
	"errors"

	"github.com/cloudflare/circl/sign/dilithium/mode3"
)

// SignKeyPair is a Dilithium3 (ML-DSA-65) key pair used for post-quantum digital signatures.
type SignKeyPair struct {
	pk *mode3.PublicKey
	sk *mode3.PrivateKey
}

// NewSignKeyPair generates a fresh Dilithium3 key pair.
func NewSignKeyPair() (*SignKeyPair, error) {
	pk, sk, err := mode3.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &SignKeyPair{pk: pk, sk: sk}, nil
}

// Sign signs the message using Dilithium3.
func (kp *SignKeyPair) Sign(msg []byte) ([]byte, error) {
	if kp.sk == nil {
		return nil, errors.New("private key is nil")
	}
	sig, err := kp.sk.Sign(rand.Reader, msg, crypto.Hash(0))
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// Export returns the serialized public key bytes.
func (kp *SignKeyPair) Export() ([]byte, error) {
	if kp.pk == nil {
		return nil, errors.New("public key is nil")
	}
	return kp.pk.Bytes(), nil
}

// VerifySignature verifies a Dilithium3 signature using a raw public key byte slice.
func VerifySignature(pubKeyBytes, msg, sig []byte) bool {
	if len(pubKeyBytes) != mode3.PublicKeySize {
		return false
	}
	var pkArray [mode3.PublicKeySize]byte
	copy(pkArray[:], pubKeyBytes)
	pk := new(mode3.PublicKey)
	pk.Unpack(&pkArray)
	return mode3.Verify(pk, msg, sig)
}
