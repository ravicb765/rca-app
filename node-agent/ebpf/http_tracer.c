// +build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>

#define MAX_DATA_LEN 100
#define EVENT_TYPE_HTTP_REQUEST  1
#define EVENT_TYPE_HTTP_RESPONSE 2
#define EVENT_TYPE_PG_QUERY      4
#define EVENT_TYPE_PG_RESPONSE   5
#define EVENT_TYPE_REDIS_COMMAND 6
#define EVENT_TYPE_REDIS_RESPONSE 7
#define EVENT_TYPE_MEMCACHED_COMMAND 8 // This was already here, but I'll make sure the implementation is complete.
#define EVENT_TYPE_MEMCACHED_RESPONSE 9 // This was already here, but I'll make sure the implementation is complete.
#define EVENT_TYPE_MYSQL_QUERY 10
#define EVENT_TYPE_MYSQL_RESPONSE 11
#define EVENT_TYPE_MONGO_COMMAND 12
#define EVENT_TYPE_MONGO_RESPONSE 13
#define EVENT_TYPE_MONGO_COMMAND 12
#define EVENT_TYPE_MONGO_RESPONSE 13
#define EVENT_TYPE_RABBITMQ_COMMAND 16
#define EVENT_TYPE_RABBITMQ_RESPONSE 17
#define EVENT_TYPE_CASSANDRA_COMMAND 18
#define EVENT_TYPE_CASSANDRA_RESPONSE 19

struct http_event { // The struct is correct, no changes needed.
    u64 timestamp; 
    u32 pid;
    u32 type;
    u32 saddr;
    u32 daddr;
    u16 sport;
    u16 dport;
    u32 status_code;
    u64 latency;
    u64 msg_len;
    char parent_method[8];
    char parent_path[64];
    char method[8];
    char path[64];
    u32 data_len;
    u8 data[MAX_DATA_LEN];
    char comm[16];
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u32));
} http_events SEC(".maps");

// Map to filter allowed ports (e.g., 80, 8080)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8)); // dummy value
    __uint(max_entries, 64);
} http_ports SEC(".maps");

// Map to filter allowed PostgreSQL ports (e.g., 5432)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} pg_ports SEC(".maps");

// Map to filter allowed CockroachDB ports (e.g., 26257)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} cockroach_ports SEC(".maps");

// Map to filter allowed YugabyteDB ports (e.g., 5433)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} yugabyte_ports SEC(".maps");

// Map to filter allowed Redis ports (e.g., 6379)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} redis_ports SEC(".maps");

// Map to filter allowed Memcached ports (e.g., 11211)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} memcached_ports SEC(".maps");

// Map to filter allowed MySQL ports (e.g., 3306)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} mysql_ports SEC(".maps");

// Map to filter allowed MariaDB ports (e.g., 3306)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} mariadb_ports SEC(".maps");

// Map to filter allowed MongoDB ports (e.g., 27017)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} mongo_ports SEC(".maps");

// I see you have a memcached_ports map already, which is great.
// Map to filter allowed Memcached ports (e.g., 11211)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} memcached_ports SEC(".maps");

// Map to filter allowed Kafka ports (e.g., 9092)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} kafka_ports SEC(".maps");

// Map to filter allowed RabbitMQ ports (e.g., 5672)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} rabbitmq_ports SEC(".maps");

// Map to filter allowed Cassandra ports (e.g., 9042)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} cassandra_ports SEC(".maps");

// Map to filter allowed CockroachDB ports (e.g., 26257)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} cockroach_ports SEC(".maps");

// Map to filter allowed YugabyteDB ports (e.g., 5433)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u16));
    __uint(value_size, sizeof(u8));
    __uint(max_entries, 64);
} yugabyte_ports SEC(".maps");

struct recv_args {
    struct sock *sk;
    struct msghdr *msg;
};

// Map to store msghdr pointer between entry and exit of tcp_recvmsg
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u64)); // pid_tgid
    __uint(value_size, sizeof(struct recv_args));
    __uint(max_entries, 1024);
} active_recv_args SEC(".maps");

// Map to correlate request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} http_req_start SEC(".maps");

// Map to correlate PG request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} pg_req_start SEC(".maps");

