//go:build ebpf
// +build ebpf

package main

// LoadEBPFObjects starts the EBPF agent (loads objects, attaches probes, and starts telemetry forwarding).
func LoadEBPFObjects() error {
	return StartEBPFAgent()
}

// StopEBPFObjects stops the running agent and cleans up resources.
func StopEBPFObjects() error {
	return StopEBPFAgent()
}
