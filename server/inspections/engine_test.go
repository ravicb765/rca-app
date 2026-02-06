package inspections

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

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
		if r.Name == "High Latency" && r.Status == "fail" {
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

func TestOpenFDCountRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "leaky-fds"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Leaky FDs"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 10,
		OpenFDs:     1500, // > 1000 threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Open File Descriptors" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High Open File Descriptors to fail")
}

func TestThreadCountRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "leaky-threads"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Leaky Threads"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 10,
		ThreadCount: 600, // > 500 threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Thread Count" && r.Status == "fail" {
			return
		}
	}
	t.Error("expected High Thread Count to fail")
}

func TestRabbitMQQueueLengthRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "rabbitmq-congested"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "RabbitMQ Congested", Type: "rabbitmq"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "producer",
		DestApp:     appID,
		Protocol:    "rabbitmq",
		QueueLength: 1500, // > 1000 threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High RabbitMQ Queue Length" && r.Status == "fail" {
			return // Test passed
		}
	}
	t.Error("expected High RabbitMQ Queue Length to fail")
}

func TestCassandraLatencyRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "cassandra-slow"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Cassandra Slow", Type: "cassandra"}

	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		Protocol:    "cassandra",
		RequestRate: 100,
		Latency:     60, // > 50ms threshold
	})

	engine.Run(sm)
	results := engine.GetResults(appID)

	for _, r := range results {
		if r.Name == "High Cassandra Latency" && r.Status == "fail" {
			return // Test passed
		}
	}
	t.Error("expected High Cassandra Latency to fail")
}

func TestPostgresLatencyRule(t *testing.T) {
	protocols := []string{"postgres", "mysql", "mariadb", "cockroachdb", "yugabytedb"}

	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			engine := NewInspectionEngine(nil)
			sm := servicemap.NewServiceMap()
			appID := "db-slow-" + proto
			sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "DB Slow", Type: proto}

			sm.Connections = append(sm.Connections, servicemap.Connection{
				SourceApp:   "client",
				DestApp:     appID,
				Protocol:    proto,
				RequestRate: 100,
				Latency:     60, // > 50ms threshold
			})

			engine.Run(sm)
			results := engine.GetResults(appID)

			for _, r := range results {
				if r.Name == "High Postgres-Compatible DB Latency" && r.Status == "fail" {
					return // Test passed
				}
			}
			t.Errorf("expected High Postgres-Compatible DB Latency to fail for protocol %s", proto)
		})
	}
}

func TestMemoryLeakTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	// Add trend rule manually
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Trend Leak",
		Category: "Resources",
		Rule:     &MemoryLeakTrendRule{Threshold: 10},
		Severity: SeverityWarning,
	})

	appID := "trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Trend Service"}

	// Simulate 5 runs with increasing memory
	for i := 0; i < 5; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 10,
			MemoryUsage: 100 + float64(i)*5, // 100, 105, 110, 115, 120 (Total increase 20 > 10)
		}}
		engine.Run(sm)
	}

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Trend Leak" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Memory Leak Trend to fail")
	}
}

func TestLoadInspectionsFromFile(t *testing.T) {
	content := `
inspections:
  - name: "Custom YAML Rule"
    category: "Custom"
    severity: "warning"
    rule_type: "latency"
    threshold: 500
    remediation: "Fix it"
`
	tmpfile, _ := os.CreateTemp("", "rules.yaml")
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString(content)
	tmpfile.Close()

	engine := NewInspectionEngine(nil)
	if err := engine.LoadInspectionsFromFile(tmpfile.Name()); err != nil {
		t.Fatalf("failed to load yaml: %v", err)
	}

	rules := engine.GetRules()
	found := false
	for _, r := range rules {
		if r.Name == "Custom YAML Rule" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom rule from YAML to be loaded")
	}
}

func TestExportResults(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "test-export"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Test Export"}

	// Trigger a failure so we have something to export
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		ErrorRate:   0.05, // 5% > 1% threshold
	})

	engine.Run(sm)

	tmpfile, err := os.CreateTemp("", "results.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	if err := engine.ExportResults(tmpfile.Name()); err != nil {
		t.Fatalf("ExportResults failed: %v", err)
	}

	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if len(content) == 0 {
		t.Error("Exported file is empty")
	}

	if !strings.Contains(string(content), "High Error Rate") {
		t.Error("Exported JSON missing expected rule name")
	}
}

func TestErrorRateSpikeRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	// Add spike rule manually to ensure it's there and configured as expected for test
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Spike Test",
		Category: "Availability",
		Rule:     &ErrorRateSpikeRule{Multiplier: 2.0, MinRate: 0.01},
		Severity: SeverityCritical,
	})

	appID := "spike-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Spike Service"}

	// 1. Stable low error rate (0.5%)
	for i := 0; i < 5; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 100,
			ErrorRate:   0.005,
		}}
		engine.Run(sm)
	}

	// 2. Sudden spike (2.0%) -> 4x increase, should trigger
	sm.Connections = []servicemap.Connection{{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		ErrorRate:   0.02,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Spike Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Error Rate Spike to fail")
	}
}

func TestLatencyDegradationRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	// Add specific rule for testing to ensure configuration matches expectations
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Latency Degradation Test",
		Category: "Performance",
		Rule:     &LatencyDegradationRule{Threshold: 0.5, MinLatency: 10},
		Severity: SeverityWarning,
	})

	appID := "latency-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Latency Service"}

	// 1. Establish baseline (10 runs with 20ms latency)
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 100,
			Latency:     20,
		}}
		engine.Run(sm)
	}

	// 2. Trigger degradation (40ms > 20ms * 1.5)
	sm.Connections = []servicemap.Connection{{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		Latency:     40,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Latency Degradation Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Latency Degradation Test to fail")
	}
}

func TestCPUThrottlingTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Throttling Trend Test",
		Category: "Performance",
		Rule:     &CPUThrottlingTrendRule{Threshold: 0.5, MinThrottling: 5.0},
		Severity: SeverityWarning,
	})

	appID := "throttling-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Throttling Service"}

	// Baseline: 10% throttling
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:     "client",
			DestApp:       appID,
			RequestRate:   100,
			CPUThrottling: 10.0,
		}}
		engine.Run(sm)
	}

	// Spike to 20% (100% increase > 50% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:     "client",
		DestApp:       appID,
		RequestRate:   100,
		CPUThrottling: 20.0,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Throttling Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Throttling Trend Test to fail")
	}
}

func TestCPUUsageTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "CPU Trend Test",
		Category: "Resources",
		Rule:     &CPUUsageTrendRule{Threshold: 0.3, MinUsage: 10.0},
		Severity: SeverityWarning,
	})

	appID := "cpu-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "CPU Trend Service"}

	// Baseline: 40% usage
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 100,
			CPUUsage:    40.0,
		}}
		engine.Run(sm)
	}

	// Spike to 60% (50% increase > 30% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		CPUUsage:    60.0,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "CPU Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CPU Trend Test to fail")
	}
}

func TestDatabasePersistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	engine := NewInspectionEngine(nil)
	// Reduce inspections to a single predictable one for testing
	engine.inspections = []Inspection{
		{
			Name:     "Test Rule",
			Category: "TestCat",
			Rule:     &ErrorRateRule{Threshold: 0.01},
			Severity: SeverityCritical,
		},
	}

	// Expect table creation
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS inspection_results").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := engine.SetDatabase(db); err != nil {
		t.Fatalf("SetDatabase failed: %v", err)
	}

	appID := "db-test"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "DB Test"}
	// Trigger failure
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		ErrorRate:   0.05,
	})

	// Expect Insert
	mock.ExpectExec("INSERT INTO inspection_results").
		WithArgs(appID, "Test Rule", "TestCat", "critical", "fail", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect Pruning
	mock.ExpectExec("DELETE FROM inspection_results").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	engine.Run(sm)

	// Test Aggregation
	rows := sqlmock.NewRows([]string{"category", "count"}).
		AddRow("TestCat", 1)

	mock.ExpectQuery("SELECT category, COUNT\\(\\*\\) FROM inspection_results").
		WithArgs(appID).
		WillReturnRows(rows)

	stats, err := engine.GetAggregatedResults(appID)
	if err != nil {
		t.Fatalf("GetAggregatedResults failed: %v", err)
	}

	if stats["TestCat"] != 1 {
		t.Errorf("expected 1 TestCat, got %d", stats["TestCat"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestPruneDatabaseHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	engine := NewInspectionEngine(nil)
	if err := engine.SetDatabase(db); err != nil {
		t.Fatalf("SetDatabase failed: %v", err)
	}

	// Expect Pruning
	mock.ExpectExec("DELETE FROM inspection_results").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 10)) // 10 rows deleted

	if err := engine.PruneDatabaseHistory(); err != nil {
		t.Fatalf("PruneDatabaseHistory failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSwapUsageTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Swap Usage Trend Test",
		Category: "Resources",
		Rule:     &SwapUsageTrendRule{Threshold: 0.2, MinSwap: 5.0},
		Severity: SeverityWarning,
	})

	appID := "swap-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Swap Trend Service"}

	// Baseline: 10% Swap
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 100,
			SwapUsage:   10.0,
		}}
		engine.Run(sm)
	}

	// Spike to 15% (50% increase > 20% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		SwapUsage:   15.0,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Swap Usage Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Swap Usage Trend Test to fail")
	}
}

