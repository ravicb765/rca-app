package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	agentebpf "github.com/ravicb765/rca-app/node-agent/ebpf"
)

// Config holds configuration for the agent
type Config struct {
	ServerEndpoint string
	HostName       string
}

// ConnectionStats represents aggregated connection data
type ConnectionStats struct {
	Source      string
	Destination string
	Packets     uint64
	Bytes       uint64
}

// ProcessStats represents CPU/Memory usage for a process
type ProcessStats struct {
	PID      int
	CPUUsage float64
	MemUsage uint64 // bytes
}

// NetworkTracer handles network-related eBPF collection
type NetworkTracer struct {
	// Conntrack table from eBPF map
	conntrackMap *ebpf.Map
	// Channel to send aggregated connection stats
	statsChan chan<- ConnectionStats
}

// EBPFManager manages eBPF programs and maps
type EBPFManager struct {
	manager       *agentebpf.Manager
	networkTracer *NetworkTracer
}

// NodeAgent is the main controller
type NodeAgent struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	ebpfManager           *EBPFManager
	config                *Config
	eventChan             chan interface{}
	httpRequestsTotal     *prometheus.CounterVec
	httpLatencyHistogram  *prometheus.HistogramVec
	httpBodySizeHistogram *prometheus.HistogramVec
	pgQueriesTotal        *prometheus.CounterVec
	pgQueryLatency        *prometheus.HistogramVec
	redisQueriesTotal     *prometheus.CounterVec
	redisQueryLatency     *prometheus.HistogramVec
	memcachedQueriesTotal *prometheus.CounterVec
	memcachedQueryLatency *prometheus.HistogramVec
	mysqlQueriesTotal     *prometheus.CounterVec
	mysqlQueryLatency     *prometheus.HistogramVec
	mongoQueriesTotal     *prometheus.CounterVec
	mongoQueryLatency     *prometheus.HistogramVec
	kafkaQueriesTotal     *prometheus.CounterVec
	kafkaQueryLatency     *prometheus.HistogramVec
	rabbitmqQueriesTotal  *prometheus.CounterVec
	rabbitmqQueryLatency  *prometheus.HistogramVec
	cassandraQueriesTotal *prometheus.CounterVec
	cassandraQueryLatency *prometheus.HistogramVec
	// Container Metrics
	containerCpuSeconds *prometheus.CounterVec
	containerMemBytes   *prometheus.GaugeVec
	containerNetBytes   *prometheus.CounterVec
	containerDiskBytes  *prometheus.CounterVec
	containerOomKills   *prometheus.CounterVec
	cgroupCache         map[uint64]string // Cache cgroup_id -> container_name
	lastCgroupScan      time.Time
	stackCounts         map[string]int
	stackCountsMu       sync.Mutex
}

