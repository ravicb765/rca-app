package cost

import (
	"sync"
	"time"
)

// CostProvider represents the cloud provider
type CostProvider string

const (
	CostProviderAWS   CostProvider = "aws"
	CostProviderGCP   CostProvider = "gcp"
	CostProviderAzure CostProvider = "azure"
	CostProviderOther CostProvider = "other"
)

// CostData represents cost information for a service
type CostData struct {
	Service   string       `json:"service"`
	Cost      float64      `json:"cost"`
	Currency  string       `json:"currency"`
	Provider  CostProvider `json:"provider"`
	Period    string       `json:"period"` // e.g., "2024-01", "2024-01-15"
	Timestamp time.Time    `json:"timestamp"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// CostTrend represents cost trend data
type CostTrend struct {
	Service      string      `json:"service"`
	Data         []CostPoint `json:"data"`
	TotalCost    float64     `json:"total_cost"`
	AverageCost  float64     `json:"average_cost"`
	TrendPercent float64     `json:"trend_percent"` // Percentage change
}

// CostPoint represents a single cost data point
type CostPoint struct {
	Period string  `json:"period"`
	Cost   float64 `json:"cost"`
}

// CostTracker manages cost tracking
type CostTracker struct {
	costs      map[string][]CostData
	mu         sync.RWMutex
	maxHistory int
}

// NewCostTracker creates a new cost tracker
func NewCostTracker() *CostTracker {
	return &CostTracker{
		costs:      make(map[string][]CostData),
		maxHistory: 365, // Keep up to 1 year of daily data
	}
}

// TrackCost records cost data for a service
func (ct *CostTracker) TrackCost(data CostData) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	ct.costs[data.Service] = append(ct.costs[data.Service], data)

	// Prune old data
	if len(ct.costs[data.Service]) > ct.maxHistory {
		ct.costs[data.Service] = ct.costs[data.Service][len(ct.costs[data.Service])-ct.maxHistory:]
	}
}

// GetCostByService retrieves cost data for a specific service
func (ct *CostTracker) GetCostByService(service string, period string) []CostData {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	costs := ct.costs[service]
	if period == "" {
		return costs
	}

	// Filter by period
	var filtered []CostData
	for _, cost := range costs {
		if cost.Period == period {
			filtered = append(filtered, cost)
		}
	}

	return filtered
}

// GetTotalCost calculates total cost across all services for a period
func (ct *CostTracker) GetTotalCost(period string) float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	total := 0.0
	for _, costs := range ct.costs {
		for _, cost := range costs {
			if period == "" || cost.Period == period {
				total += cost.Cost
			}
		}
	}

	return total
}

// GetCostTrend calculates cost trend for a service over a duration
func (ct *CostTracker) GetCostTrend(service string, days int) *CostTrend {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	costs := ct.costs[service]
	if len(costs) == 0 {
		return &CostTrend{Service: service}
	}

	// Group by period
	periodCosts := make(map[string]float64)
	for _, cost := range costs {
		periodCosts[cost.Period] += cost.Cost
	}

	// Convert to sorted data points
	var dataPoints []CostPoint
	totalCost := 0.0
	for period, cost := range periodCosts {
		dataPoints = append(dataPoints, CostPoint{
			Period: period,
			Cost:   cost,
		})
		totalCost += cost
	}

	// Calculate average
	avgCost := 0.0
	if len(dataPoints) > 0 {
		avgCost = totalCost / float64(len(dataPoints))
	}

	// Calculate trend (compare first and last period)
	trendPercent := 0.0
	if len(dataPoints) >= 2 {
		firstCost := dataPoints[0].Cost
		lastCost := dataPoints[len(dataPoints)-1].Cost
		if firstCost > 0 {
			trendPercent = ((lastCost - firstCost) / firstCost) * 100
		}
	}

	return &CostTrend{
		Service:      service,
		Data:         dataPoints,
		TotalCost:    totalCost,
		AverageCost:  avgCost,
		TrendPercent: trendPercent,
	}
}

// GetAllServices returns a list of all services with cost data
func (ct *CostTracker) GetAllServices() []string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	services := make([]string, 0, len(ct.costs))
	for service := range ct.costs {
		services = append(services, service)
	}

	return services
}

// GetCostsByProvider returns costs grouped by provider
func (ct *CostTracker) GetCostsByProvider(period string) map[CostProvider]float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	providerCosts := make(map[CostProvider]float64)
	for _, costs := range ct.costs {
		for _, cost := range costs {
			if period == "" || cost.Period == period {
				providerCosts[cost.Provider] += cost.Cost
			}
		}
	}

	return providerCosts
}
