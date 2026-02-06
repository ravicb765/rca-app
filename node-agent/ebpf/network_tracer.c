#include <linux/bpf.h>
#include <linux/types.h>
#include <linux/ptrace.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Define the event structure to send to userspace
struct conn_event {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u64 timestamp;
    __u32 pid;
    char comm[16];
};

// Map for sending events
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

// Minimal struct definitions to access socket fields
// This avoids dependency on full kernel headers for this example
struct sock_common {
    union {
        struct {
            __be32 skc_daddr;
            __be32 skc_rcv_saddr;
        };
    };
    union {
        // padding
    };
    union {
        struct {
            __be16 skc_dport;
            __u16  skc_num;
        };
    };
    short unsigned int skc_family;
} __attribute__((preserve_access_index));

struct sock {
    struct sock_common __sk_common;
} __attribute__((preserve_access_index));

SEC("kprobe/tcp_connect")
int kprobe__tcp_connect(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct conn_event event = {};
    short unsigned int family = 0;

    // Read address family
    bpf_probe_read(&family, sizeof(family), &sk->__sk_common.skc_family);

    // Only trace IPv4 (AF_INET = 2)
    if (family != 2) {
        return 0;
    }

    // Capture process info
    __u64 id = bpf_get_current_pid_tgid();
    event.pid = id >> 32;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));
    event.timestamp = bpf_ktime_get_ns();

    // Capture connection details
    bpf_probe_read(&event.saddr, sizeof(event.saddr), &sk->__sk_common.skc_rcv_saddr);
    bpf_probe_read(&event.daddr, sizeof(event.daddr), &sk->__sk_common.skc_daddr);
    bpf_probe_read(&event.dport, sizeof(event.dport), &sk->__sk_common.skc_dport);
    bpf_probe_read(&event.sport, sizeof(event.sport), &sk->__sk_common.skc_num);

    // Submit event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));

    return 0;
}

char LICENSE[] SEC("license") = "GPL";