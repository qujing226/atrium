package netexp

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/qujing226/atrium/sac"
)

func TestTLS13LocalAuthUsesRealTLS13(t *testing.T) {
	result, err := RunTLS(context.Background(), TLSExperiment{
		Mode:             sac.ModeTLS13LocalAuth,
		Payloads:         payloads("m1", "m2"),
		OperationTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("run tls local auth: %v", err)
	}

	if !result.UsedTLS {
		t.Fatalf("UsedTLS = false, want true")
	}
	if result.TLSVersion != tls.VersionTLS13 {
		t.Fatalf("TLSVersion = 0x%x, want TLS 1.3 0x%x", result.TLSVersion, tls.VersionTLS13)
	}
	if result.Delivered != 2 {
		t.Fatalf("delivered = %d, want 2", result.Delivered)
	}
	if result.InvalidDeliveries != 0 {
		t.Fatalf("invalid deliveries = %d, want 0", result.InvalidDeliveries)
	}
}

func TestTLS13AppLayerExternalVerifierDropsInvalidData(t *testing.T) {
	result, err := RunTLS(context.Background(), TLSExperiment{
		Mode:             sac.ModeTLS13AppLayerExternalVerifier,
		VerifierDelay:    100 * time.Millisecond,
		EvidenceValid:    false,
		Payloads:         payloads("m1", "m2"),
		BufferConfig:     sac.Config{MaxBufferedMessages: 4, MaxBufferedBytes: 1024},
		OperationTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("run tls external verifier: %v", err)
	}

	if !result.UsedTLS {
		t.Fatalf("UsedTLS = false, want true")
	}
	if result.TLSVersion != tls.VersionTLS13 {
		t.Fatalf("TLSVersion = 0x%x, want TLS 1.3 0x%x", result.TLSVersion, tls.VersionTLS13)
	}
	if result.FramesWritten != 2 || result.FramesRead != 2 {
		t.Fatalf("frames written/read = %d/%d, want 2/2", result.FramesWritten, result.FramesRead)
	}
	if result.Delivered != 0 {
		t.Fatalf("delivered = %d, want 0", result.Delivered)
	}
	if result.InvalidDeliveries != 0 {
		t.Fatalf("invalid deliveries = %d, want 0", result.InvalidDeliveries)
	}
	if !result.Aborted {
		t.Fatalf("aborted = false, want true")
	}
	if result.TimeToFirstFrame >= result.VerificationLatency {
		t.Fatalf("TTFF = %s, want less than verifier latency %s", result.TimeToFirstFrame, result.VerificationLatency)
	}
}

func TestTLS13UnsupportedStandardFeaturesAreExplicit(t *testing.T) {
	_, err := RunTLS(context.Background(), TLSExperiment{
		Mode:             sac.ModeTLS13EarlyData0RTT,
		Payloads:         payloads("early"),
		OperationTimeout: time.Second,
	})
	if !errors.Is(err, ErrUnsupportedTLSFeature) {
		t.Fatalf("0-RTT error = %v, want %v", err, ErrUnsupportedTLSFeature)
	}

	_, err = RunTLS(context.Background(), TLSExperiment{
		Mode:             sac.ModeTLS13PostHandshakeAuth,
		Payloads:         payloads("pha"),
		OperationTimeout: time.Second,
	})
	if !errors.Is(err, ErrUnsupportedTLSFeature) {
		t.Fatalf("post-handshake auth error = %v, want %v", err, ErrUnsupportedTLSFeature)
	}
}

func payloads(values ...string) [][]byte {
	out := make([][]byte, 0, len(values))
	for _, value := range values {
		out = append(out, []byte(value))
	}
	return out
}
