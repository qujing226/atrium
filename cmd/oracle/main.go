package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cloudflare/circl/kem/kyber/kyber768"
	"github.com/qujing226/atrium/pkg/blockchain"
)

type ResolveResponse struct {
	Document []byte `json:"document"`
	Error    string `json:"error,omitempty"`
}

type RegisterRequest struct {
	Did      string `json:"did"`
	Document []byte `json:"document"`
}

func main() {
	port := flag.String("port", "8080", "HTTP port for the Oracle")
	latencyMs := flag.Int("latency", 500, "Simulated blockchain consensus latency in ms")
	flag.Parse()

	fmt.Printf("=== QLink Abstract Oracle (Latency: %dms) ===\n", *latencyMs)

	latency := time.Duration(*latencyMs) * time.Millisecond
	oracle := blockchain.NewOracle(latency)

	// Pre-seed some default identities (Optional, as clients will now auto-register)
	for _, name := range []string{"alice", "bob"} {
		did := "did:qlink:" + name
		edPk, _, _ := ed25519.GenerateKey(rand.Reader)
		kyberPk, _, _ := kyber768.GenerateKeyPair(rand.Reader)
		kyberPkBytes := make([]byte, kyber768.PublicKeySize)
		kyberPk.Pack(kyberPkBytes)
		doc := make([]byte, 32+kyber768.PublicKeySize)
		copy(doc[0:32], edPk)
		copy(doc[32:], kyberPkBytes)
		oracle.Register(did, doc)
	}

	// Endpoint: Resolve DID
	http.HandleFunc("/resolve", func(w http.ResponseWriter, r *http.Request) {
		did := r.URL.Query().Get("did")
		if did == "" {
			http.Error(w, "missing did parameter", http.StatusBadRequest)
			return
		}
		doc, err := oracle.Resolve(did)
		resp := ResolveResponse{}
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Document = doc
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Endpoint: Register/Update DID
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Did == "" || len(req.Document) == 0 {
			http.Error(w, "invalid registration data", http.StatusBadRequest)
			return
		}
		oracle.Register(req.Did, req.Document)
		log.Printf("Identity Registered/Updated: %s (Size: %d)", req.Did, len(req.Document))
		w.WriteHeader(http.StatusOK)
	})

	// Endpoint: Mutate DID (for attack simulation)
	http.HandleFunc("/attack/mutate", func(w http.ResponseWriter, r *http.Request) {
		did := r.URL.Query().Get("did")
		if did == "" {
			http.Error(w, "missing did parameter", http.StatusBadRequest)
			return
		}
		err := oracle.UpdateDID(did)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Successfully mutated DID: %s\n", did)
	})

	log.Printf("Oracle listening on :%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
