package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestServicesEndpoint_NoClient(t *testing.T) {
	r := newRouter(nil)
	s := httptest.NewServer(r)
	defer s.Close()

	resp, err := http.Get(s.URL + "/api/v1/cluster/services")
	if err != nil {
		t.Fatalf("GET services failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}
}

func TestPodsEndpoint_NoClient(t *testing.T) {
	r := newRouter(nil)
	s := httptest.NewServer(r)
	defer s.Close()

	resp, err := http.Get(s.URL + "/api/v1/cluster/pods")
	if err != nil {
		t.Fatalf("GET pods failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	r := newRouter(nil)
	s := httptest.NewServer(r)
	defer s.Close()

	resp, err := http.Get(s.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET metrics failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}
}

func TestBuildKubeClient_InvalidKubeconfig(t *testing.T) {
	// Set KUBECONFIG to a non-existent file and ensure buildKubeClient errors
	prev := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", "/nonexistent-kubeconfig")
	defer os.Setenv("KUBECONFIG", prev)

	_, err := buildKubeClient()
	if err == nil {
		t.Fatalf("expected error when KUBECONFIG is invalid, got nil")
	}
}
