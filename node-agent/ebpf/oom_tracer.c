// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct oom_event {
	__u64 cgroup_id;
	__u32 pid;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} oom_events SEC(".maps");

SEC("tracepoint/oom/mark_victim")
int trace_mark_victim(struct trace_event_raw_mark_victim *ctx) {
	struct oom_event event = {};

	event.pid = ctx->pid;
	event.cgroup_id = bpf_get_current_cgroup_id();
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	bpf_perf_event_output(ctx, &oom_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";