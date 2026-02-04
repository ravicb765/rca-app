// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// --- Maps ---

// CPU usage: cgroup_id -> total_ns
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u64));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 1024);
} cgroup_cpu_usage SEC(".maps");

struct io_stats {
    u64 read_bytes;
    u64 write_bytes;
    u64 read_ops;
    u64 write_ops;
};

// Disk I/O usage: cgroup_id -> stats
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u64));
    __uint(value_size, sizeof(struct io_stats));
    __uint(max_entries, 1024);
} cgroup_io_usage SEC(".maps");

// Network I/O usage: cgroup_id -> stats (reusing io_stats for rx/tx)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u64));
    __uint(value_size, sizeof(struct io_stats));
    __uint(max_entries, 1024);
} cgroup_net_usage SEC(".maps");

// Track task start time for CPU calculation: pid -> timestamp
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} task_start_time SEC(".maps");

// --- CPU Tracking ---

SEC("tp/sched/sched_switch")
int handle_sched_switch(struct trace_event_raw_sched_switch *ctx) {
    u64 ts = bpf_ktime_get_ns();
    u32 prev_pid = ctx->prev_pid;
    u32 next_pid = ctx->next_pid;

    // 1. Account for the task switching out (prev)
    u64 *start_ts = bpf_map_lookup_elem(&task_start_time, &prev_pid);
    if (start_ts) {
        u64 delta = ts - *start_ts;
        u64 cgroup_id = bpf_get_current_cgroup_id(); // Gets ID of current context (prev task)
        
        u64 *usage = bpf_map_lookup_elem(&cgroup_cpu_usage, &cgroup_id);
        if (!usage) {
            u64 init_val = delta;
            bpf_map_update_elem(&cgroup_cpu_usage, &cgroup_id, &init_val, BPF_ANY);
        } else {
            __sync_fetch_and_add(usage, delta);
        }
    }

    // 2. Record start time for task switching in (next)
    bpf_map_update_elem(&task_start_time, &next_pid, &ts, BPF_ANY);
    return 0;
}

// --- Disk I/O Tracking ---

SEC("tp/block/block_rq_complete")
int handle_block_rq_complete(struct trace_event_raw_block_rq_complete *ctx) {
    u64 cgroup_id = bpf_get_current_cgroup_id();
    u64 nr_bytes = ctx->nr_sector * 512;
    char rwbs[8];
    bpf_probe_read_kernel(rwbs, sizeof(rwbs), ctx->rwbs);
    
    struct io_stats *stats = bpf_map_lookup_elem(&cgroup_io_usage, &cgroup_id);
    if (!stats) {
        struct io_stats new_stats = {};
        bpf_map_update_elem(&cgroup_io_usage, &cgroup_id, &new_stats, BPF_ANY);
        stats = bpf_map_lookup_elem(&cgroup_io_usage, &cgroup_id);
        if (!stats) return 0;
    }

    if (rwbs[0] == 'R') {
        __sync_fetch_and_add(&stats->read_bytes, nr_bytes);
        __sync_fetch_and_add(&stats->read_ops, 1);
    } else if (rwbs[0] == 'W') {
        __sync_fetch_and_add(&stats->write_bytes, nr_bytes);
        __sync_fetch_and_add(&stats->write_ops, 1);
    }
    return 0;
}

// --- Network I/O Tracking (TCP) ---

static __always_inline void update_net_stats(u64 cgroup_id, u64 bytes, bool is_rx) {
    struct io_stats *stats = bpf_map_lookup_elem(&cgroup_net_usage, &cgroup_id);
    if (!stats) {
        struct io_stats new_stats = {};
        bpf_map_update_elem(&cgroup_net_usage, &cgroup_id, &new_stats, BPF_ANY);
        stats = bpf_map_lookup_elem(&cgroup_net_usage, &cgroup_id);
        if (!stats) return;
    }
    if (is_rx) __sync_fetch_and_add(&stats->read_bytes, bytes);
    else       __sync_fetch_and_add(&stats->write_bytes, bytes);
}

SEC("kprobe/tcp_sendmsg")
int kprobe_tcp_sendmsg_cnt(struct pt_regs *ctx) {
    update_net_stats(bpf_get_current_cgroup_id(), (size_t)PT_REGS_PARM3(ctx), false);
    return 0;
}

SEC("kretprobe/tcp_recvmsg")
int kretprobe_tcp_recvmsg_cnt(struct pt_regs *ctx) {
    int ret = PT_REGS_RC(ctx);
    if (ret > 0) update_net_stats(bpf_get_current_cgroup_id(), ret, true);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";