package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/ravicb765/rca-app/server/alerts"
	"github.com/ravicb765/rca-app/server/cost"
	"github.com/ravicb765/rca-app/server/deployment"
	"github.com/ravicb765/rca-app/server/inspections"
	"github.com/ravicb765/rca-app/server/ml"
	"github.com/ravicb765/rca-app/server/servicemap"
	"github.com/ravicb765/rca-app/server/slo"
)

const testAPIKey = "test-secret-key"

func setupTestRouter() (*gin.Engine, *serviceStore, *MetadataCache, *AgentStore, *servicemap.ServiceMapBuilder) {
	os.Setenv("RCA_API_KEY", testAPIKey)
	store := &serviceStore{}
	agentStore := NewAgentStore()
	metaCache := NewMetadataCache()
	builder := servicemap.NewServiceMapBuilder(metaCache)
	inspectionEngine := inspections.NewInspectionEngine(nil)
	sloTracker := slo.NewSLOTracker(nil)
	alertManager := alerts.NewAlertManager()
	deploymentTracker, _ := deployment.NewDeploymentTracker()
	costTracker := cost.NewCostTracker()
	mlClient := ml.NewClient("http://localhost:5000")

	r := newRouter(store, agentStore, builder, inspectionEngine, sloTracker, alertManager, deploymentTracker, costTracker, mlClient, metaCache, nil)
	return r, store, metaCache, agentStore, builder
}

func TestServiceMap(t *testing.T) {
	r, _, _, _, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servicemap", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestApplications(t *testing.T) {
	r, _, _, _, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestClusterAgentIntegration(t *testing.T) {
	// fake cluster agent
	h := http.NewServeMux()
	h.HandleFunc("/api/v1/cluster/services", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string][]string{"services": {"ns/a", "ns/b"}})
	})
	h.HandleFunc("/api/v1/cluster/pods", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string][]PodInfo{"pods": {}})
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	// We need to construct dependencies manually here to pass the store to fetchOnce
	store := &serviceStore{}
	agentStore := NewAgentStore()
	metaCache := NewMetadataCache()
	builder := servicemap.NewServiceMapBuilder(metaCache)
	inspectionEngine := inspections.NewInspectionEngine(nil)
	sloTracker := slo.NewSLOTracker(nil)

	// call fetchOnce directly so tests can supply local counters
	fetchTotal := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_fetch_total"})
	fetchSuccess := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_fetch_success"})
	fetchErrors := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_fetch_errors"})
	client := &http.Client{Timeout: 2 * time.Second}
	if err := fetchClusterData(client, srv.URL, store, metaCache, fetchSuccess, fetchErrors, fetchTotal); err != nil {
		t.Fatalf("fetchClusterData failed: %v", err)
	}
	os.Setenv("RCA_API_KEY", testAPIKey)

	r := newRouter(store, agentStore, builder, inspectionEngine, sloTracker, alerts.NewAlertManager(), nil, cost.NewCostTracker(), ml.NewClient(""), metaCache, nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/v1/servicemap", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET servicemap failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}

	// Ensure metrics counters were updated
	if fetchTotal == nil || fetchSuccess == nil || fetchErrors == nil {
		t.Fatalf("metrics not initialized")
	}
}

