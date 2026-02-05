// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct page_fault_event {
	__u32 pid;
	__u64 address;
	__u64 ip;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} page_fault_events SEC(".maps");

SEC("tracepoint/exceptions/page_fault_user")
int trace_page_fault_user(struct trace_event_raw_page_fault_user *ctx) {
	struct page_fault_event event = {};

	event.pid = bpf_get_current_pid_tgid() >> 32;
	event.address = ctx->address;
	event.ip = ctx->ip;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	bpf_perf_event_output(ctx, &page_fault_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";