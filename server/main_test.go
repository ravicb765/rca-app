package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceMap(t *testing.T) {
	r := newRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servicemap", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestApplications(t *testing.T) {
	r := newRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAgentEndpoints(t *testing.T) {
	r := newRouter()

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
