package main

import "fmt"

// LoadEBPFObjects is a stub implementation used when not building with the 'ebpf' tag.
// Build with `-tags ebpf` to enable the real loader (see loader_ebpf.go).
func LoadEBPFObjects() error {
	fmt.Println("EBPF loader not enabled; build with '-tags ebpf' to enable loading")
	return nil
}

// StopEBPFObjects is a no-op when EBPF support is not compiled in.
func StopEBPFObjects() error {
	return nil
}