func NewNodeAgent(cfg *Config) *NodeAgent {
	ctx, cancel := context.WithCancel(context.Background())
	na := &NodeAgent{
		ctx:         ctx,
		cancel:      cancel,
		config:      cfg,
		eventChan:   make(chan interface{}, 1000),
		ebpfManager: &EBPFManager{},
		cgroupCache: make(map[uint64]string),
		stackCounts: make(map[string]int),
	}

	na.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_http_requests_total",
			Help: "Total number of HTTP requests by destination and status code",
		},
		[]string{"destination", "status_code"},
	)
	prometheus.MustRegister(na.httpRequestsTotal)

	na.httpLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_http_latency_seconds",
			Help:    "HTTP request latency distribution",
			Buckets: prometheus.DefBuckets, // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.httpLatencyHistogram)

	na.httpBodySizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_http_body_size_bytes",
			Help:    "Size of HTTP request/response bodies",
			Buckets: prometheus.ExponentialBuckets(100, 10, 5), // 100, 1000, 10000...
		},
		[]string{"destination", "direction"},
	)
	prometheus.MustRegister(na.httpBodySizeHistogram)

	na.pgQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_pg_queries_total",
			Help: "Total number of PostgreSQL queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.pgQueriesTotal)

	na.pgQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_pg_query_latency_seconds",
			Help:    "PostgreSQL query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.pgQueryLatency)

	na.redisQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_redis_queries_total",
			Help: "Total number of Redis queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.redisQueriesTotal)

	na.redisQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_redis_query_latency_seconds",
			Help:    "Redis query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.redisQueryLatency)

	na.memcachedQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_memcached_queries_total",
			Help: "Total number of Memcached queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.memcachedQueriesTotal)

	na.memcachedQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_memcached_query_latency_seconds",
			Help:    "Memcached query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.memcachedQueryLatency)

	na.mysqlQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_mysql_queries_total",
			Help: "Total number of MySQL queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.mysqlQueriesTotal)

	na.mysqlQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_mysql_query_latency_seconds",
			Help:    "MySQL query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.mysqlQueryLatency)

	na.mongoQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_mongo_queries_total",
			Help: "Total number of MongoDB queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.mongoQueriesTotal)

	na.mongoQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_mongo_query_latency_seconds",
			Help:    "MongoDB query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.mongoQueryLatency)

	na.kafkaQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_kafka_queries_total",
			Help: "Total number of Kafka queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.kafkaQueriesTotal)

	na.kafkaQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_kafka_query_latency_seconds",
			Help:    "Kafka query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.kafkaQueryLatency)

	na.rabbitmqQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_rabbitmq_queries_total",
			Help: "Total number of RabbitMQ queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.rabbitmqQueriesTotal)

	na.rabbitmqQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_rabbitmq_query_latency_seconds",
			Help:    "RabbitMQ query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.rabbitmqQueryLatency)

	na.cassandraQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_cassandra_queries_total",
			Help: "Total number of Cassandra queries by destination and command",
		},
		[]string{"destination", "command"},
	)
	prometheus.MustRegister(na.cassandraQueriesTotal)

	na.cassandraQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_agent_cassandra_query_latency_seconds",
			Help:    "Cassandra query latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination"},
	)
	prometheus.MustRegister(na.cassandraQueryLatency)

	// Container Metrics
	na.containerCpuSeconds = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_container_cpu_seconds_total",
			Help: "Total CPU time spent by container",
		},
		[]string{"container_name"},
	)
	prometheus.MustRegister(na.containerCpuSeconds)

	na.containerMemBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_agent_container_memory_rss_bytes",
			Help: "Container memory RSS",
		},
		[]string{"container_name"},
	)
	prometheus.MustRegister(na.containerMemBytes)

	na.containerDiskBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_container_disk_bytes_total",
			Help: "Disk I/O bytes",
		},
		[]string{"container_name", "operation"}, // read/write
	)
	prometheus.MustRegister(na.containerDiskBytes)

	na.containerNetBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_container_network_bytes_total",
			Help: "Container network I/O bytes",
		},
		[]string{"container_name", "direction"}, // rx/tx
	)
	prometheus.MustRegister(na.containerNetBytes)

	na.containerOomKills = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_agent_container_oom_kills_total",
			Help: "Total number of OOM kills per container",
		},
		[]string{"container_name"},
	)
	prometheus.MustRegister(na.containerOomKills)

	return na
}

