package slo

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// SLOType represents the type of SLO
type SLOType string

const (
	SLOTypeAvailability SLOType = "availability"
	SLOTypeLatency      SLOType = "latency"
)

// SLOIndicator defines the Prometheus queries for calculating SLI
type SLOIndicator struct {
	Success string `json:"success"` // Query for successful events
	Total   string `json:"total"`   // Query for total events
}

// SLO represents a Service Level Objective
type SLO struct {
	Name        string       `json:"name"`
	Application string       `json:"application,omitempty"`
	Type        SLOType      `json:"type"`
	Target      float64      `json:"target"`       // Target percentage (e.g., 99.9)
	Window      string       `json:"window"`       // Time window (e.g., "30d", "7d")
	Indicator   SLOIndicator `json:"indicator"`
	Description string       `json:"description,omitempty"`
}

// SLOStatus represents the current status of an SLO
type SLOStatus struct {
	Name               string    `json:"name"`
	ActualSLI          float64   `json:"actual_sli"`           // Actual SLI percentage
	Target             float64   `json:"target"`               // Target percentage
	IsViolated         bool      `json:"is_violated"`
	ErrorBudget        float64   `json:"error_budget"`         // Remaining error budget (0-100)
	ErrorBudgetPercent float64   `json:"error_budget_percent"` // Percentage of error budget remaining
	LastChecked        time.Time `json:"last_checked"`
	Message            string    `json:"message,omitempty"`
}

// MetricsClient is an interface for querying metrics (allows mocking in tests)
type MetricsClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error)
}

// SLOTracker manages and monitors SLOs
type SLOTracker struct {
	slos         map[string]*SLO
	statusCache  map[string]*SLOStatus
	metricsAPI   MetricsClient
	mu           sync.RWMutex
	cacheTTL     time.Duration
}

// NewSLOTracker creates a new SLO tracker
func NewSLOTracker(metricsAPI MetricsClient) *SLOTracker {
	return &SLOTracker{
		slos:        make(map[string]*SLO),
		statusCache: make(map[string]*SLOStatus),
		metricsAPI:  metricsAPI,
		cacheTTL:    30 * time.Second,
	}
}

// AddSLO registers a new SLO
func (t *SLOTracker) AddSLO(slo *SLO) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if slo.Name == "" {
		return fmt.Errorf("SLO name cannot be empty")
	}
	if slo.Target <= 0 || slo.Target > 100 {
		return fmt.Errorf("SLO target must be between 0 and 100")
	}

	t.slos[slo.Name] = slo
	return nil
}

// RemoveSLO removes an SLO
func (t *SLOTracker) RemoveSLO(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.slos[name]; !exists {
		return fmt.Errorf("SLO %s not found", name)
	}

	delete(t.slos, name)
	delete(t.statusCache, name)
	return nil
}

// GetSLO retrieves an SLO by name
func (t *SLOTracker) GetSLO(name string) (*SLO, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	slo, exists := t.slos[name]
	if !exists {
		return nil, fmt.Errorf("SLO %s not found", name)
	}

	return slo, nil
}

// ListSLOs returns all registered SLOs
func (t *SLOTracker) ListSLOs() []*SLO {
	t.mu.RLock()
	defer t.mu.RUnlock()

	slos := make([]*SLO, 0, len(t.slos))
	for _, slo := range t.slos {
		slos = append(slos, slo)
	}
	return slos
}

