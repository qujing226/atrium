package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/qujing226/atrium/pkg/blockchain"
	"github.com/qujing226/atrium/pkg/client"
)

func main() {
	name := flag.String("name", "alice", "Your DID name")
	relay := flag.String("relay", "localhost:9000", "Relay server address")
	oracleUrl := flag.String("oracle", "http://localhost:8080", "Oracle HTTP URL")
	peer := flag.String("peer", "bob", "Target DID to chat with")
	benchmark := flag.Int("benchmark", 0, "Number of automated handshakes to run for data collection")
	flag.Parse()

	myDid := "did:qlink:" + *name
	peerDid := "did:qlink:" + *peer

	remoteOracle := blockchain.NewRemoteOracle(*oracleUrl)
	var c *client.Client

	cache := blockchain.NewOptimisticCache(remoteOracle, func(did string, c_bytes, f_bytes []byte) {
		if c != nil {
			c.OnChainVerification(did, c_bytes, f_bytes)
		}
	})

	var err error
	c, err = client.NewClient(myDid, cache, *relay)
	if err != nil {
		panic(err)
	}

	c.OnMessage = func(sender string, msg []byte) {
		// Log delivery for data collection
		if *benchmark > 0 {
			// In benchmark mode, we might want to log the latency from send to deliver
		} else {
			fmt.Printf("\n[RECEIVED from %s] >> %s\n> ", sender, string(msg))
		}
	}

	if err := c.Start(); err != nil {
		panic(err)
	}

	// 自动化压测模式
	if *benchmark > 0 && *name == "alice" {
		runBenchmark(c, peerDid, *benchmark)
		return
	}

	fmt.Println("Type /connect to initiate handshake with peer.")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		if text == "/connect" {
			start := time.Now()
			if err := c.Handshake(peerDid); err != nil {
				fmt.Printf("Handshake error: %v\n", err)
			} else {
				fmt.Printf("Handshake Finished in %v (TTFB)\n", time.Since(start))
			}
			continue
		}
		c.SendMessage(text)
	}
}

func runBenchmark(c *client.Client, peerDid string, iterations int) {
	fmt.Printf("!!! STARTING BENCHMARK: %d iterations !!!\n", iterations)

	file, _ := os.Create("latency_metrics.csv")
	defer file.Close()
	writer := csv.NewWriter(file)
	writer.Write([]string{"Iteration", "TTFB_ms"})
	defer writer.Flush()

	for i := 1; i <= iterations; i++ {
		fmt.Printf("Iteration %d/%d...\n", i, iterations)

		start := time.Now()
		err := c.Handshake(peerDid)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			ms := float64(duration.Microseconds()) / 1000.0
			writer.Write([]string{strconv.Itoa(i), fmt.Sprintf("%.3f", ms)})
			fmt.Printf("  Handshake TTFB: %.3f ms\n", ms)
		}

		// 等待一段时间再进行下一次，确保背景验证完成
		time.Sleep(2 * time.Second)
	}
	fmt.Println("Benchmark complete. Data saved to latency_metrics.csv")
}