func (a *NodeAgent) Start() error {
	log.Println("Starting Node Agent...")
	log.Printf("Server endpoint: %s", a.config.ServerEndpoint)

	// 1. Load eBPF objects
	if err := a.loadEBPF(); err != nil {
		log.Printf("Warning: eBPF loader error: %v", err)
	} else {
		log.Println("eBPF objects loaded")
	}

	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/debug/flamegraph", a.flamegraphHandler)
		log.Println("Starting metrics server on :9100")
		if err := http.ListenAndServe(":9100", nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// 2. Start background tasks
	a.wg.Add(1)
	go a.runEventLoop()

	a.wg.Add(1)
	go a.runProcessCollector()

	a.wg.Add(1)
	go a.runContainerCollector()

	return nil
}

func (a *NodeAgent) loadEBPF() error {
	// Load and attach eBPF programs using the manager
	mgr, err := agentebpf.LoadAndAttach()
	if err != nil {
		return fmt.Errorf("loading ebpf: %w", err)
	}
	a.ebpfManager.manager = mgr

	// Configure HTTP ports to trace (e.g. 80, 8080)
	if err := mgr.AddHttpPort(80); err != nil {
		log.Printf("Error adding port 80 to http tracer: %v", err)
	}
	if err := mgr.AddHttpPort(8080); err != nil {
		log.Printf("Error adding port 8080 to http tracer: %v", err)
	}

	if err := mgr.AddPgPort(5432); err != nil {
		log.Printf("Error adding port 5432 to pg tracer: %v", err)
	}

	if err := mgr.AddRedisPort(6379); err != nil {
		log.Printf("Error adding port 6379 to redis tracer: %v", err)
	}

	if err := mgr.AddMemcachedPort(11211); err != nil {
		log.Printf("Error adding port 11211 to memcached tracer: %v", err)
	}

	if err := mgr.AddMysqlPort(3306); err != nil {
		log.Printf("Error adding port 3306 to mysql tracer: %v", err)
	}

	if err := mgr.AddMariaDBPort(3306); err != nil {
		log.Printf("Error adding port 3306 to mariadb tracer: %v", err)
	}

	if err := mgr.AddMongoPort(27017); err != nil {
		log.Printf("Error adding port 27017 to mongo tracer: %v", err)
	}

	if err := mgr.AddKafkaPort(9092); err != nil {
		log.Printf("Error adding port 9092 to kafka tracer: %v", err)
	}

	if err := mgr.AddRabbitMQPort(5672); err != nil {
		log.Printf("Error adding port 5672 to rabbitmq tracer: %v", err)
	}

	if err := mgr.AddCassandraPort(9042); err != nil {
		log.Printf("Error adding port 9042 to cassandra tracer: %v", err)
	}

	if err := mgr.AddCockroachDBPort(26257); err != nil {
		log.Printf("Error adding port 26257 to cockroachdb tracer: %v", err)
	}

	if err := mgr.AddYugabyteDBPort(5433); err != nil {
		log.Printf("Error adding port 5433 to yugabytedb tracer: %v", err)
	}

	// Attach Profiler (runs on all CPUs but filters by map)
	if err := mgr.AttachProfiler(); err != nil {
		log.Printf("Error attaching profiler: %v", err)
	}

	// Start handling events
	a.wg.Add(1)
	go a.handlePerfEvents(mgr.NetReader())
	a.wg.Add(1)
	go a.handleHttpEvents(mgr.HttpReader())
	a.wg.Add(1)
	go a.handleOomEvents(mgr.OomReader())
	a.wg.Add(1)
	go a.handleStackEvents(mgr.StackReader())

	return nil
}

func (a *NodeAgent) Stop() {
	log.Println("Stopping Node Agent...")
	a.cancel()

	if a.ebpfManager.manager != nil {
		a.ebpfManager.manager.Close()
	}

	a.wg.Wait()
	log.Println("Node Agent stopped")
}

func (a *NodeAgent) runEventLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case event := <-a.eventChan:
			log.Printf("Processing event: %v", event)
		case <-ticker.C:
			log.Println("Agent heartbeat: healthy")
		}
	}
}

func (a *NodeAgent) flamegraphHandler(w http.ResponseWriter, r *http.Request) {
	a.stackCountsMu.Lock()
	defer a.stackCountsMu.Unlock()

	w.Header().Set("Content-Type", "text/plain")
	for stack, count := range a.stackCounts {
		if stack != "" {
			fmt.Fprintf(w, "%s %d\n", stack, count)
		}
	}
}

func (a *NodeAgent) runContainerCollector() {
	defer a.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.collectContainerMetrics()
		}
	}
}

type IOStats struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
}