// Map to correlate Redis request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} redis_req_start SEC(".maps");

// Map to correlate Memcached request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} memcached_req_start SEC(".maps");

// Map to correlate MySQL request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} mysql_req_start SEC(".maps");

// Map to correlate Mongo request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} mongo_req_start SEC(".maps");

// I see you have a memcached_req_start map already.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} memcached_req_start SEC(".maps");

// Map to correlate Kafka request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} kafka_req_start SEC(".maps");

// Map to correlate RabbitMQ request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} rabbitmq_req_start SEC(".maps");

// Map to correlate Cassandra request start time with response for latency
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(struct sock *));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 10240);
} cassandra_req_start SEC(".maps");

// Map to store SSL buffer pointer for SSL_read (uprobe -> uretprobe)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u64)); // pid_tgid
    __uint(value_size, sizeof(void *)); // buffer pointer
    __uint(max_entries, 1024);
} ssl_read_args SEC(".maps");

// Map to store active HTTP request context per thread for correlation
struct http_req_context {
    char method[8];
    char path[64];
};
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(u64)); // pid_tgid
    __uint(value_size, sizeof(struct http_req_context));
    __uint(max_entries, 10240);
} active_http_context SEC(".maps");

static __always_inline bool check_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&http_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&http_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_pg_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&pg_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&pg_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_redis_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&redis_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&redis_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_memcached_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&memcached_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&memcached_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_mysql_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&mysql_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&mysql_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_mariadb_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&mariadb_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&mariadb_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_mongo_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&mongo_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&mongo_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_memcached_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&memcached_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&memcached_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_kafka_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&kafka_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&kafka_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_rabbitmq_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&rabbitmq_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&rabbitmq_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_cassandra_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&cassandra_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&cassandra_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_cockroach_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&cockroach_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&cockroach_ports, &dport);
    if (val) return true;
    
    return false;
}

static __always_inline bool check_yugabyte_port(struct sock *sk) {
    u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    u16 dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    
    u8 *val = bpf_map_lookup_elem(&yugabyte_ports, &sport);
    if (val) return true;
    
    val = bpf_map_lookup_elem(&yugabyte_ports, &dport);
    if (val) return true;
    
    return false;
}

// Helper to determine HTTP type from buffer
static __always_inline u32 detect_http_type(char *buf) {
    char p[4];
    if (bpf_probe_read_user(&p, sizeof(p), buf) < 0) return 0;

    // Check for "HTTP" (Response)
    if (p[0] == 'H' && p[1] == 'T' && p[2] == 'T' && p[3] == 'P') {
        return EVENT_TYPE_HTTP_RESPONSE;
    }

    // Check for Methods (Request)
    if ((p[0] == 'G' && p[1] == 'E' && p[2] == 'T') ||
        (p[0] == 'P' && p[1] == 'O' && p[2] == 'S' && p[3] == 'T') ||
        (p[0] == 'P' && p[1] == 'U' && p[2] == 'T') ||
        (p[0] == 'D' && p[1] == 'E' && p[2] == 'L' && p[3] == 'E') ||
        (p[0] == 'H' && p[1] == 'E' && p[2] == 'A' && p[3] == 'D')) {
        return EVENT_TYPE_HTTP_REQUEST;
    }
    return 0;
}

