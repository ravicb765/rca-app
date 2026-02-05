// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct context_switch_event {
	__u32 prev_pid;
	__u32 next_pid;
	__u8 next_comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} context_switch_events SEC(".maps");

SEC("tracepoint/sched/sched_switch")
int trace_sched_switch(struct trace_event_raw_sched_switch *ctx) {
	struct context_switch_event event = {};
	event.prev_pid = ctx->prev_pid;
	event.next_pid = ctx->next_pid;
	bpf_probe_read_kernel_str(&event.next_comm, sizeof(event.next_comm), ctx->next_comm);
	bpf_perf_event_output(ctx, &context_switch_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";