package stream

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event represents a generic data point in the stream
type Event struct {
	ID        string
	Timestamp time.Time
	Type      string
	Payload   []byte
	TenantID  string
}

// Processor defines the interface for stream processing components
type Processor interface {
	Process(ctx context.Context, input <-chan Event) (<-chan Event, error)
}

// Pipeline manages the flow of events through processors
type Pipeline struct {
	processors []Processor
}

func NewPipeline(processors ...Processor) *Pipeline {
	return &Pipeline{processors: processors}
}

func (p *Pipeline) Run(ctx context.Context, source <-chan Event) <-chan Event {
	current := source
	var err error
	for _, proc := range p.processors {
		current, err = proc.Process(ctx, current)
		if err != nil {
			// In production, handle error gracefully (dead letter queue, etc.)
			fmt.Printf("Pipeline error: %v\n", err)
			return nil
		}
	}
	return current
}

// MetricAggregator is a concrete processor for rolling up metrics
type MetricAggregator struct {
	WindowSize time.Duration
	mu         sync.Mutex
	state      map[string]float64
}

func (ma *MetricAggregator) Process(ctx context.Context, input <-chan Event) (<-chan Event, error) {
	output := make(chan Event)
	go func() {
		defer close(output)
		ticker := time.NewTicker(ma.WindowSize)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-input:
				if !ok {
					return
				}
				// Mock logic: Aggregate if type is 'metric'
				if evt.Type == "metric" {
					ma.mu.Lock()
					ma.state[evt.TenantID] += 1.0 // Simple count
					ma.mu.Unlock()
				}
				// Pass-through
				output <- evt
			case <-ticker.C:
				// Emit aggregation
				ma.mu.Lock()
				for tenant, count := range ma.state {
					output <- Event{
						Type:     "aggregation",
						Timestamp: time.Now(),
						Payload:   []byte(fmt.Sprintf("count=%f", count)),
						TenantID:  tenant,
					}
					delete(ma.state, tenant) // Reset window
				}
				ma.mu.Unlock()
			}
		}
	}()
	return output, nil
}
