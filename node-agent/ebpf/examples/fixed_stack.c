#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

// Fixed version: reduce stack usage and avoid large local arrays.
// Use a small buffer and/or offload larger buffers to maps.

SEC("kprobe/tcp_connect")
int trace_connect_fixed(struct pt_regs *ctx) {
    // Small stack usage (safe)
    char small_buf[64];
    small_buf[0] = 0;

    // Example: increment a counter in a map (safe map usage shown elsewhere)
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
