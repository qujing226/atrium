package tests

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/cloudflare/circl/sign/dilithium/mode3"
	"golang.org/x/crypto/sha3"
)

var (
	msg = []byte("QLink: Decentralized Post-Quantum Authenticated Key Exchange Protocol for WAN Environments")
)

// =====================================================================
// Signature Benchmarks
// =====================================================================

func BenchmarkEd25519_KeyGen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = ed25519.GenerateKey(rand.Reader)
	}
}

func BenchmarkDilithium3_KeyGen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = mode3.GenerateKey(rand.Reader)
	}
}

func BenchmarkEd25519_Sign(b *testing.B) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ed25519.Sign(priv, msg)
	}
}

func BenchmarkDilithium3_Sign(b *testing.B) {
	_, priv, _ := mode3.GenerateKey(rand.Reader)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = priv.Sign(rand.Reader, msg, crypto.Hash(0))
	}
}

func BenchmarkEd25519_Verify(b *testing.B) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := ed25519.Sign(priv, msg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ed25519.Verify(pub, msg, sig)
	}
}

func BenchmarkDilithium3_Verify(b *testing.B) {
	pub, priv, _ := mode3.GenerateKey(rand.Reader)
	sig, _ := priv.Sign(rand.Reader, msg, crypto.Hash(0))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mode3.Verify(pub, msg, sig)
	}
}

// =====================================================================
// Hash Benchmarks
// =====================================================================

func BenchmarkHash_SHA256(b *testing.B) {
	b.SetBytes(int64(len(msg)))
	for i := 0; i < b.N; i++ {
		h := sha256.Sum256(msg)
		_ = h
	}
}

func BenchmarkHash_SHA3_384(b *testing.B) {
	b.SetBytes(int64(len(msg)))
	for i := 0; i < b.N; i++ {
		h := sha3.Sum384(msg)
		_ = h
	}
}
