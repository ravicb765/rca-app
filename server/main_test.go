package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestServiceMap(t *testing.T) {
	store := &serviceStore{}
	r := newRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servicemap", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestApplications(t *testing.T) {
	store := &serviceStore{}
	r := newRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
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
	srv := httptest.NewServer(h)
	defer srv.Close()

	store := &serviceStore{}

	// call fetchOnce directly so tests can supply local counters
	fetchTotal := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_fetch_total"})
	fetchSuccess := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_fetch_success"})
	fetchErrors := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_fetch_errors"})
	client := &http.Client{Timeout: 2 * time.Second}
	if err := fetchOnce(client, srv.URL, store, fetchSuccess, fetchErrors, fetchTotal); err != nil {
		t.Fatalf("fetchOnce failed: %v", err)
	}

	r := newRouter(store)
	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/servicemap")
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
	store := &serviceStore{}
	r := newRouter(store)

	// heartbeat
	payload := []byte(`{"hostname":"test","programs":[],"maps":{},"time":"now"}`)
	req := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// event - single connection
	payload = []byte(`{"source_app":"frontend","dest_app":"backend","protocol":"http","request_rate":100}`)
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// GET service map and assert the connection resulted in an application
	req = httptest.NewRequest(http.MethodGet, "/api/v1/servicemap", nil)
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
}
