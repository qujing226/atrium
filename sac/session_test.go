package sac

import (
	"errors"
	"testing"
)

func TestSpeculativeSessionBuffersThenReleasesInOrder(t *testing.T) {
	s := NewSession(Config{
		MaxBufferedMessages: 4,
		MaxBufferedBytes:    1024,
	})

	if s.State() != StateSpeculative {
		t.Fatalf("new session state = %s, want %s", s.State(), StateSpeculative)
	}

	delivered, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("first")})
	if err != nil {
		t.Fatalf("receive first: %v", err)
	}
	if len(delivered) != 0 {
		t.Fatalf("speculative receive delivered %d messages, want 0", len(delivered))
	}

	delivered, err = s.ReceivePlaintext(Message{Sequence: 2, Payload: []byte("second")})
	if err != nil {
		t.Fatalf("receive second: %v", err)
	}
	if len(delivered) != 0 {
		t.Fatalf("speculative receive delivered %d messages, want 0", len(delivered))
	}

	released, err := s.VerifySuccess()
	if err != nil {
		t.Fatalf("verify success: %v", err)
	}
	if got, want := payloads(released), []string{"first", "second"}; !equalStrings(got, want) {
		t.Fatalf("released payloads = %v, want %v", got, want)
	}
	if s.State() != StateVerified {
		t.Fatalf("state after verify = %s, want %s", s.State(), StateVerified)
	}
}

func TestVerifiedSessionDeliversImmediately(t *testing.T) {
	s := NewVerifiedSession(Config{
		MaxBufferedMessages: 4,
		MaxBufferedBytes:    1024,
	})

	delivered, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("now")})
	if err != nil {
		t.Fatalf("receive verified: %v", err)
	}
	if got, want := payloads(delivered), []string{"now"}; !equalStrings(got, want) {
		t.Fatalf("delivered payloads = %v, want %v", got, want)
	}
}

func TestVerificationFailureAbortsAndDropsBufferedPlaintext(t *testing.T) {
	s := NewSession(Config{
		MaxBufferedMessages: 4,
		MaxBufferedBytes:    1024,
	})

	if _, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("dirty")}); err != nil {
		t.Fatalf("receive speculative: %v", err)
	}

	if err := s.VerifyFailure(errors.New("stale evidence")); err != nil {
		t.Fatalf("verify failure: %v", err)
	}

	released, err := s.VerifySuccess()
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("verify success after abort error = %v, want %v", err, ErrAborted)
	}
	if len(released) != 0 {
		t.Fatalf("released after abort = %d messages, want 0", len(released))
	}
}

func TestSpeculativeBufferLimitAborts(t *testing.T) {
	s := NewSession(Config{
		MaxBufferedMessages: 1,
		MaxBufferedBytes:    1024,
	})

	if _, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("one")}); err != nil {
		t.Fatalf("receive one: %v", err)
	}
	_, err := s.ReceivePlaintext(Message{Sequence: 2, Payload: []byte("two")})
	if !errors.Is(err, ErrBufferLimitExceeded) {
		t.Fatalf("receive over limit error = %v, want %v", err, ErrBufferLimitExceeded)
	}
	if s.State() != StateAborted {
		t.Fatalf("state after overflow = %s, want %s", s.State(), StateAborted)
	}
}

func TestDuplicateSequenceRejected(t *testing.T) {
	s := NewSession(Config{
		MaxBufferedMessages: 4,
		MaxBufferedBytes:    1024,
	})

	if _, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("one")}); err != nil {
		t.Fatalf("receive one: %v", err)
	}
	_, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("again")})
	if !errors.Is(err, ErrReplay) {
		t.Fatalf("duplicate error = %v, want %v", err, ErrReplay)
	}
}

func TestSequenceGapRejected(t *testing.T) {
	s := NewSession(Config{
		MaxBufferedMessages: 4,
		MaxBufferedBytes:    1024,
	})

	if _, err := s.ReceivePlaintext(Message{Sequence: 1, Payload: []byte("one")}); err != nil {
		t.Fatalf("receive one: %v", err)
	}
	_, err := s.ReceivePlaintext(Message{Sequence: 3, Payload: []byte("three")})
	if !errors.Is(err, ErrSequenceGap) {
		t.Fatalf("gap error = %v, want %v", err, ErrSequenceGap)
	}
	if s.State() != StateAborted {
		t.Fatalf("state after gap = %s, want %s", s.State(), StateAborted)
	}
}

func payloads(messages []Message) []string {
	out := make([]string, 0, len(messages))
	for _, msg := range messages {
		out = append(out, string(msg.Payload))
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
