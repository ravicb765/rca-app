package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
)

// ConnEvent matches the C struct conn_event in ebpf/network_tracer.c
type ConnEvent struct {
	Saddr     uint32
	Daddr     uint32
	Sport     uint16
	Dport     uint16
	_         [4]byte // Padding for alignment (timestamp is 8-byte aligned)
	Timestamp uint64
	Pid       uint32
	Comm      [16]byte
}

func main() {
	// 1. Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal(err)
	}

	// 2. Load the compiled eBPF program
	// Ensure you have run 'make' in node-agent/ebpf/ to generate this file
	spec, err := ebpf.LoadCollectionSpec("ebpf/network_tracer.o")
	if err != nil {
		log.Fatalf("Failed to load collection spec: %v", err)
	}

	// 3. Instantiate the collection (maps and programs)
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}
	defer coll.Close()

	// 4. Attach the kprobe to tcp_connect
	kp, err := link.Kprobe("tcp_connect", coll.Programs["kprobe__tcp_connect"], nil)
	if err != nil {
		log.Fatalf("Failed to attach kprobe: %v", err)
	}
	defer kp.Close()

	log.Println("eBPF tracer loaded. Waiting for TCP connect events...")

	// 5. Open a perf event reader from the 'events' map
	rd, err := perf.NewReader(coll.Maps["events"], os.Getpagesize())
	if err != nil {
		log.Fatalf("Failed to create perf reader: %v", err)
	}
	defer rd.Close()

	// 6. Listen for events
	go func() {
		for {
			record, err := rd.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					return
				}
				log.Printf("Error reading perf event: %v", err)
				continue
			}

			var event ConnEvent
			// Parse the raw bytes into our Go struct
			if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
				log.Printf("Failed to parse event: %v", err)
				continue
			}

			comm := string(bytes.TrimRight(event.Comm[:], "\x00"))
			src := net.IPv4(byte(event.Saddr), byte(event.Saddr>>8), byte(event.Saddr>>16), byte(event.Saddr>>24))
			dst := net.IPv4(byte(event.Daddr), byte(event.Daddr>>8), byte(event.Daddr>>16), byte(event.Daddr>>24))

			// Dport is in network byte order (Big Endian), while we read it as Little Endian. Swap bytes.
			dport := (event.Dport >> 8) | (event.Dport << 8)

			fmt.Printf("[%s] PID:%d Comm:%s %s:%d -> %s:%d\n", time.Now().Format(time.RFC3339), event.Pid, comm, src, event.Sport, dst, dport)
		}
	}()

	// 7. Self-monitoring routine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			// In production, this would be exported to Prometheus
			fmt.Printf("[%s] [AGENT_MONITOR] Alloc=%v MiB, TotalAlloc=%v MiB, Sys=%v MiB, NumGC=%v\n",
				time.Now().Format(time.RFC3339), m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024, m.NumGC)
		}
	}()

	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)
	<-stopper
}
