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

// OomEvent matches the C struct layout
type OomEvent struct {
	CgroupId uint64
	Pid      uint32
	Comm     [16]byte
}

// ProcessEvent matches the C struct layout
type ProcessEvent struct {
	Pid      uint32
	Ppid     uint32
	Comm     [16]byte
	Filename [256]byte
}

// FileEvent matches the C struct layout
type FileEvent struct {
	Pid      uint32
	Comm     [16]byte
	Filename [256]byte
}

// TcpRetransEvent matches the C struct layout
type TcpRetransEvent struct {
	Saddr uint32
	Daddr uint32
	Sport uint16
	Dport uint16
	Pid   uint32
	Comm  [16]byte
}

// DnsEvent matches the C struct layout
type DnsEvent struct {
	Pid       uint32
	LatencyNs uint64
	Comm      [16]byte
}

// DropEvent matches the C struct layout
type DropEvent struct {
	Pid      uint32
	Location uint64
	Protocol uint32
	Comm     [16]byte
}

// ProcessExitEvent matches the C struct layout
type ProcessExitEvent struct {
	Pid  uint32
	Tgid uint32
	Comm [16]byte
}

// UdpEvent matches the C struct layout
type UdpEvent struct {
	Pid       uint32
	Len       uint32
	Comm      [16]byte
	Direction uint8
}

// PageFaultEvent matches the C struct layout
type PageFaultEvent struct {
	Pid     uint32
	Address uint64
	Ip      uint64
	Comm    [16]byte
}

