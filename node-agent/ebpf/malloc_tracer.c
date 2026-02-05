// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct malloc_event {
	__u32 pid;
	__u64 size;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} malloc_events SEC(".maps");

SEC("uprobe/libc:malloc")
int probe_malloc(struct pt_regs *ctx) {
	struct malloc_event event = {};
	event.pid = bpf_get_current_pid_tgid() >> 32;
	event.size = PT_REGS_PARM1(ctx);
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	bpf_perf_event_output(ctx, &malloc_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

// Note: We are not tracing free() for rate calculation in this simple example,
// but typically one would trace free to calculate net allocation or leak detection.
// For "allocation rate", malloc is sufficient.

char LICENSE[] SEC("license") = "GPL";