func TestGenerateMarkdownReport(t *testing.T) {
	engine := NewInspectionEngine(nil)
	sm := servicemap.NewServiceMap()
	appID := "report-test"
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Report Test"}

	// Trigger a failure
	sm.Connections = append(sm.Connections, servicemap.Connection{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		ErrorRate:   0.05,
	})

	engine.Run(sm)

	tmpfile, err := os.CreateTemp("", "report.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	if err := engine.GenerateMarkdownReport(tmpfile.Name()); err != nil {
		t.Fatalf("GenerateMarkdownReport failed: %v", err)
	}

	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "# Inspection Report") {
		t.Error("Markdown report missing header")
	}
	if !strings.Contains(string(content), "High Error Rate") {
		t.Error("Markdown report missing expected rule name")
	}
	if !strings.Contains(string(content), "❌") {
		t.Error("Markdown report missing failure icon")
	}
}

func TestContainerRestartTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Restart Trend Test",
		Category: "Stability",
		Rule:     &ContainerRestartTrendRule{Threshold: 1.0, MinRestarts: 1.0},
		Severity: SeverityCritical,
	})

	appID := "restart-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Restart Trend Service"}

	// Baseline: 0 restarts
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:    "client",
			DestApp:      appID,
			RequestRate:  100,
			RestartCount: 0,
		}}
		engine.Run(sm)
	}

	// Spike to 2 restarts
	sm.Connections = []servicemap.Connection{{
		SourceApp:    "client",
		DestApp:      appID,
		RequestRate:  100,
		RestartCount: 2,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Restart Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Restart Trend Test to fail")
	}
}

func TestEmailReport(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "report.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString("Report content")
	tmpfile.Close()

	engine := NewInspectionEngine(nil)
	// Use invalid host to trigger network error, verifying method execution
	err = engine.EmailReport("invalid-host", "25", "from@example.com", "to@example.com", "pass", "Subject", tmpfile.Name())

	// We expect an error, but specifically a network error or lookup error, not a file error.
	// If err is nil, it means it somehow succeeded (unlikely) or failed silently.
	if err == nil {
		t.Error("expected error due to invalid smtp host")
	}
}

func TestGoroutineCountTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Goroutine Trend Test",
		Category: "Resources",
		Rule:     &GoroutineCountTrendRule{Threshold: 0.2, MinGoroutines: 100},
		Severity: SeverityWarning,
	})

	appID := "goroutine-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Goroutine Trend Service"}

	// Baseline: 100 goroutines
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:      "client",
			DestApp:        appID,
			RequestRate:    100,
			GoroutineCount: 100,
		}}
		engine.Run(sm)
	}

	// Spike to 130 (30% increase > 20% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:      "client",
		DestApp:        appID,
		RequestRate:    100,
		GoroutineCount: 130,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Goroutine Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Goroutine Trend Test to fail")
	}
}

