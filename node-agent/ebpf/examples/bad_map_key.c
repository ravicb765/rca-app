#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16);
    __type(key, u64); // deliberately use u64 key
    __type(value, u64);
} my_map SEC(".maps");

SEC("kprobe/tcp_connect")
int trace_connect2(struct pt_regs *ctx) {
    u32 key = 1; // wrong key type used to access map (u32 vs u64)
    u64 *val = bpf_map_lookup_elem(&my_map, &key);
    if (val) {
        *val = *val + 1;
    }
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
