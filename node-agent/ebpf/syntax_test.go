package ebpf_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClangSyntax(t *testing.T) {
	// Check for clang availability
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not found; skipping eBPF syntax checks")
	}
	// Gather C files
	files, _ := filepath.Glob("node-agent/ebpf/*.c")
	more, _ := filepath.Glob("node-agent/ebpf/*/*.c")
	files = append(files, more...)
	if len(files) == 0 {
		t.Skip("no eBPF C files to check")
	}
	args := append([]string{"-fsyntax-only"}, files...)
	cmd := exec.Command("clang", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("clang syntax check failed: %s", string(out))
	}
}
