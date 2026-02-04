package main

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

func TestProcessCassandraEvent(t *testing.T) {
	// Initialize NodeAgent with metrics
	cfg := &Config{}
	agent := NewNodeAgent(cfg)

	// Test Case 1: Cassandra Command
	// 16777343 is 127.0.0.1 in Little Endian (0x0100007f)
	cmdEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeCassandraCommand,
		Daddr:   16777343,
		Dport:   9042,
		DataLen: 26,
	}
	copy(cmdEvent.Data[:], "SELECT * FROM users")

	agent.processCassandraEvent(cmdEvent)

	// Verify Metric
	m := &dto.Metric{}
	agent.cassandraQueriesTotal.WithLabelValues("127.0.0.1:9042", "SELECT").Write(m)
	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter 1, got %v", m.Counter.GetValue())
	}

	// Test Case 2: Cassandra Response
	respEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeCassandraResponse,
		Daddr:   16777343,
		Dport:   9042,
		Latency: 10000000, // 10ms
	}

	agent.processCassandraEvent(respEvent)

	// Verify Histogram (checking count for simplicity)
	h := &dto.Metric{}
	agent.cassandraQueryLatency.WithLabelValues("127.0.0.1:9042").Write(h)
	if h.Histogram.GetSampleCount() != 1 {
		t.Errorf("Expected histogram count 1, got %v", h.Histogram.GetSampleCount())
	}
}
