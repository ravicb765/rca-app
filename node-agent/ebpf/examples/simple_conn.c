// Simple example eBPF program for compile+syntax verification
// This program is intentionally minimal and only demonstrates structures
// and a perf event map that user-space code can read. It is suitable for
// compile and syntax-only checks in CI.

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>

struct conn_event {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u64 ts_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

SEC("kprobe/tcp_connect")
int bpf_prog1(struct pt_regs *ctx) {
    // This is a no-op safe program for syntax checking only
    struct conn_event e = {};
    e.ts_ns = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &e, sizeof(e));
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
