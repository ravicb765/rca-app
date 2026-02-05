// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

struct tcp_retrans_event {
	__u32 saddr;
	__u32 daddr;
	__u16 sport;
	__u16 dport;
	__u32 pid;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} tcp_retrans_events SEC(".maps");

SEC("tracepoint/tcp/tcp_retransmit_skb")
int trace_tcp_retransmit_skb(struct trace_event_raw_tcp_retransmit_skb *ctx) {
	struct tcp_retrans_event event = {};

	event.pid = bpf_get_current_pid_tgid() >> 32;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	event.saddr = ctx->saddr;
	event.daddr = ctx->daddr;
	event.sport = ctx->sport;
	event.dport = ctx->dport;

	bpf_perf_event_output(ctx, &tcp_retrans_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";