//go:build ebpf
// +build ebpf

package main

import (
	"fmt"
	"os"

	"github.com/cilium/ebpf"
)

// LoadEBPFObjects loads eBPF objects from node-agent/ebpf/*.o and verifies they can be loaded.
func LoadEBPFObjects() error {
	obj := "node-agent/ebpf/network_tracer.o"
	if _, err := os.Stat(obj); os.IsNotExist(err) {
		return fmt.Errorf("ebpf object not found: %s", obj)
	}

	spec, err := ebpf.LoadCollectionSpec(obj)
	if err != nil {
		return fmt.Errorf("failed to load collection spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer coll.Close()

	// For now, we load and immediately close. Attaching to kprobes requires extra privileges and platform-specific logic.
	return nil
}
