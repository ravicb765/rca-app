package main

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

func TestProcessMongoEvent(t *testing.T) {
	// Initialize NodeAgent with metrics
	cfg := &Config{}
	agent := NewNodeAgent(cfg)

	// Test Case 1: MongoDB Command
	// 16777343 is 127.0.0.1 in Little Endian (0x0100007f)
	cmdEvent := &agentebpf.HttpEvent{
		Type:  agentebpf.EventTypeMongoCommand,
		Daddr: 16777343,
		Dport: 27017,
	}
	// Simulate extracted command "find"
	copy(cmdEvent.Method[:], "find")

	agent.processMongoEvent(cmdEvent)

	// Verify Metric
	m := &dto.Metric{}
	agent.mongoQueriesTotal.WithLabelValues("127.0.0.1:27017", "find").Write(m)
	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter 1, got %v", m.Counter.GetValue())
	}

	// Test Case 2: MongoDB Response
	respEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeMongoResponse,
		Daddr:   16777343,
		Dport:   27017,
		Latency: 15000000, // 15ms
	}

	agent.processMongoEvent(respEvent)

	// Verify Histogram (checking count for simplicity)
	h := &dto.Metric{}
	agent.mongoQueryLatency.WithLabelValues("127.0.0.1:27017").Write(h)
	if h.Histogram.GetSampleCount() != 1 {
		t.Errorf("Expected histogram count 1, got %v", h.Histogram.GetSampleCount())
	}
}
