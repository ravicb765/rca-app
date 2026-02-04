package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

	// fetch once by calling the server directly as startClusterFetcher would do
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(srv.URL + "/api/v1/cluster/services")
	if err != nil {
		t.Fatalf("failed to get from fake cluster-agent: %v", err)
	}
	var payload map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	resp.Body.Close()
	if s, ok := payload["services"]; ok {
		store.set(s)
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

	// event
	payload = []byte(`{"map":"m","cpu":0,"data":"0xdeadbeef"}`)
	req = httptest.NewRequest("POST", "/api/v1/agent/event", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
