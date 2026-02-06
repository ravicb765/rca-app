package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInspectionEngine_RunAll(t *testing.T) {
	engine := NewEngine()

	// Mock Inspection
	mock := &mockInspection{name: "Test Check", pass: true}
	engine.Register(mock)

	results := engine.RunAll()
	assert.Len(t, results, 1)
	assert.Equal(t, "pass", results[0].Status)
	assert.Equal(t, "Test Check", results[0].Name)
}

func TestBuiltInInspections(t *testing.T) {
	check := &HighErrorRateInspection{Threshold: 0.1}
	// We mocked the internal logic in implementation to return mock data for demo,
	// so we verify that behavior.
	res, err := check.Run()
	assert.NoError(t, err)
	assert.NotNil(t, res)
	// Current mock implementation hardcodes 0.05 rate vs 0.1 threshold -> Pass
	assert.Equal(t, "pass", res.Status)
	
	latencyCheck := &LatencyInspection{ThresholdMs: 100.0}
	res2, err := latencyCheck.Run()
	assert.NoError(t, err)
	// Mock hardcodes 150ms vs 100ms -> Fail
	assert.Equal(t, "fail", res2.Status)
}

// Mock type
type mockInspection struct {
	name string
	pass bool
}

func (m *mockInspection) Name() string     { return m.name }
func (m *mockInspection) Category() string { return "Test" }
func (m *mockInspection) Run() (*InspectionResult, error) {
	status := "fail"
	if m.pass {
		status = "pass"
	}
	return &InspectionResult{
		Name:   m.name,
		Status: status,
	}, nil
}
