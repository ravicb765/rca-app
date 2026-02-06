package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
)

type EventPayload struct {
	Map        string `json:"map"`
	Timestamp  int64  `json:"timestamp_ns"`
	DataBase64 string `json:"data_base64,omitempty"`
	Data       string `json:"data,omitempty"`
}

func makePayload(mapName string, rec perf.Record, sendHex bool) EventPayload {
	var dataB64 string
	var dataHex string
	if rec.RawSample != nil {
		dataB64 = base64.StdEncoding.EncodeToString(rec.RawSample)
		dataHex = "0x" + strings.ToLower(hex.EncodeToString(rec.RawSample))
	}
	p := EventPayload{Map: mapName, Timestamp: time.Now().UnixNano()}
	if sendHex {
		p.Data = dataHex
	} else {
		p.DataBase64 = dataB64
	}
	return p
}

func postEvent(client *http.Client, url string, payload EventPayload, token string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}


func main() {
	mapPath := flag.String("map", "", "Path to pinned perf event map (e.g. /sys/fs/bpf/my_perf_map)")
	mapName := flag.String("map-name", "perf_events", "Logical name to include in event payload")
	server := flag.String("server", "http://localhost:8080/api/v1/agent/event", "Server URL to POST events to")
	token := flag.String("token", "", "Optional bearer token for server auth")
	pageSize := flag.Int("pagesize", os.Getpagesize()*8, "Perf reader page size (bytes)")
	sendHex := flag.Bool("send-hex", false, "Send perf data as hex in 'data' field instead of base64 in 'data_base64'")
	flag.Parse()

	if *mapPath == "" {
		log.Fatal("--map must be provided and point to a pinned perf map path")
	}

	m, err := ebpf.LoadPinnedMap(*mapPath, nil)
	if err != nil {
		log.Fatalf("failed to load pinned map %s: %v", *mapPath, err)
	}
	defer m.Close()

	reader, err := perf.NewReader(m, *pageSize)
	if err != nil {
		log.Fatalf("failed to create perf reader: %v", err)
	}
	defer reader.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("listening for perf events from %s, posting to %s", *mapPath, *server)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		default:
			rec, err := reader.Read()
			if err != nil {
				if err == perf.ErrClosed {
					return
				}
				log.Printf("perf reader error: %v", err)
				// small sleep to avoid hot loop
				time.Sleep(200 * time.Millisecond)
				continue
			}
			payload := makePayload(*mapName, rec, *sendHex)
			if err := postEvent(client, *server, payload, *token); err != nil {
				log.Printf("failed to post event: %v", err)
			}
		}
	}
}
