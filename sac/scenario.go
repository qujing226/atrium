package sac

import "time"

type Mode uint8

const (
	ModeStrict Mode = iota
	ModeOptimistic
	ModeSAC
)

type Scenario struct {
	Mode            Mode
	VerifierLatency time.Duration
	EvidenceValid   bool
	Workload        [][]byte
	Config          Config
}

type Metrics struct {
	TimeToFirstFrame            time.Duration
	TimeToFirstVerifiedDelivery time.Duration
	VerificationLatency         time.Duration

	Delivered         int
	InvalidDeliveries int
	Aborted           bool

	DIGPeakMessages int
	DIGPeakBytes    int
}

func RunScenario(sc Scenario) Metrics {
	metrics := Metrics{
		VerificationLatency: sc.VerifierLatency,
	}

	switch sc.Mode {
	case ModeStrict:
		return runStrict(sc, metrics)
	case ModeOptimistic:
		return runOptimistic(sc, metrics)
	case ModeSAC:
		return runSAC(sc, metrics)
	default:
		metrics.Aborted = true
		return metrics
	}
}

func runStrict(sc Scenario, metrics Metrics) Metrics {
	if !sc.EvidenceValid {
		metrics.Aborted = true
		return metrics
	}

	if len(sc.Workload) > 0 {
		metrics.TimeToFirstFrame = sc.VerifierLatency
		metrics.TimeToFirstVerifiedDelivery = sc.VerifierLatency
	}
	metrics.Delivered = len(sc.Workload)
	return metrics
}

func runOptimistic(sc Scenario, metrics Metrics) Metrics {
	metrics.Delivered = len(sc.Workload)
	if !sc.EvidenceValid {
		metrics.InvalidDeliveries = len(sc.Workload)
		metrics.Aborted = true
	}
	return metrics
}

func runSAC(sc Scenario, metrics Metrics) Metrics {
	session := NewSession(sc.Config)
	for i, payload := range sc.Workload {
		_, err := session.ReceivePlaintext(Message{
			Sequence: uint64(i + 1),
			Payload:  payload,
		})
		if err != nil {
			metrics.Aborted = true
			return metrics
		}
		metrics.DIGPeakMessages = max(metrics.DIGPeakMessages, i+1)
		metrics.DIGPeakBytes += len(payload)
	}

	if !sc.EvidenceValid {
		_ = session.VerifyFailure(nil)
		metrics.Aborted = true
		return metrics
	}

	released, err := session.VerifySuccess()
	if err != nil {
		metrics.Aborted = true
		return metrics
	}
	metrics.Delivered = len(released)
	if len(released) > 0 {
		metrics.TimeToFirstVerifiedDelivery = sc.VerifierLatency
	}
	return metrics
}
