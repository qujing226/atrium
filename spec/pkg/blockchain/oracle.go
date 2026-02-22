package blockchain

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cloudflare/circl/kem/kyber/kyber768"
)

// DocumentEntry represents a versioned DID document on the chain.
type DocumentEntry struct {
	Content   []byte
	Version   uint64
	Timestamp time.Time
}

// Oracle simulates a "Live Blockchain" that maintains the authoritative state of DIDs.
// It supports dynamic updates to simulate key rotation or compromise.
type Oracle struct {
	docs map[string]*DocumentEntry
	mu   sync.RWMutex

	// Simulation parameters
	latency time.Duration
}

func NewOracle(latency time.Duration) *Oracle {
	return &Oracle{
		docs:    make(map[string]*DocumentEntry),
		latency: latency,
	}
}

// Register (Genesis)
func (o *Oracle) Register(did string, doc []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.docs[did] = &DocumentEntry{
		Content:   doc,
		Version:   1,
		Timestamp: time.Now(),
	}
}

// Resolve simulates a chain query with network latency.
func (o *Oracle) Resolve(did string) ([]byte, error) {
	// Simulate Network Delay
	if o.latency > 0 {
		time.Sleep(o.latency)
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	entry, ok := o.docs[did]
	if !ok {
		return nil, errors.New("did not found")
	}

	// Return a copy
	out := make([]byte, len(entry.Content))
	copy(out, entry.Content)
	return out, nil
}

// UpdateDID simulates a key rotation or revocation event.
// This is used by the test runner to trigger an "Attack" scenario.
// It generates a NEW Kyber keypair and updates the document.
func (o *Oracle) UpdateDID(did string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry, ok := o.docs[did]
	if !ok {
		return errors.New("did not exist")
	}

	// Generate NEW keys (simulate rotation/compromise)
	pk, _, err := kyber768.GenerateKeyPair(rand.Reader)
	if err != nil {
		return err
	}
	
	// Pack the new public key
	pkBytes := make([]byte, kyber768.PublicKeySize)
	pk.Pack(pkBytes)

	// In our simple simulation, the doc is just [Ed25519_PK || Kyber_PK]
	// We keep the Ed25519 part (first 32 bytes) but swap the Kyber part.
	if len(entry.Content) < 32 {
		return errors.New("invalid existing doc format")
	}

	newDoc := make([]byte, len(entry.Content))
	copy(newDoc, entry.Content) // Copy Ed25519
	// Overwrite Kyber part (from byte 32 onwards)
	// Note: We assume the doc length is fixed for this prototype
	copy(newDoc[32:], pkBytes)

	entry.Content = newDoc
	entry.Version++
	entry.Timestamp = time.Now()

	fmt.Printf("[Oracle] DID UPDATE EVENT: %s updated to Version %d (New Kyber Key)\n", did, entry.Version)
	return nil
}

// Implements the Chain interface used by OptimisticCache
func (o *Oracle) RegisterDidDoc(did string, doc []byte) {
	o.Register(did, doc)
}

func (o *Oracle) ResolveDidDoc(did string) ([]byte, error) {
	return o.Resolve(did)
}
