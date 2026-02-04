package ebpf

import (
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
)

// Event types matching C code
const (
	EventTypeConnect = 1
	EventTypeAccept  = 2
	EventTypeClose   = 3
)

const (
	EventTypeHttpRequest       = 1
	EventTypeHttpResponse      = 2
	EventTypePgQuery           = 4
	EventTypePgResponse        = 5
	EventTypeRedisCommand      = 6
	EventTypeRedisResponse     = 7
	EventTypeMemcachedCommand  = 8
	EventTypeMemcachedResponse = 9
	EventTypeMysqlQuery        = 10
	EventTypeMysqlResponse     = 11
	EventTypeMongoCommand      = 12
	EventTypeMongoResponse     = 13
	EventTypeKafkaCommand      = 14
	EventTypeKafkaResponse     = 15
	EventTypeRabbitMQCommand   = 16
	EventTypeRabbitMQResponse  = 17
	EventTypeCassandraCommand  = 18
	EventTypeCassandraResponse = 19
	MaxHttpDataLen             = 100
)

// ConnEvent matches the C struct layout
type ConnEvent struct {
	Timestamp uint64
	Saddr     uint32
	Daddr     uint32
	Pid       uint32
	Type      uint32
	Sport     uint16
	Dport     uint16
	Comm      [16]byte
	_pad      uint32
}

// HttpEvent matches the C struct layout in http_tracer.c
type HttpEvent struct {
	Timestamp    uint64
	Pid          uint32
	Type         uint32
	Saddr        uint32
	Daddr        uint32
	Sport        uint16
	Dport        uint16
	StatusCode   uint32
	Latency      uint64
	MsgLen       uint64
	ParentMethod [8]byte
	ParentPath   [64]byte
	Method       [8]byte
	Path         [64]byte
	DataLen      uint32
	Data         [MaxHttpDataLen]uint8
	Comm         [16]byte
}

// Manager handles the eBPF lifecycle
type Manager struct {
	netObjs    *NetworkTracerObjects
	httpObjs   *HttpTracerObjects
	links      []link.Link
	netReader  *perf.Reader
	httpReader *perf.Reader
}

// LoadAndAttach loads the eBPF program and attaches kprobes
func LoadAndAttach() (*Manager, error) {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock limit: %w", err)
	}

	objs := NetworkTracerObjects{}
	if err := LoadNetworkTracerObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("loading network objects: %w", err)
	}

	httpObjs := HttpTracerObjects{}
	if err := LoadHttpTracerObjects(&httpObjs, nil); err != nil {
		return nil, fmt.Errorf("loading http objects: %w", err)
	}

	m := &Manager{netObjs: &objs, httpObjs: &httpObjs}

	// --- Attach Network Tracer Kprobes ---
	// tcp_connect
	kp, err := link.Kprobe("tcp_connect", objs.KprobeTcpConnect, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kprobe/tcp_connect: %w", err)
	}
	m.links = append(m.links, kp)

	krp, err := link.Kretprobe("tcp_connect", objs.KretprobeTcpConnect, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kretprobe/tcp_connect: %w", err)
	}
	m.links = append(m.links, krp)

	// inet_csk_accept
	krpAccept, err := link.Kretprobe("inet_csk_accept", objs.KretprobeInetCskAccept, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kretprobe/inet_csk_accept: %w", err)
	}
	m.links = append(m.links, krpAccept)

	// tcp_close
	kpClose, err := link.Kprobe("tcp_close", objs.KprobeTcpClose, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kprobe/tcp_close: %w", err)
	}
	m.links = append(m.links, kpClose)

	// --- Attach HTTP Tracer Kprobes ---
	// tcp_sendmsg
	kpSend, err := link.Kprobe("tcp_sendmsg", httpObjs.KprobeTcpSendmsg, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kprobe/tcp_sendmsg: %w", err)
	}
	m.links = append(m.links, kpSend)

	// tcp_recvmsg
	kpRecv, err := link.Kprobe("tcp_recvmsg", httpObjs.KprobeTcpRecvmsg, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kprobe/tcp_recvmsg: %w", err)
	}
	m.links = append(m.links, kpRecv)

	krpRecv, err := link.Kretprobe("tcp_recvmsg", httpObjs.KretprobeTcpRecvmsg, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kretprobe/tcp_recvmsg: %w", err)
	}
	m.links = append(m.links, krpRecv)

	// tcp_close (cleanup for http)
	kpHttpClose, err := link.Kprobe("tcp_close", httpObjs.KprobeTcpClose, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching kprobe/tcp_close (http): %w", err)
	}
	m.links = append(m.links, kpHttpClose)

	// --- Create perf readers ---
	rd, err := perf.NewReader(objs.Events, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating network perf reader: %w", err)
	}
	m.netReader = rd

	httpRd, err := perf.NewReader(httpObjs.HttpEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating http perf reader: %w", err)
	}
	m.httpReader = httpRd

	return m, nil
}

