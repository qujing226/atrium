package sac

import (
	"testing"
	"time"
)

func TestScenarioComparesStrictOptimisticAndSACWithInvalidEvidence(t *testing.T) {
	workload := [][]byte{
		[]byte("m1"),
		[]byte("m2"),
		[]byte("m3"),
	}
	verifierLatency := 150 * time.Millisecond

	strict := RunScenario(Scenario{
		Mode:            ModeStrict,
		VerifierLatency: verifierLatency,
		EvidenceValid:   false,
		Workload:        workload,
	})
	if strict.TimeToFirstFrame != 0 {
		t.Fatalf("strict TTFF = %s, want 0 for rejected evidence", strict.TimeToFirstFrame)
	}
	if strict.InvalidDeliveries != 0 {
		t.Fatalf("strict invalid deliveries = %d, want 0", strict.InvalidDeliveries)
	}
	if !strict.Aborted {
		t.Fatalf("strict aborted = false, want true")
	}

	optimistic := RunScenario(Scenario{
		Mode:            ModeOptimistic,
		VerifierLatency: verifierLatency,
		EvidenceValid:   false,
		Workload:        workload,
	})
	if optimistic.TimeToFirstFrame != 0 {
		t.Fatalf("optimistic TTFF = %s, want 0", optimistic.TimeToFirstFrame)
	}
	if optimistic.InvalidDeliveries != len(workload) {
		t.Fatalf("optimistic invalid deliveries = %d, want %d", optimistic.InvalidDeliveries, len(workload))
	}

	dig := RunScenario(Scenario{
		Mode:            ModeSAC,
		VerifierLatency: verifierLatency,
		EvidenceValid:   false,
		Workload:        workload,
		Config: Config{
			MaxBufferedMessages: 8,
			MaxBufferedBytes:    1024,
		},
	})
	if dig.TimeToFirstFrame != 0 {
		t.Fatalf("sac TTFF = %s, want 0", dig.TimeToFirstFrame)
	}
	if dig.InvalidDeliveries != 0 {
		t.Fatalf("sac invalid deliveries = %d, want 0", dig.InvalidDeliveries)
	}
	if dig.DIGPeakMessages != len(workload) {
		t.Fatalf("sac DIG peak messages = %d, want %d", dig.DIGPeakMessages, len(workload))
	}
	if !dig.Aborted {
		t.Fatalf("sac aborted = false, want true")
	}
}

func TestScenarioSACDeliversAfterValidVerification(t *testing.T) {
	metrics := RunScenario(Scenario{
		Mode:            ModeSAC,
		VerifierLatency: 200 * time.Millisecond,
		EvidenceValid:   true,
		Workload: [][]byte{
			[]byte("hello"),
			[]byte("world"),
		},
		Config: Config{
			MaxBufferedMessages: 4,
			MaxBufferedBytes:    1024,
		},
	})

	if metrics.TimeToFirstFrame != 0 {
		t.Fatalf("TTFF = %s, want 0", metrics.TimeToFirstFrame)
	}
	if metrics.TimeToFirstVerifiedDelivery != 200*time.Millisecond {
		t.Fatalf("TTFVD = %s, want %s", metrics.TimeToFirstVerifiedDelivery, 200*time.Millisecond)
	}
	if metrics.Delivered != 2 {
		t.Fatalf("delivered = %d, want 2", metrics.Delivered)
	}
	if metrics.InvalidDeliveries != 0 {
		t.Fatalf("invalid deliveries = %d, want 0", metrics.InvalidDeliveries)
	}
}

func TestStandardTLSBaselinesAreNamedWithoutSyntheticPolicies(t *testing.T) {
	workload := [][]byte{
		[]byte("m1"),
		[]byte("m2"),
	}
	verifierLatency := 250 * time.Millisecond

	tlsLocal := RunScenario(Scenario{
		Mode:            ModeTLS13LocalAuth,
		VerifierLatency: verifierLatency,
		EvidenceValid:   false,
		Workload:        workload,
	})
	if tlsLocal.Delivered != len(workload) {
		t.Fatalf("tls local delivered = %d, want %d", tlsLocal.Delivered, len(workload))
	}
	if tlsLocal.InvalidDeliveries != 0 {
		t.Fatalf("tls local invalid deliveries = %d, want 0", tlsLocal.InvalidDeliveries)
	}

	tlsExternalVerifier := RunScenario(Scenario{
		Mode:            ModeTLS13AppLayerExternalVerifier,
		VerifierLatency: verifierLatency,
		EvidenceValid:   false,
		Workload:        workload,
		Config: Config{
			MaxBufferedMessages: 4,
			MaxBufferedBytes:    1024,
		},
	})
	if tlsExternalVerifier.TimeToFirstFrame != 0 {
		t.Fatalf("tls external verifier TTFF = %s, want 0", tlsExternalVerifier.TimeToFirstFrame)
	}
	if tlsExternalVerifier.InvalidDeliveries != 0 {
		t.Fatalf("tls external verifier invalid deliveries = %d, want 0", tlsExternalVerifier.InvalidDeliveries)
	}
	if !tlsExternalVerifier.Aborted {
		t.Fatalf("tls external verifier aborted = false, want true")
	}
}

func TestCiphertextQueueDoesNotMakeSpeculativeCryptoProgress(t *testing.T) {
	metrics := RunScenario(Scenario{
		Mode:            ModeCiphertextQueue,
		VerifierLatency: 100 * time.Millisecond,
		EvidenceValid:   true,
		Workload: [][]byte{
			[]byte("queued"),
		},
	})

	if metrics.TimeToFirstFrame != 100*time.Millisecond {
		t.Fatalf("ciphertext queue TTFF = %s, want %s", metrics.TimeToFirstFrame, 100*time.Millisecond)
	}
	if metrics.SpeculativeDecrypts != 0 {
		t.Fatalf("ciphertext queue speculative decrypts = %d, want 0", metrics.SpeculativeDecrypts)
	}
	if metrics.Delivered != 1 {
		t.Fatalf("ciphertext queue delivered = %d, want 1", metrics.Delivered)
	}
}