func (a *NodeAgent) collectContainerMetrics() {
	if a.ebpfManager.manager == nil {
		return
	}
	cpuMap, ioMap, netMap := a.ebpfManager.manager.GetContainerMaps()
	if cpuMap == nil {
		return
	}

	// 1. Collect CPU Stats from eBPF
	var cgroupID uint64
	var cpuTimeNs uint64
	cpuIter := cpuMap.Iterate()
	for cpuIter.Next(&cgroupID, &cpuTimeNs) {
		name := a.resolveCgroup(cgroupID)
		if name != "" {
			seconds := float64(cpuTimeNs) / 1e9
			a.containerCpuSeconds.WithLabelValues(name).Set(seconds)

			// Simple Profiling Trigger:
			// If we detect high activity (this is a naive check, ideally we check rate of change),
			// we enable profiling for this container.
			// In a real scenario, we'd calculate rate > 0.8 cores, etc.
			// Here we just enable it if it exists to demonstrate the mechanism.
			profileMap := a.ebpfManager.manager.GetProfileMap()
			var val uint8 = 1
			profileMap.Put(cgroupID, val)

			// Also collect Memory (from cgroupfs, not eBPF)
			// Assuming cgroup v2 or v1 path structure
			// This is a simplified check; production needs robust path handling
			memPath := fmt.Sprintf("/sys/fs/cgroup/kubepods/burstable/%s/memory.current", name) // Example path
			// Try reading memory usage
			if data, err := os.ReadFile(memPath); err == nil {
				if val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
					a.containerMemBytes.WithLabelValues(name).Set(float64(val))
				}
			}
		}
	}

	// 2. Collect Disk I/O Stats from eBPF
	var ioStats IOStats
	ioIter := ioMap.Iterate()
	for ioIter.Next(&cgroupID, &ioStats) {
		name := a.resolveCgroup(cgroupID)
		if name != "" {
			a.containerDiskBytes.WithLabelValues(name, "read").Set(float64(ioStats.ReadBytes))
			a.containerDiskBytes.WithLabelValues(name, "write").Set(float64(ioStats.WriteBytes))
		}
	}

	// 3. Collect Network Stats from eBPF
	var netStats IOStats
	netIter := netMap.Iterate()
	for netIter.Next(&cgroupID, &netStats) {
		name := a.resolveCgroup(cgroupID)
		if name != "" {
			a.containerNetBytes.WithLabelValues(name, "rx").Set(float64(netStats.ReadBytes))
			a.containerNetBytes.WithLabelValues(name, "tx").Set(float64(netStats.WriteBytes))
		}
	}
}

// resolveCgroup maps a cgroup ID (inode) to a container name
func (a *NodeAgent) resolveCgroup(id uint64) string {
	// 1. Check cache first
	if name, ok := a.cgroupCache[id]; ok {
		return name
	}

	// 2. If not found, check if we should scan (rate limited)
	if time.Since(a.lastCgroupScan) < 10*time.Second {
		return ""
	}

	// 3. Refresh cache by scanning /proc once
	a.refreshCgroupCache()

	// 4. Check cache again
	if name, ok := a.cgroupCache[id]; ok {
		return name
	}
	return ""
}

func (a *NodeAgent) refreshCgroupCache() {
	a.lastCgroupScan = time.Now()
	matches, _ := filepath.Glob("/proc/[0-9]*/cgroup")
	for _, path := range matches {
		// Read cgroup file
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Parse cgroup file: "0::/kubepods/..."
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) < 3 {
				continue
			}
			cgroupPath := parts[2]

			// We only care about container paths
			if !strings.Contains(cgroupPath, "docker-") && !strings.Contains(cgroupPath, "kubepods") {
				continue
			}

			// Stat the cgroup path to get the Inode (ID)
			fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
			fi, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			stat, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				continue
			}

			// Extract name from path
			pathParts := strings.Split(cgroupPath, "/")
			if len(pathParts) > 0 {
				name := pathParts[len(pathParts)-1]
				// Cleanup name (remove .scope, etc if needed)
				name = strings.TrimSuffix(name, ".scope")
				a.cgroupCache[stat.Ino] = name
			}
		}
	}
}

func (a *NodeAgent) runProcessCollector() {
	defer a.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			stats, err := collectProcessMetrics()
			if err != nil {
				log.Printf("Error collecting process metrics: %v", err)
				continue
			}
			// In a real implementation, we would batch and send these to the server
			// For now, we log the top consumer to demonstrate collection
			if len(stats) > 0 {
				log.Printf("Collected metrics for %d processes. Top PID: %d (CPU: %.2f%%)",
					len(stats), stats[0].PID, stats[0].CPUUsage)
			}
		}
	}
}

// collectProcessMetrics scans /proc to get basic metrics
// This is a simplified implementation. Production agents often use eBPF for this too
// (e.g., sched_switch tracepoints) but /proc is standard for basic stats.
func collectProcessMetrics() ([]ProcessStats, error) {
	matches, err := filepath.Glob("/proc/[0-9]*")
	if err != nil {
		return nil, err
	}

	var stats []ProcessStats
	for _, path := range matches {
		base := filepath.Base(path)
		pid, err := strconv.Atoi(base)
		if err != nil {
			continue
		}

		// Read /proc/[pid]/stat
		data, err := os.ReadFile(filepath.Join(path, "stat"))
		if err != nil {
			continue
		}
		fields := bytes.Fields(data)
		if len(fields) < 24 {
			continue
		}

		// RSS is field 24 (pages)
		rssPages, _ := strconv.ParseUint(string(fields[23]), 10, 64)
		rssBytes := rssPages * uint64(os.Getpagesize())

		stats = append(stats, ProcessStats{PID: pid, MemUsage: rssBytes, CPUUsage: 0.0})
	}
	return stats, nil
}

