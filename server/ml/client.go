package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AnalysisRequest is the payload sent to the ML service
type AnalysisRequest struct {
	ApplicationID string                 `json:"application_id"`
	StartTime     string                 `json:"start_time"`
	EndTime       string                 `json:"end_time"`
	Metrics       map[string]interface{} `json:"metrics"`
}

// AnalysisResponse is the response returned by the ML service
type AnalysisResponse struct {
	ApplicationID string   `json:"application_id"`
	Analysis      Analysis `json:"analysis"`
}

// Analysis contains the detailed results of the RCA
type Analysis struct {
	IsAnomaly   bool     `json:"is_anomaly"`
	RootCause   string   `json:"root_cause"`
	Confidence  float64  `json:"confidence"`
	Reasoning   string   `json:"reasoning"`
	Remediation []string `json:"remediation"`
	Timestamp   string   `json:"timestamp,omitempty"`
}

// Client handles communication with the ML service
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new ML service client
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Analyze sends an analysis request to the ML service
func (c *Client) Analyze(ctx context.Context, req AnalysisRequest) (*Analysis, error) {
	url := fmt.Sprintf("%s/analyze", c.BaseURL)
	
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call ML service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ML service returned unexpected status: %s", resp.Status)
	}

	var analysisResp AnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&analysisResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Add server-side timestamp if missing
	if analysisResp.Analysis.Timestamp == "" {
		analysisResp.Analysis.Timestamp = time.Now().Format(time.RFC3339)
	}

	return &analysisResp.Analysis, nil
}
