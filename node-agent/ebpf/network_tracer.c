// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_endian.h>

char __license[] SEC("license") = "Dual MIT/GPL";

struct conn_event {
	u32 saddr;
	u32 daddr;
	u16 sport;
	u16 dport;
	u64 timestamp;
	u32 pid;
	char comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(u32));
} events SEC(".maps");

SEC("kprobe/tcp_connect")
int kprobe_tcp_connect(struct pt_regs *ctx) {
	struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
	struct conn_event event = {};

	// Get process info
	u64 id = bpf_get_current_pid_tgid();
	event.pid = id >> 32;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	// Read socket fields using CO-RE
	// source address
	event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
	// destination address
	event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
	
	// source port (host byte order in kernel usually)
	event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
	
	// destination port (network byte order in kernel)
	u16 dport_be = BPF_CORE_READ(sk, __sk_common.skc_dport);
	event.dport = bpf_ntohs(dport_be);
	
	event.timestamp = bpf_ktime_get_ns();

	bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}