func (m *Manager) Close() {
	if m.netReader != nil {
		m.netReader.Close()
	}
	if m.httpReader != nil {
		m.httpReader.Close()
	}
	for _, l := range m.links {
		l.Close()
	}
	if m.netObjs != nil {
		m.netObjs.Close()
	}
	if m.httpObjs != nil {
		m.httpObjs.Close()
	}
}

func (m *Manager) NetReader() *perf.Reader {
	return m.netReader
}

func (m *Manager) HttpReader() *perf.Reader {
	return m.httpReader
}

// AddHttpPort adds a port to the HTTP tracing filter
func (m *Manager) AddHttpPort(port uint16) error {
	// The map name in C is http_ports, so bpf2go generates HttpPorts in the objects struct
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.HttpPorts.Put(port, val)
}

// AddPgPort adds a port to the PostgreSQL tracing filter
func (m *Manager) AddPgPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.PgPorts.Put(port, val)
}

// AddRedisPort adds a port to the Redis tracing filter
func (m *Manager) AddRedisPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.RedisPorts.Put(port, val)
}

// AddMemcachedPort adds a port to the Memcached tracing filter
func (m *Manager) AddMemcachedPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.MemcachedPorts.Put(port, val)
}

// AddMysqlPort adds a port to the MySQL tracing filter
func (m *Manager) AddMysqlPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.MysqlPorts.Put(port, val)
}

// AddMariaDBPort adds a port to the MariaDB tracing filter
func (m *Manager) AddMariaDBPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.MariadbPorts.Put(port, val)
}

// AddMongoPort adds a port to the MongoDB tracing filter
func (m *Manager) AddMongoPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.MongoPorts.Put(port, val)
}

// AddKafkaPort adds a port to the Kafka tracing filter
func (m *Manager) AddKafkaPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.KafkaPorts.Put(port, val)
}

// AddRabbitMQPort adds a port to the RabbitMQ tracing filter
func (m *Manager) AddRabbitMQPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.RabbitmqPorts.Put(port, val)
}

// AddCassandraPort adds a port to the Cassandra tracing filter
func (m *Manager) AddCassandraPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.CassandraPorts.Put(port, val)
}

// AddCockroachDBPort adds a port to the CockroachDB tracing filter
func (m *Manager) AddCockroachDBPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.CockroachPorts.Put(port, val)
}

// AddYugabyteDBPort adds a port to the YugabyteDB tracing filter
func (m *Manager) AddYugabyteDBPort(port uint16) error {
	if m.httpObjs == nil {
		return fmt.Errorf("http tracer not loaded")
	}
	var val uint8 = 1
	return m.httpObjs.YugabytePorts.Put(port, val)
}

// AttachSSL attaches uprobes to the specified OpenSSL library path
func (m *Manager) AttachSSL(libPath string) error {
	// SSL_write
	up, err := link.OpenExecutable(libPath)
	if err != nil {
		return err
	}

	sslWrite, err := up.Uprobe("SSL_write", m.httpObjs.ProbeSslWrite, nil)
	if err != nil {
		return fmt.Errorf("attaching uprobe/SSL_write: %w", err)
	}
	m.links = append(m.links, sslWrite)

	// Note: SSL_read hooks would be attached similarly
	return nil
}
