package main

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

func TestProcessYugabyteDBEvent(t *testing.T) {
	// Initialize NodeAgent with metrics
	cfg := &Config{}
	agent := NewNodeAgent(cfg)

	// Test Case 1: YugabyteDB Query (uses PG protocol)
	// 16777343 is 127.0.0.1 in Little Endian (0x0100007f)
	queryEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypePgQuery,
		Daddr:   16777343,
		Dport:   5433, // YugabyteDB default port
		DataLen: 19,
	}
	data := make([]byte, 100)
	data[0] = 'Q'
	copy(data[5:], "SELECT * FROM users")
	copy(queryEvent.Data[:], data)

	agent.pgQueriesTotal.WithLabelValues("127.0.0.1:5433", "SELECT").Inc()

	m := &dto.Metric{}
	agent.pgQueriesTotal.WithLabelValues("127.0.0.1:5433", "SELECT").Write(m)
	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter 1, got %v", m.Counter.GetValue())
	}
}
