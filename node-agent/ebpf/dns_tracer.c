// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>

struct dns_event {
	__u32 pid;
	__u64 latency_ns;
	__u8 comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} dns_events SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(key_size, sizeof(__u64)); // key: pid << 32 | txid
	__uint(value_size, sizeof(__u64)); // value: timestamp
	__uint(max_entries, 10240);
} dns_queries SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(key_size, sizeof(__u32)); // key: pid
	__uint(value_size, sizeof(__u64)); // value: buf ptr
	__uint(max_entries, 10240);
} active_recv_buf SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_sendto")
int trace_enter_sendto(struct trace_event_raw_sys_enter *ctx) {
	// args: fd, buf, len, flags, dest_addr, addrlen
	struct sockaddr_in addr = {};
	void *dest_addr_ptr = (void *)ctx->args[4];
	if (!dest_addr_ptr) return 0;

	bpf_probe_read_user(&addr, sizeof(addr), dest_addr_ptr);
	if (addr.sin_family != AF_INET) return 0; // Only IPv4 for simplicity
	if (bpf_ntohs(addr.sin_port) != 53) return 0;

	void *buf_ptr = (void *)ctx->args[1];
	if (!buf_ptr) return 0;

	__u16 txid = 0;
	bpf_probe_read_user(&txid, sizeof(txid), buf_ptr);

	__u32 pid = bpf_get_current_pid_tgid() >> 32;
	__u64 key = ((__u64)pid << 32) | txid;
	__u64 ts = bpf_ktime_get_ns();

	bpf_map_update_elem(&dns_queries, &key, &ts, BPF_ANY);
	return 0;
}

SEC("tracepoint/syscalls/sys_enter_recvfrom")
int trace_enter_recvfrom(struct trace_event_raw_sys_enter *ctx) {
	// args: fd, buf, len, flags, src_addr, addrlen
	__u32 pid = bpf_get_current_pid_tgid() >> 32;
	__u64 buf_ptr = ctx->args[1];
	bpf_map_update_elem(&active_recv_buf, &pid, &buf_ptr, BPF_ANY);
	return 0;
}

SEC("tracepoint/syscalls/sys_exit_recvfrom")
int trace_exit_recvfrom(struct trace_event_raw_sys_exit *ctx) {
	__u32 pid = bpf_get_current_pid_tgid() >> 32;
	__u64 *buf_ptr_ptr = bpf_map_lookup_elem(&active_recv_buf, &pid);
	if (!buf_ptr_ptr) return 0;

	void *buf_ptr = (void *)*buf_ptr_ptr;
	bpf_map_delete_elem(&active_recv_buf, &pid);

	if (ctx->ret <= 0) return 0; // Error or no data

	__u16 txid = 0;
	bpf_probe_read_user(&txid, sizeof(txid), buf_ptr);

	__u64 key = ((__u64)pid << 32) | txid;
	__u64 *start_ts = bpf_map_lookup_elem(&dns_queries, &key);
	if (!start_ts) return 0;

	struct dns_event event = {};
	event.pid = pid;
	event.latency_ns = bpf_ktime_get_ns() - *start_ts;
	bpf_get_current_comm(&event.comm, sizeof(event.comm));

	bpf_perf_event_output(ctx, &dns_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
	bpf_map_delete_elem(&dns_queries, &key);

	return 0;
}

char LICENSE[] SEC("license") = "GPL";