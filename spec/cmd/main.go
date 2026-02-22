package main

import (
	"fmt"
	"time"

	didv1 "github.com/qujing226/QLink/spec/gen/qlink/did/v1"
	"github.com/qujing226/QLink/spec/pkg/blockchain"
	"github.com/qujing226/QLink/spec/pkg/client"
	"github.com/qujing226/QLink/spec/pkg/server"
)

const (
	RelayPort = "9000"
	RelayAddr = "localhost:" + RelayPort
)

func main() {
	fmt.Println("=== QLink Protocol Simulation: The Attack Scenario ===")

	// 1. Start Relay
	relay := server.NewRelayServer()
	if err := relay.Start(RelayPort); err != nil {
		panic(err)
	}
	defer relay.Stop()
	time.Sleep(100 * time.Millisecond)

	// 2. Setup Oracle (The Authority)
	// Latency = 500ms to allow Speculative Window
	oracle := blockchain.NewOracle(500 * time.Millisecond)

	// In our refactored client.go, Client injects its own callback via a setter?
	// Ah, in client.go NewClient: "assume main.go responsible for injection".
	// Let's fix this wiring.
	// We need a way to link the Cache callback to the Client instance.
	// Since Client is created AFTER Cache, we can use a closure proxy.
	
	var aliceClient *client.Client
	var bobClient   *client.Client

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

	// 3. Init Clients
	var err error
	bobClient, err = client.NewClient("did:qlink:bob", bobCache, RelayAddr)
	if err != nil { panic(err) }
	bobClient.OnMessage = func(sender string, msg []byte) {
		fmt.Printf("✅ [Bob App] RECEIVED: '%s'\n", string(msg))
	}
	bobClient.Start()

	aliceClient, err = client.NewClient("did:qlink:alice", aliceCache, RelayAddr)
	if err != nil { panic(err) }
	aliceClient.OnMessage = func(sender string, msg []byte) {
		fmt.Printf("✅ [Alice App] RECEIVED: '%s'\n", string(msg))
	}
	aliceClient.Start()

	time.Sleep(500 * time.Millisecond)

	// =========================================================================
	// Phase 1: Warm-up (Establish valid cache)
	// =========================================================================
	fmt.Println("\n--- Phase 1: Warm-up (Populating Cache) ---")
	if err := aliceClient.Handshake("did:qlink:bob"); err != nil {
		panic(err)
	}
	// Wait for background verification to complete and cache to be marked valid
	time.Sleep(1 * time.Second) 
	fmt.Println(">>> Phase 1 Complete. Caches are warm.")

	// =========================================================================
	// Phase 2: The Attack (Oracle Mutation)
	// =========================================================================
	fmt.Println("\n--- Phase 2: The Attack (Oracle Mutation) ---")
	fmt.Println(">>> Simulating Bob's key compromise/rotation on chain...")
	// Attack: Bob's key changes on chain!
	// Alice doesn't know yet. Her cache has the OLD key.
	if err := oracle.UpdateDID("did:qlink:bob"); err != nil {
		panic(err)
	}

	// =========================================================================
	// Phase 3: Speculative Execution under Attack
	// =========================================================================
	fmt.Println("\n--- Phase 3: Speculative Handshake (Using Stale Cache) ---")
	// Alice initiates handshake using STALE cache (Fast Path)
	start := time.Now()
	if err := aliceClient.Handshake("did:qlink:bob"); err != nil {
		fmt.Printf("Handshake failed immediately: %v\n", err)
	} else {
		fmt.Printf(">>> Handshake 2 (Speculative) Finished in %v\n", time.Since(start))
	}

	// Alice sends a secret message immediately (Optimistic Send)
	fmt.Println(">>> Alice sends 'Secret Payload' (Optimistically)...")
	// Use manual SendPacket for more granular control if needed, but Client.SendMessage works.
	aliceClient.SendMessage("My Secret Payload")

	fmt.Println(">>> Waiting for Background Verification (500ms)...")
	
	// =========================================================================
	// Phase 4: The Truth Revealed (Eventual Consistency)
	// =========================================================================
	time.Sleep(1 * time.Second)
	
	fmt.Println("\n--- Simulation Complete ---")
	fmt.Println("Check the logs above:")
	fmt.Println("1. Did Alice report 'CRITICAL: Chain mismatch'?")
	fmt.Println("2. Did Bob print 'RECEIVED: My Secret Payload'? (He SHOULD NOT)")
}

// Helper to manually construct header (used by relay internally, but main.go uses client lib)
// No changes needed here as client lib handles packet construction.
func newHeader(from, to string) *didv1.Header {
	return &didv1.Header{
		Timestamp: time.Now().UnixMilli(),
		FromDid:   from,
		ToDid:     to,
	}
}