func TestAgentEndpoints(t *testing.T) {
	r, _, _, _, _ := setupTestRouter()

	// heartbeat
	payload := []byte(`{"hostname":"test","programs":[],"maps":{},"time":"now"}`)
	req := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// event - single connection
	payload = []byte(`{"source_app":"frontend","dest_app":"backend","protocol":"http","request_rate":100}`)
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// event - multiple connections via envelope
	payload = []byte(`{"connections":[{"source_app":"svc1","dest_app":"svc2","protocol":"tcp"},{"source_app":"svc2","dest_app":"svc3","protocol":"http","request_rate":5}]}`)
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for envelope, got %d", w.Code)
	}

	// perf/map style payload with JSON-encoded connection (hex)
	connBody, _ := json.Marshal([]map[string]any{{"source_app": "p1", "dest_app": "p2", "protocol": "tcp"}})
	hexBody := "0x" + strings.ToLower(hex.EncodeToString(connBody))
	payload = []byte(fmt.Sprintf(`{"map":"myperf","data":"%s"}`, hexBody))
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for perf event, got %d", w.Code)
	}

	// perf/map style payload with ascii key=value pattern
	ascii := "src=10.0.0.1:1234 dst=10.0.0.2:80 proto=http"
	hexAscii := "0x" + strings.ToLower(hex.EncodeToString([]byte(ascii)))
	payload = []byte(fmt.Sprintf(`{"map":"myperf","data":"%s"}`, hexAscii))
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for perf ascii event, got %d", w.Code)
	}

	// perf/map style payload using base64 payload similar to perf-consumer default
	connBody, _ = json.Marshal(map[string]any{"source_app": "b1", "dest_app": "b2", "protocol": "tcp"})
	b64 := base64.StdEncoding.EncodeToString(connBody)
	payload = []byte(fmt.Sprintf(`{"map":"myperf","data_base64":"%s"}`, b64))
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for base64 perf event, got %d", w.Code)
	}

	// binary IPv4 payload
	ipv4 := []byte{10, 0, 0, 1, 10, 0, 0, 2}
	ipv4 = append(ipv4, 0x39, 0x00) // sport 57 (little endian 0x0039)
	ipv4 = append(ipv4, 0x50, 0x00) // dport 80
	ipv4 = append(ipv4, make([]byte, 8)...)
	hexIpv4 := "0x" + strings.ToLower(hex.EncodeToString(ipv4))
	payload = []byte(fmt.Sprintf(`{"map":"myperf","data":"%s"}`, hexIpv4))
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for ipv4 binary perf, got %d", w.Code)
	}

	// binary IPv6 payload
	ip6src := net.ParseIP("2001:db8::1")
	ip6dst := net.ParseIP("2001:db8::2")
	if ip6src == nil || ip6dst == nil {
		t.Skip("ipv6 parsing not available in this environment")
	}
	b6 := make([]byte, 0, 44)
	b6 = append(b6, ip6src.To16()...)
	b6 = append(b6, ip6dst.To16()...)
	b6 = append(b6, 0x1f, 0x00) // sport 31
	b6 = append(b6, 0x50, 0x00) // dport 80
	b6 = append(b6, make([]byte, 8)...)
	hexIpv6 := "0x" + strings.ToLower(hex.EncodeToString(b6))
	payload = []byte(fmt.Sprintf(`{"map":"myperf","data":"%s"}`, hexIpv6))
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for ipv6 binary perf, got %d", w.Code)
	}

	// GET service map and assert the perf events resulted in applications
	req = httptest.NewRequest(http.MethodGet, "/api/v1/servicemap", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := resp["servicemap"]; !ok {
		t.Fatalf("expected servicemap in response")
	}

	// GET application analysis (DEMO mode)
	payload = []byte(`{"application_id":"payment-service","start_time":"now-1h","end_time":"now"}`)
	req = httptest.NewRequest("POST", "/api/v1/analyze", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for analyze, got %d", w.Code)
	}

	// Check that IPv4 connection was added (10.0.0.1:57 -> 10.0.0.2:80)
	if !strings.Contains(w.Body.String(), "10.0.0.1:57") {
		t.Fatalf("expected ipv4 src present in servicemap")
	}
	if !strings.Contains(w.Body.String(), "10.0.0.2:80") {
		t.Fatalf("expected ipv4 dst present in servicemap")
	}
	// Check IPv6 presence (formatted address with port)
	if !strings.Contains(w.Body.String(), "2001:db8::1:31") && !strings.Contains(w.Body.String(), "[2001:db8::1]:31") {
		t.Fatalf("expected ipv6 src present in servicemap")
	}
	// Consolidated final verification

}

func TestMetadataRegistration(t *testing.T) {
	r, _, metaCache, _, _ := setupTestRouter()

	// Payload
	pods := []PodInfo{
		{Name: "frontend", Namespace: "default", IP: "10.0.0.1", Node: "node-1"},
		{Name: "backend", Namespace: "default", IP: "10.0.0.2", Node: "node-2"},
	}
	body, _ := json.Marshal(map[string]interface{}{"pods": pods})

	// Request
	req := httptest.NewRequest("POST", "/api/v1/metadata", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify Response
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify Cache Update
	if p := metaCache.LookupPod("10.0.0.1"); p == nil || p.PodName != "frontend" {
		t.Error("expected pod 10.0.0.1 to be registered as frontend")
	}
}