static __always_inline void process_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len == 0) return;

    // If type is 0, try to detect it
    if (type == 0) {
        type = detect_http_type(buf);
    }
    if (type == 0) return;

    // Filter out health checks (e.g., "GET /healthz")
    if (type == EVENT_TYPE_HTTP_REQUEST) {
        char uri[12];
        if (bpf_probe_read_user(uri, sizeof(uri), buf) == 0) {
            if (uri[0]=='G' && uri[1]=='E' && uri[2]=='T' && uri[3]==' ' &&
                uri[4]=='/' && uri[5]=='h' && uri[6]=='e' && uri[7]=='a' && 
                uri[8]=='l' && uri[9]=='t' && uri[10]=='h' && uri[11]=='z') {
                return;
            }
        }

        // Filter out specific User-Agents (e.g., "User-Agent: curl")
        // We scan the buffer for the signature.
        #pragma unroll
        for (int i = 0; i < 50; i++) {
            if (i + 16 > len) break;
            char ua[16];
            // Read 16 bytes to check for "User-Agent: curl"
            if (bpf_probe_read_user(ua, sizeof(ua), buf + i) == 0) {
                if (ua[0]=='U' && ua[1]=='s' && ua[2]=='e' && ua[3]=='r' &&
                    ua[12]=='c' && ua[13]=='u' && ua[14]=='r' && ua[15]=='l') {
                    return; // Drop event
                }
            }
        }

        u64 ts = bpf_ktime_get_ns();
        if (sk) {
            bpf_map_update_elem(&http_req_start, &sk, &ts, BPF_ANY);
        }
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();
    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    if (sk) {
        event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
        event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
        event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
        event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    }
    event.msg_len = len;

    // Read data into event buffer early to parse it
    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;
    bpf_probe_read_user(&event.data, copy_len & MAX_DATA_LEN, buf);

    if (type == EVENT_TYPE_HTTP_REQUEST) {
        // Parse Method (copy until space)
        #pragma unroll
        for (int i = 0; i < 7; i++) {
            if (i >= copy_len) break;
            if (event.data[i] == ' ') {
                event.method[i] = 0;
                break;
            }
            event.method[i] = event.data[i];
        }
        event.method[7] = 0;

        // Parse Path (skip method, find next space)
        int start = 0;
        #pragma unroll
        for (int i = 0; i < 8; i++) {
            if (i >= copy_len) break;
            if (event.data[i] == ' ') {
                start = i + 1;
                break;
            }
        }

        if (start > 0 && start < MAX_DATA_LEN) {
            #pragma unroll
            for (int i = 0; i < 63; i++) {
                if (start + i >= copy_len || event.data[start + i] == ' ') {
                    event.path[i] = 0;
                    break;
                }
                event.path[i] = event.data[start + i];
            }
            event.path[63] = 0;
        }

        // Store context for correlation (PID-based)
        u64 pid_tgid = bpf_get_current_pid_tgid();
        struct http_req_context ctx_val = {};
        __builtin_memcpy(ctx_val.method, event.method, sizeof(ctx_val.method));
        __builtin_memcpy(ctx_val.path, event.path, sizeof(ctx_val.path));
        bpf_map_update_elem(&active_http_context, &pid_tgid, &ctx_val, BPF_ANY);
    }

    if (type == EVENT_TYPE_HTTP_RESPONSE) {
        // Parse Status Code: Look for "HTTP/1.x YYY"
        // We scan the first 15 bytes for a space
        #pragma unroll
        for (int i = 0; i < 15; i++) {
            if (i >= len) break;
            char c;
            bpf_probe_read_user(&c, sizeof(c), buf + i);
            if (c == ' ') {
                char code[3];
                if (bpf_probe_read_user(code, sizeof(code), buf + i + 1) == 0) {
                    event.status_code = (code[0]-'0')*100 + (code[1]-'0')*10 + (code[2]-'0');
                }
                break;
            }
        }

        // Calculate Latency
        if (sk) {
            u64 *start_ts = bpf_map_lookup_elem(&http_req_start, &sk);
            if (start_ts) {
                event.latency = event.timestamp - *start_ts;
                bpf_map_delete_elem(&http_req_start, &sk);
            }
            // Clear correlation context on response
            u64 pid_tgid = bpf_get_current_pid_tgid();
            bpf_map_delete_elem(&active_http_context, &pid_tgid);
        }
    }

    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_pg_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len == 0) return;

    // Simple Postgres Query Detection
    // Client Query: 'Q' (byte) + Length (4 bytes) + Query String
    char type_byte;
    bpf_probe_read_user(&type_byte, sizeof(type_byte), buf);

    if (type == EVENT_TYPE_PG_QUERY) {
        if (type_byte != 'Q') return; // Only trace simple queries for now
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&pg_req_start, &sk, &ts, BPF_ANY);
    }
    // For responses, we might see 'T' (RowDescription) or 'D' (DataRow)

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_PG_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&pg_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&pg_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // Skip the 1-byte type and 4-byte length for queries to get to the SQL?
    // For simplicity, just capture the raw buffer starting at 0
    bpf_probe_read_user(&event.data, copy_len & MAX_DATA_LEN, buf);
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_redis_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len == 0) return;

    if (type == EVENT_TYPE_REDIS_COMMAND) {
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&redis_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    if (type == EVENT_TYPE_REDIS_COMMAND) {
        char c;
        int offset = 0;
        
        // Read first char to determine format
        if (bpf_probe_read_user(&c, 1, buf) == 0) {
            if (c == '*') {
                // RESP Array: *<argc>\r\n$<len>\r\n<cmd>...
                // Skip *<argc>\r\n
                #pragma unroll
                for (int i = 0; i < 6; i++) {
                    offset++;
                    if (offset >= MAX_DATA_LEN || offset >= len) break;
                    bpf_probe_read_user(&c, 1, buf + offset);
                    if (c == '\r') {
                        offset += 2; // Skip \r\n
                        break;
                    }
                }
                
                // Check for Bulk String start '$'
                if (offset < len && offset < MAX_DATA_LEN) {
                    bpf_probe_read_user(&c, 1, buf + offset);
                    if (c == '$') {
                         // Skip $<len>\r\n
                        #pragma unroll
                        for (int i = 0; i < 6; i++) {
                            offset++;
                            if (offset >= MAX_DATA_LEN || offset >= len) break;
                            bpf_probe_read_user(&c, 1, buf + offset);
                            if (c == '\r') {
                                offset += 2; // Skip \r\n
                                break;
                            }
                        }
                        
                        // Read Command
                        #pragma unroll
                        for (int i = 0; i < 7; i++) {
                            if (offset + i >= len || offset + i >= MAX_DATA_LEN) break;
                            bpf_probe_read_user(&c, 1, buf + offset + i);
                            if (c == '\r') {
                                event.method[i] = 0;
                                break;
                            }
                            event.method[i] = c;
                        }
                    }
                }
            } else {
                // Inline command: <cmd> ...
                #pragma unroll
                for (int i = 0; i < 7; i++) {
                    if (i >= len || i >= MAX_DATA_LEN) break;
                    bpf_probe_read_user(&c, 1, buf + i);
                    if (c == ' ' || c == '\r') {
                        event.method[i] = 0;
                        break;
                    }
                    event.method[i] = c;
                }
            }
        }
        // Ensure null termination
        event.method[7] = 0;
    }

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_REDIS_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&redis_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&redis_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // Capture raw Redis protocol data (RESP)
    // Userspace can parse "*2\r\n$3\r\nGET..."
    bpf_probe_read_user(&event.data, copy_len & MAX_DATA_LEN, buf);
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_memcached_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len == 0) return;

    if (type == EVENT_TYPE_MEMCACHED_COMMAND) {
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&memcached_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    if (type == EVENT_TYPE_MEMCACHED_COMMAND) {
        // Parse Memcached text protocol: <command> <key> ...
        // e.g., "get mykey\r\n" or "set mykey ...\r\n"
        char c;
        #pragma unroll
        for (int i = 0; i < 7; i++) {
            if (i >= len || i >= MAX_DATA_LEN) break;
            bpf_probe_read_user(&c, 1, buf + i);
            if (c == ' ' || c == '\r') {
                event.method[i] = 0;
                break;
            }
            event.method[i] = c;
        }
        event.method[7] = 0;
    }

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_MEMCACHED_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&memcached_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&memcached_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    event.data_len = 0; // Don't capture data for now to save space
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_memcached_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len == 0) return;

    if (type == EVENT_TYPE_MEMCACHED_COMMAND) {
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&memcached_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    if (type == EVENT_TYPE_MEMCACHED_COMMAND) {
        // Parse Memcached text protocol: <command> <key> ...
        // e.g., "get mykey\r\n" or "set mykey ...\r\n"
        char c;
        #pragma unroll
        for (int i = 0; i < 7; i++) {
            if (i >= len || i >= MAX_DATA_LEN) break;
            bpf_probe_read_user(&c, 1, buf + i);
            if (c == ' ' || c == '\r') {
                event.method[i] = 0;
                break;
            }
            event.method[i] = c;
        }
        event.method[7] = 0;
    }

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_MEMCACHED_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&memcached_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&memcached_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    event.data_len = 0; // Don't capture data for now to save space
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_mysql_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len < 5) return; // Header is 4 bytes + at least 1 byte payload

    // MySQL Packet Header: [3 bytes length][1 byte seq]
    // Payload starts at offset 4.
    // COM_QUERY is 0x03.

    if (type == EVENT_TYPE_MYSQL_QUERY) {
        u8 cmd;
        if (bpf_probe_read_user(&cmd, 1, buf + 4) == 0) {
            if (cmd != 0x03) return; // Only trace COM_QUERY
        } else {
            return;
        }
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&mysql_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_MYSQL_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&mysql_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&mysql_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // Capture payload starting from offset 5 (skip header + cmd byte) for queries
    // For responses, just capture raw
    int offset = (type == EVENT_TYPE_MYSQL_QUERY) ? 5 : 0;
    if (offset >= copy_len) offset = 0;
    
    bpf_probe_read_user(&event.data, (copy_len - offset) & MAX_DATA_LEN, buf + offset);
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_mongo_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len < 16) return; // Header is 16 bytes

    // MongoDB Wire Protocol Header
    // int32   messageLength;
    // int32   messageLength;
    // int32   requestID;
    // int32   responseTo;
    // int32   opCode;

    if (type == EVENT_TYPE_MONGO_COMMAND) {
        u32 opcode;
        if (bpf_probe_read_user(&opcode, 4, buf + 12) == 0) {
            // OP_MSG = 2013, OP_QUERY = 2004
            if (opcode != 2013 && opcode != 2004) return;
        } else {
            return;
        }
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&mongo_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_MONGO_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&mongo_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&mongo_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // For OP_MSG (2013), payload starts at 16 + 4 (flagBits) = 20.
    // Section 0 (Body) starts with kind (1 byte) = 0.
    // Then BSON document. BSON starts with size (4 bytes).
    // Then first element type (1 byte) + e_name (cstring). The first key is usually the command.
    // Offset: 16 (header) + 4 (flags) + 1 (kind) + 4 (bson size) + 1 (type) = 26.
    
    if (type == EVENT_TYPE_MONGO_COMMAND) {
        int offset = 26;
        // Check bounds
        if (offset < len && offset < MAX_DATA_LEN) {
            // Read command name (cstring)
            #pragma unroll
            for (int i = 0; i < 7; i++) {
                if (offset + i >= len || offset + i >= MAX_DATA_LEN) break;
                char c;
                bpf_probe_read_user(&c, 1, buf + offset + i);
                if (c == 0) {
                    event.method[i] = 0;
                    break;
                }
                event.method[i] = c;
            }
            event.method[7] = 0;
        }
        // Capture raw payload for debugging/full parsing in userspace
        int data_offset = 16;
        if (data_offset >= copy_len) data_offset = 0;
        bpf_probe_read_user(&event.data, (copy_len - data_offset) & MAX_DATA_LEN, buf + data_offset);
    } else {
        bpf_probe_read_user(&event.data, copy_len & MAX_DATA_LEN, buf);
    }

    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_kafka_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len < 6) return; // Size (4) + ApiKey (2)

    if (type == EVENT_TYPE_KAFKA_COMMAND) {
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&kafka_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    if (type == EVENT_TYPE_KAFKA_COMMAND) {
        // Kafka Request Header: Size (4), ApiKey (2), ApiVersion (2), ...
        // Read ApiKey (at offset 4) and store it in status_code field for userspace.
        s16 api_key;
        if (bpf_probe_read_user(&api_key, sizeof(api_key), buf + 4) == 0) {
            event.status_code = bpf_ntohs(api_key);
        }
    }

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_KAFKA_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&kafka_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&kafka_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // Capture raw payload for now
    bpf_probe_read_user(&event.data, copy_len & MAX_DATA_LEN, buf);
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_rabbitmq_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len < 8) return; // Frame header is 7 bytes + at least 1 byte payload

    // AMQP 0-9-1 Frame Header:
    // Type (1 byte), Channel (2 bytes), Size (4 bytes)
    // Payload starts at offset 7.
    // Method Frame (Type 1): Class ID (2 bytes), Method ID (2 bytes)

    if (type == EVENT_TYPE_RABBITMQ_COMMAND) {
        u8 frame_type;
        if (bpf_probe_read_user(&frame_type, 1, buf) == 0) {
            if (frame_type != 1) return; // Only trace Method frames
        } else {
            return;
        }
        u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&rabbitmq_req_start, &sk, &ts, BPF_ANY);
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_RABBITMQ_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&rabbitmq_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&rabbitmq_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // Capture payload starting from offset 7 (skip header)
    int offset = 7;
    if (offset >= copy_len) offset = 0;
    bpf_probe_read_user(&event.data, (copy_len - offset) & MAX_DATA_LEN, buf + offset);
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

