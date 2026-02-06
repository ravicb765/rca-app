// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define EVENT_TYPE_CONNECT 1
#define EVENT_TYPE_ACCEPT  2
#define EVENT_TYPE_CLOSE   3

struct conn_event {
    u64 timestamp;
    u32 saddr;
    u32 daddr;
    u32 pid;
    u32 type;
    u16 sport;
    u16 dport;
    char comm[16];
    u32 _pad; // Ensure 8-byte alignment/padding
};

// Map to send events to userspace
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u32));
} events SEC(".maps");

// Map to correlate tcp_connect entry with return
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct sock *));
    __uint(max_entries, 10240);
} connect_start SEC(".maps");

static __always_inline void submit_event(void *ctx, struct sock *sk, u32 type) {
    struct conn_event event = {};
    
    // Only handle IPv4 for this example (family 2)
    u16 family = BPF_CORE_READ(sk, __sk_common.skc_family);
    if (family != 2) {
        return;
    }

    event.timestamp = bpf_ktime_get_ns();
    event.type = type;
    event.pid = bpf_get_current_pid_tgid() >> 32;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

// Hook: Entry of tcp_connect
// We store the socket pointer to retrieve it on return
SEC("kprobe/tcp_connect")
int kprobe_tcp_connect(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    u32 tid = bpf_get_current_pid_tgid();
    
    bpf_map_update_elem(&connect_start, &tid, &sk, BPF_ANY);
    return 0;
}

// Hook: Return of tcp_connect
// Check if connection was successful (ret == 0)
SEC("kretprobe/tcp_connect")
int kretprobe_tcp_connect(struct pt_regs *ctx) {
    u32 tid = bpf_get_current_pid_tgid();
    struct sock **skpp;
    
    skpp = bpf_map_lookup_elem(&connect_start, &tid);
    if (!skpp) return 0;

    struct sock *sk = *skpp;
    int ret = PT_REGS_RC(ctx);

    if (ret == 0) {
        submit_event(ctx, sk, EVENT_TYPE_CONNECT);
    }

    bpf_map_delete_elem(&connect_start, &tid);
    return 0;
}

// Hook: Return of inet_csk_accept
// Returns the new socket on success
SEC("kretprobe/inet_csk_accept")
int kretprobe_inet_csk_accept(struct pt_regs *ctx) {
    struct sock *newsk = (struct sock *)PT_REGS_RC(ctx);

    if (newsk) {
        submit_event(ctx, newsk, EVENT_TYPE_ACCEPT);
    }
    return 0;
}

// Hook: Entry of tcp_close
SEC("kprobe/tcp_close")
int kprobe_tcp_close(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    
    submit_event(ctx, sk, EVENT_TYPE_CLOSE);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";