// Manager handles the eBPF lifecycle
type Manager struct {
	netObjs           *NetworkTracerObjects
	httpObjs          *HttpTracerObjects
	oomObjs           *OomTracerObjects
	processObjs       *ProcessTracerObjects
	fileObjs          *FileTracerObjects
	tcpRetransObjs    *TcpRetransTracerObjects
	dnsObjs           *DnsTracerObjects
	kfreeSkbObjs      *KfreeSkbTracerObjects
	processExitObjs   *ProcessExitTracerObjects
	udpObjs           *UdpTracerObjects
	pageFaultObjs     *PageFaultTracerObjects
	links             []link.Link
	netReader         *perf.Reader
	httpReader        *perf.Reader
	oomReader         *perf.Reader
	processReader     *perf.Reader
	fileReader        *perf.Reader
	tcpRetransReader  *perf.Reader
	dnsReader         *perf.Reader
	kfreeSkbReader    *perf.Reader
	processExitReader *perf.Reader
	udpReader         *perf.Reader
	pageFaultReader   *perf.Reader
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

	oomObjs := OomTracerObjects{}
	if err := LoadOomTracerObjects(&oomObjs, nil); err != nil {
		return nil, fmt.Errorf("loading oom objects: %w", err)
	}

	processObjs := ProcessTracerObjects{}
	if err := LoadProcessTracerObjects(&processObjs, nil); err != nil {
		return nil, fmt.Errorf("loading process objects: %w", err)
	}

	fileObjs := FileTracerObjects{}
	if err := LoadFileTracerObjects(&fileObjs, nil); err != nil {
		return nil, fmt.Errorf("loading file objects: %w", err)
	}

	tcpRetransObjs := TcpRetransTracerObjects{}
	if err := LoadTcpRetransTracerObjects(&tcpRetransObjs, nil); err != nil {
		return nil, fmt.Errorf("loading tcp retrans objects: %w", err)
	}

	dnsObjs := DnsTracerObjects{}
	if err := LoadDnsTracerObjects(&dnsObjs, nil); err != nil {
		return nil, fmt.Errorf("loading dns objects: %w", err)
	}

	kfreeSkbObjs := KfreeSkbTracerObjects{}
	if err := LoadKfreeSkbTracerObjects(&kfreeSkbObjs, nil); err != nil {
		return nil, fmt.Errorf("loading kfree_skb objects: %w", err)
	}

	processExitObjs := ProcessExitTracerObjects{}
	if err := LoadProcessExitTracerObjects(&processExitObjs, nil); err != nil {
		return nil, fmt.Errorf("loading process_exit objects: %w", err)
	}

	udpObjs := UdpTracerObjects{}
	if err := LoadUdpTracerObjects(&udpObjs, nil); err != nil {
		return nil, fmt.Errorf("loading udp objects: %w", err)
	}

	pageFaultObjs := PageFaultTracerObjects{}
	if err := LoadPageFaultTracerObjects(&pageFaultObjs, nil); err != nil {
		return nil, fmt.Errorf("loading page_fault objects: %w", err)
	}

	m := &Manager{netObjs: &objs, httpObjs: &httpObjs, oomObjs: &oomObjs, processObjs: &processObjs, fileObjs: &fileObjs, tcpRetransObjs: &tcpRetransObjs, dnsObjs: &dnsObjs, kfreeSkbObjs: &kfreeSkbObjs, processExitObjs: &processExitObjs, udpObjs: &udpObjs, pageFaultObjs: &pageFaultObjs}

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

	// --- Attach OOM Tracer Tracepoints ---
	// oom/mark_victim
	tpOom, err := link.Tracepoint("oom", "mark_victim", oomObjs.TraceMarkVictim, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/oom/mark_victim: %w", err)
	}
	m.links = append(m.links, tpOom)

	// --- Attach Process Tracer Tracepoints ---
	// syscalls/sys_enter_execve
	tpExec, err := link.Tracepoint("syscalls", "sys_enter_execve", processObjs.TraceEnterExecve, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/syscalls/sys_enter_execve: %w", err)
	}
	m.links = append(m.links, tpExec)

	// --- Attach File Tracer Tracepoints ---
	// syscalls/sys_enter_open
	tpOpen, err := link.Tracepoint("syscalls", "sys_enter_open", fileObjs.TraceEnterOpen, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/syscalls/sys_enter_open: %w", err)
	}
	m.links = append(m.links, tpOpen)

	// syscalls/sys_enter_openat
	tpOpenAt, err := link.Tracepoint("syscalls", "sys_enter_openat", fileObjs.TraceEnterOpenat, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/syscalls/sys_enter_openat: %w", err)
	}
	m.links = append(m.links, tpOpenAt)

	// --- Attach TCP Retrans Tracer Tracepoints ---
	// tcp/tcp_retransmit_skb
	tpRetrans, err := link.Tracepoint("tcp", "tcp_retransmit_skb", tcpRetransObjs.TraceTcpRetransmitSkb, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/tcp/tcp_retransmit_skb: %w", err)
	}
	m.links = append(m.links, tpRetrans)

	// --- Attach DNS Tracer Tracepoints ---
	// syscalls/sys_enter_sendto
	tpSendto, err := link.Tracepoint("syscalls", "sys_enter_sendto", dnsObjs.TraceEnterSendto, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/syscalls/sys_enter_sendto: %w", err)
	}
	m.links = append(m.links, tpSendto)

	// syscalls/sys_enter_recvfrom and sys_exit_recvfrom
	tpRecvfromEnter, err := link.Tracepoint("syscalls", "sys_enter_recvfrom", dnsObjs.TraceEnterRecvfrom, nil)
	if err == nil {
		m.links = append(m.links, tpRecvfromEnter)
	}
	tpRecvfromExit, err := link.Tracepoint("syscalls", "sys_exit_recvfrom", dnsObjs.TraceExitRecvfrom, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/syscalls/sys_exit_recvfrom: %w", err)
	}
	m.links = append(m.links, tpRecvfromExit)

	// --- Attach KfreeSkb Tracer Tracepoints ---
	// skb/kfree_skb
	tpDrop, err := link.Tracepoint("skb", "kfree_skb", kfreeSkbObjs.TraceKfreeSkb, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/skb/kfree_skb: %w", err)
	}
	m.links = append(m.links, tpDrop)

	// --- Attach Process Exit Tracer Tracepoints ---
	// sched/sched_process_exit
	tpExit, err := link.Tracepoint("sched", "sched_process_exit", processExitObjs.TraceSchedProcessExit, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/sched/sched_process_exit: %w", err)
	}
	m.links = append(m.links, tpExit)

	// --- Attach UDP Tracer Tracepoints ---
	// udp/udp_fail_queue_rcv_skb
	tpUdp, err := link.Tracepoint("udp", "udp_fail_queue_rcv_skb", udpObjs.TraceUdpFailQueueRcvSkb, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/udp/udp_fail_queue_rcv_skb: %w", err)
	}
	m.links = append(m.links, tpUdp)

	// --- Attach Page Fault Tracer Tracepoints ---
	// exceptions/page_fault_user
	tpPf, err := link.Tracepoint("exceptions", "page_fault_user", pageFaultObjs.TracePageFaultUser, nil)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("attaching tracepoint/exceptions/page_fault_user: %w", err)
	}
	m.links = append(m.links, tpPf)

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

	oomRd, err := perf.NewReader(oomObjs.OomEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating oom perf reader: %w", err)
	}
	m.oomReader = oomRd

	processRd, err := perf.NewReader(processObjs.ProcessEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating process perf reader: %w", err)
	}
	m.processReader = processRd

	fileRd, err := perf.NewReader(fileObjs.FileEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating file perf reader: %w", err)
	}
	m.fileReader = fileRd

	tcpRetransRd, err := perf.NewReader(tcpRetransObjs.TcpRetransEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating tcp retrans perf reader: %w", err)
	}
	m.tcpRetransReader = tcpRetransRd

	dnsRd, err := perf.NewReader(dnsObjs.DnsEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating dns perf reader: %w", err)
	}
	m.dnsReader = dnsRd

	dropRd, err := perf.NewReader(kfreeSkbObjs.DropEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating kfree_skb perf reader: %w", err)
	}
	m.kfreeSkbReader = dropRd

	exitRd, err := perf.NewReader(processExitObjs.ProcessExitEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating process_exit perf reader: %w", err)
	}
	m.processExitReader = exitRd

	udpRd, err := perf.NewReader(udpObjs.UdpEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating udp perf reader: %w", err)
	}
	m.udpReader = udpRd

	pfRd, err := perf.NewReader(pageFaultObjs.PageFaultEvents, 4096)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating page_fault perf reader: %w", err)
	}
	m.pageFaultReader = pfRd

	return m, nil
}