// CheckSLO checks a single SLO and returns its status
func (t *SLOTracker) CheckSLO(ctx context.Context, slo *SLO) (*SLOStatus, error) {
	// Check cache first
	t.mu.RLock()
	if cached, exists := t.statusCache[slo.Name]; exists {
		if time.Since(cached.LastChecked) < t.cacheTTL {
			t.mu.RUnlock()
			return cached, nil
		}
	}
	t.mu.RUnlock()

	// Query Prometheus for SLI
	actualSLI, err := t.calculateSLI(ctx, slo)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate SLI for %s: %w", slo.Name, err)
	}

	// Calculate error budget
	// Error budget = (1 - target) * 100
	// Remaining error budget = (actualSLI - target) / (1 - target/100) * 100
	errorBudgetTotal := 100 - slo.Target
	errorBudgetUsed := slo.Target - actualSLI
	errorBudgetRemaining := errorBudgetTotal - errorBudgetUsed
	errorBudgetPercent := 0.0
	if errorBudgetTotal > 0 {
		errorBudgetPercent = (errorBudgetRemaining / errorBudgetTotal) * 100
	}

	status := &SLOStatus{
		Name:               slo.Name,
		ActualSLI:          actualSLI,
		Target:             slo.Target,
		IsViolated:         actualSLI < slo.Target,
		ErrorBudget:        errorBudgetRemaining,
		ErrorBudgetPercent: errorBudgetPercent,
		LastChecked:        time.Now(),
	}

	if status.IsViolated {
		status.Message = fmt.Sprintf("SLO violated: %.2f%% < %.2f%%", actualSLI, slo.Target)
	} else {
		status.Message = fmt.Sprintf("SLO met: %.2f%% >= %.2f%%", actualSLI, slo.Target)
	}

	// Update cache
	t.mu.Lock()
	t.statusCache[slo.Name] = status
	t.mu.Unlock()

	return status, nil
}

// CheckAll checks all registered SLOs
func (t *SLOTracker) CheckAll(ctx context.Context) ([]*SLOStatus, error) {
	t.mu.RLock()
	slos := make([]*SLO, 0, len(t.slos))
	for _, slo := range t.slos {
		slos = append(slos, slo)
	}
	t.mu.RUnlock()

	statuses := make([]*SLOStatus, 0, len(slos))
	for _, slo := range slos {
		status, err := t.CheckSLO(ctx, slo)
		if err != nil {
			// Log error but continue checking other SLOs
			status = &SLOStatus{
				Name:        slo.Name,
				Target:      slo.Target,
				IsViolated:  true,
				LastChecked: time.Now(),
				Message:     fmt.Sprintf("Error checking SLO: %v", err),
			}
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// calculateSLI calculates the Service Level Indicator from Prometheus metrics
func (t *SLOTracker) calculateSLI(ctx context.Context, slo *SLO) (float64, error) {
	// Build the query based on SLO type
	var query string
	switch slo.Type {
	case SLOTypeAvailability:
		// Calculate availability as (success / total) * 100
		query = fmt.Sprintf("(sum(increase(%s[%s])) / sum(increase(%s[%s]))) * 100",
			slo.Indicator.Success, slo.Window,
			slo.Indicator.Total, slo.Window)
	case SLOTypeLatency:
		// For latency SLOs, the indicator.Success should be a histogram_quantile query
		// and indicator.Total should be the threshold
		query = slo.Indicator.Success
	default:
		return 0, fmt.Errorf("unsupported SLO type: %s", slo.Type)
	}

	// Query Prometheus
	result, _, err := t.metricsAPI.Query(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("prometheus query failed: %w", err)
	}

	// Extract the value
	switch v := result.(type) {
	case *model.Scalar:
		return float64(v.Value), nil
	case model.Vector:
		if len(v) > 0 {
			return float64(v[0].Value), nil
		}
		return 0, fmt.Errorf("no data returned from query")
	default:
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}
}

// UpdateSLO updates an existing SLO
func (t *SLOTracker) UpdateSLO(name string, updated *SLO) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.slos[name]; !exists {
		return fmt.Errorf("SLO %s not found", name)
	}

	// Preserve the name
	updated.Name = name
	t.slos[name] = updated

	// Invalidate cache
	delete(t.statusCache, name)

	return nil
}

// GetStatus retrieves the cached status for an SLO
func (t *SLOTracker) GetStatus(name string) (*SLOStatus, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	status, exists := t.statusCache[name]
	if !exists {
		return nil, fmt.Errorf("no status available for SLO %s", name)
	}

	return status, nil
}
