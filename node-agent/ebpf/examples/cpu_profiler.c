#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct cpu_sample {
    __u64 ts_ns;
    __u32 pid;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} perfbuf SEC(".maps");

SEC("perf_event")
int sample_cpu(struct bpf_perf_event_data *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    struct cpu_sample s = {};
    s.ts_ns = bpf_ktime_get_ns();
    s.pid = pid;

    bpf_perf_event_output(ctx, &perfbuf, BPF_F_CURRENT_CPU, &s, sizeof(s));
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