func (a *NodeAgent) handlePerfEvents(rd *perf.Reader) {
	defer a.wg.Done()
	for {
		record, err := rd.Read()
		if err != nil {
			if perf.IsClosed(err) {
				return
			}
			log.Printf("Error reading perf event: %v", err)
			continue
		}

		if len(record.RawSample) < 48 { // Size of ConnEvent
			log.Printf("Invalid sample size: %d", len(record.RawSample))
			continue
		}

		var event agentebpf.ConnEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Printf("Failed to decode event: %v", err)
			continue
		}

		srcIP := int2ip(event.Saddr)
		dstIP := int2ip(event.Daddr)
		comm := string(bytes.TrimRight(event.Comm[:], "\x00"))

		typeStr := "UNKNOWN"
		switch event.Type {
		case agentebpf.EventTypeConnect:
			typeStr = "CONNECT"
		case agentebpf.EventTypeAccept:
			typeStr = "ACCEPT"
		case agentebpf.EventTypeClose:
			typeStr = "CLOSE"
		}

		log.Printf("[%s] %s:%d -> %s:%d (pid: %d, comm: %s)",
			typeStr, srcIP, event.Sport, dstIP, event.Dport, event.Pid, comm)
	}
}

func (a *NodeAgent) handleHttpEvents(rd *perf.Reader) {
	defer a.wg.Done()
	for {
		record, err := rd.Read()
		if err != nil {
			if perf.IsClosed(err) {
				return
			}
			log.Printf("Error reading http perf event: %v", err)
			continue
		}

		var event agentebpf.HttpEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Printf("Failed to decode http event: %v", err)
			continue
		}

		comm := string(bytes.TrimRight(event.Comm[:], "\x00"))
		payload := string(event.Data[:event.DataLen])
		method := string(bytes.TrimRight(event.Method[:], "\x00"))
		path := string(bytes.TrimRight(event.Path[:], "\x00"))
		// Sanitize payload for logging (replace newlines)
		payload = strconv.Quote(payload)

		direction := "REQ"
		statusStr := ""
		latencyStr := ""

		// Handle PostgreSQL Events
		if event.Type == agentebpf.EventTypePgQuery || event.Type == agentebpf.EventTypePgResponse {
			dstIP := int2ip(event.Daddr)
			svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

			if event.Type == agentebpf.EventTypePgQuery {
				parentMethod := string(bytes.TrimRight(event.ParentMethod[:], "\x00"))
				parentPath := string(bytes.TrimRight(event.ParentPath[:], "\x00"))
				correlationInfo := ""
				if parentMethod != "" {
					correlationInfo = fmt.Sprintf(" [Triggered by %s %s]", parentMethod, parentPath)
				}

				log.Printf("[PG] Query Len:%d Data:%s%s", event.MsgLen, payload, correlationInfo)

				var command string
				// The eBPF program sends the raw buffer. For a simple query ('Q'),
				// the SQL starts at byte 5.
				if len(event.Data) > 5 && event.Data[0] == 'Q' {
					sqlQuery := event.Data[5:event.DataLen]
					fields := bytes.Fields([]byte(sqlQuery))
					if len(fields) > 0 {
						command = string(fields[0])
					}
				}
				if command != "" {
					a.pgQueriesTotal.WithLabelValues(svc, command).Inc()
				}
			} else if event.Type == agentebpf.EventTypePgResponse {
				latencyMs := event.Latency / 1000000
				log.Printf("[PG] Response Len:%d Latency:%dms", event.MsgLen, latencyMs)
				a.pgQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
				if latencyMs > 100 { // Example threshold: 100ms
					log.Printf("SLOW QUERY DETECTED on %s. Latency: %dms", svc, latencyMs)
				}
			}
			continue // Skip the rest of the HTTP logic
		}

		// Handle Redis Events
		if event.Type == agentebpf.EventTypeRedisCommand || event.Type == agentebpf.EventTypeRedisResponse {
			dstIP := int2ip(event.Daddr)
			svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

			if event.Type == agentebpf.EventTypeRedisCommand {
				parentMethod := string(bytes.TrimRight(event.ParentMethod[:], "\x00"))
				parentPath := string(bytes.TrimRight(event.ParentPath[:], "\x00"))
				correlationInfo := ""
				if parentMethod != "" {
					correlationInfo = fmt.Sprintf(" [Triggered by %s %s]", parentMethod, parentPath)
				}

				log.Printf("[REDIS] Command Len:%d Data:%s%s", event.MsgLen, payload, correlationInfo)

				var command string = method
				if command == "" {
					// Simple heuristic for RESP: *2\r\n$3\r\nGET...
					// Or inline: GET key
					// We'll just try to grab the first word if it's not a symbol
					if len(event.Data) > 0 {
						// If it starts with *, it's an array. We could parse it, but for now just log "RESP"
						if event.Data[0] == '*' {
							command = "RESP_ARRAY"
						} else {
							fields := bytes.Fields(event.Data[:event.DataLen])
							if len(fields) > 0 {
								command = string(fields[0])
							}
						}
					}
				}
				if command != "" {
					a.redisQueriesTotal.WithLabelValues(svc, command).Inc()
				}
			} else if event.Type == agentebpf.EventTypeRedisResponse {
				a.redisQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
			}
			continue
		}

		// Handle Memcached Events
		if event.Type == agentebpf.EventTypeMemcachedCommand || event.Type == agentebpf.EventTypeMemcachedResponse {
			a.processMemcachedEvent(&event)
			continue
		}

		// Handle MySQL Events
		if event.Type == agentebpf.EventTypeMysqlQuery || event.Type == agentebpf.EventTypeMysqlResponse {
			a.processMysqlEvent(&event)
			continue
		}

		// Handle Mongo Events
		if event.Type == agentebpf.EventTypeMongoCommand || event.Type == agentebpf.EventTypeMongoResponse {
			a.processMongoEvent(&event)
			continue
		}

		// Handle Kafka Events
		if event.Type == agentebpf.EventTypeKafkaCommand || event.Type == agentebpf.EventTypeKafkaResponse {
			a.processKafkaEvent(&event)
			continue
		}

		// Handle RabbitMQ Events
		if event.Type == agentebpf.EventTypeRabbitMQCommand || event.Type == agentebpf.EventTypeRabbitMQResponse {
			a.processRabbitMQEvent(&event)
			continue
		}

		// Handle Cassandra Events
		if event.Type == agentebpf.EventTypeCassandraCommand || event.Type == agentebpf.EventTypeCassandraResponse {
			a.processCassandraEvent(&event)
			continue
		}

		if event.Type == agentebpf.EventTypeHttpResponse {
			direction = "RESP"
			statusStr = fmt.Sprintf(" Status:%d", event.StatusCode)
			latencyStr = fmt.Sprintf(" Latency:%dms", event.Latency/1000000)

			// Update Prometheus metrics
			dstIP := int2ip(event.Daddr)
			svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)
			a.httpRequestsTotal.WithLabelValues(svc, strconv.Itoa(int(event.StatusCode))).Inc()
			a.httpLatencyHistogram.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
			a.httpBodySizeHistogram.WithLabelValues(svc, "response").Observe(float64(event.MsgLen))
		} else {
			// It's a request, track stats
			srcIP := int2ip(event.Saddr)
			dstIP := int2ip(event.Daddr)

			log.Printf("[HTTP REQ] %s:%d -> %s:%d %s %s (pid: %d)",
				srcIP, event.Sport, dstIP, event.Dport, method, path, event.Pid)

			svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)
			a.httpBodySizeHistogram.WithLabelValues(svc, "request").Observe(float64(event.MsgLen))
		}

		if direction == "RESP" {
			log.Printf("[HTTP RESP] PID:%d Comm:%s%s%s Data:%s",
				event.Pid, comm, statusStr, latencyStr, payload)
		}
	}
}

