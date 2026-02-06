#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

SEC("kprobe/tcp_connect")
int trace_connect(struct pt_regs *ctx) {
    // Intentionally large stack buffer to exceed eBPF stack
    char large_buf[1024];
    // Touch the buffer to make the verifier see stack usage
    large_buf[0] = 0;
    large_buf[1023] = 1;
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
