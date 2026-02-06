package main

import "testing"

func TestLoaderStub(t *testing.T) {
	// When built without 'ebpf' tag the stub should succeed
	if err := LoadEBPFObjects(); err != nil {
		t.Fatalf("expected nil error from LoadEBPFObjects stub, got %v", err)
	}
	if err := StopEBPFObjects(); err != nil {
		t.Fatalf("expected nil error from StopEBPFObjects, got %v", err)
	}
}
