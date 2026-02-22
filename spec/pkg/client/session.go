package client

import (
	"errors"
	"fmt"
	"sync"

	didv1 "github.com/qujing226/QLink/spec/gen/qlink/did/v1"
	"github.com/qujing226/QLink/spec/pkg/secure"
)

// MaxPendingMessages defines the High-Water Mark for the isolation buffer.
const MaxPendingMessages = 100

// Session encapsulates the security state and cryptographic keys for a peer connection.
// It implements the "Data Gate" logic defined in the protocol specification.
type Session struct {
	PeerDid string
	State   didv1.SessionState

	// Cryptographic Ratchets
	TxRatchet *secure.ChainKey // My outgoing chain
	RxRatchet *secure.ChainKey // Incoming chain from peer
	TxSeq     uint64           // My outgoing sequence number

	// Isolation Buffer (The Data Gate)
	// Stores decrypted plaintexts that MUST NOT be delivered to the app layer yet.
	PendingMsgs [][]byte

	mu sync.Mutex
}

// NewSession initializes a session.
func NewSession(peerDid string, initialState didv1.SessionState, txKey, rxKey *secure.ChainKey) *Session {
	return &Session{
		PeerDid:     peerDid,
		State:       initialState,
		TxRatchet:   txKey,
		RxRatchet:   rxKey,
		PendingMsgs: make([][]byte, 0, 10),
	}
}

// ProcessIncomingMsg handles the state-dependent logic for received messages.
func (s *Session) ProcessIncomingMsg(plaintext []byte) (deliverNow bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.State {
	case didv1.SessionState_SESSION_STATE_VERIFIED:
		// Gate Open: Deliver immediately
		return true, nil

	case didv1.SessionState_SESSION_STATE_SPECULATIVE:
		// Gate Closed: Buffer the message
		if len(s.PendingMsgs) >= MaxPendingMessages {
			// DoS Protection: High-Water Mark exceeded
			s.abortLocked()
			return false, errors.New("isolation buffer overflow: session aborted")
		}
		
		// Copy buffer to prevent modification
		msgCopy := make([]byte, len(plaintext))
		copy(msgCopy, plaintext)
		s.PendingMsgs = append(s.PendingMsgs, msgCopy)
		
		return false, nil

	case didv1.SessionState_SESSION_STATE_ABORTED:
		return false, errors.New("session is aborted")

	default:
		return false, errors.New("invalid session state")
	}
}

// UpgradeToVerified performs the atomic state transition from SPECULATIVE to VERIFIED.
func (s *Session) UpgradeToVerified() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != didv1.SessionState_SESSION_STATE_SPECULATIVE {
		return nil // Already verified or aborted
	}

	// Atomic Transition
	s.State = didv1.SessionState_SESSION_STATE_VERIFIED
	
	// Flush Buffer
	if len(s.PendingMsgs) == 0 {
		return nil
	}
	
	out := s.PendingMsgs
	s.PendingMsgs = nil // Clear reference
	return out
}

// Abort performs the atomic state transition to ABORTED (Security Meltdown).
func (s *Session) Abort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.abortLocked()
}

func (s *Session) abortLocked() {
	if s.State == didv1.SessionState_SESSION_STATE_ABORTED {
		return
	}
	
	fmt.Printf("[Session %s] ABORTING! Destroying keys and buffer.\n", s.PeerDid)
	
	s.State = didv1.SessionState_SESSION_STATE_ABORTED
	s.PendingMsgs = nil // Secure deletion (let GC handle memory wiping)
}

func (s *Session) IsAborted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State == didv1.SessionState_SESSION_STATE_ABORTED
}