func (m *Manager) Close() {
	if m.netReader != nil {
		m.netReader.Close()
	}
	if m.httpReader != nil {
		m.httpReader.Close()
	}
	if m.oomReader != nil {
		m.oomReader.Close()
	}
	if m.processReader != nil {
		m.processReader.Close()
	}
	if m.fileReader != nil {
		m.fileReader.Close()
	}
	if m.tcpRetransReader != nil {
		m.tcpRetransReader.Close()
	}
	if m.dnsReader != nil {
		m.dnsReader.Close()
	}
	if m.kfreeSkbReader != nil {
		m.kfreeSkbReader.Close()
	}
	if m.processExitReader != nil {
		m.processExitReader.Close()
	}
	if m.udpReader != nil {
		m.udpReader.Close()
	}
	if m.pageFaultReader != nil {
		m.pageFaultReader.Close()
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
	if m.oomObjs != nil {
		m.oomObjs.Close()
	}
	if m.processObjs != nil {
		m.processObjs.Close()
	}
	if m.fileObjs != nil {
		m.fileObjs.Close()
	}
	if m.tcpRetransObjs != nil {
		m.tcpRetransObjs.Close()
	}
	if m.dnsObjs != nil {
		m.dnsObjs.Close()
	}
	if m.kfreeSkbObjs != nil {
		m.kfreeSkbObjs.Close()
	}
	if m.processExitObjs != nil {
		m.processExitObjs.Close()
	}
	if m.udpObjs != nil {
		m.udpObjs.Close()
	}
	if m.pageFaultObjs != nil {
		m.pageFaultObjs.Close()
	}
}

func (m *Manager) NetReader() *perf.Reader {
	return m.netReader
}

func (m *Manager) HttpReader() *perf.Reader {
	return m.httpReader
}

func (m *Manager) OomReader() *perf.Reader {
	return m.oomReader
}

func (m *Manager) ProcessReader() *perf.Reader {
	return m.processReader
}

func (m *Manager) FileReader() *perf.Reader {
	return m.fileReader
}

func (m *Manager) TcpRetransReader() *perf.Reader {
	return m.tcpRetransReader
}

func (m *Manager) DnsReader() *perf.Reader {
	return m.dnsReader
}

func (m *Manager) KfreeSkbReader() *perf.Reader {
	return m.kfreeSkbReader
}

func (m *Manager) ProcessExitReader() *perf.Reader {
	return m.processExitReader
}

func (m *Manager) UdpReader() *perf.Reader {
	return m.udpReader
}

func (m *Manager) PageFaultReader() *perf.Reader {
	return m.pageFaultReader
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
