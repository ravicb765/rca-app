package main

import (
	"encoding/binary"
	"testing"

	dto "github.com/prometheus/client_model/go"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

func TestProcessRabbitMQEvent(t *testing.T) {
	// Initialize NodeAgent with metrics
	cfg := &Config{}
	agent := NewNodeAgent(cfg)

	// Test Case 1: RabbitMQ Command (Basic.Publish)
	// 16777343 is 127.0.0.1 in Little Endian (0x0100007f)
	cmdEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeRabbitMQCommand,
		Daddr:   16777343,
		Dport:   5672,
		DataLen: 4,
	}
	// Class ID 60 (Basic), Method ID 40 (Publish)
	binary.BigEndian.PutUint16(cmdEvent.Data[0:2], 60)
	binary.BigEndian.PutUint16(cmdEvent.Data[2:4], 40)

	agent.processRabbitMQEvent(cmdEvent)

	// Verify Metric
	m := &dto.Metric{}
	agent.rabbitmqQueriesTotal.WithLabelValues("127.0.0.1:5672", "Basic.Publish").Write(m)
	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter 1 for Basic.Publish, got %v", m.Counter.GetValue())
	}

	// Test Case 2: RabbitMQ Response
	respEvent := &agentebpf.HttpEvent{
		Type:    agentebpf.EventTypeRabbitMQResponse,
		Daddr:   16777343,
		Dport:   5672,
		Latency: 2000000, // 2ms
	}

	agent.processRabbitMQEvent(respEvent)

	// Verify Histogram (checking count for simplicity)
	h := &dto.Metric{}
	agent.rabbitmqQueryLatency.WithLabelValues("127.0.0.1:5672").Write(h)
	if h.Histogram.GetSampleCount() != 1 {
		t.Errorf("Expected histogram count 1, got %v", h.Histogram.GetSampleCount())
	}
}
