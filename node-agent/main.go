package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	defer func() {
		if err := StopEBPFObjects(); err != nil {
			log.Println("error stopping ebpf agent:", err)
		}
	}()

	// Wait for signals to gracefully exit
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	// give background goroutines a moment to stop
	time.Sleep(1 * time.Second)
	fmt.Println("node-agent shutting down")
}
