// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

struct file_event {
	__u32 pid;
	__u8 comm[16];
	__u8 filename[256];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} file_events SEC(".maps");

static __always_inline int trace_open_common(void *ctx, const char *filename) {
	struct file_event event = {};

	event.pid = bpf_get_current_pid_tgid() >> 32;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));
	bpf_probe_read_user_str(&event.filename, sizeof(event.filename), filename);

	bpf_perf_event_output(ctx, &file_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	return 0;
}

SEC("tracepoint/syscalls/sys_enter_open")
int trace_enter_open(struct trace_event_raw_sys_enter *ctx) {
	return trace_open_common(ctx, (const char *)ctx->args[0]);
}

SEC("tracepoint/syscalls/sys_enter_openat")
int trace_enter_openat(struct trace_event_raw_sys_enter *ctx) {
	return trace_open_common(ctx, (const char *)ctx->args[1]);
}

char LICENSE[] SEC("license") = "GPL";