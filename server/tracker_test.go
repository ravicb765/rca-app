package slo

import (
	"context"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type mockMetricsClient struct {
	value model.Value
}

func (m *mockMetricsClient) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	return m.value, nil, nil
}

func TestSLOTracker_CheckSLO(t *testing.T) {
	// Mock returning a scalar value (e.g. 0.9995 for 99.95%)
	mockClient := &mockMetricsClient{
		value: &model.Scalar{
			Value:     0.9995,
			Timestamp: model.TimeFromUnix(0),
		},
	}

	tracker := NewSLOTracker(mockClient)
	slo := &SLO{
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.9,
		Window: "30d",
		Indicator: SLOIndicator{
			Success: "good",
			Total:   "total",
		},
	}
	tracker.AddSLO(slo)

	status, err := tracker.CheckSLO(context.Background(), slo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.IsViolated {
		t.Error("expected SLO to be met")
	}

	// 99.95% > 99.9%
	if status.ActualSLI != 99.95 {
		t.Errorf("expected SLI 99.95, got %f", status.ActualSLI)
	}
}

func TestSLOTracker_CheckAll(t *testing.T) {
	// Mock returning a scalar value (e.g. 0.9995 for 99.95%)
	mockClient := &mockMetricsClient{
		value: &model.Scalar{
			Value:     0.9995,
			Timestamp: model.TimeFromUnix(0),
		},
	}

	tracker := NewSLOTracker(mockClient)
	tracker.AddSLO(&SLO{
		Name:   "SLO 1",
		Type:   SLOTypeAvailability,
		Target: 99.9,
		Window: "30d",
		Indicator: SLOIndicator{Success: "good", Total: "total"},
	})
	tracker.AddSLO(&SLO{
		Name:   "SLO 2",
		Type:   SLOTypeAvailability,
		Target: 99.99, // Higher target, should fail with 99.95
		Window: "30d",
		Indicator: SLOIndicator{Success: "good", Total: "total"},
	})

	statuses, err := tracker.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	if statuses[0].IsViolated {
		t.Error("expected SLO 1 to be met")
	}

	if !statuses[1].IsViolated {
		t.Error("expected SLO 2 to be violated")
	}
}
