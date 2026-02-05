// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct runq_event {
	__u32 pid;
	__u64 latency_ns;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u64));
	__uint(max_entries, 10240);
} enqueue_time SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} runq_events SEC(".maps");

static __always_inline int trace_enqueue(u32 pid) {
	u64 ts = bpf_ktime_get_ns();
	bpf_map_update_elem(&enqueue_time, &pid, &ts, BPF_ANY);
	return 0;
}

SEC("tracepoint/sched/sched_wakeup")
int trace_sched_wakeup(struct trace_event_raw_sched_wakeup_template *ctx) {
	return trace_enqueue(ctx->pid);
}

SEC("tracepoint/sched/sched_wakeup_new")
int trace_sched_wakeup_new(struct trace_event_raw_sched_wakeup_template *ctx) {
	return trace_enqueue(ctx->pid);
}

SEC("tracepoint/sched/sched_switch")
int trace_sched_switch(struct trace_event_raw_sched_switch *ctx) {
	u32 pid = ctx->next_pid;
	u64 *tsp = bpf_map_lookup_elem(&enqueue_time, &pid);
	if (!tsp) return 0;

	struct runq_event event = {};
	event.pid = pid;
	event.latency_ns = bpf_ktime_get_ns() - *tsp;
	bpf_probe_read_kernel_str(&event.comm, sizeof(event.comm), ctx->next_comm);

	bpf_perf_event_output(ctx, &runq_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	bpf_map_delete_elem(&enqueue_time, &pid);
	return 0;
}

char LICENSE[] SEC("license") = "GPL";