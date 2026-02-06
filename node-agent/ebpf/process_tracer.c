// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

struct process_event {
	__u32 pid;
	__u32 ppid;
	__u8 comm[16];
	__u8 filename[256];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} process_events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_execve")
int trace_enter_execve(struct trace_event_raw_sys_enter *ctx) {
	struct process_event event = {};
	struct task_struct *task = (struct task_struct *)bpf_get_current_task();

	event.pid = bpf_get_current_pid_tgid() >> 32;
	event.ppid = BPF_CORE_READ(task, real_parent, tgid);
	bpf_get_current_comm(&event.comm, sizeof(event.comm));
	bpf_probe_read_user_str(&event.filename, sizeof(event.filename), (const char *)ctx->args[0]);

	bpf_perf_event_output(ctx, &process_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

char LICENSE[] SEC("license") = "GPL";