func (a *NodeAgent) processMemcachedEvent(event *agentebpf.HttpEvent) {
	dstIP := int2ip(event.Daddr)
	svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

	if event.Type == agentebpf.EventTypeMemcachedCommand {
		command := string(bytes.TrimRight(event.Method[:], "\x00"))
		if command == "" {
			command = "UNKNOWN"
		}
		a.memcachedQueriesTotal.WithLabelValues(svc, command).Inc()

		parentMethod := string(bytes.TrimRight(event.ParentMethod[:], "\x00"))
		if parentMethod != "" {
			log.Printf("[MEMCACHED] %s triggered by %s", command, parentMethod)
		}
	} else if event.Type == agentebpf.EventTypeMemcachedResponse {
		a.memcachedQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
	}
}

func (a *NodeAgent) processMysqlEvent(event *agentebpf.HttpEvent) {
	dstIP := int2ip(event.Daddr)
	svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

	if event.Type == agentebpf.EventTypeMysqlQuery {
		// Payload starts at offset 0 (we skipped header in eBPF)
		query := string(event.Data[:event.DataLen])
		// Simple command extraction (first word)
		fields := bytes.Fields([]byte(query))
		command := "UNKNOWN"
		if len(fields) > 0 {
			command = string(fields[0])
		}
		a.mysqlQueriesTotal.WithLabelValues(svc, command).Inc()
		log.Printf("[MYSQL] Query: %s", query)
	} else if event.Type == agentebpf.EventTypeMysqlResponse {
		a.mysqlQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
	}
}

