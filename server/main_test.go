package main

import (
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
