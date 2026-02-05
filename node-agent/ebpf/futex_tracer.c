// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define FUTEX_WAIT 0

struct futex_event {
	__u32 pid;
	__u64 duration_ns;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u64));
	__uint(max_entries, 10240);
} futex_start SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} futex_events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_futex")
int trace_enter_futex(struct trace_event_raw_sys_enter *ctx) {
	// args: uaddr, op, val, timeout, uaddr2, val3
	int op = (int)ctx->args[1];
	// Mask out private/clock bits to check basic op
	if ((op & 127) != FUTEX_WAIT) return 0;

	__u32 pid = bpf_get_current_pid_tgid() >> 32;
	__u64 ts = bpf_ktime_get_ns();
	bpf_map_update_elem(&futex_start, &pid, &ts, BPF_ANY);
	return 0;
}

SEC("tracepoint/syscalls/sys_exit_futex")
int trace_exit_futex(struct trace_event_raw_sys_exit *ctx) {
	__u32 pid = bpf_get_current_pid_tgid() >> 32;
	__u64 *tsp = bpf_map_lookup_elem(&futex_start, &pid);
	if (!tsp) return 0;

	struct futex_event event = {};
	event.pid = pid;
	event.duration_ns = bpf_ktime_get_ns() - *tsp;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	bpf_perf_event_output(ctx, &futex_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	bpf_map_delete_elem(&futex_start, &pid);

	return 0;
}

char LICENSE[] SEC("license") = "GPL";