func (a *NodeAgent) processMongoEvent(event *agentebpf.HttpEvent) {
	dstIP := int2ip(event.Daddr)
	svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

	if event.Type == agentebpf.EventTypeMongoCommand {
		// Payload starts at offset 0 (we skipped header in eBPF)
		// For OP_MSG, we might see flagBits (4 bytes) + section kind (1 byte) + BSON
		// eBPF now extracts the command into Method
		command := string(bytes.TrimRight(event.Method[:], "\x00"))
		if command == "" {
			command = "UNKNOWN"
		}
		a.mongoQueriesTotal.WithLabelValues(svc, command).Inc()

	} else if event.Type == agentebpf.EventTypeMongoResponse {
		a.mongoQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
	}
}

var kafkaAPIKeys = map[uint32]string{
	0:  "Produce",
	1:  "Fetch",
	2:  "ListOffsets",
	3:  "Metadata",
	8:  "OffsetCommit",
	9:  "OffsetFetch",
	10: "FindCoordinator",
	11: "JoinGroup",
	12: "Heartbeat",
	13: "LeaveGroup",
	14: "SyncGroup",
	15: "DescribeGroups",
	16: "ListGroups",
}

func (a *NodeAgent) processKafkaEvent(event *agentebpf.HttpEvent) {
	dstIP := int2ip(event.Daddr)
	svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

	if event.Type == agentebpf.EventTypeKafkaCommand {
		command, ok := kafkaAPIKeys[event.StatusCode]
		if !ok {
			command = fmt.Sprintf("ApiKey-%d", event.StatusCode)
		}
		a.kafkaQueriesTotal.WithLabelValues(svc, command).Inc()
	} else if event.Type == agentebpf.EventTypeKafkaResponse {
		a.kafkaQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
	}
}

func (a *NodeAgent) processRabbitMQEvent(event *agentebpf.HttpEvent) {
	dstIP := int2ip(event.Daddr)
	svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

	if event.Type == agentebpf.EventTypeRabbitMQCommand {
		// Payload starts at offset 0 (we skipped header in eBPF)
		// Method Frame: Class ID (2 bytes), Method ID (2 bytes)
		// We can extract these to identify the command.
		var classID, methodID uint16
		if event.DataLen >= 4 {
			classID = binary.BigEndian.Uint16(event.Data[0:2])
			methodID = binary.BigEndian.Uint16(event.Data[2:4])
		}

		command := fmt.Sprintf("Class-%d-Method-%d", classID, methodID)
		// Map common methods
		if classID == 60 && methodID == 40 {
			command = "Basic.Publish"
		} else if classID == 60 && methodID == 70 {
			command = "Basic.Get"
		}
		a.rabbitmqQueriesTotal.WithLabelValues(svc, command).Inc()
	} else if event.Type == agentebpf.EventTypeRabbitMQResponse {
		a.rabbitmqQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
	}
}

