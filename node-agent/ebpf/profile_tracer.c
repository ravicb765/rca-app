// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

struct {
    __uint(type, BPF_MAP_TYPE_STACK_TRACE);
    __uint(key_size, sizeof(u32));
    __uint(value_size, 100 * sizeof(u64)); // MAX_STACK_DEPTH * 8
    __uint(max_entries, 10000);
} stack_traces SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u32)); // stack_id
    __uint(value_size, sizeof(u64)); // count
    __uint(max_entries, 10000);
} counts SEC(".maps");

// Perf event to sample at frequency
SEC("perf_event")
int profile_cpu(struct bpf_perf_event_data *ctx) {
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 cpu = bpf_get_smp_processor_id();
    
    // Capture usage: Stack ID
    // BPF_F_USER_STACK | BPF_F_MULTI_STACK? Just Kernel+User
    u32 stack_id = bpf_get_stackid(ctx, &stack_traces, BPF_F_USER_STACK);
    
    // Using positive stack_id for user, maybe negative logic for kernel?
    // Simplified: Just count user stacks for application profiling
    
    if ((int)stack_id >= 0) {
        u64 *val = bpf_map_lookup_elem(&counts, &stack_id);
        if (!val) {
            u64 one = 1;
            bpf_map_update_elem(&counts, &stack_id, &one, BPF_ANY);
        } else {
            __sync_fetch_and_add(val, 1);
        }
    }
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
