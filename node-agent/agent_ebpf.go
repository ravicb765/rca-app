//go:build ebpf
// +build ebpf

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// Agent maintains loaded collections and attached links so they remain valid for the lifetime of the process.
type Agent struct {
	endpoint string
	cols     []*ebpf.Collection
	links    []link.Link
	mu       sync.Mutex
	done     chan struct{}
}

var globalAgent *Agent

func StartEBPFAgent() error {
	if globalAgent != nil {
		return fmt.Errorf("ebpf agent already started")
	}
	endpoint := os.Getenv("RCA_APP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}

	a := &Agent{endpoint: endpoint, done: make(chan struct{})}
	globalAgent = a
	if err := a.loadAndAttach(); err != nil {
		// keep agent reference for possible partial cleanup
		return err
	}
	go a.backgroundLoop()
	return nil
}

func StopEBPFAgent() error {
	if globalAgent == nil {
		return nil
	}
	a := globalAgent
	close(a.done)
	// cleanup links and collections
	a.mu.Lock()
	for _, l := range a.links {
		_ = l.Close()
	}
	for _, c := range a.cols {
		_ = c.Close()
	}
	a.links = nil
	a.cols = nil
	a.mu.Unlock()
	globalAgent = nil
	return nil
}

func (a *Agent) loadAndAttach() error {
	files, err := filepath.Glob("node-agent/ebpf/*.o")
	if err != nil {
		return fmt.Errorf("failed to glob ebpf objects: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no ebpf objects found in node-agent/ebpf")
	}

	for _, f := range files {
		spec, err := ebpf.LoadCollectionSpec(f)
		if err != nil {
			return fmt.Errorf("failed to load spec %s: %w", f, err)
		}
		coll, err := ebpf.NewCollection(spec)
		if err != nil {
			return fmt.Errorf("failed to create collection from %s: %w", f, err)
		}

		a.mu.Lock()
		a.cols = append(a.cols, coll)
		a.mu.Unlock()

		// Attach programs based on their section (kprobe/kretprobe/tracepoint)
		for name, ps := range spec.Programs {
			section := ps.Section
			p := coll.Programs[name]
			if p == nil {
				// some toolchains name programs differently; try lookup by section name as fallback
				if alt := coll.Programs[strings.ReplaceAll(section, "/", "_")]; alt != nil {
					p = alt
				}
			}
			if p == nil {
				// not found; continue
				continue
			}

			if strings.HasPrefix(section, "kprobe/") {
				sym := strings.TrimPrefix(section, "kprobe/")
				l, err := link.Kprobe(sym, p, nil)
				if err != nil {
					// non-fatal — keep going but record the error
					fmt.Printf("warning: failed to attach kprobe %s for %s: %v\n", sym, name, err)
					continue
				}
				a.mu.Lock()
				a.links = append(a.links, l)
				a.mu.Unlock()
				fmt.Printf("attached kprobe %s (%s)\n", sym, name)
			} else if strings.HasPrefix(section, "kretprobe/") {
				sym := strings.TrimPrefix(section, "kretprobe/")
				l, err := link.Kretprobe(sym, p, nil)
				if err != nil {
					fmt.Printf("warning: failed to attach kretprobe %s for %s: %v\n", sym, name, err)
					continue
				}
				a.mu.Lock()
				a.links = append(a.links, l)
				a.mu.Unlock()
				fmt.Printf("attached kretprobe %s (%s)\n", sym, name)
			} else if strings.HasPrefix(section, "tracepoint/") {
				parts := strings.SplitN(strings.TrimPrefix(section, "tracepoint/"), "/", 2)
				if len(parts) == 2 {
					category, tp := parts[0], parts[1]
					l, err := link.AttachTracepoint(link.TracepointOptions{Category: category, Name: tp, Program: p})
					if err != nil {
						fmt.Printf("warning: failed to attach tracepoint %s/%s for %s: %v\n", category, tp, name, err)
						continue
					}
					a.mu.Lock()
					a.links = append(a.links, l)
					a.mu.Unlock()
					fmt.Printf("attached tracepoint %s/%s (%s)\n", category, tp, name)
				}
			} else {
				// Unknown section — skip attachment
				fmt.Printf("info: program %s section %s loaded but not attached (unknown section)\n", name, section)
			}
		}
	}
	return nil
}

func (a *Agent) gatherStatus() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	progNames := []string{}
	mapNames := []string{}
	for _, c := range a.cols {
		for name := range c.Programs {
			progNames = append(progNames, name)
		}
		for name := range c.Maps {
			mapNames = append(mapNames, name)
		}
	}
	return map[string]any{
		"hostname": func() string { h, _ := os.Hostname(); return h }(),
		"programs": progNames,
		"maps":     mapNames,
		"time":     time.Now().UTC().Format(time.RFC3339),
	}
}

func (a *Agent) backgroundLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			fmt.Println("ebpf agent stopping background loop")
			return
		case <-ticker.C:
			status := a.gatherStatus()
			buf := &bytes.Buffer{}
			_ = json.NewEncoder(buf).Encode(status)
			url := strings.TrimRight(a.endpoint, "/") + "/api/v1/agent/heartbeat"
			resp, err := http.Post(url, "application/json", buf)
			if err != nil {
				fmt.Printf("failed to post heartbeat to %s: %v\n", url, err)
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			fmt.Printf("heartbeat sent: status=%d programs=%d maps=%d\n", resp.StatusCode, len(status["programs"].([]string)), len(status["maps"].([]string)))
		}
	}
}
