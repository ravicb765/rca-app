#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct conn_event {
    __u64 ts_ns;
    __u32 pid;
    __u32 tid;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

SEC("kprobe/tcp_connect")
int trace_tcp_connect(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u32 tid = (__u32)pid_tgid;

    struct conn_event ev = {};
    ev.ts_ns = bpf_ktime_get_ns();
    ev.pid = pid;
    ev.tid = tid;

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &ev, sizeof(ev));
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
