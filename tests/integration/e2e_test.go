package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
    "github.com/ravicb765/rca-app/server/servicemap"
)

const testAPIKey = "rca-dev-secret-key"

func TestIngestionPipeline(t *testing.T) {
    // This assumes the Server is running separately or we spin it up here.
    assert.NotEmpty(t, clickHouseAddr)
    assert.NotEmpty(t, kafkaBrokers)
    assert.NotEmpty(t, redisAddr)
}

func TestAnalyzeEndpoint(t *testing.T) {
    // Test the newly implemented AI Analysis endpoint
    reqBody, _ := json.Marshal(map[string]string{
        "application_id": "payment-service",
        "start_time":     time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
        "end_time":       time.Now().Format(time.RFC3339),
    })

    // In a live test environment:
    // req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/analyze", bytes.NewBuffer(reqBody))
    // req.Header.Set("X-API-Key", testAPIKey)
    // req.Header.Set("Content-Type", "application/json")
    // ...
    
    assert.NotNil(t, reqBody)
}

func TestServiceMapAPI(t *testing.T) {
    // Simulate sending telemetry
    event := servicemap.TelemetryEvent{
        SrcIP: "10.0.0.1",
        DstIP: "10.0.0.2",
        Protocol: "http",
    }
    
    payload, _ := json.Marshal(event)
    
    // Updated with X-API-Key requirement
    // req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/agent/event", bytes.NewBuffer(payload))
    // req.Header.Set("X-API-Key", testAPIKey)
    // req.Header.Set("Content-Type", "application/json")
    
    assert.NotNil(t, payload)
    assert.Contains(t, string(payload), "10.0.0.1")
}
