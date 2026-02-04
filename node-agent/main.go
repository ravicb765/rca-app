package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

// Config holds configuration for the agent
type Config struct {
	ServerEndpoint string
	HostName       string
}

// ConnectionStats represents aggregated connection data
type ConnectionStats struct {
	Source      string
	Destination string
	Packets     uint64
	Bytes       uint64
}

// NetworkTracer handles network-related eBPF collection
type NetworkTracer struct {
	// Conntrack table from eBPF map
	conntrackMap *ebpf.Map
	// Channel to send aggregated connection stats
	statsChan chan<- ConnectionStats
}

// EBPFManager manages eBPF programs and maps
type EBPFManager struct {
	programs      map[string]*ebpf.Program
	perfReaders   map[string]*perf.Reader
	links         []link.Link
	tracerObjs    *agentebpf.NetworkTracerObjects
	networkTracer *NetworkTracer
}

// NodeAgent is the main controller
type NodeAgent struct {
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	ebpfManager *EBPFManager
	config      *Config
	eventChan   chan interface{}
}

func NewNodeAgent(cfg *Config) *NodeAgent {
	ctx, cancel := context.WithCancel(context.Background())
	return &NodeAgent{
		ctx:       ctx,
		cancel:    cancel,
		config:    cfg,
		eventChan: make(chan interface{}, 1000),
		ebpfManager: &EBPFManager{
			programs:    make(map[string]*ebpf.Program),
			perfReaders: make(map[string]*perf.Reader),
			links:       []link.Link{},
		},
	}
}

func (a *NodeAgent) Start() error {
	log.Println("Starting Node Agent...")
	log.Printf("Server endpoint: %s", a.config.ServerEndpoint)

	// 1. Load eBPF objects
	if err := a.loadEBPF(); err != nil {
		log.Printf("Warning: eBPF loader error: %v", err)
	} else {
		log.Println("eBPF objects loaded")
	}

	// 2. Start background tasks
	a.wg.Add(1)
	go a.runEventLoop()

	return nil
}

func (a *NodeAgent) loadEBPF() error {
	// Load the network tracer objects
	objs, err := agentebpf.LoadNetworkTracer("ebpf/network_tracer.o")
	if err != nil {
		return fmt.Errorf("loading network tracer: %w", err)
	}
	a.ebpfManager.tracerObjs = objs

	// Attach the kprobe
	l, err := agentebpf.AttachNetworkTracer(objs)
	if err != nil {
		objs.Close()
		return fmt.Errorf("attaching network tracer: %w", err)
	}
	a.ebpfManager.links = append(a.ebpfManager.links, l)

	// Create perf reader
	rd, err := perf.NewReader(objs.Events, 4096)
	if err != nil {
		return fmt.Errorf("creating perf reader: %w", err)
	}
	a.ebpfManager.perfReaders["events"] = rd

	// Start handling events
	a.wg.Add(1)
	go a.handlePerfEvents(rd)

	return nil
}

func (a *NodeAgent) Stop() {
	log.Println("Stopping Node Agent...")
	a.cancel()

	// Close perf readers
	for name, r := range a.ebpfManager.perfReaders {
		r.Close()
		log.Printf("Closed perf reader: %s", name)
	}

	// Close links
	for _, l := range a.ebpfManager.links {
		l.Close()
	}

	// Close objects
	if a.ebpfManager.tracerObjs != nil {
		a.ebpfManager.tracerObjs.Close()
	}

	a.wg.Wait()
	log.Println("Node Agent stopped")
}

func (a *NodeAgent) runEventLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case event := <-a.eventChan:
			log.Printf("Processing event: %v", event)
		case <-ticker.C:
			log.Println("Agent heartbeat: healthy")
		}
	}
}

// ConnEvent matches the C struct in network_tracer.c
type ConnEvent struct {
	Saddr     uint32
	Daddr     uint32
	Sport     uint16
	Dport     uint16
	_         uint32 // Padding for alignment (timestamp is 8-byte aligned)
	Timestamp uint64
	Pid       uint32
	Comm      [16]byte
	_         uint32 // Padding at end of struct
}

func (a *NodeAgent) handlePerfEvents(rd *perf.Reader) {
	defer a.wg.Done()
	for {
		record, err := rd.Read()
		if err != nil {
			if perf.IsClosed(err) {
				return
			}
			log.Printf("Error reading perf event: %v", err)
			continue
		}

		if len(record.RawSample) < 48 {
			// Should be 48 bytes with padding
			continue
		}

		var event ConnEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Printf("Failed to decode event: %v", err)
			continue
		}

		srcIP := int2ip(event.Saddr)
		dstIP := int2ip(event.Daddr)
		comm := string(bytes.TrimRight(event.Comm[:], "\x00"))

		log.Printf("Connect: %s:%d -> %s:%d (pid: %d, comm: %s)",
			srcIP, event.Sport, dstIP, event.Dport, event.Pid, comm)
	}
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ip, nn)
	return ip
}

func main() {
	endpoint := os.Getenv("RCA_APP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}

	hostname, _ := os.Hostname()
	config := &Config{
		ServerEndpoint: endpoint,
		HostName:       hostname,
	}

	agent := NewNodeAgent(config)

	if err := agent.Start(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Wait for signals to gracefully exit
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	agent.Stop()
}
