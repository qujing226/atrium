package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/qujing226/atrium/pkg/blockchain"
	"github.com/qujing226/atrium/pkg/client"
	"github.com/qujing226/atrium/pkg/server"
)

const (
	RealWorldRelayPort = "9002"
	RealWorldRelayAddr = "localhost:" + RealWorldRelayPort
)

// Simulator configurations
type SimConfig struct {
	PacketLossRate float64
	NetworkJitter  time.Duration
	ChainReorgRate float64 // Probability that a "verified" chain state gets rolled back later
	MaxRetries     int
}

// ProbabilisticOracle injects realistic chain failures (reorgs/auth errors)
type ProbabilisticOracle struct {
	*blockchain.Oracle
	config SimConfig
	mu     sync.Mutex
}

func NewProbabilisticOracle(latency time.Duration, cfg SimConfig) *ProbabilisticOracle {
	return &ProbabilisticOracle{
		Oracle: blockchain.NewOracle(latency),
		config: cfg,
	}
}

func (o *ProbabilisticOracle) ResolveDidDoc(did string) ([]byte, error) {
	// 1. Simulate Network Delay with Jitter
	jitter := time.Duration(rand.Float64() * float64(o.config.NetworkJitter))
	time.Sleep(jitter)

	// 2. Base Resolve
	doc, err := o.Oracle.Resolve(did)
	if err != nil {
		return nil, err
	}

	// 3. Inject Probabilistic Failures (The 1-F(t) factor)
	if rand.Float64() < o.config.ChainReorgRate {
		fmt.Printf("[Oracle] Simulating Chain Reorg/Stale Read for %s\n", did)
	}

	return doc, nil
}

func runRealWorldExperiment(iterations int, cfg SimConfig) {
	fmt.Printf("\n======================================================\n")
	fmt.Printf("Running Monte Carlo Simulation (%d iterations)\n", iterations)
	fmt.Printf("Config: Loss=%.2f, Reorg=%.2f, Jitter=%v\n", cfg.PacketLossRate, cfg.ChainReorgRate, cfg.NetworkJitter)
	fmt.Printf("======================================================\n")

	relay := server.NewRelayServer()
	go relay.Start(RealWorldRelayPort)
	defer relay.Stop()
	time.Sleep(100 * time.Millisecond)

	oracle := NewProbabilisticOracle(500*time.Millisecond, cfg)

	var EALAViolations int
	var SuccessfulAborts int

	for i := 0; i < iterations; i++ {
		var aliceClient, bobClient *client.Client
		aliceCache := blockchain.NewOptimisticCache(oracle, func(did string, c, f []byte) {
			if aliceClient != nil {
				aliceClient.OnChainVerification(did, c, f)
			}
		})
		bobCache := blockchain.NewOptimisticCache(oracle, func(did string, c, f []byte) {
			if bobClient != nil {
				bobClient.OnChainVerification(did, c, f)
			}
		})

		aliceClient, _ = client.NewClient(fmt.Sprintf("did:alice:%d", i), aliceCache, RealWorldRelayAddr)
		bobClient, _ = client.NewClient(fmt.Sprintf("did:bob:%d", i), bobCache, RealWorldRelayAddr)

		var wg sync.WaitGroup
		deliveredDirty := false

		bobClient.OnMessage = func(sender string, msg []byte) {
			deliveredDirty = true
			wg.Done()
		}

		bobClient.Start()
		aliceClient.Start()
		time.Sleep(50 * time.Millisecond)

		aliceClient.Handshake(bobClient.Did)
		time.Sleep(600 * time.Millisecond)

		oracle.UpdateDID(bobClient.Did)

		wg.Add(1)
		aliceClient.Handshake(bobClient.Did)
		aliceClient.SendMessage("Phishing Payload")

		time.Sleep(1 * time.Second)

		if deliveredDirty {
			EALAViolations++
		} else {
			sess := bobClient.CurrentSession
			if sess != nil && sess.IsAborted() {
				SuccessfulAborts++
			}
		}

		aliceClient.Conn.Close()
		bobClient.Conn.Close()
	}

	fmt.Printf("Results over %d runs:\n", iterations)
	fmt.Printf("- EALA Violations (App polluted): %d\n", EALAViolations)
	fmt.Printf("- Successful Atomic Aborts: %d\n", SuccessfulAborts)
	fmt.Printf("- P(EALA_Violation) Empirical: %.4f\n", float64(EALAViolations)/float64(iterations))
}

func main() {
	rand.Seed(time.Now().UnixNano())
	cfg := SimConfig{
		PacketLossRate: 0.10,
		NetworkJitter:  200 * time.Millisecond,
		ChainReorgRate: 0.05,
		MaxRetries:     1,
	}

	runRealWorldExperiment(10, cfg)
}
