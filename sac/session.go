package sac

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	ErrAborted             = errors.New("sac: session aborted")
	ErrInvalidState        = errors.New("sac: invalid state")
	ErrReplay              = errors.New("sac: duplicate or decreasing sequence")
	ErrSequenceGap         = errors.New("sac: sequence gap")
	ErrBufferLimitExceeded = errors.New("sac: isolation buffer limit exceeded")
)

type State uint8

const (
	StateIdle State = iota
	StateSpeculative
	StateVerified
	StateAborted
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateSpeculative:
		return "SPECULATIVE"
	case StateVerified:
		return "VERIFIED"
	case StateAborted:
		return "ABORTED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

type Config struct {
	MaxBufferedMessages int
	MaxBufferedBytes    int
}

type Message struct {
	Sequence uint64
	Payload  []byte
}

type Session struct {
	mu sync.Mutex

	state State
	cfg   Config

	buffer       map[uint64]Message
	bufferedByte int
	maxSequence  uint64
}

func NewSession(cfg Config) *Session {
	return &Session{
		state:  StateSpeculative,
		cfg:    cfg,
		buffer: make(map[uint64]Message),
	}
}

func NewVerifiedSession(cfg Config) *Session {
	return &Session{
		state:  StateVerified,
		cfg:    cfg,
		buffer: make(map[uint64]Message),
	}
}

func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) ReceivePlaintext(msg Message) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateAborted {
		return nil, ErrAborted
	}
	if msg.Sequence <= s.maxSequence {
		s.abortLocked()
		return nil, ErrReplay
	}
	if msg.Sequence != s.maxSequence+1 {
		s.abortLocked()
		return nil, ErrSequenceGap
	}

	switch s.state {
	case StateSpeculative:
		if err := s.bufferLocked(msg); err != nil {
			s.abortLocked()
			return nil, err
		}
		s.maxSequence = msg.Sequence
		return nil, nil
	case StateVerified:
		s.maxSequence = msg.Sequence
		return []Message{copyMessage(msg)}, nil
	default:
		return nil, ErrInvalidState
	}
}

func (s *Session) VerifySuccess() ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state {
	case StateSpeculative:
		s.state = StateVerified
		return s.releaseLocked(), nil
	case StateVerified:
		return nil, nil
	case StateAborted:
		return nil, ErrAborted
	default:
		return nil, ErrInvalidState
	}
}

func (s *Session) VerifyFailure(cause error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateAborted {
		return errors.New(ErrAborted.Error() + cause.Error())
	}
	s.abortLocked()
	return nil
}

func (s *Session) bufferLocked(msg Message) error {
	nextMessages := len(s.buffer) + 1
	nextBytes := s.bufferedByte + len(msg.Payload)

	if s.cfg.MaxBufferedMessages > 0 && nextMessages > s.cfg.MaxBufferedMessages {
		return ErrBufferLimitExceeded
	}
	if s.cfg.MaxBufferedBytes > 0 && nextBytes > s.cfg.MaxBufferedBytes {
		return ErrBufferLimitExceeded
	}

	cp := copyMessage(msg)
	s.buffer[msg.Sequence] = cp
	s.bufferedByte = nextBytes
	return nil
}

func (s *Session) releaseLocked() []Message {
	if len(s.buffer) == 0 {
		return nil
	}

	sequences := make([]uint64, 0, len(s.buffer))
	for seq := range s.buffer {
		sequences = append(sequences, seq)
	}
	sort.Slice(sequences, func(i, j int) bool {
		return sequences[i] < sequences[j]
	})

	out := make([]Message, 0, len(sequences))
	for _, seq := range sequences {
		out = append(out, copyMessage(s.buffer[seq]))
	}
	s.buffer = make(map[uint64]Message)
	s.bufferedByte = 0
	return out
}

func (s *Session) abortLocked() {
	s.state = StateAborted
	s.buffer = nil
	s.bufferedByte = 0
}

func copyMessage(msg Message) Message {
	payload := make([]byte, len(msg.Payload))
	copy(payload, msg.Payload)
	return Message{
		Sequence: msg.Sequence,
		Payload:  payload,
	}
}
