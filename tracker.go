package slo

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type SLOType string

const (
	SLOTypeAvailability SLOType = "availability"
	SLOTypeLatency      SLOType = "latency"
)

type SLOIndicator struct {
	Success   string `json:"success,omitempty"`
	Total     string `json:"total,omitempty"`
	Metric    string `json:"metric,omitempty"`
	Threshold string `json:"threshold,omitempty"`
}

type SLO struct {
	Name        string       `json:"name"`
	Application string       `json:"application"`
	Type        SLOType      `json:"type"`
	Target      float64      `json:"target"`
	Window      string       `json:"window"`
	Indicator   SLOIndicator `json:"indicator"`
}

type SLOStatus struct {
	SLO         *SLO    `json:"slo"`
	ActualSLI   float64 `json:"actual_sli"`
	ErrorBudget float64 `json:"error_budget"`
	IsViolated  bool    `json:"is_violated"`
}

type MetricsClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error)
}

type SLOTracker struct {
	slos   []*SLO
	client MetricsClient
}

func NewSLOTracker(client MetricsClient) *SLOTracker {
	return &SLOTracker{
		slos:   []*SLO{},
		client: client,
	}
}

func (st *SLOTracker) AddSLO(slo *SLO) {
	st.slos = append(st.slos, slo)
}

func (st *SLOTracker) CheckSLO(ctx context.Context, slo *SLO) (*SLOStatus, error) {
	var query string

	if slo.Type == SLOTypeAvailability {
		query = fmt.Sprintf(
			"sum(rate(%s[%s])) / sum(rate(%s[%s]))",
			slo.Indicator.Success,
			slo.Window,
			slo.Indicator.Total,
			slo.Window,
		)
	} else if slo.Type == SLOTypeLatency {
		if slo.Indicator.Threshold != "" {
			query = fmt.Sprintf(
				"sum(rate(%s_bucket{le=\"%s\"}[%s])) / sum(rate(%s_count[%s]))",
				slo.Indicator.Metric,
				slo.Indicator.Threshold,
				slo.Window,
				slo.Indicator.Metric,
				slo.Window,
			)
		}
	}

	if query == "" {
		return nil, fmt.Errorf("unsupported SLO configuration")
	}

	val, _, err := st.client.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	var sli float64
	switch v := val.(type) {
	case *model.Scalar:
		sli = float64(v.Value)
	case model.Vector:
		if len(v) > 0 {
			sli = float64(v[0].Value)
		}
	default:
		return nil, fmt.Errorf("unexpected query result type: %T", val)
	}

	sliPercent := sli * 100
	errorBudget := sliPercent - slo.Target

	return &SLOStatus{
		SLO:         slo,
		ActualSLI:   sliPercent,
		ErrorBudget: errorBudget,
		IsViolated:  sliPercent < slo.Target,
	}, nil
}

func (st *SLOTracker) CheckAll(ctx context.Context) ([]*SLOStatus, error) {
	var results []*SLOStatus
	for _, s := range st.slos {
		status, err := st.CheckSLO(ctx, s)
		if err != nil {
			return nil, err
		}
		results = append(results, status)
	}
	return results, nil
}
