package health

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Severity levels
type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
)

// InspectionResult represents the outcome of a check
type InspectionResult struct {
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Status      string    `json:"status"` // "pass", "fail"
	Severity    Severity  `json:"severity"`
	Description string    `json:"description"`
	Remediation string    `json:"remediation,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Inspection defines the interface for all health checks
type Inspection interface {
	Name() string
	Category() string
	Run() (*InspectionResult, error)
}

// Engine manages and runs inspections
type Engine struct {
	inspections []Inspection
	results     []InspectionResult
	mu          sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		inspections: []Inspection{},
		results:     []InspectionResult{},
	}
}

func (e *Engine) Register(i Inspection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inspections = append(e.inspections, i)
}

func (e *Engine) RunAll() []InspectionResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	var batch []InspectionResult
	for _, i := range e.inspections {
		res, err := i.Run()
		if err != nil {
			// Create a failure result if execution failed
			res = &InspectionResult{
				Name:        i.Name(),
				Category:    i.Category(),
				Status:      "fail",
				Severity:    SevWarning,
				Description: fmt.Sprintf("Inspection failed to run: %v", err),
				Timestamp:   time.Now(),
			}
		}
		// If it passed, we might still want to record it?
		// Usually we record failures or all. Let's record all for the plugin.
		batch = append(batch, *res)
	}
	e.results = batch // Simplified: overwrite last results. In prod could append history.
	return batch
}

func (e *Engine) GetResults() []InspectionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.results
}

// --- Built-in Inspections ---

type HighErrorRateInspection struct {
	Threshold float64
}

func (i *HighErrorRateInspection) Name() string { return "High Failure Rate" }
func (i *HighErrorRateInspection) Category() string { return "Reliability" }
func (i *HighErrorRateInspection) Run() (*InspectionResult, error) {
	// In a real app, this would query Prometheus/ServiceMap
	// Mocking a failure case for demonstration
	currentRate := 0.05 // 5%
	
	res := &InspectionResult{
		Name:      i.Name(),
		Category:  i.Category(),
		Timestamp: time.Now(),
	}

	if currentRate > i.Threshold {
		res.Status = "fail"
		res.Severity = SevCritical
		res.Description = fmt.Sprintf("Global error rate is %.2f%% (Threshold: %.2f%%)", currentRate*100, i.Threshold*100)
		res.Remediation = "1. Check server logs for 5xx errors.\n2. Verify upstream dependency health.\n3. Scale up replicas if CPU is saturated."
	} else {
		res.Status = "pass"
		res.Severity = SevInfo
		res.Description = "Error rate is within normal limits."
	}
	return res, nil
}

type LatencyInspection struct {
	ThresholdMs float64
}

func (i *LatencyInspection) Name() string { return "API Latency Check" }
func (i *LatencyInspection) Category() string { return "Performance" }
func (i *LatencyInspection) Run() (*InspectionResult, error) {
	currentP95 := 150.0 // ms
	
	res := &InspectionResult{
		Name:      i.Name(),
		Category:  i.Category(),
		Timestamp: time.Now(),
	}

	if currentP95 > i.ThresholdMs {
		res.Status = "fail"
		res.Severity = SevWarning
		res.Description = fmt.Sprintf("P95 Latency is %.0fms (Threshold: %.0fms)", currentP95, i.ThresholdMs)
		res.Remediation = "1. Profile database queries.\n2. Check for resource throttling."
	} else {
		res.Status = "pass"
		res.Severity = SevInfo
		res.Description = "Latency is optimal."
	}
	return res, nil
}