static __always_inline void process_cassandra_data(struct pt_regs *ctx, struct sock *sk, u32 type, char *buf, u32 len) {
    if (len < 9) return; // Header is 9 bytes

    // CQL Frame Header:
    // Version (1), Flags (1), Stream (2), Opcode (1), Length (4)
    // Opcode 0x07 = QUERY

    if (type == EVENT_TYPE_CASSANDRA_COMMAND) {
        u8 version, opcode;
        bpf_probe_read_user(&version, 1, buf);
        bpf_probe_read_user(&opcode, 1, buf + 4);
        
        // Request versions 0x03, 0x04, 0x05. Opcode 0x07 is QUERY.
        if ((version & 0x7F) >= 3 && opcode == 0x07) {
             u64 ts = bpf_ktime_get_ns();
             bpf_map_update_elem(&cassandra_req_start, &sk, &ts, BPF_ANY);
        } else {
            return; 
        }
    }

    struct http_event event = {};
    event.timestamp = bpf_ktime_get_ns();

    // Correlate with active HTTP request
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct http_req_context *ctx_val = bpf_map_lookup_elem(&active_http_context, &pid_tgid);
    if (ctx_val) {
        __builtin_memcpy(event.parent_method, ctx_val->method, sizeof(event.parent_method));
        __builtin_memcpy(event.parent_path, ctx_val->path, sizeof(event.parent_path));
    }

    if (type == EVENT_TYPE_CASSANDRA_RESPONSE) {
        u64 *start_ts = bpf_map_lookup_elem(&cassandra_req_start, &sk);
        if (start_ts) {
            event.latency = event.timestamp - *start_ts;
            bpf_map_delete_elem(&cassandra_req_start, &sk);
        }
    }

    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = type;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    event.saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event.daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    event.sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    event.dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event.msg_len = len;

    u32 copy_len = len;
    if (copy_len > MAX_DATA_LEN) copy_len = MAX_DATA_LEN;
    event.data_len = copy_len;

    // Capture payload starting from offset 9 (skip header)
    // For QUERY, body starts with [long string] query. [long string] is [int] len + bytes.
    // We skip the 4 bytes length of the string to get to the query text at offset 13.
    int offset = (type == EVENT_TYPE_CASSANDRA_COMMAND) ? 13 : 9;
    if (offset >= copy_len) offset = 0;
    bpf_probe_read_user(&event.data, (copy_len - offset) & MAX_DATA_LEN, buf + offset);
    bpf_perf_event_output(ctx, &http_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
}

// Hook: tcp_sendmsg(struct sock *sk, struct msghdr *msg, size_t size)
SEC("kprobe/tcp_sendmsg")
int kprobe_tcp_sendmsg(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);
    size_t size = (size_t)PT_REGS_PARM3(ctx);

    // Extract iov from msghdr
    struct iovec *iov = BPF_CORE_READ(msg, msg_iter.iov);
    char *buf = BPF_CORE_READ_USER(iov, iov_base);

    if (check_pg_port(sk)) {
        process_pg_data(ctx, sk, EVENT_TYPE_PG_QUERY, buf, size);
        return 0;
    }

    if (check_redis_port(sk)) {
        process_redis_data(ctx, sk, EVENT_TYPE_REDIS_COMMAND, buf, size);
        return 0;
    }

    if (check_memcached_port(sk)) {
        process_memcached_data(ctx, sk, EVENT_TYPE_MEMCACHED_COMMAND, buf, size);
        return 0;
    }

    if (check_mysql_port(sk)) {
        process_mysql_data(ctx, sk, EVENT_TYPE_MYSQL_QUERY, buf, size);
        return 0;
    }

    if (check_mariadb_port(sk)) {
        process_mysql_data(ctx, sk, EVENT_TYPE_MYSQL_QUERY, buf, size);
        return 0;
    }

    if (check_mongo_port(sk)) {
        process_mongo_data(ctx, sk, EVENT_TYPE_MONGO_COMMAND, buf, size);
        return 0;
    }
    // In kprobe_tcp_sendmsg...
    if (check_cockroach_port(sk)) {
    process_pg_data(ctx, sk, EVENT_TYPE_PG_QUERY, buf, size);
    return 0;
    }

    if (check_yugabyte_port(sk)) {
    process_pg_data(ctx, sk, EVENT_TYPE_PG_QUERY, buf, size);
    return 0;
    }

    if (check_memcached_port(sk)) {
        process_memcached_data(ctx, sk, EVENT_TYPE_MEMCACHED_COMMAND, buf, size);
        return 0;
    }

    if (check_kafka_port(sk)) {
        process_kafka_data(ctx, sk, EVENT_TYPE_KAFKA_COMMAND, buf, size);
        return 0;
    }

    if (check_rabbitmq_port(sk)) {
        process_rabbitmq_data(ctx, sk, EVENT_TYPE_RABBITMQ_COMMAND, buf, size);
        return 0;
    }

    if (check_cassandra_port(sk)) {
        process_cassandra_data(ctx, sk, EVENT_TYPE_CASSANDRA_COMMAND, buf, size);
        return 0;
    }

    if (check_cockroach_port(sk)) {
        process_pg_data(ctx, sk, EVENT_TYPE_PG_QUERY, buf, size);
        return 0;
    }

    if (check_yugabyte_port(sk)) {
        process_pg_data(ctx, sk, EVENT_TYPE_PG_QUERY, buf, size);
        return 0;
    }

    if (!check_port(sk)) return 0;

    process_data(ctx, sk, 0, buf, size); // 0 = auto-detect type
    return 0;
}