func TestScheduleEmailReports(t *testing.T) {
	engine := NewInspectionEngine(nil)
	tmpfile, err := os.CreateTemp("", "report_schedule.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Schedule with a very short interval
	stop := engine.ScheduleEmailReports(10*time.Millisecond, "invalid-host", "25", "from", "to", "pass", "Subject", tmpfile.Name())
	defer close(stop)

	// Wait for at least one tick
	time.Sleep(50 * time.Millisecond)

	if _, err := os.Stat(tmpfile.Name()); err != nil {
		t.Errorf("Report file check failed: %v", err)
	}
}

func TestOpenFDCountTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Open FD Trend Test",
		Category: "Resources",
		Rule:     &OpenFDCountTrendRule{Threshold: 0.2, MinFDs: 100},
		Severity: SeverityWarning,
	})

	appID := "fd-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "FD Trend Service"}

	// Baseline: 100 FDs
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 100,
			OpenFDs:     100,
		}}
		engine.Run(sm)
	}

	// Spike to 130 (30% increase > 20% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		OpenFDs:     130,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Open FD Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Open FD Trend Test to fail")
	}
}

func TestSendSlackNotification(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload["text"] != "Test Message" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := &SlackAlertProvider{WebhookURL: ts.URL}
	if err := provider.Send("Test Subject", "Test Message"); err != nil {
		t.Fatalf("SendSlackNotification failed: %v", err)
	}
}

func TestThreadCountTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Thread Trend Test",
		Category: "Resources",
		Rule:     &ThreadCountTrendRule{Threshold: 0.2, MinThreads: 50},
		Severity: SeverityWarning,
	})

	appID := "thread-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Thread Trend Service"}

	// Baseline: 50 threads
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:   "client",
			DestApp:     appID,
			RequestRate: 100,
			ThreadCount: 50,
		}}
		engine.Run(sm)
	}

	// Spike to 70 (40% increase > 20% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:   "client",
		DestApp:     appID,
		RequestRate: 100,
		ThreadCount: 70,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Thread Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Thread Trend Test to fail")
	}
}

func TestConnectionPoolTrendRule(t *testing.T) {
	engine := NewInspectionEngine(nil)
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Connection Pool Trend Test",
		Category: "Database",
		Rule:     &ConnectionPoolTrendRule{Threshold: 0.2, MinConnections: 10},
		Severity: SeverityWarning,
	})

	appID := "conn-pool-trend-service"
	sm := servicemap.NewServiceMap()
	sm.Applications[appID] = &servicemap.Application{ID: appID, Name: "Conn Pool Trend Service"}

	// Baseline: 10 connections
	for i := 0; i < 10; i++ {
		sm.Connections = []servicemap.Connection{{
			SourceApp:         "client",
			DestApp:           appID,
			RequestRate:       100,
			ActiveConnections: 10,
		}}
		engine.Run(sm)
	}

	// Spike to 15 (50% increase > 20% threshold)
	sm.Connections = []servicemap.Connection{{
		SourceApp:         "client",
		DestApp:           appID,
		RequestRate:       100,
		ActiveConnections: 15,
	}}
	engine.Run(sm)

	results := engine.GetResults(appID)
	found := false
	for _, r := range results {
		if r.Name == "Connection Pool Trend Test" && r.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Connection Pool Trend Test to fail")
	}
}

func TestScheduleNotifications(t *testing.T) {
	// Mock Slack/Teams server
	received := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		if strings.Contains(payload["text"], "Inspection Alert Summary") {
			received <- true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	engine := NewInspectionEngine(nil)
	// Add a failing rule to generate an alert
	engine.inspections = append(engine.inspections, Inspection{
		Name:     "Always Fail",
		Category: "Test",
		Rule:     &ErrorRateRule{Threshold: -1.0}, // Always fails
		Severity: SeverityCritical,
	})

	sm := servicemap.NewServiceMap()
	sm.Applications["test-app"] = &servicemap.Application{ID: "test-app", Name: "Test App"}
	sm.Connections = []servicemap.Connection{{SourceApp: "client", DestApp: "test-app", ErrorRate: 0.0}}
	engine.Run(sm)

	stop := engine.ScheduleSlackNotifications(10*time.Millisecond, ts.URL)
	defer close(stop)

	select {
	case <-received:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Timed out waiting for notification")
	}
}
