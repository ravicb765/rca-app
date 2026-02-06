package main

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

func TestProcessMemcachedEvent(t *testing.T) {
	// Initialize NodeAgent with metrics
	cfg := &Config{}
	agent := NewNodeAgent(cfg)

	// Test Case 1: Memcached Command
	// 16777343 is 127.0.0.1 in Little Endian (0x0100007f)
	cmdEvent := &agentebpf.HttpEvent{
		Type:  agentebpf.EventTypeMemcachedCommand,
		Daddr: 16777343,
		Dport: 11211,
	}
	copy(cmdEvent.Method[:], "get")

	agent.processMemcachedEvent(cmdEvent)

	// Verify Metric
	m := &dto.Metric{}
	agent.memcachedQueriesTotal.WithLabelValues("127.0.0.1:11211", "get").Write(m)
	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter 1, got %v", m.Counter.GetValue())
	}

	// Test Case 2: Memcached Response
	respEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeMemcachedResponse,
		Daddr:   16777343,
		Dport:   11211,
		Latency: 5000000, // 5ms
	}

	agent.processMemcachedEvent(respEvent)

	// Verify Histogram (checking count for simplicity)
	h := &dto.Metric{}
	agent.memcachedQueryLatency.WithLabelValues("127.0.0.1:11211").Write(h)
	if h.Histogram.GetSampleCount() != 1 {
		t.Errorf("Expected histogram count 1, got %v", h.Histogram.GetSampleCount())
	}
}
