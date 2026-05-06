package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/qujing226/atrium/pkg/blockchain"
	"github.com/qujing226/atrium/pkg/client"
	"github.com/qujing226/atrium/pkg/server"
)

const (
	RelayPort = "9001"
	RelayAddr = "localhost:" + RelayPort
)

func runExperiment(chainLatency time.Duration, isStaleCache bool) {
	fmt.Printf("\n--- Experiment: Chain Latency = %v, Use Stale Cache = %v ---\n", chainLatency, isStaleCache)

	// 1. Setup Infrastructure
	relay := server.NewRelayServer()
	go relay.Start(RelayPort)
	defer relay.Stop()
	time.Sleep(100 * time.Millisecond)

	oracle := blockchain.NewOracle(chainLatency)

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

	aliceClient, _ = client.NewClient("did:qlink:alice", aliceCache, RelayAddr)
	bobClient, _ = client.NewClient("did:qlink:bob", bobCache, RelayAddr)

	var wg sync.WaitGroup
	deliveredMessages := 0

	bobClient.OnMessage = func(sender string, msg []byte) {
		deliveredMessages++
		fmt.Printf("[Bob App] Data Gate Opened! Delivered message: '%s'\n", string(msg))
		wg.Done()
	}

	bobClient.Start()
	aliceClient.Start()
	time.Sleep(200 * time.Millisecond)

	// Populate caches
	aliceClient.Handshake("did:qlink:bob")
	time.Sleep(chainLatency + 200*time.Millisecond)

	if isStaleCache {
		oracle.UpdateDID("did:qlink:bob")
	}

	// 2. Measure TTFB (Time to First Byte)
	start := time.Now()
	err := aliceClient.Handshake("did:qlink:bob")
	if err != nil {
		fmt.Printf("Handshake failed: %v\n", err)
		return
	}

	// App layer initiates send immediately after Handshake returns
	wg.Add(1)
	aliceClient.SendMessage("Test Payload 1")

	handshakeDone := time.Since(start)
	fmt.Printf("[Metrics] TTFB (Handshake Return Time): %v\n", handshakeDone)

	// Wait for delivery or abort
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		totalDeliveryTime := time.Since(start)
		fmt.Printf("[Metrics] Total time until App Delivery: %v\n", totalDeliveryTime)
	case <-time.After(chainLatency + 1*time.Second):
		fmt.Printf("[Metrics] Simulation timed out. Delivered messages: %d\n", deliveredMessages)
	}

	if isStaleCache && deliveredMessages > 0 {
		fmt.Printf("FAIL: Dirty data delivered to application!\n")
	} else if isStaleCache && deliveredMessages == 0 {
		fmt.Printf("PASS: Data Gate successfully blocked dirty data.\n")
	} else if !isStaleCache && deliveredMessages > 0 {
		fmt.Printf("PASS: Clean data delivered successfully.\n")
	}
}

func main() {
	// Exp 1: Fast Chain, No Attack
	runExperiment(100*time.Millisecond, false)

	// Exp 2: Slow Chain (1 second), No Attack (Shows TTFB benefit)
	runExperiment(1*time.Second, false)

	// Exp 3: Extremely Slow Chain (5 seconds), WITH Attack (Shows Data Gate)
	runExperiment(5*time.Second, true)
}
