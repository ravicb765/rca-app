package stream

import (
	"context"
	"log"
)

// Aggregator defines the interface for processing aggregated metrics
type Aggregator interface {
	Process(data map[string]float64)
}

// StreamProcessor handles consuming metrics from Kafka
type StreamProcessor struct {
	aggregator Aggregator
	brokers    []string
	topic      string
}

// NewStreamProcessor creates a new processor instance
func NewStreamProcessor(agg Aggregator, brokers []string, topic string) *StreamProcessor {
	return &StreamProcessor{
		aggregator: agg,
		brokers:    brokers,
		topic:      topic,
	}
}

// Start begins consuming messages
func (s *StreamProcessor) Start(ctx context.Context) {
	log.Printf("Starting stream processor for topic %s on brokers %v", s.topic, s.brokers)
	// Placeholder for Kafka consumer loop
	<-ctx.Done()
}

// Close cleans up resources
func (s *StreamProcessor) Close() {
	log.Println("Closing stream processor")
}
