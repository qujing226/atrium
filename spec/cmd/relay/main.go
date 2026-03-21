package main

import (
	"flag"
	"fmt"
	"github.com/qujing226/QLink/spec/pkg/server"
)

func main() {
	port := flag.String("port", "9000", "Port to listen on")
	flag.Parse()

	fmt.Println("=== QLink Relay Server ===")
	
	relay := server.NewRelayServer()
	if err := relay.Start(*port); err != nil {
		panic(err)
	}

	// Keep main thread alive
	select {}
}
