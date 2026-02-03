package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	endpoint := os.Getenv("RCA_APP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}
	fmt.Println("node-agent starting")
	fmt.Println("server endpoint:", endpoint)

	if err := LoadEBPFObjects(); err != nil {
		log.Println("eBPF loader error:", err)
	} else {
		log.Println("eBPF objects loaded (or loader stub ran)")
	}

	// TODO: push telemetry to server and implement exporters
}
