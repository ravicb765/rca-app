package inspections

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/ravicb765/rca-app/server/servicemap"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type InspectionResult struct {
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Severity    Severity  `json:"severity"`
	Status      string    `json:"status"` // "pass" or "fail"
	Description string    `json:"description"`
	Remediation string    `json:"remediation,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type Inspection struct {
	Name        string
	Category    string
	Rule        Rule
	Severity    Severity
	Remediation string
	Threshold   string `json:"threshold,omitempty"`
}

type AppMetrics struct {
	RequestRate       float64
	ErrorRate         float64
	Latency           float64
	MemoryUsage       float64 // in MB
	CPUUsage          float64 // percentage
	DiskUsage         float64 // percentage
	IOLoad            float64 // arbitrary unit
	ActiveConnections float64
	PacketLoss        float64 // percentage
	Http5xxRate       float64 // percentage (0.0-1.0)
	IOWait            float64 // percentage
	SwapUsage         float64 // percentage
	RestartCount      float64 // count
	CPUThrottling     float64 // percentage
	GoroutineCount    float64 // count
	OpenFDs           float64 // count
	ThreadCount       float64 // count
}

type Rule interface {
	Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string)
}

type InspectionEngine struct {
	inspections      []Inspection
	results          map[string][]InspectionResult
	history          map[string][]InspectionResult
	mu               sync.RWMutex
	HistoryRetention time.Duration
	inspectionStatus *prometheus.GaugeVec
}

func NewInspectionEngine(reg prometheus.Registerer) *InspectionEngine {
	engine := &InspectionEngine{
		results:          make(map[string][]InspectionResult),
		history:          make(map[string][]InspectionResult),
		HistoryRetention: 1 * time.Hour,
		inspectionStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rca_inspection_status",
				Help: "Status of inspections (0=pass, 1=fail)",
			},
			[]string{"application", "name", "category", "severity"},
		),
	}

	if reg != nil {
		reg.MustRegister(engine.inspectionStatus)
	}

	engine.inspections = []Inspection{
		{
			Name:        "High Error Rate",
			Category:    "Availability",
			Rule:        &ErrorRateRule{Threshold: 0.01}, // 1%
			Severity:    SeverityCritical,
			Remediation: "Check application logs for errors and upstream dependencies.",
			Threshold:   "1%",
		},
		{
			Name:        "High Latency",
			Category:    "Performance",
			Rule:        &LatencyRule{Threshold: 200}, // 200ms
			Severity:    SeverityWarning,
			Remediation: "Investigate slow database queries or resource contention.",
			Threshold:   "200ms",
		},
		{
			Name:        "Memory Leak Detection",
			Category:    "Resources",
			Rule:        &MemoryLeakRule{Threshold: 512}, // 512 MB
			Severity:    SeverityWarning,
			Remediation: "Check for memory leaks or increase container limits.",
			Threshold:   "512MB",
		},
		{
			Name:        "High CPU Usage",
			Category:    "Resources",
			Rule:        &CPUUsageRule{Threshold: 80}, // 80%
			Severity:    SeverityWarning,
			Remediation: "Optimize CPU intensive tasks or scale up.",
			Threshold:   "80%",
		},
		{
			Name:        "High Disk Usage",
			Category:    "Resources",
			Rule:        &DiskUsageRule{Threshold: 90}, // 90%
			Severity:    SeverityCritical,
			Remediation: "Clean up disk space or expand storage.",
			Threshold:   "90%",
		},
		{
			Name:        "High IO Load",
			Category:    "Performance",
			Rule:        &IOLoadRule{Threshold: 10}, // Arbitrary threshold
			Severity:    SeverityWarning,
			Remediation: "Investigate disk I/O bottlenecks.",
			Threshold:   "10",
		},
		{
			Name:        "Database Connection Pool Exhaustion",
			Category:    "Database",
			Rule:        &ConnectionPoolExhaustionRule{Threshold: 100}, // 100 active connections
			Severity:    SeverityCritical,
			Remediation: "Check database connection pool settings and scale if necessary.",
			Threshold:   "100 connections",
		},
		{
			Name:        "Network Packet Loss",
			Category:    "Network",
			Rule:        &PacketLossRule{Threshold: 1.0}, // 1%
			Severity:    SeverityWarning,
			Remediation: "Check network connectivity and switch/router logs.",
			Threshold:   "1%",
		},
		{
			Name:        "High HTTP 5xx Rate",
			Category:    "Availability",
			Rule:        &Http5xxRateRule{Threshold: 0.05}, // 5%
			Severity:    SeverityCritical,
			Remediation: "Check application logs for server-side exceptions.",
			Threshold:   "5%",
		},
		{
			Name:        "High Disk I/O Wait",
			Category:    "Performance",
			Rule:        &IOWaitRule{Threshold: 10.0}, // 10%
			Severity:    SeverityWarning,
			Remediation: "Check for slow disk operations or saturated storage bandwidth.",
			Threshold:   "10%",
		},
		{
			Name:        "High Memory Swap Usage",
			Category:    "Resources",
			Rule:        &SwapUsageRule{Threshold: 10.0}, // 10%
			Severity:    SeverityWarning,
			Remediation: "Check for memory pressure and increase RAM if needed.",
			Threshold:   "10%",
		},
		{
			Name:        "High Container Restarts",
			Category:    "Stability",
			Rule:        &RestartCountRule{Threshold: 3}, // > 3 restarts
			Severity:    SeverityCritical,
			Remediation: "Investigate crash loops, OOM kills, or liveness probe failures.",
			Threshold:   "3",
		},
		{
			Name:        "High CPU Throttling",
			Category:    "Performance",
			Rule:        &CPUThrottlingRule{Threshold: 5.0}, // 5% throttling
			Severity:    SeverityWarning,
			Remediation: "Increase CPU limits or optimize application performance.",
			Threshold:   "5%",
		},
		{
			Name:        "High Goroutine Count",
			Category:    "Resources",
			Rule:        &GoroutineCountRule{Threshold: 10000}, // 10k goroutines
			Severity:    SeverityWarning,
			Remediation: "Check for goroutine leaks or excessive concurrency.",
			Threshold:   "10000",
		},
		{
			Name:        "High Open File Descriptors",
			Category:    "Resources",
			Rule:        &OpenFDCountRule{Threshold: 1000}, // 1000 FDs
			Severity:    SeverityWarning,
			Remediation: "Check for file descriptor leaks (sockets, files) or increase ulimit.",
			Threshold:   "1000",
		},
		{
			Name:        "High Thread Count",
			Category:    "Resources",
			Rule:        &ThreadCountRule{Threshold: 500}, // 500 threads
			Severity:    SeverityWarning,
			Remediation: "Check for thread leaks or excessive concurrency.",
			Threshold:   "500",
		},
	}
	return engine
}

func (e *InspectionEngine) Run(sm *servicemap.ServiceMap) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Aggregate metrics per app from connections (incoming traffic)
	appMetrics := make(map[string]AppMetrics)
	type tempStats struct {
		totalReq      float64
		totalErr      float64
		totalLat      float64
		maxMem        float64
		maxCPU        float64
		maxDisk       float64
		maxIO         float64
		totalConns    float64
		totalPktLoss  float64
		total5xx      float64
		maxIOWait     float64
		maxSwap       float64
		maxRestart    float64
		maxThrottling float64
		maxGoroutines float64
		maxOpenFDs    float64
		maxThreads    float64
	}
	stats := make(map[string]*tempStats)

	for _, conn := range sm.Connections {
		if _, ok := stats[conn.DestApp]; !ok {
			stats[conn.DestApp] = &tempStats{}
		}
		s := stats[conn.DestApp]
		s.totalReq += conn.RequestRate
		s.totalErr += conn.ErrorRate * conn.RequestRate
		s.totalLat += conn.Latency * conn.RequestRate
		if conn.MemoryUsage > s.maxMem {
			s.maxMem = conn.MemoryUsage
		}
		if conn.CPUUsage > s.maxCPU {
			s.maxCPU = conn.CPUUsage
		}
		if conn.DiskUsage > s.maxDisk {
			s.maxDisk = conn.DiskUsage
		}
		if conn.IOLoad > s.maxIO {
			s.maxIO = conn.IOLoad
		}
		s.totalConns += conn.ActiveConnections
		s.totalPktLoss += conn.PacketLoss * conn.RequestRate
		s.total5xx += conn.Http5xxRate * conn.RequestRate
		if conn.IOWait > s.maxIOWait {
			s.maxIOWait = conn.IOWait
		}
		if conn.SwapUsage > s.maxSwap {
			s.maxSwap = conn.SwapUsage
		}
		if conn.RestartCount > s.maxRestart {
			s.maxRestart = conn.RestartCount
		}
		if conn.CPUThrottling > s.maxThrottling {
			s.maxThrottling = conn.CPUThrottling
		}
		if conn.GoroutineCount > s.maxGoroutines {
			s.maxGoroutines = conn.GoroutineCount
		}
		if conn.OpenFDs > s.maxOpenFDs {
			s.maxOpenFDs = conn.OpenFDs
		}
		if conn.ThreadCount > s.maxThreads {
			s.maxThreads = conn.ThreadCount
		}
	}

	for id, s := range stats {
		if s.totalReq > 0 {
			appMetrics[id] = AppMetrics{
				RequestRate:       s.totalReq,
				ErrorRate:         s.totalErr / s.totalReq,
				Latency:           s.totalLat / s.totalReq,
				MemoryUsage:       s.maxMem,
				CPUUsage:          s.maxCPU,
				DiskUsage:         s.maxDisk,
				IOLoad:            s.maxIO,
				ActiveConnections: s.totalConns,
				PacketLoss:        s.totalPktLoss / s.totalReq,
				Http5xxRate:       s.total5xx / s.totalReq,
				IOWait:            s.maxIOWait,
				SwapUsage:         s.maxSwap,
				RestartCount:      s.maxRestart,
				CPUThrottling:     s.maxThrottling,
				GoroutineCount:    s.maxGoroutines,
				OpenFDs:           s.maxOpenFDs,
				ThreadCount:       s.maxThreads,
			}
		}
	}

	for id, app := range sm.Applications {
		metrics := appMetrics[id]
		var results []InspectionResult

		for _, insp := range e.inspections {
			pass, desc := insp.Rule.Evaluate(app, metrics)
			status := "pass"
			if !pass {
				status = "fail"
			}

			val := 0.0
			if !pass {
				val = 1.0
			}
			// Update Prometheus metric
			e.inspectionStatus.WithLabelValues(id, insp.Name, insp.Category, string(insp.Severity)).Set(val)

			res := InspectionResult{
				Name:        insp.Name,
				Category:    insp.Category,
				Severity:    insp.Severity,
				Status:      status,
				Description: desc,
				Timestamp:   time.Now(),
			}
			if !pass {
				res.Remediation = insp.Remediation
			}
			results = append(results, res)
		}
		e.results[id] = results
		e.history[id] = append(e.history[id], results...)
	}

	// Prune history
	cutoff := time.Now().Add(-e.HistoryRetention)
	for id, history := range e.history {
		var kept []InspectionResult
		for _, h := range history {
			if h.Timestamp.After(cutoff) {
				kept = append(kept, h)
			}
		}
		if len(kept) == 0 {
			delete(e.history, id)
		} else {
			e.history[id] = kept
		}
	}
}

func (e *InspectionEngine) GetResults(appID string) []InspectionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.results[appID]
}

func (e *InspectionEngine) GetHistory(appID string) []InspectionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.history[appID]
}

func (e *InspectionEngine) GetRules() []Inspection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inspections
}
