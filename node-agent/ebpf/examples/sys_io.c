#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, u32);
    __type(value, u64);
} read_bytes SEC(".maps");

SEC("kprobe/__x64_sys_read")
int trace_read(struct pt_regs *ctx) {
    ssize_t count = (ssize_t)PT_REGS_PARM3(ctx);
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    __u64 val = (count < 0) ? 0 : (__u64)count;
    bpf_map_update_elem(&read_bytes, &pid, &val, BPF_ANY);
    return 0;
}

SEC("kprobe/__x64_sys_write")
int trace_write(struct pt_regs *ctx) {
    ssize_t count = (ssize_t)PT_REGS_PARM3(ctx);
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    __u64 val = (count < 0) ? 0 : (__u64)count;
    bpf_map_update_elem(&read_bytes, &pid, &val, BPF_ANY);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
