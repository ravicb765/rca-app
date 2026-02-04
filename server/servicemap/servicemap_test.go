package servicemap

import "testing"

func TestBuildServiceMap(t *testing.T) {
	conns := []Connection{
		{SourceApp: "frontend", DestApp: "backend", Protocol: "http", RequestRate: 100},
		{SourceApp: "backend", DestApp: "postgres", Protocol: "tcp", RequestRate: 50},
		{SourceApp: "cron", DestApp: "backend", Protocol: "http", RequestRate: 1},
	}

	sm := BuildServiceMap(conns)

	if len(sm.Applications) != 4 {
		t.Fatalf("expected 4 applications, got %d", len(sm.Applications))
	}

	if !sm.HasDependency("frontend", "backend") {
		t.Fatalf("expected frontend -> backend dependency")
	}

	if !sm.HasDependency("backend", "postgres") {
		t.Fatalf("expected backend -> postgres dependency")
	}

	if sm.HasDependency("postgres", "backend") {
		t.Fatalf("did not expect postgres -> backend dependency")
	}
}

// MockMetadataCache for testing
type MockMetadataCache struct {
	pods map[string]*Instance
}

func (m *MockMetadataCache) LookupPod(ip string) *Instance {
	return m.pods[ip]
}

func TestServiceMapBuilder(t *testing.T) {
	// Setup mock metadata
	mockCache := &MockMetadataCache{
		pods: map[string]*Instance{
			"10.0.0.1": {ID: "frontend-svc", Name: "frontend"},
			"10.0.0.2": {ID: "backend-svc", Name: "backend"},
		},
	}

	builder := NewServiceMapBuilder(mockCache)

	// Simulate events
	events := []TelemetryEvent{
		{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "http", RequestRate: 10},
		{SrcIP: "10.0.0.2", DstIP: "1.1.1.1", Protocol: "tcp", RequestRate: 5}, // External dest
	}

	sm := builder.Update(events)

	// Verify applications
	if _, ok := sm.Applications["frontend-svc"]; !ok {
		t.Error("expected frontend-svc application")
	}
	if _, ok := sm.Applications["backend-svc"]; !ok {
		t.Error("expected backend-svc application")
	}

	// Verify dependency
	if !sm.HasDependency("frontend-svc", "backend-svc") {
		t.Error("expected frontend -> backend dependency")
	}
}

func TestDetectCycles(t *testing.T) {
	sm := NewServiceMap()

	// A -> B -> C -> A
	sm.Applications["A"] = &Application{ID: "A", Name: "A", Downstreams: []string{"B"}}
	sm.Applications["B"] = &Application{ID: "B", Name: "B", Downstreams: []string{"C"}}
	sm.Applications["C"] = &Application{ID: "C", Name: "C", Downstreams: []string{"A"}}

	cycles := sm.DetectCycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(cycles))
	}

	if len(cycles[0]) != 3 {
		t.Errorf("expected cycle length 3, got %d: %v", len(cycles[0]), cycles[0])
	}
}