// Hook: tcp_recvmsg(struct sock *sk, struct msghdr *msg, size_t len, ...)
SEC("kprobe/tcp_recvmsg")
int kprobe_tcp_recvmsg(struct pt_regs *ctx) {
    u64 id = bpf_get_current_pid_tgid();
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);

    // Filter by port (HTTP or PG)
    if (!check_port(sk) && !check_pg_port(sk) && !check_redis_port(sk) && !check_memcached_port(sk) && !check_mysql_port(sk) && !check_mariadb_port(sk) && !check_mongo_port(sk) && !check_kafka_port(sk) && !check_rabbitmq_port(sk) && !check_cassandra_port(sk) && !check_cockroach_port(sk) && !check_yugabyte_port(sk)) return 0;

    struct recv_args args = { .sk = sk, .msg = msg };
    bpf_map_update_elem(&active_recv_args, &id, &args, BPF_ANY);
    return 0;
}

// Hook: Return of tcp_recvmsg
SEC("kretprobe/tcp_recvmsg")
int kretprobe_tcp_recvmsg(struct pt_regs *ctx) {
    u64 id = bpf_get_current_pid_tgid();
    struct recv_args *args;

    args = bpf_map_lookup_elem(&active_recv_args, &id);
    if (!args) return 0;

    struct msghdr *msg = args->msg;
    struct sock *sk = args->sk;
    int ret = PT_REGS_RC(ctx);

    if (ret > 0) {
        // Extract iov from msghdr (user buffer is now filled)
        struct iovec *iov = BPF_CORE_READ(msg, msg_iter.iov);
        char *buf = BPF_CORE_READ_USER(iov, iov_base);
        
        if (check_pg_port(sk)) {
            process_pg_data(ctx, sk, EVENT_TYPE_PG_RESPONSE, buf, ret);
        } else if (check_redis_port(sk)) {
            process_redis_data(ctx, sk, EVENT_TYPE_REDIS_RESPONSE, buf, ret);
        } else if (check_memcached_port(sk)) {
            process_memcached_data(ctx, sk, EVENT_TYPE_MEMCACHED_RESPONSE, buf, ret);
        } else if (check_mysql_port(sk)) {
            process_mysql_data(ctx, sk, EVENT_TYPE_MYSQL_RESPONSE, buf, ret);
        } else if (check_mariadb_port(sk)) {
            process_mysql_data(ctx, sk, EVENT_TYPE_MYSQL_RESPONSE, buf, ret);
        } else if (check_kafka_port(sk)) {
            process_kafka_data(ctx, sk, EVENT_TYPE_KAFKA_RESPONSE, buf, ret);
        } else if (check_mongo_port(sk)) {
            process_mongo_data(ctx, sk, EVENT_TYPE_MONGO_RESPONSE, buf, ret);
        } else if (check_rabbitmq_port(sk)) {
            process_rabbitmq_data(ctx, sk, EVENT_TYPE_RABBITMQ_RESPONSE, buf, ret);
        } else if (check_cassandra_port(sk)) {
            process_cassandra_data(ctx, sk, EVENT_TYPE_CASSANDRA_RESPONSE, buf, ret);
        } else if (check_cockroach_port(sk)) {
            process_pg_data(ctx, sk, EVENT_TYPE_PG_RESPONSE, buf, ret);
        } else if (check_yugabyte_port(sk)) {
            process_pg_data(ctx, sk, EVENT_TYPE_PG_RESPONSE, buf, ret);
        } else {
            process_data(ctx, sk, 0, buf, ret); // 0 = auto-detect type
        }
    }

    bpf_map_delete_elem(&active_recv_args, &id);
    return 0;
}

