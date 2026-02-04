package main

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

func TestProcessMysqlEvent(t *testing.T) {
	// Initialize NodeAgent with metrics
	cfg := &Config{}
	agent := NewNodeAgent(cfg)

	// Test Case 1: MySQL Query
	// 16777343 is 127.0.0.1 in Little Endian (0x0100007f)
	queryEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeMysqlQuery,
		Daddr:   16777343,
		Dport:   3306,
		DataLen: 19,
	}
	copy(queryEvent.Data[:], "SELECT * FROM users")

	agent.processMysqlEvent(queryEvent)

	// Verify Metric
	m := &dto.Metric{}
	agent.mysqlQueriesTotal.WithLabelValues("127.0.0.1:3306", "SELECT").Write(m)
	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter 1, got %v", m.Counter.GetValue())
	}

	// Test Case 2: MySQL Response
	respEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeMysqlResponse,
		Daddr:   16777343,
		Dport:   3306,
		Latency: 10000000, // 10ms
	}

	agent.processMysqlEvent(respEvent)

	// Verify Histogram (checking count for simplicity)
	h := &dto.Metric{}
	agent.mysqlQueryLatency.WithLabelValues("127.0.0.1:3306").Write(h)
	if h.Histogram.GetSampleCount() != 1 {
		t.Errorf("Expected histogram count 1, got %v", h.Histogram.GetSampleCount())
	}
}
