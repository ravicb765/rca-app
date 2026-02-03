#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16);
    __type(key, u64); // key type is u64
    __type(value, u64);
} my_map SEC(".maps");

SEC("kprobe/tcp_connect")
int trace_connect_map_fixed(struct pt_regs *ctx) {
    u64 key = 1; // correct key type (u64)
    u64 zero = 0;
    bpf_map_update_elem(&my_map, &key, &zero, BPF_NOEXIST);

    u64 *val = bpf_map_lookup_elem(&my_map, &key);
    if (val) {
        __sync_fetch_and_add(val, 1);
    }
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