// Hook: tcp_close to clean up maps
SEC("kprobe/tcp_close")
int kprobe_tcp_close(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    bpf_map_delete_elem(&http_req_start, &sk);
    bpf_map_delete_elem(&pg_req_start, &sk);
    bpf_map_delete_elem(&redis_req_start, &sk);
    bpf_map_delete_elem(&memcached_req_start, &sk);
    bpf_map_delete_elem(&mysql_req_start, &sk);
    bpf_map_delete_elem(&kafka_req_start, &sk);
    bpf_map_delete_elem(&mongo_req_start, &sk);
    bpf_map_delete_elem(&rabbitmq_req_start, &sk);
    bpf_map_delete_elem(&cassandra_req_start, &sk);
    // Note: We don't clear active_http_context here because it's PID-bound, not socket-bound,
    // and usually cleared on HTTP Response.
    return 0;
}

// --- OpenSSL Uprobes ---

// SSL_write(SSL *ssl, const void *buf, int num)
SEC("uprobe/SSL_write")
int probe_ssl_write(struct pt_regs *ctx) {
    u64 id = bpf_get_current_pid_tgid();
    void *buf = (void *)PT_REGS_PARM2(ctx);
    int len = (int)PT_REGS_PARM3(ctx);

    // Pass NULL as sk; process_data handles it by skipping IP/Port extraction
    process_data(ctx, NULL, 0, buf, len);
    return 0;
}

// SSL_read(SSL *ssl, void *buf, int num)
SEC("uprobe/SSL_read")
int probe_ssl_read(struct pt_regs *ctx) {
    u64 id = bpf_get_current_pid_tgid();
    void *buf = (void *)PT_REGS_PARM2(ctx);
    
    // Store buffer pointer to read on return
    bpf_map_update_elem(&ssl_read_args, &id, &buf, BPF_ANY);
    return 0;
}

SEC("uretprobe/SSL_read")
int retprobe_ssl_read(struct pt_regs *ctx) {
    u64 id = bpf_get_current_pid_tgid();
    void **bufp = bpf_map_lookup_elem(&ssl_read_args, &id);
    if (!bufp) return 0;
    
    int ret = PT_REGS_RC(ctx);
    if (ret > 0) {
        process_data(ctx, NULL, 0, *bufp, ret);
    }

    // If ret > 0, data is in *bufp
    bpf_map_delete_elem(&ssl_read_args, &id);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";