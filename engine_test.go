package inspections

import (
	"testing"
	"time"

	"github.com/ravicb765/rca-app/server/servicemap"
)

func TestInspectionEngine(t *testing.T) {
	engine := NewInspectionEngine(nil)

	// Setup a service map with a high error rate app
	sm := servicemap.NewServiceMap()
	appID := "bad-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Bad Service"}

	// Add connections with errors
	// RequestRate: 100, ErrorRate: 0.05 (5%) -> Should trigger High Error Rate (threshold 1%)
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		ErrorRate:   0.05,
		Latency:     10,
	})

	engine.Run(sm)

	results := engine.GetResults(appID)
	if len(results) == 0 {
		t.Fatalf("expected inspection results for %s", appID)
	}

	foundErrorRate := false
	for _, r := range results {
		if r.Name == "High Error Rate" {
			foundErrorRate = true
			if r.Status != "fail" {
				t.Errorf("expected High Error Rate to fail, got %s", r.Status)
			}
		}
	}

	if !foundErrorRate {
		t.Error("expected High Error Rate inspection result")
	}
}

func TestMemoryLeakRule(t *testing.T) {
	engine := NewInspectionEngine(nil)

	sm := servicemap.NewServiceMap()
	appID := "leaky-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Leaky Service"}

	// Add connection with high memory usage (600MB > 512MB threshold)
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 10,
		MemoryUsage: 600,
	})

	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Memory Leak Detection" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Memory Leak Detection to fail")
	}
}

func TestResourceRules(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "resource-hog"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Resource Hog"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 10,
		CPUUsage:    90, // > 80
		DiskUsage:   95, // > 90
		IOLoad:      15, // > 10
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	failures := 0
	for _, r := range results {
		if r.Status == "fail" {
			failures++
		}
	}

	if failures < 3 {
		t.Errorf("expected at least 3 resource failures, got %d", failures)
	}
}

func TestConnectionPoolRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "db-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "DB Service"}

	// Add connections that sum up to > 100
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:         "client-1",
		DestApp:           appID,
		ActiveConnections: 60,
	})
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:         "client-2",
		DestApp:           appID,
		ActiveConnections: 50,
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	if len(results) == 0 || results[len(results)-1].Status != "fail" {
		t.Error("expected Database Connection Pool Exhaustion to fail")
	}
}

func TestPacketLossRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "lossy-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Lossy Service"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		PacketLoss:  5.0, // 5%
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "Network Packet Loss" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected Network Packet Loss to fail")
}

func TestHttp5xxRateRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "error-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Error Service"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		Http5xxRate: 0.10, // 10% > 5% threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High HTTP 5xx Rate" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High HTTP 5xx Rate to fail")
}

func TestHistoryPruning(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.HistoryRetention = 1 * time.Hour

	appID := "prune-test"
	// Manually inject history
	// 1. Old result (should be pruned)
	oldRes := InspectionResult{
		Name:      "Old",
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	// 2. New result (should be kept)
	newRes := InspectionResult{
		Name:      "New",
		Timestamp: time.Now(),
	}

	engine.history[appID] = []InspectionResult{oldRes, newRes}

	// Run engine with empty map to trigger pruning logic without adding new results
	engine.Run(servicemap.NewServiceMap())

	hist := engine.GetHistory(appID)
	if len(hist) != 1 {
		t.Errorf("expected 1 history item, got %d", len(hist))
	}
	if len(hist) > 0 && hist[0].Name != "New" {
		t.Errorf("expected 'New' result, got %s", hist[0].Name)
	}
}

func TestSlowHTTPResponseRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "slow-http-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Slow HTTP Service", Type: "http"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		Latency:     600, // > 500ms
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "Slow HTTP Response" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected Slow HTTP Response to fail")
}

func TestIOWaitRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "io-wait-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "IO Wait Service"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		IOWait:      25.0, // > 10%
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Disk I/O Wait" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High Disk I/O Wait to fail")
}

func TestSwapUsageRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "swap-hog"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Swap Hog"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 10,
		SwapUsage:   15.0, // > 10%
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Memory Swap Usage" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High Memory Swap Usage to fail")
}

func TestRestartCountRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "crashing-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Crashing Service"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:    "client",
		DestApp:      appID,
		RequestRate:  10,
		RestartCount: 5, // > 3 threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Container Restarts" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High Container Restarts to fail")
}

func TestCPUThrottlingRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "throttled-service"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Throttled Service"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:     "client",
		DestApp:       appID,
		RequestRate:   10,
		CPUThrottling: 15.0, // > 5% threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High CPU Throttling" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High CPU Throttling to fail")
}

func TestGoroutineCountRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "leaky-goroutines"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Leaky Goroutines"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:      "client",
		DestApp:        appID,
		RequestRate:    10,
		GoroutineCount: 15000, // > 10000 threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Goroutine Count" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High Goroutine Count to fail")
}
