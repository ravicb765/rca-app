#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct http_event {
    __u64 ts_ns;
    __u32 pid;
    __u8 method; // 0 = unknown, 1 = GET, 2 = POST
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// NOTE: This is a simplified example. Parsing TCP payloads is platform-specific
// and often requires reading skb or user buffers which may not be safe in all kernels.
// This example emits an event when tcp_cleanup_rbuf is called; concrete payload
// parsing should be implemented carefully.
SEC("kprobe/tcp_cleanup_rbuf")
int trace_tcp_payload(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    struct http_event ev = {};
    ev.ts_ns = bpf_ktime_get_ns();
    ev.pid = pid;
    ev.method = 0; // unknown in this simple example

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &ev, sizeof(ev));
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
