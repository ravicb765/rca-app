// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct drop_event {
	__u32 pid;
	__u64 location;
	__u32 protocol;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} drop_events SEC(".maps");

SEC("tracepoint/skb/kfree_skb")
int trace_kfree_skb(struct trace_event_raw_kfree_skb *ctx) {
	struct drop_event event = {};

	event.pid = bpf_get_current_pid_tgid() >> 32;
	event.location = (__u64)ctx->location;
	event.protocol = ctx->protocol;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	bpf_perf_event_output(ctx, &drop_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";