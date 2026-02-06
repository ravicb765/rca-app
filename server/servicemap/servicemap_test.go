package servicemap

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type MockMetadataCache struct {
	mock.Mock
}

func (m *MockMetadataCache) LookupPod(ip string) *Instance {
	args := m.Called(ip)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*Instance)
}

// --- Tests ---

func TestClassifyService(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		port     uint16
		protocol string
		want     string
	}{
		{"Postgres Name", "my-postgres-db", 0, "", "postgres"},
		{"MySQL Port", "db-1", 3306, "tcp", "mysql"},
		{"Redis Port", "cache", 6379, "tcp", "redis"},
		{"HTTP Inference", "api-gateway", 80, "tcp", "http"},
		{"GRPC Proto", "svc", 9000, "grpc", "grpc"},
		{"Unknown", "random-svc", 12345, "udp", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyService(tt.input, tt.port, tt.protocol)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServiceMapBuilder_Update(t *testing.T) {
	// Setup Mock
	mockCache := new(MockMetadataCache)
	mockCache.On("LookupPod", "10.0.0.1").Return(&Instance{ID: "frontend", Name: "Frontend"})
	mockCache.On("LookupPod", "10.0.0.2").Return(&Instance{ID: "backend", Name: "Backend"})
	mockCache.On("LookupPod", "10.0.0.3").Return(&Instance{ID: "db", Name: "Database"})

	builder := NewServiceMapBuilder(mockCache)

	// table driven inputs
	epoches := []struct {
		name     string
		events   []TelemetryEvent
		validate func(*testing.T, *ServiceMap)
	}{
		{
			name: "Initial Connection",
			events: []TelemetryEvent{
				{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "http", RequestRate: 10, ErrorRate: 0.1},
			},
			validate: func(t *testing.T, sm *ServiceMap) {
				assert.Contains(t, sm.Applications, "frontend")
				assert.Contains(t, sm.Applications, "backend")
				assert.Len(t, sm.Connections, 1)
				assert.Equal(t, "frontend", sm.Connections[0].SourceApp)
				assert.Equal(t, "backend", sm.Connections[0].DestApp)
				assert.Equal(t, 10.0, sm.Connections[0].RequestRate)
			},
		},
		{
			name: "Update Stats (Moving Average)",
			events: []TelemetryEvent{
				{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "http", RequestRate: 20, ErrorRate: 0.0},
			},
			validate: func(t *testing.T, sm *ServiceMap) {
				// Alpha is 0.3. Previous(10) * 0.7 + Current(20) * 0.3 = 7 + 6 = 13
				assert.InDelta(t, 13.0, sm.Connections[0].RequestRate, 0.1)
			},
		},
		{
			name: "New Downstream Dependency",
			events: []TelemetryEvent{
				{SrcIP: "10.0.0.2", DstIP: "10.0.0.3", Protocol: "tcp", RequestRate: 50},
			},
			validate: func(t *testing.T, sm *ServiceMap) {
				assert.Contains(t, sm.Applications, "db")
				assert.True(t, sm.HasDependency("backend", "db"))
			},
		},
	}

	for _, tt := range epoches {
		t.Run(tt.name, func(t *testing.T) {
			builder.Update(tt.events)
			sm := builder.GetServiceMap()
			tt.validate(t, sm)
		})
	}
}

func TestServiceMap_DetectCycles(t *testing.T) {
	sm := NewServiceMap()
	
	// A -> B -> C -> A
	sm.Applications["A"] = &Application{ID: "A", Name: "A", Downstreams: []string{"B"}}
	sm.Applications["B"] = &Application{ID: "B", Name: "B", Downstreams: []string{"C"}}
	sm.Applications["C"] = &Application{ID: "C", Name: "C", Downstreams: []string{"A"}}
	
	// D -> E (No cycle)
	sm.Applications["D"] = &Application{ID: "D", Name: "D", Downstreams: []string{"E"}}
	sm.Applications["E"] = &Application{ID: "E", Name: "E", Downstreams: []string{}}

	cycles := sm.DetectCycles()
	
	assert.Len(t, cycles, 1, "Should detect exactly one cycle")
	if len(cycles) > 0 {
		// Cycle could be represented starting at any node, so we check length and membership
		assert.Len(t, cycles[0], 3)
		assert.Contains(t, cycles[0], "A")
		assert.Contains(t, cycles[0], "B")
		assert.Contains(t, cycles[0], "C")
	}
}

func TestServiceMapBuilder_ConcurrentUpdates(t *testing.T) {
	mockCache := new(MockMetadataCache)
	// Return nil or dummy for random IPs
	mockCache.On("LookupPod", mock.Anything).Return(&Instance{ID: "service-A"}) 
	
	builder := NewServiceMapBuilder(mockCache)
	
	var wg sync.WaitGroup
	workers := 10
	iterations := 100
	
	// concurrently update
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Writing
				builder.Update([]TelemetryEvent{
					{
						SrcIP: fmt.Sprintf("1.1.1.%d", id), 
						DstIP: "2.2.2.2", 
						RequestRate: float64(j),
					},
				})
				
				// Identify potential Race Conditions by reading simultaneously
				if j % 10 == 0 {
					_ = builder.GetServiceMap()
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	sm := builder.GetServiceMap()
	assert.NotNil(t, sm)
}

func TestServiceMapBuilder_Prune(t *testing.T) {
	mockCache := new(MockMetadataCache)
	mockCache.On("LookupPod", mock.Anything).Return(nil) // External IPs
	builder := NewServiceMapBuilder(mockCache)

	// Add connection
	builder.Update([]TelemetryEvent{{SrcIP: "1.2.3.4", DstIP: "5.6.7.8", RequestRate: 1}})
	
	time.Sleep(50 * time.Millisecond)
	
	// Prune with short TTL - logic says "remove if LastSeen < Now - TTL"
	// connection LastSeen is ~50ms ago.
	// If we prune with TTL = 10ms, Cutoff = Now - 10ms. 
	// LastSeen (Now-50ms) < Cutoff (Now-10ms). Should be removed.
	builder.Prune(10 * time.Millisecond)
	
	sm := builder.GetServiceMap()
	assert.Empty(t, sm.Connections)
}

// --- Benchmarks ---

func BenchmarkServiceMapBuilder_Update(b *testing.B) {
	mockCache := new(MockMetadataCache)
	mockCache.On("LookupPod", mock.Anything).Return(&Instance{ID: "bench-svc"})
	builder := NewServiceMapBuilder(mockCache)
	
	events := []TelemetryEvent{
		{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", RequestRate: 100},
		{SrcIP: "10.0.0.2", DstIP: "10.0.0.3", RequestRate: 100},
		{SrcIP: "10.0.0.3", DstIP: "10.0.0.1", RequestRate: 100},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Update(events)
	}
}

func BenchmarkDetectCycles(b *testing.B) {
	sm := NewServiceMap()
	// Create a large graph
	count := 100
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("svc-%d", i)
		next := fmt.Sprintf("svc-%d", (i+1)%count) // Big cycle
		sm.Applications[id] = &Application{ID: id, Name: id, Downstreams: []string{next}}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.DetectCycles()
	}
}