func (a *NodeAgent) processCassandraEvent(event *agentebpf.HttpEvent) {
	dstIP := int2ip(event.Daddr)
	svc := fmt.Sprintf("%s:%d", dstIP, event.Dport)

	if event.Type == agentebpf.EventTypeCassandraCommand {
		// Payload starts at offset 0 (we skipped header in eBPF)
		// It should be the query string.
		query := string(event.Data[:event.DataLen])
		// Simple command extraction (first word)
		fields := bytes.Fields([]byte(query))
		command := "UNKNOWN"
		if len(fields) > 0 {
			command = string(fields[0])
		}
		a.cassandraQueriesTotal.WithLabelValues(svc, command).Inc()
		log.Printf("[CASSANDRA] Query: %s", query)
	} else if event.Type == agentebpf.EventTypeCassandraResponse {
		a.cassandraQueryLatency.WithLabelValues(svc).Observe(float64(event.Latency) / 1e9)
	}
}

func (a *NodeAgent) handleOomEvents(rd *perf.Reader) {
	defer a.wg.Done()
	for {
		record, err := rd.Read()
		if err != nil {
			if perf.IsClosed(err) {
				return
			}
			continue
		}

		var event agentebpf.OomEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			continue
		}

		name := a.resolveCgroup(event.CgroupId)
		if name == "" {
			name = "unknown"
		}
		log.Printf("OOM Kill detected: container=%s pid=%d comm=%s", name, event.Pid, event.Comm)
		a.containerOomKills.WithLabelValues(name).Inc()
	}
}

// Symbol represents a kernel symbol
type Symbol struct {
	Addr uint64
	Name string
}

// KernelSymbols handles resolution of kernel addresses to names
type KernelSymbols struct {
	symbols []Symbol
}

// NewKernelSymbols loads symbols from /proc/kallsyms
func NewKernelSymbols() (*KernelSymbols, error) {
	f, err := os.Open("/proc/kallsyms")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var symbols []Symbol
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		addr, err := strconv.ParseUint(fields[0], 16, 64)
		if err != nil {
			continue
		}
		symbols = append(symbols, Symbol{Addr: addr, Name: fields[2]})
	}

	// Sort by address for binary search
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Addr < symbols[j].Addr
	})

	return &KernelSymbols{symbols: symbols}, nil
}

// Resolve returns the function name for a given address
func (k *KernelSymbols) Resolve(addr uint64) string {
	i := sort.Search(len(k.symbols), func(i int) bool {
		return k.symbols[i].Addr > addr
	})
	if i == 0 {
		return "unknown"
	}
	return k.symbols[i-1].Name
}

func (a *NodeAgent) handleStackEvents(rd *perf.Reader) {
	defer a.wg.Done()

	ksyms, err := NewKernelSymbols()
	if err != nil {
		log.Printf("Warning: Failed to load kernel symbols: %v", err)
	}
	stackMap := a.ebpfManager.manager.GetStackTracesMap()
	symbolCache := NewUserSymbolCache(ksyms, stackMap) // Assuming symbolizer.go exists

	for {
		record, err := rd.Read()
		if err != nil {
			if perf.IsClosed(err) {
				return
			}
			log.Printf("Error reading stack event: %v", err)
			continue
		}

		var event agentebpf.StackEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Printf("Failed to decode stack event: %v", err)
			continue
		}

		kernelStack := symbolCache.ResolveStack(event.KernelStackId, event.Pid, true)
		userStack := symbolCache.ResolveStack(event.UserStackId, event.Pid, false)

		// For FlameGraphs, combine user and kernel stacks
		fullStack := userStack
		if kernelStack != "" {
			if fullStack != "" {
				fullStack += ";"
			}
			fullStack += kernelStack
		}

		if fullStack != "" {
			a.stackCountsMu.Lock()
			a.stackCounts[fullStack]++
			a.stackCountsMu.Unlock()
		}
	}
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ip, nn)
	return ip
}

func main() {
	endpoint := os.Getenv("RCA_APP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}

	hostname, _ := os.Hostname()
	config := &Config{
		ServerEndpoint: endpoint,
		HostName:       hostname,
	}

	agent := NewNodeAgent(config)

	if err := agent.Start(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Wait for signals to gracefully exit
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	agent.Stop()
}
