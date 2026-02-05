// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

struct udp_event {
	__u32 pid;
	__u32 len;
	__u8 comm[16];
	__u8 direction; // 0: send, 1: recv
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} udp_events SEC(".maps");

SEC("tracepoint/udp/udp_fail_queue_rcv_skb")
int trace_udp_fail_queue_rcv_skb(struct trace_event_raw_udp_fail_queue_rcv_skb *ctx) {
	struct udp_event event = {};
	event.pid = bpf_get_current_pid_tgid() >> 32;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));
	bpf_perf_event_output(ctx, &udp_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";