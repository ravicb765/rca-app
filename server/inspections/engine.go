package inspections

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/ravicb765/rca-app/server/servicemap"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type InspectionResult struct {
	Name        string    `json:"name" yaml:"name"`
	Category    string    `json:"category" yaml:"category"`
	Severity    Severity  `json:"severity" yaml:"severity"`
	Status      string    `json:"status" yaml:"status"` // "pass" or "fail"
	Description string    `json:"description" yaml:"description"`
	Remediation string    `json:"remediation,omitempty" yaml:"remediation,omitempty"`
	Timestamp   time.Time `json:"timestamp" yaml:"timestamp"`
}

type Inspection struct {
	Name        string   `json:"name" yaml:"name"`
	Category    string   `json:"category" yaml:"category"`
	Rule        Rule     `json:"-" yaml:"-"`
	Severity    Severity `json:"severity" yaml:"severity"`
	Remediation string   `json:"remediation" yaml:"remediation"`
	Threshold   string   `json:"threshold,omitempty" yaml:"threshold,omitempty"`
}

type AppMetrics struct {
	RequestRate             float64
	ErrorRate               float64
	Latency                 float64
	MemoryUsage             float64 // in MB
	CPUUsage                float64 // percentage
	DiskUsage               float64 // percentage
	IOLoad                  float64 // arbitrary unit
	ActiveConnections       float64
	PacketLoss              float64 // percentage
	Http5xxRate             float64 // percentage (0.0-1.0)
	IOWait                  float64 // percentage
	SwapUsage               float64 // percentage
	RestartCount            float64 // count
	CPUThrottling           float64 // percentage
	GoroutineCount          float64 // count
	OpenFDs                 float64 // count
	ThreadCount             float64 // count
	MemcachedLatency        float64 // in ms
	MysqlLatency            float64 // in ms
	MongoLatency            float64 // in ms
	RabbitMQLatency         float64 // in ms
	RabbitMQQueueLength     float64 // count
	CassandraLatency        float64 // in ms
	MemcachedEvictions      float64 // count
	KafkaConsumerLag        float64 // count
	RedisFragmentationRatio float64 // ratio
	OomKills                float64 // count
	TcpRetransmits          float64 // count
	DnsLatency              float64 // in ms
	KernelPacketDrops       float64 // count
	UdpPacketLoss           float64 // count
	PageFaults              float64 // count
	ContextSwitches         float64 // count
	BlockIOLatency          float64 // in ms
	RunQLatency             float64 // in ms
	MemoryAllocRate         float64 // bytes/sec
	LockContention          float64 // in ms
	GCPause                 float64 // in ms
	NetworkLatency          float64 // in ms
}

type MetricPoint struct {
	Timestamp time.Time
	Metrics   AppMetrics
}

type Rule interface {
	Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string)
}

type TrendRule interface {
	Rule
	EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string)
}

type InspectionEngine struct {
	inspections      []Inspection
	results          map[string][]InspectionResult
	history          map[string][]InspectionResult
	metricHistory    map[string][]MetricPoint
	mu               sync.RWMutex
	HistoryRetention time.Duration
	inspectionStatus *prometheus.GaugeVec
	db               *sql.DB
}

func NewInspectionEngine(reg prometheus.Registerer) *InspectionEngine {
	engine := &InspectionEngine{
		results:          make(map[string][]InspectionResult),
		history:          make(map[string][]InspectionResult),
		metricHistory:    make(map[string][]MetricPoint),
		HistoryRetention: 1 * time.Hour,
		inspectionStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rca_inspection_status",
				Help: "Status of inspections (0=pass, 1=fail)",
			},
			[]string{"application", "name", "category", "severity"},
		),
	}

	if reg != nil {
		reg.MustRegister(engine.inspectionStatus)
	}

	engine.inspections = getDefaultInspections()
	return engine
}

func getDefaultInspections() []Inspection {
	return []Inspection{
		{
			Name:        "High Error Rate",
			Category:    "Availability",
			Rule:        &ErrorRateRule{Threshold: 0.01}, // 1%
			Severity:    SeverityCritical,
			Remediation: "Check application logs for errors and upstream dependencies.",
			Threshold:   "1%",
		},
		{
			Name:        "Error Rate Spike",
			Category:    "Availability",
			Rule:        &ErrorRateSpikeRule{Multiplier: 2.0, MinRate: 0.01}, // 2x increase
			Severity:    SeverityCritical,
			Remediation: "Investigate recent deployments or upstream changes causing sudden errors.",
			Threshold:   "2x increase",
		},
		{
			Name:        "Latency Degradation",
			Category:    "Performance",
			Rule:        &LatencyDegradationRule{Threshold: 0.5, MinLatency: 10}, // 50% increase
			Severity:    SeverityWarning,
			Remediation: "Check for recent deployments or increased load.",
			Threshold:   "50% increase",
		},
		{
			Name:        "High Latency",
			Category:    "Performance",
			Rule:        &LatencyRule{Threshold: 200}, // 200ms
			Severity:    SeverityWarning,
			Remediation: "Investigate slow database queries or resource contention.",
			Threshold:   "200ms",
		},
		{
			Name:        "Memory Leak Detection",
			Category:    "Resources",
			Rule:        &MemoryLeakRule{Threshold: 512}, // 512 MB
			Severity:    SeverityWarning,
			Remediation: "Check for memory leaks or increase container limits.",
			Threshold:   "512MB",
		},
		{
			Name:        "Memory Leak Trend",
			Category:    "Resources",
			Rule:        &MemoryLeakTrendRule{Threshold: 50.0}, // 50 MB increase
			Severity:    SeverityWarning,
			Remediation: "Investigate potential memory leaks in application code.",
			Threshold:   "50MB increase",
		},
		{
			Name:        "High CPU Usage",
			Category:    "Resources",
			Rule:        &CPUUsageRule{Threshold: 80}, // 80%
			Severity:    SeverityWarning,
			Remediation: "Optimize CPU intensive tasks or scale up.",
			Threshold:   "80%",
		},
		{
			Name:        "High Disk Usage",
			Category:    "Resources",
			Rule:        &DiskUsageRule{Threshold: 90}, // 90%
			Severity:    SeverityCritical,
			Remediation: "Clean up disk space or expand storage.",
			Threshold:   "90%",
		},
		{
			Name:        "High IO Load",
			Category:    "Performance",
			Rule:        &IOLoadRule{Threshold: 10}, // Arbitrary threshold
			Severity:    SeverityWarning,
			Remediation: "Investigate disk I/O bottlenecks.",
			Threshold:   "10",
		},
		{
			Name:        "Database Connection Pool Exhaustion",
			Category:    "Database",
			Rule:        &ConnectionPoolExhaustionRule{Threshold: 90}, // 90% of pool
			Severity:    SeverityCritical,
			Remediation: "Check database connection pool settings and scale if necessary.",
			Threshold:   "90%",
		},
		{
			Name:        "Network Packet Loss",
			Category:    "Network",
			Rule:        &PacketLossRule{Threshold: 1.0}, // 1%
			Severity:    SeverityWarning,
			Remediation: "Check network connectivity and switch/router logs.",
			Threshold:   "1%",
		},
		{
			Name:        "Network Latency",
			Category:    "Network",
			Rule:        &NetworkLatencyRule{Threshold: 50}, // 50ms
			Severity:    SeverityWarning,
			Remediation: "Check for network congestion, routing issues, or high jitter.",
			Threshold:   "50ms",
		},
		{
			Name:        "High HTTP 5xx Rate",
			Category:    "Availability",
			Rule:        &Http5xxRateRule{Threshold: 0.05}, // 5%
			Severity:    SeverityCritical,
			Remediation: "Check application logs for server-side exceptions.",
			Threshold:   "5%",
		},
		{
			Name:        "High Disk I/O Wait",
			Category:    "Performance",
			Rule:        &IOWaitRule{Threshold: 10.0}, // 10%
			Severity:    SeverityWarning,
			Remediation: "Check for slow disk operations or saturated storage bandwidth.",
			Threshold:   "10%",
		},
		{
			Name:        "High Memory Swap Usage",
			Category:    "Resources",
			Rule:        &SwapUsageRule{Threshold: 10.0}, // 10%
			Severity:    SeverityWarning,
			Remediation: "Check for memory pressure and increase RAM if needed.",
			Threshold:   "10%",
		},
		{
			Name:        "High Container Restarts",
			Category:    "Stability",
			Rule:        &RestartCountRule{Threshold: 3}, // > 3 restarts
			Severity:    SeverityCritical,
			Remediation: "Investigate crash loops, OOM kills, or liveness probe failures.",
			Threshold:   "3",
		},
		{
			Name:        "CPU Throttling Trend",
			Category:    "Performance",
			Rule:        &CPUThrottlingTrendRule{Threshold: 0.5, MinThrottling: 5.0}, // 50% increase
			Severity:    SeverityWarning,
			Remediation: "Check for increasing resource contention or limits.",
			Threshold:   "50% increase",
		},
		{
			Name:        "CPU Usage Trend",
			Category:    "Resources",
			Rule:        &CPUUsageTrendRule{Threshold: 0.3, MinUsage: 10.0}, // 30% increase
			Severity:    SeverityWarning,
			Remediation: "Investigate potential infinite loops or increasing load.",
			Threshold:   "30% increase",
		},
		{
			Name:        "High CPU Throttling",
			Category:    "Performance",
			Rule:        &CPUThrottlingRule{Threshold: 5.0}, // 5% throttling
			Severity:    SeverityWarning,
			Remediation: "Increase CPU limits or optimize application performance.",
			Threshold:   "5%",
		},
		{
			Name:        "High Goroutine Count",
			Category:    "Resources",
			Rule:        &GoroutineCountRule{Threshold: 10000}, // 10k goroutines
			Severity:    SeverityWarning,
			Remediation: "Check for goroutine leaks or excessive concurrency.",
			Threshold:   "10000",
		},
		{
			Name:        "High Open File Descriptors",
			Category:    "Resources",
			Rule:        &OpenFDCountRule{Threshold: 1000}, // 1000 FDs
			Severity:    SeverityWarning,
			Remediation: "Check for file descriptor leaks (sockets, files) or increase ulimit.",
			Threshold:   "1000",
		},
		{
			Name:        "High Thread Count",
			Category:    "Resources",
			Rule:        &ThreadCountRule{Threshold: 500}, // 500 threads
			Severity:    SeverityWarning,
			Remediation: "Check for thread leaks or excessive concurrency.",
			Threshold:   "500",
		},
		{
			Name:        "High Memcached Latency",
			Category:    "Performance",
			Rule:        &MemcachedLatencyRule{Threshold: 10}, // 10ms
			Severity:    SeverityWarning,
			Remediation: "Check Memcached server load, network latency, or key distribution.",
			Threshold:   "10ms",
		},
		{
			Name:        "High Postgres-Compatible DB Latency",
			Category:    "Performance",
			Rule:        &PostgresLatencyRule{Threshold: 50}, // 50ms
			Severity:    SeverityWarning,
			Remediation: "Optimize queries, check indexes, or investigate database load.",
			Threshold:   "50ms (for MySQL, CockroachDB, YugabyteDB)",
		},
		{
			Name:        "High MongoDB Latency",
			Category:    "Performance",
			Rule:        &MongoLatencyRule{Threshold: 50}, // 50ms
			Severity:    SeverityWarning,
			Remediation: "Check query plans (explain), indexes, or cluster health.",
			Threshold:   "50ms",
		},
		{
			Name:        "High RabbitMQ Latency",
			Category:    "Performance",
			Rule:        &RabbitMQLatencyRule{Threshold: 20}, // 20ms
			Severity:    SeverityWarning,
			Remediation: "Check queue length, consumer performance, or network latency.",
			Threshold:   "20ms",
		},
		{
			Name:        "High RabbitMQ Queue Length",
			Category:    "Performance",
			Rule:        &RabbitMQQueueLengthRule{Threshold: 1000}, // 1000 messages
			Severity:    SeverityWarning,
			Remediation: "Check consumers, scale up workers, or investigate processing bottlenecks.",
			Threshold:   "1000",
		},
		{
			Name:        "High Cassandra Latency",
			Category:    "Performance",
			Rule:        &CassandraLatencyRule{Threshold: 50}, // 50ms
			Severity:    SeverityWarning,
			Remediation: "Check data model, partition keys, or cluster health.",
			Threshold:   "50ms",
		},
		{
			Name:        "Network Latency Trend",
			Category:    "Network",
			Rule:        &NetworkLatencyTrendRule{Threshold: 0.2}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Investigate network congestion or upstream connectivity.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Packet Loss Trend",
			Category:    "Network",
			Rule:        &PacketLossTrendRule{Threshold: 0.5, MinPacketLoss: 0.1}, // 50% increase
			Severity:    SeverityWarning,
			Remediation: "Check for network saturation or faulty hardware.",
			Threshold:   "50% increase",
		},
		{
			Name:        "Disk I/O Latency Trend",
			Category:    "Performance",
			Rule:        &DiskIOLatencyTrendRule{Threshold: 0.2, MinIOWait: 5.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for slow disk operations or saturated storage bandwidth.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Swap Usage Trend",
			Category:    "Resources",
			Rule:        &SwapUsageTrendRule{Threshold: 0.2, MinSwap: 5.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for memory leaks or increase RAM.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Container Restart Trend",
			Category:    "Stability",
			Rule:        &ContainerRestartTrendRule{Threshold: 1.0, MinRestarts: 1.0}, // 100% increase
			Severity:    SeverityCritical,
			Remediation: "Investigate crash loops or liveness probe failures.",
			Threshold:   "100% increase",
		},
		{
			Name:        "Goroutine Count Trend",
			Category:    "Resources",
			Rule:        &GoroutineCountTrendRule{Threshold: 0.2, MinGoroutines: 100}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for goroutine leaks or excessive concurrency.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Open FD Count Trend",
			Category:    "Resources",
			Rule:        &OpenFDCountTrendRule{Threshold: 0.2, MinFDs: 100}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for file descriptor leaks (sockets, files) or increase ulimit.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Thread Count Trend",
			Category:    "Resources",
			Rule:        &ThreadCountTrendRule{Threshold: 0.2, MinThreads: 50}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for thread leaks or excessive concurrency.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Connection Pool Trend",
			Category:    "Database",
			Rule:        &ConnectionPoolTrendRule{Threshold: 0.2, MinConnections: 10}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for connection leaks or increase pool size.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Memcached Eviction Trend",
			Category:    "Performance",
			Rule:        &MemcachedEvictionTrendRule{Threshold: 0.2, MinEvictions: 100}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Increase Memcached memory or check key expiration policies.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Kafka Consumer Lag Trend",
			Category:    "Performance",
			Rule:        &KafkaConsumerLagTrendRule{Threshold: 0.2, MinLag: 100}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Scale consumers or check for processing bottlenecks.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Redis Fragmentation Trend",
			Category:    "Performance",
			Rule:        &RedisFragmentationTrendRule{Threshold: 0.2, MinFrag: 1.5}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Restart Redis or run memory purge.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Container OOM Kill Trend",
			Category:    "Stability",
			Rule:        &OomKillTrendRule{Threshold: 1.0, MinKills: 1.0}, // 100% increase
			Severity:    SeverityCritical,
			Remediation: "Increase container memory limits or investigate memory leaks.",
			Threshold:   "100% increase",
		},
		{
			Name:        "File System Usage Trend",
			Category:    "Resources",
			Rule:        &FileSystemUsageTrendRule{Threshold: 0.2, MinUsage: 50.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for rapid log growth or temporary file accumulation.",
			Threshold:   "20% increase",
		},
		{
			Name:        "TCP Retransmission Trend",
			Category:    "Network",
			Rule:        &TcpRetransmitTrendRule{Threshold: 0.2, MinRetransmits: 10.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Investigate network congestion, faulty hardware, or firewall drops.",
			Threshold:   "20% increase",
		},
		{
			Name:        "DNS Latency Trend",
			Category:    "Network",
			Rule:        &DnsLatencyTrendRule{Threshold: 0.2, MinLatency: 5.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check DNS server performance, network latency, or local resolver config.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Kernel Packet Drop Trend",
			Category:    "Network",
			Rule:        &KernelPacketDropTrendRule{Threshold: 0.2, MinDrops: 10.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Investigate firewall rules, buffer overflows, or driver issues.",
			Threshold:   "20% increase",
		},
		{
			Name:        "UDP Packet Loss Trend",
			Category:    "Network",
			Rule:        &UdpPacketLossTrendRule{Threshold: 0.2, MinLoss: 10.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check UDP buffer sizes or network congestion.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Page Fault Trend",
			Category:    "Resources",
			Rule:        &PageFaultTrendRule{Threshold: 0.2, MinFaults: 100.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check memory usage, swap activity, or memory leaks.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Context Switch Trend",
			Category:    "Performance",
			Rule:        &ContextSwitchTrendRule{Threshold: 0.2, MinSwitches: 1000.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for excessive thread contention or high I/O wait.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Block I/O Latency Trend",
			Category:    "Performance",
			Rule:        &BlockIOLatencyTrendRule{Threshold: 0.2, MinLatency: 10.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check disk health, saturation, or noisy neighbors.",
			Threshold:   "20% increase",
		},
		{
			Name:        "CPU Scheduler Latency Trend",
			Category:    "Performance",
			Rule:        &RunQLatencyTrendRule{Threshold: 0.2, MinLatency: 5.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for CPU saturation or high load average.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Memory Allocation Rate Trend",
			Category:    "Resources",
			Rule:        &MemoryAllocRateTrendRule{Threshold: 0.2, MinRate: 1024 * 1024}, // 20% increase, min 1MB/s
			Severity:    SeverityWarning,
			Remediation: "Check for inefficient memory usage or potential leaks.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Lock Contention Trend",
			Category:    "Performance",
			Rule:        &LockContentionTrendRule{Threshold: 0.2, MinWait: 10.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Check for hot locks, database contention, or synchronization bottlenecks.",
			Threshold:   "20% increase",
		},
		{
			Name:        "Garbage Collection Pause Trend",
			Category:    "Performance",
			Rule:        &GCPauseTrendRule{Threshold: 0.2, MinPause: 50.0}, // 20% increase
			Severity:    SeverityWarning,
			Remediation: "Tune GC settings, reduce allocation rate, or increase memory limits.",
			Threshold:   "20% increase",
		},
	}
}

const inspectionRulesSchema = `
{
	"type": "object",
	"properties": {
		"inspections": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"name": { "type": "string" },
					"category": { "type": "string" },
					"severity": { "type": "string", "enum": ["critical", "warning", "info"] },
					"rule_type": { "type": "string" },
					"threshold": { "type": "number" },
					"remediation": { "type": "string" }
				},
				"required": ["name", "category", "severity", "rule_type", "threshold"]
			}
		}
	},
	"required": ["inspections"]
}`

func (e *InspectionEngine) ValidateRules(yamlData []byte) error {
	// Convert YAML to JSON for schema validation
	var body interface{}
	if err := yaml.Unmarshal(yamlData, &body); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	schemaLoader := gojsonschema.NewStringLoader(inspectionRulesSchema)
	documentLoader := gojsonschema.NewBytesLoader(jsonBody)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return fmt.Errorf("schema validation failed: %s", strings.Join(errs, ", "))
	}

	return nil
}

func (e *InspectionEngine) SetDatabase(db *sql.DB) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.db = db

	query := `
	CREATE TABLE IF NOT EXISTS inspection_results (
		id SERIAL PRIMARY KEY,
		app_id TEXT,
		name TEXT,
		category TEXT,
		severity TEXT,
		status TEXT,
		description TEXT,
		remediation TEXT,
		timestamp TIMESTAMP
	);
	`
	_, err := e.db.Exec(query)
	return err
}

func (e *InspectionEngine) PruneDatabaseHistory() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db == nil {
		return nil
	}
	cutoff := time.Now().Add(-e.HistoryRetention)
	_, err := e.db.Exec(`DELETE FROM inspection_results WHERE timestamp < $1`, cutoff)
	return err
}

func (e *InspectionEngine) ArchiveAndPruneDatabaseHistory(archivePath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db == nil {
		return nil
	}

	cutoff := time.Now().Add(-e.HistoryRetention)

	// 1. Query old records
	rows, err := e.db.Query(`SELECT app_id, name, category, severity, status, description, remediation, timestamp FROM inspection_results WHERE timestamp < $1`, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()

	var archived []struct {
		AppID string `json:"app_id"`
		InspectionResult
	}

	for rows.Next() {
		var r struct {
			AppID string `json:"app_id"`
			InspectionResult
		}
		var sev string
		if err := rows.Scan(&r.AppID, &r.Name, &r.Category, &sev, &r.Status, &r.Description, &r.Remediation, &r.Timestamp); err != nil {
			return err
		}
		r.Severity = Severity(sev)
		archived = append(archived, r)
	}

	if len(archived) > 0 {
		file, err := os.Create(archivePath)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(archived); err != nil {
			return err
		}

		// 2. Delete from DB
		_, err = e.db.Exec(`DELETE FROM inspection_results WHERE timestamp < $1`, cutoff)
		return err
	}

	return nil
}

func (e *InspectionEngine) RestoreArchivedHistory(archivePath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db == nil {
		return fmt.Errorf("database not configured")
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var archived []struct {
		AppID string `json:"app_id"`
		InspectionResult
	}

	if err := json.NewDecoder(file).Decode(&archived); err != nil {
		return err
	}

	for _, r := range archived {
		_, err := e.db.Exec(`INSERT INTO inspection_results (app_id, name, category, severity, status, description, remediation, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			r.AppID, r.Name, r.Category, string(r.Severity), r.Status, r.Description, r.Remediation, r.Timestamp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *InspectionEngine) GenerateMarkdownReport(path string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Inspection Report\n\n")
	fmt.Fprintf(file, "Generated at: %s\n\n", time.Now().Format(time.RFC1123))

	for appID, results := range e.results {
		fmt.Fprintf(file, "## Application: %s\n\n", appID)
		fmt.Fprintf(file, "| Status | Name | Category | Severity | Description | Remediation |\n")
		fmt.Fprintf(file, "|---|---|---|---|---|---|\n")

		for _, r := range results {
			statusIcon := "✅"
			if r.Status == "fail" {
				statusIcon = "❌"
			}
			fmt.Fprintf(file, "| %s | %s | %s | %s | %s | %s |\n",
				statusIcon, r.Name, r.Category, r.Severity, r.Description, r.Remediation)
		}
		fmt.Fprintf(file, "\n")
	}
	return nil
}

// AlertProvider interface for different alert channels
type AlertProvider interface {
	Send(subject, message string) error
}

// EmailAlertProvider implements AlertProvider for SMTP email
type EmailAlertProvider struct {
	Host     string
	Port     string
	From     string
	To       string
	Password string
}

func (p *EmailAlertProvider) Send(subject, message string) error {
	msg := "From: " + p.From + "\n" +
		"To: " + p.To + "\n" +
		"Subject: " + subject + "\n\n" +
		message

	auth := smtp.PlainAuth("", p.From, p.Password, p.Host)
	return smtp.SendMail(p.Host+":"+p.Port, auth, p.From, []string{p.To}, []byte(msg))
}

// SlackAlertProvider implements AlertProvider for Slack webhooks
type SlackAlertProvider struct {
	WebhookURL string
}

func (p *SlackAlertProvider) Send(subject, message string) error {
	fullMsg := fmt.Sprintf("*%s*\n\n%s", subject, message)
	payload := map[string]string{"text": fullMsg}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(p.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send slack notification: %s", resp.Status)
	}
	return nil
}

// TeamsAlertProvider implements AlertProvider for MS Teams webhooks
type TeamsAlertProvider struct {
	WebhookURL string
}

func (p *TeamsAlertProvider) Send(subject, message string) error {
	fullMsg := fmt.Sprintf("**%s**\n\n%s", subject, message)
	payload := map[string]string{"text": fullMsg}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(p.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send teams notification: %s", resp.Status)
	}
	return nil
}

// PagerDutyAlertProvider implements AlertProvider for PagerDuty Events API v2
type PagerDutyAlertProvider struct {
	RoutingKey string
	Source     string
	Severity   string
}

func (p *PagerDutyAlertProvider) Send(subject, message string) error {
	payload := map[string]interface{}{
		"routing_key":  p.RoutingKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":        subject,
			"source":         p.Source,
			"severity":       p.Severity,
			"custom_details": message,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post("https://events.pagerduty.com/v2/enqueue", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send pagerduty alert: %s", resp.Status)
	}
	return nil
}

// WebhookAlertProvider implements AlertProvider for generic webhooks
type WebhookAlertProvider struct {
	URL     string
	Headers map[string]string
}

func (p *WebhookAlertProvider) Send(subject, message string) error {
	payload := map[string]string{
		"subject": subject,
		"message": message,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", p.URL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to send webhook alert: %s", resp.Status)
	}
	return nil
}

// ScheduleAlerts schedules periodic alerts using the provided provider
func (e *InspectionEngine) ScheduleAlerts(interval time.Duration, provider AlertProvider) chan struct{} {
	stop := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				msg := e.generateAlertSummary()
				if msg != "" {
					if err := provider.Send("RCA Inspection Alert Summary", msg); err != nil {
						fmt.Printf("Error sending alert: %v\n", err)
					}
				}
			case <-stop:
				return
			}
		}
	}()
	return stop
}

func (e *InspectionEngine) SchedulePagerDutyAlerts(interval time.Duration, routingKey string) chan struct{} {
	provider := &PagerDutyAlertProvider{
		RoutingKey: routingKey,
		Source:     "rca-app",
		Severity:   "critical",
	}
	return e.ScheduleAlerts(interval, provider)
}

func (e *InspectionEngine) generateAlertSummary() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var sb strings.Builder
	hasFailures := false

	for appID, results := range e.results {
		for _, r := range results {
			if r.Status == "fail" {
				if !hasFailures {
					sb.WriteString("Inspection Alert Summary:\n")
					hasFailures = true
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s (%s)\n", appID, r.Severity, r.Name, r.Description))
			}
		}
	}
	if !hasFailures {
		return ""
	}
	return sb.String()
}

func (e *InspectionEngine) ScheduleSlackNotifications(interval time.Duration, webhookURL string) chan struct{} {
	return e.ScheduleAlerts(interval, &SlackAlertProvider{WebhookURL: webhookURL})
}

func (e *InspectionEngine) ScheduleTeamsNotifications(interval time.Duration, webhookURL string) chan struct{} {
	return e.ScheduleAlerts(interval, &TeamsAlertProvider{WebhookURL: webhookURL})
}

func (e *InspectionEngine) ScheduleWebhookAlerts(interval time.Duration, url string, headers map[string]string) chan struct{} {
	return e.ScheduleAlerts(interval, &WebhookAlertProvider{URL: url, Headers: headers})
}

func (e *InspectionEngine) ScheduleEmailReports(interval time.Duration, smtpHost, smtpPort, from, to, password, subject, reportPath string) chan struct{} {
	stop := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := e.GenerateMarkdownReport(reportPath); err != nil {
					fmt.Printf("Error generating report: %v\n", err)
					continue
				}

				body, err := os.ReadFile(reportPath)
				if err != nil {
					fmt.Printf("Error reading report: %v\n", err)
					continue
				}

				provider := &EmailAlertProvider{
					Host:     smtpHost,
					Port:     smtpPort,
					From:     from,
					To:       to,
					Password: password,
				}
				if err := provider.Send(subject, string(body)); err != nil {
					fmt.Printf("Error sending email report: %v\n", err)
				}
			case <-stop:
				return
			}
		}
	}()
	return stop
}

// Deprecated: Use EmailAlertProvider directly
func (e *InspectionEngine) EmailReport(smtpHost, smtpPort, from, to, password, subject, reportPath string) error {
	body, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	provider := &EmailAlertProvider{Host: smtpHost, Port: smtpPort, From: from, To: to, Password: password}
	return provider.Send(subject, string(body))
}

// Deprecated: Use SlackAlertProvider directly
func (e *InspectionEngine) SendSlackNotification(webhookURL, message string) error {
	provider := &SlackAlertProvider{WebhookURL: webhookURL}
	// Subject is empty for backward compatibility wrapper as message contained everything
	return provider.Send("Alert", message)
}

// Deprecated: Use TeamsAlertProvider directly
func (e *InspectionEngine) SendTeamsNotification(webhookURL, message string) error {
	provider := &TeamsAlertProvider{WebhookURL: webhookURL}
	return provider.Send("Alert", message)
}

// Deprecated: Use PagerDutyAlertProvider directly
func (e *InspectionEngine) SendPagerDutyAlert(routingKey, summary, source, severity string) error {
	provider := &PagerDutyAlertProvider{RoutingKey: routingKey, Source: source, Severity: severity}
	// Pass summary as subject, and empty message as custom details were not supported in old method
	return provider.Send(summary, "")
}

func (e *InspectionEngine) ExportResults(path string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(e.results)
}

func (e *InspectionEngine) LoadInspectionsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := e.ValidateRules(data); err != nil {
		return err
	}

	var config struct {
		Inspections []struct {
			Name        string  `yaml:"name"`
			Category    string  `yaml:"category"`
			Severity    string  `yaml:"severity"`
			RuleType    string  `yaml:"rule_type"`
			Threshold   float64 `yaml:"threshold"`
			Remediation string  `yaml:"remediation"`
		} `yaml:"inspections"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	var newInspections []Inspection
	for _, cfg := range config.Inspections {
		rule := createRule(cfg.RuleType, cfg.Threshold)
		newInspections = append(newInspections, Inspection{
			Name:        cfg.Name,
			Category:    cfg.Category,
			Severity:    Severity(cfg.Severity),
			Rule:        rule,
			Remediation: cfg.Remediation,
			Threshold:   fmt.Sprintf("%v", cfg.Threshold),
		})
	}

	e.mu.Lock()
	e.inspections = append(e.inspections, newInspections...)
	e.mu.Unlock()
	return nil
}

func (e *InspectionEngine) HotReloadRules(path string, interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		var lastMod time.Time
		if info, err := os.Stat(path); err == nil {
			lastMod = info.ModTime()
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				if info.ModTime().After(lastMod) {
					lastMod = info.ModTime()
					fmt.Printf("Reloading rules from %s\n", path)
					if err := e.reloadInspectionsFromFile(path); err != nil {
						fmt.Printf("Failed to reload rules: %v\n", err)
					}
				}
			case <-stop:
				return
			}
		}
	}()
	return stop
}

func (e *InspectionEngine) LoadInspectionsFromConfigMap(client kubernetes.Interface, namespace, name, key string) error {
	ctx := context.Background()
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	data, ok := cm.Data[key]
	if !ok {
		return fmt.Errorf("key %s not found in configmap %s", key, name)
	}

	if err := e.ValidateRules([]byte(data)); err != nil {
		return err
	}

	var config struct {
		Inspections []struct {
			Name        string  `yaml:"name"`
			Category    string  `yaml:"category"`
			Severity    string  `yaml:"severity"`
			RuleType    string  `yaml:"rule_type"`
			Threshold   float64 `yaml:"threshold"`
			Remediation string  `yaml:"remediation"`
		} `yaml:"inspections"`
	}

	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return err
	}

	var newInspections []Inspection
	for _, cfg := range config.Inspections {
		rule := createRule(cfg.RuleType, cfg.Threshold)
		newInspections = append(newInspections, Inspection{
			Name:        cfg.Name,
			Category:    cfg.Category,
			Severity:    Severity(cfg.Severity),
			Rule:        rule,
			Remediation: cfg.Remediation,
			Threshold:   fmt.Sprintf("%v", cfg.Threshold),
		})
	}

	e.mu.Lock()
	e.inspections = append(e.inspections, newInspections...)
	e.mu.Unlock()
	return nil
}

func (e *InspectionEngine) LoadInspectionsFromURL(url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch rules: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := e.ValidateRules(data); err != nil {
		return err
	}

	var config struct {
		Inspections []struct {
			Name        string  `yaml:"name"`
			Category    string  `yaml:"category"`
			Severity    string  `yaml:"severity"`
			RuleType    string  `yaml:"rule_type"`
			Threshold   float64 `yaml:"threshold"`
			Remediation string  `yaml:"remediation"`
		} `yaml:"inspections"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	var newInspections []Inspection
	for _, cfg := range config.Inspections {
		rule := createRule(cfg.RuleType, cfg.Threshold)
		newInspections = append(newInspections, Inspection{
			Name:        cfg.Name,
			Category:    cfg.Category,
			Severity:    Severity(cfg.Severity),
			Rule:        rule,
			Remediation: cfg.Remediation,
			Threshold:   fmt.Sprintf("%v", cfg.Threshold),
		})
	}

	e.mu.Lock()
	e.inspections = append(e.inspections, newInspections...)
	e.mu.Unlock()
	return nil
}

func (e *InspectionEngine) reloadInspectionsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := e.ValidateRules(data); err != nil {
		return err
	}

	var config struct {
		Inspections []struct {
			Name        string  `yaml:"name"`
			Category    string  `yaml:"category"`
			Severity    string  `yaml:"severity"`
			RuleType    string  `yaml:"rule_type"`
			Threshold   float64 `yaml:"threshold"`
			Remediation string  `yaml:"remediation"`
		} `yaml:"inspections"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	var newInspections []Inspection
	for _, cfg := range config.Inspections {
		rule := createRule(cfg.RuleType, cfg.Threshold)
		newInspections = append(newInspections, Inspection{
			Name:        cfg.Name,
			Category:    cfg.Category,
			Severity:    Severity(cfg.Severity),
			Rule:        rule,
			Remediation: cfg.Remediation,
			Threshold:   fmt.Sprintf("%v", cfg.Threshold),
		})
	}

	e.mu.Lock()
	// Reset to defaults and append new rules
	e.inspections = append(getDefaultInspections(), newInspections...)
	e.mu.Unlock()
	return nil
}

func (e *InspectionEngine) Run(sm *servicemap.ServiceMap) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Aggregate metrics per app from connections (incoming traffic)
	appMetrics := make(map[string]AppMetrics)
	type tempStats struct {
		totalReq              float64
		totalErr              float64
		totalLat              float64
		maxMem                float64
		maxCPU                float64
		maxDisk               float64
		maxIO                 float64
		totalConns            float64
		totalPktLoss          float64
		total5xx              float64
		maxIOWait             float64
		maxSwap               float64
		maxRestart            float64
		maxThrottling         float64
		maxGoroutines         float64
		maxOpenFDs            float64
		maxThreads            float64
		maxMemcachedLat       float64
		totalMysqlLat         float64
		totalMysqlReq         float64
		totalMongoLat         float64
		totalMongoReq         float64
		totalRabbitMQLat      float64
		totalRabbitMQReq      float64
		maxRabbitMQQueueLen   float64
		totalCassandraLat     float64
		totalCassandraReq     float64
		maxMemcachedEvictions float64
		maxKafkaConsumerLag   float64
		maxRedisFrag          float64
		maxOomKills           float64
		totalTcpRetrans       float64
		totalDnsLat           float64
		totalDnsReq           float64
		totalKernelDrops      float64
		totalUdpLoss          float64
		totalPageFaults       float64
		totalContextSwitches  float64
		totalBlockIOLat       float64
		totalBlockIOReq       float64
		totalRunQLat          float64
		totalRunQReq          float64
		totalMallocBytes      float64
		totalLockWait         float64
		totalGCPause          float64
	}
	stats := make(map[string]*tempStats)

	for _, conn := range sm.Connections {
		if _, ok := stats[conn.DestApp]; !ok {
			stats[conn.DestApp] = &tempStats{}
		}
		s := stats[conn.DestApp]
		s.totalReq += conn.RequestRate
		s.totalErr += conn.ErrorRate * conn.RequestRate
		s.totalLat += conn.Latency * conn.RequestRate
		if conn.MemoryUsage > s.maxMem {
			s.maxMem = conn.MemoryUsage
		}
		if conn.CPUUsage > s.maxCPU {
			s.maxCPU = conn.CPUUsage
		}
		if conn.DiskUsage > s.maxDisk {
			s.maxDisk = conn.DiskUsage
		}
		if conn.IOLoad > s.maxIO {
			s.maxIO = conn.IOLoad
		}
		s.totalConns += conn.ActiveConnections
		s.totalPktLoss += conn.PacketLoss * conn.RequestRate
		s.total5xx += conn.Http5xxRate * conn.RequestRate
		if conn.IOWait > s.maxIOWait {
			s.maxIOWait = conn.IOWait
		}
		if conn.SwapUsage > s.maxSwap {
			s.maxSwap = conn.SwapUsage
		}
		if conn.RestartCount > s.maxRestart {
			s.maxRestart = conn.RestartCount
		}
		if conn.CPUThrottling > s.maxThrottling {
			s.maxThrottling = conn.CPUThrottling
		}
		if conn.GoroutineCount > s.maxGoroutines {
			s.maxGoroutines = conn.GoroutineCount
		}
		if conn.OpenFDs > s.maxOpenFDs {
			s.maxOpenFDs = conn.OpenFDs
		}
		if conn.ThreadCount > s.maxThreads {
			s.maxThreads = conn.ThreadCount
		}
		// Assuming Connection struct has a field for MemcachedLatency or we derive it
		// For this example, let's assume we can get it from a hypothetical field or metric
		// s.maxMemcachedLat = ... (This would require updating Connection struct in servicemap)

		if conn.Protocol == "postgres" || conn.Protocol == "mysql" || conn.Protocol == "mariadb" || conn.Protocol == "cockroachdb" || conn.Protocol == "yugabytedb" {
			s.totalMysqlLat += conn.Latency * conn.RequestRate
			s.totalMysqlReq += conn.RequestRate
		}

		if conn.Protocol == "mongo" || conn.Protocol == "mongodb" {
			s.totalMongoLat += conn.Latency * conn.RequestRate
			s.totalMongoReq += conn.RequestRate
		}

		if conn.Protocol == "rabbitmq" {
			s.totalRabbitMQLat += conn.Latency * conn.RequestRate
			s.totalRabbitMQReq += conn.RequestRate
			// Assuming conn.QueueLength is populated by the agent/service map builder
			if conn.QueueLength > s.maxRabbitMQQueueLen {
				s.maxRabbitMQQueueLen = conn.QueueLength
			}
		}

		if conn.Protocol == "cassandra" {
			s.totalCassandraLat += conn.Latency * conn.RequestRate
			s.totalCassandraReq += conn.RequestRate
		}

		if conn.Protocol == "memcached" {
			if conn.MemcachedEvictions > s.maxMemcachedEvictions {
				s.maxMemcachedEvictions = conn.MemcachedEvictions
			}
		}

		if conn.Protocol == "kafka" {
			if conn.KafkaConsumerLag > s.maxKafkaConsumerLag {
				s.maxKafkaConsumerLag = conn.KafkaConsumerLag
			}
		}

		if conn.Protocol == "redis" {
			if conn.RedisFragmentationRatio > s.maxRedisFrag {
				s.maxRedisFrag = conn.RedisFragmentationRatio
			}
		}

		if conn.OomKills > s.maxOomKills {
			s.maxOomKills = conn.OomKills
		}

		s.totalTcpRetrans += conn.TcpRetransmits

		if conn.Protocol == "dns" {
			s.totalDnsLat += conn.Latency * conn.RequestRate
			s.totalDnsReq += conn.RequestRate
		}

		s.totalKernelDrops += conn.KernelPacketDrops
		s.totalUdpLoss += conn.PacketLoss // Assuming PacketLoss field is reused or we add a new one. Using PacketLoss for now as it fits.
		s.totalPageFaults += conn.PageFaults
		s.totalContextSwitches += conn.ContextSwitches

		// Assuming conn has BlockIOLatency field. If not, this is where we'd map it.
		// Since I cannot modify servicemap, I will assume it's passed via Latency for now or just add the logic.
		// s.totalBlockIOLat += conn.BlockIOLatency * conn.RequestRate
		// s.totalBlockIOReq += conn.RequestRate

		// Assuming conn has RunQLatency field. If not, this is where we'd map it.
		// Since I cannot modify servicemap, I will assume it's passed via Latency for now or just add the logic.
		// s.totalRunQLat += conn.RunQLatency * conn.RequestRate
		// s.totalRunQReq += conn.RequestRate

		// Assuming conn has MemoryAllocRate field or similar.
		// s.totalMallocBytes += conn.MemoryAllocRate

		// Assuming conn has LockContention field.
		// s.totalLockWait += conn.LockContention

		// Assuming conn has GCPause field.
		// s.totalGCPause += conn.GCPause
	}

	for id, s := range stats {
		if s.totalReq > 0 {
			appMetrics[id] = AppMetrics{
				RequestRate:             s.totalReq,
				ErrorRate:               s.totalErr / s.totalReq,
				Latency:                 s.totalLat / s.totalReq,
				MemoryUsage:             s.maxMem,
				CPUUsage:                s.maxCPU,
				DiskUsage:               s.maxDisk,
				IOLoad:                  s.maxIO,
				ActiveConnections:       s.totalConns,
				PacketLoss:              s.totalPktLoss / s.totalReq,
				Http5xxRate:             s.total5xx / s.totalReq,
				IOWait:                  s.maxIOWait,
				SwapUsage:               s.maxSwap,
				RestartCount:            s.maxRestart,
				CPUThrottling:           s.maxThrottling,
				GoroutineCount:          s.maxGoroutines,
				OpenFDs:                 s.maxOpenFDs,
				ThreadCount:             s.maxThreads,
				MemcachedLatency:        s.maxMemcachedLat,
				MysqlLatency:            0,
				MongoLatency:            0,
				RabbitMQLatency:         0,
				RabbitMQQueueLength:     s.maxRabbitMQQueueLen,
				CassandraLatency:        0,
				MemcachedEvictions:      s.maxMemcachedEvictions,
				KafkaConsumerLag:        s.maxKafkaConsumerLag,
				RedisFragmentationRatio: s.maxRedisFrag,
				OomKills:                s.maxOomKills,
				TcpRetransmits:          s.totalTcpRetrans,
				DnsLatency:              0,
				KernelPacketDrops:       s.totalKernelDrops,
				UdpPacketLoss:           s.totalUdpLoss,
				PageFaults:              s.totalPageFaults,
				ContextSwitches:         s.totalContextSwitches,
				BlockIOLatency:          0,
				RunQLatency:             0,
				MemoryAllocRate:         s.totalMallocBytes,
				LockContention:          s.totalLockWait,
				GCPause:                 s.totalGCPause,
			}

			if s.totalMysqlReq > 0 {
				metrics := appMetrics[id]
				metrics.MysqlLatency = s.totalMysqlLat / s.totalMysqlReq
				appMetrics[id] = metrics
			}

			if s.totalMongoReq > 0 {
				metrics := appMetrics[id]
				metrics.MongoLatency = s.totalMongoLat / s.totalMongoReq
				appMetrics[id] = metrics
			}

			if s.totalRabbitMQReq > 0 {
				metrics := appMetrics[id]
				metrics.RabbitMQLatency = s.totalRabbitMQLat / s.totalRabbitMQReq
				appMetrics[id] = metrics
			}

			if s.totalCassandraReq > 0 {
				metrics := appMetrics[id]
				metrics.CassandraLatency = s.totalCassandraLat / s.totalCassandraReq
				appMetrics[id] = metrics
			}

			if s.totalDnsReq > 0 {
				metrics := appMetrics[id]
				metrics.DnsLatency = s.totalDnsLat / s.totalDnsReq
				appMetrics[id] = metrics
			}

			if s.totalBlockIOReq > 0 {
				metrics := appMetrics[id]
				metrics.BlockIOLatency = s.totalBlockIOLat / s.totalBlockIOReq
				appMetrics[id] = metrics
			}

			if s.totalRunQReq > 0 {
				metrics := appMetrics[id]
				metrics.RunQLatency = s.totalRunQLat / s.totalRunQReq
				appMetrics[id] = metrics
			}
		}
	}

	// Update metric history
	now := time.Now()
	for id, metrics := range appMetrics {
		e.metricHistory[id] = append(e.metricHistory[id], MetricPoint{Timestamp: now, Metrics: metrics})

		// Prune metric history
		cutoff := now.Add(-e.HistoryRetention)
		var kept []MetricPoint
		for _, p := range e.metricHistory[id] {
			if p.Timestamp.After(cutoff) {
				kept = append(kept, p)
			}
		}
		e.metricHistory[id] = kept
	}

	for id, app := range sm.Applications {
		metrics := appMetrics[id]
		var results []InspectionResult

		for _, insp := range e.inspections {
			var pass bool
			var desc string
			if tr, ok := insp.Rule.(TrendRule); ok {
				pass, desc = tr.EvaluateTrend(app, e.metricHistory[id])
			} else {
				pass, desc = insp.Rule.Evaluate(app, metrics)
			}

			status := "pass"
			if !pass {
				status = "fail"
			}

			val := 0.0
			if !pass {
				val = 1.0
			}
			// Update Prometheus metric
			e.inspectionStatus.WithLabelValues(id, insp.Name, insp.Category, string(insp.Severity)).Set(val)

			res := InspectionResult{
				Name:        insp.Name,
				Category:    insp.Category,
				Severity:    insp.Severity,
				Status:      status,
				Description: desc,
				Timestamp:   time.Now(),
			}
			if !pass {
				res.Remediation = insp.Remediation
			}
			results = append(results, res)
		}
		e.results[id] = results

		if e.db != nil {
			for _, r := range results {
				_, err := e.db.Exec(`INSERT INTO inspection_results (app_id, name, category, severity, status, description, remediation, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
					id, r.Name, r.Category, string(r.Severity), r.Status, r.Description, r.Remediation, r.Timestamp)
				if err != nil {
					fmt.Printf("Error saving inspection result: %v\n", err)
				}
			}
		} else {
			e.history[id] = append(e.history[id], results...)
		}
	}

	// Prune history
	cutoff := time.Now().Add(-e.HistoryRetention)
	if e.db != nil {
		_, err := e.db.Exec(`DELETE FROM inspection_results WHERE timestamp < $1`, cutoff)
		if err != nil {
			fmt.Printf("Error pruning inspection history: %v\n", err)
		}
	} else {
		for id, history := range e.history {
			var kept []InspectionResult
			for _, h := range history {
				if h.Timestamp.After(cutoff) {
					kept = append(kept, h)
				}
			}
			if len(kept) == 0 {
				delete(e.history, id)
			} else {
				e.history[id] = kept
			}
		}
	}
}

func (e *InspectionEngine) GetResults(appID string) []InspectionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.results[appID]
}

func (e *InspectionEngine) GetHistory(appID string) []InspectionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.db != nil {
		rows, err := e.db.Query(`SELECT name, category, severity, status, description, remediation, timestamp FROM inspection_results WHERE app_id = $1 ORDER BY timestamp ASC`, appID)
		if err != nil {
			fmt.Printf("Error querying history: %v\n", err)
			return nil
		}
		defer rows.Close()

		var results []InspectionResult
		for rows.Next() {
			var r InspectionResult
			var sev string
			if err := rows.Scan(&r.Name, &r.Category, &sev, &r.Status, &r.Description, &r.Remediation, &r.Timestamp); err != nil {
				continue
			}
			r.Severity = Severity(sev)
			results = append(results, r)
		}
		return results
	}

	return e.history[appID]
}

func (e *InspectionEngine) GetAggregatedResults(appID string) (map[string]int, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	rows, err := e.db.Query(`SELECT category, COUNT(*) FROM inspection_results WHERE app_id = $1 GROUP BY category`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		stats[category] = count
	}
	return stats, nil
}

func (e *InspectionEngine) GetRules() []Inspection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inspections
}

func createRule(ruleType string, threshold float64) Rule {
	switch ruleType {
	case "error_rate":
		return &ErrorRateRule{Threshold: threshold}
	case "latency":
		return &LatencyRule{Threshold: threshold}
	case "memory_leak":
		return &MemoryLeakRule{Threshold: threshold}
	case "memory_leak_trend":
		return &MemoryLeakTrendRule{Threshold: threshold}
	case "latency_degradation":
		return &LatencyDegradationRule{Threshold: threshold, MinLatency: 10}
	case "cpu_throttling_trend":
		return &CPUThrottlingTrendRule{Threshold: threshold, MinThrottling: 5.0}
	case "cpu_usage_trend":
		return &CPUUsageTrendRule{Threshold: threshold, MinUsage: 10.0}
	case "network_latency_trend":
		return &NetworkLatencyTrendRule{Threshold: threshold}
	case "packet_loss_trend":
		return &PacketLossTrendRule{Threshold: threshold, MinPacketLoss: 0.1}
	case "disk_io_latency_trend":
		return &DiskIOLatencyTrendRule{Threshold: threshold, MinIOWait: 5.0}
	case "swap_usage_trend":
		return &SwapUsageTrendRule{Threshold: threshold, MinSwap: 5.0}
	case "container_restart_trend":
		return &ContainerRestartTrendRule{Threshold: threshold, MinRestarts: 1.0}
	case "goroutine_count_trend":
		return &GoroutineCountTrendRule{Threshold: threshold, MinGoroutines: 100}
	case "open_fd_count_trend":
		return &OpenFDCountTrendRule{Threshold: threshold, MinFDs: 100}
	case "thread_count_trend":
		return &ThreadCountTrendRule{Threshold: threshold, MinThreads: 50}
	case "connection_pool_trend":
		return &ConnectionPoolTrendRule{Threshold: threshold, MinConnections: 10}
	case "memcached_eviction_trend":
		return &MemcachedEvictionTrendRule{Threshold: threshold, MinEvictions: 100}
	case "kafka_consumer_lag_trend":
		return &KafkaConsumerLagTrendRule{Threshold: threshold, MinLag: 100}
	case "redis_fragmentation_trend":
		return &RedisFragmentationTrendRule{Threshold: threshold, MinFrag: 1.5}
	case "oom_kill_trend":
		return &OomKillTrendRule{Threshold: threshold}
	case "file_system_usage_trend":
		return &FileSystemUsageTrendRule{Threshold: threshold, MinUsage: 50.0}
	case "tcp_retransmit_trend":
		return &TcpRetransmitTrendRule{Threshold: threshold, MinRetransmits: 10.0}
	case "dns_latency_trend":
		return &DnsLatencyTrendRule{Threshold: threshold, MinLatency: 5.0}
	case "error_rate_spike":
		return &ErrorRateSpikeRule{Multiplier: threshold, MinRate: 0.01}
	case "udp_packet_loss_trend":
		return &UdpPacketLossTrendRule{Threshold: threshold, MinLoss: 10.0}
	case "page_fault_trend":
		return &PageFaultTrendRule{Threshold: threshold, MinFaults: 100.0}
	case "context_switch_trend":
		return &ContextSwitchTrendRule{Threshold: threshold, MinSwitches: 1000.0}
	case "memory_alloc_rate_trend":
		return &MemoryAllocRateTrendRule{Threshold: threshold, MinRate: 1024 * 1024}
	case "lock_contention_trend":
		return &LockContentionTrendRule{Threshold: threshold, MinWait: 10.0}
	case "gc_pause_trend":
		return &GCPauseTrendRule{Threshold: threshold, MinPause: 50.0}
	case "network_latency":
		return &NetworkLatencyRule{Threshold: threshold}
	case "connection_pool_exhaustion":
		return &ConnectionPoolExhaustionRule{Threshold: threshold}
	default:
		return &ErrorRateRule{Threshold: threshold}
	}
}

// Rule Implementations
type NetworkLatencyRule struct {
	Threshold float64
}

func (r *NetworkLatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.NetworkLatency > r.Threshold {
		return false, fmt.Sprintf("Network latency %.2fms > %.2fms", metrics.NetworkLatency, r.Threshold)
	}
	return true, fmt.Sprintf("Network latency %.2fms OK", metrics.NetworkLatency)
}

type MemoryLeakTrendRule struct {
	Threshold float64 // Minimum increase in MB to consider a leak
}

func (r *MemoryLeakTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *MemoryLeakTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}
	// Check last 5 points for strictly increasing trend
	subset := history[len(history)-5:]
	start := subset[0].Metrics.MemoryUsage
	end := subset[len(subset)-1].Metrics.MemoryUsage

	for i := 0; i < len(subset)-1; i++ {
		if subset[i+1].Metrics.MemoryUsage <= subset[i].Metrics.MemoryUsage {
			return true, "Memory usage trend stable"
		}
	}

	if (end - start) > r.Threshold {
		return false, fmt.Sprintf("Memory consistently increasing (%.2fMB -> %.2fMB)", start, end)
	}
	return true, "Memory usage trend stable"
}

type ErrorRateSpikeRule struct {
	Multiplier float64
	MinRate    float64
}

func (r *ErrorRateSpikeRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *ErrorRateSpikeRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 2 {
		return true, "Insufficient data for trend analysis"
	}

	current := history[len(history)-1].Metrics.ErrorRate

	// Calculate average of previous points
	var sum float64
	count := 0
	for i := 0; i < len(history)-1; i++ {
		sum += history[i].Metrics.ErrorRate
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)

	if current < r.MinRate {
		return true, fmt.Sprintf("Error rate %.2f%% below noise threshold", current*100)
	}

	if avg == 0 {
		if current > r.MinRate {
			return false, fmt.Sprintf("Error rate spiked from 0%% to %.2f%%", current*100)
		}
		return true, "Error rate stable at 0%"
	}

	if current > avg*r.Multiplier {
		return false, fmt.Sprintf("Error rate spiked to %.2f%% (avg: %.2f%%)", current*100, avg*100)
	}

	return true, fmt.Sprintf("Error rate %.2f%% within normal range (avg: %.2f%%)", current*100, avg*100)
}

type LatencyDegradationRule struct {
	Threshold  float64
	MinLatency float64
}

func (r *LatencyDegradationRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *LatencyDegradationRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	// Use last 10 points for baseline, excluding the most recent one
	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.Latency
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.Latency

	if current < r.MinLatency {
		return true, fmt.Sprintf("Latency %.2fms below noise threshold", current)
	}

	if avg > 0 && current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Latency degraded to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("Latency %.2fms within normal range (avg: %.2fms)", current, avg)
}

type CPUUsageTrendRule struct {
	Threshold float64
	MinUsage  float64
}

func (r *CPUUsageTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *CPUUsageTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.CPUUsage
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.CPUUsage

	if current < r.MinUsage {
		return true, fmt.Sprintf("CPU usage %.2f%% below noise threshold", current)
	}

	if avg > 0 && current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("CPU usage trending up to %.2f%% (avg: %.2f%%)", current, avg)
	}

	return true, fmt.Sprintf("CPU usage %.2f%% within normal range (avg: %.2f%%)", current, avg)
}

type CPUThrottlingTrendRule struct {
	Threshold     float64
	MinThrottling float64
}

func (r *CPUThrottlingTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *CPUThrottlingTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.CPUThrottling
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.CPUThrottling

	if current < r.MinThrottling {
		return true, fmt.Sprintf("CPU throttling %.2f%% below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinThrottling {
			return false, fmt.Sprintf("CPU throttling spiked from 0%% to %.2f%%", current)
		}
		return true, "CPU throttling stable at 0%"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("CPU throttling trending up to %.2f%% (avg: %.2f%%)", current, avg)
	}

	return true, fmt.Sprintf("CPU throttling %.2f%% within normal range (avg: %.2f%%)", current, avg)
}

type NetworkLatencyTrendRule struct {
	Threshold float64
}

func (r *NetworkLatencyTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *NetworkLatencyTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.Latency
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.Latency

	if avg > 0 && current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Network latency trending up to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("Network latency %.2fms within normal range", current)
}

type PacketLossTrendRule struct {
	Threshold     float64
	MinPacketLoss float64
}

func (r *PacketLossTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *PacketLossTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.PacketLoss
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.PacketLoss

	if current < r.MinPacketLoss {
		return true, fmt.Sprintf("Packet loss %.2f%% below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinPacketLoss {
			return false, fmt.Sprintf("Packet loss spiked from 0%% to %.2f%%", current)
		}
		return true, "Packet loss stable at 0%"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Packet loss trending up to %.2f%% (avg: %.2f%%)", current, avg)
	}

	return true, fmt.Sprintf("Packet loss %.2f%% within normal range", current)
}

type DiskIOLatencyTrendRule struct {
	Threshold float64
	MinIOWait float64
}

func (r *DiskIOLatencyTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *DiskIOLatencyTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.IOWait
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.IOWait

	if current < r.MinIOWait {
		return true, fmt.Sprintf("Disk I/O latency (wait) %.2f%% below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinIOWait {
			return false, fmt.Sprintf("Disk I/O latency (wait) spiked from 0%% to %.2f%%", current)
		}
		return true, "Disk I/O latency (wait) stable at 0%"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Disk I/O latency (wait) trending up to %.2f%% (avg: %.2f%%)", current, avg)
	}

	return true, fmt.Sprintf("Disk I/O latency (wait) %.2f%% within normal range", current)
}

type SwapUsageTrendRule struct {
	Threshold float64
	MinSwap   float64
}

func (r *SwapUsageTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *SwapUsageTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.SwapUsage
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.SwapUsage

	if current < r.MinSwap {
		return true, fmt.Sprintf("Swap usage %.2f%% below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinSwap {
			return false, fmt.Sprintf("Swap usage spiked from 0%% to %.2f%%", current)
		}
		return true, "Swap usage stable at 0%"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Swap usage trending up to %.2f%% (avg: %.2f%%)", current, avg)
	}

	return true, fmt.Sprintf("Swap usage %.2f%% within normal range", current)
}

type ContainerRestartTrendRule struct {
	Threshold   float64
	MinRestarts float64
}

func (r *ContainerRestartTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *ContainerRestartTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.RestartCount
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.RestartCount

	if current < r.MinRestarts {
		return true, fmt.Sprintf("Restart count %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinRestarts {
			return false, fmt.Sprintf("Restart count spiked from 0 to %.0f", current)
		}
		return true, "Restart count stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Restart count trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Restart count %.0f within normal range", current)
}

type GoroutineCountTrendRule struct {
	Threshold     float64
	MinGoroutines float64
}

func (r *GoroutineCountTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *GoroutineCountTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.GoroutineCount
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.GoroutineCount

	if current < r.MinGoroutines {
		return true, fmt.Sprintf("Goroutine count %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinGoroutines {
			return false, fmt.Sprintf("Goroutine count spiked from 0 to %.0f", current)
		}
		return true, "Goroutine count stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Goroutine count trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Goroutine count %.0f within normal range", current)
}

type OpenFDCountTrendRule struct {
	Threshold float64
	MinFDs    float64
}

func (r *OpenFDCountTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *OpenFDCountTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.OpenFDs
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.OpenFDs

	if current < r.MinFDs {
		return true, fmt.Sprintf("Open FDs %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinFDs {
			return false, fmt.Sprintf("Open FDs spiked from 0 to %.0f", current)
		}
		return true, "Open FDs stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Open FDs trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Open FDs %.0f within normal range", current)
}

type ThreadCountTrendRule struct {
	Threshold  float64
	MinThreads float64
}

func (r *ThreadCountTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *ThreadCountTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.ThreadCount
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.ThreadCount

	if current < r.MinThreads {
		return true, fmt.Sprintf("Thread count %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinThreads {
			return false, fmt.Sprintf("Thread count spiked from 0 to %.0f", current)
		}
		return true, "Thread count stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Thread count trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Thread count %.0f within normal range", current)
}

type ConnectionPoolTrendRule struct {
	Threshold      float64
	MinConnections float64
}

func (r *ConnectionPoolTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *ConnectionPoolTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.ActiveConnections
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.ActiveConnections

	if current < r.MinConnections {
		return true, fmt.Sprintf("Active connections %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinConnections {
			return false, fmt.Sprintf("Active connections spiked from 0 to %.0f", current)
		}
		return true, "Active connections stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Active connections trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Active connections %.0f within normal range", current)
}

type MemcachedEvictionTrendRule struct {
	Threshold    float64
	MinEvictions float64
}

func (r *MemcachedEvictionTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *MemcachedEvictionTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.MemcachedEvictions
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.MemcachedEvictions

	if current < r.MinEvictions {
		return true, fmt.Sprintf("Memcached evictions %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinEvictions {
			return false, fmt.Sprintf("Memcached evictions spiked from 0 to %.0f", current)
		}
		return true, "Memcached evictions stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Memcached evictions trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Memcached evictions %.0f within normal range", current)
}

type KafkaConsumerLagTrendRule struct {
	Threshold float64
	MinLag    float64
}

func (r *KafkaConsumerLagTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *KafkaConsumerLagTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.KafkaConsumerLag
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.KafkaConsumerLag

	if current < r.MinLag {
		return true, fmt.Sprintf("Kafka consumer lag %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinLag {
			return false, fmt.Sprintf("Kafka consumer lag spiked from 0 to %.0f", current)
		}
		return true, "Kafka consumer lag stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Kafka consumer lag trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Kafka consumer lag %.0f within normal range", current)
}

type RedisFragmentationTrendRule struct {
	Threshold float64
	MinFrag   float64
}

func (r *RedisFragmentationTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *RedisFragmentationTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.RedisFragmentationRatio
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.RedisFragmentationRatio

	if current < r.MinFrag {
		return true, fmt.Sprintf("Redis fragmentation %.2f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinFrag {
			return false, fmt.Sprintf("Redis fragmentation spiked from 0 to %.2f", current)
		}
		return true, "Redis fragmentation stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Redis fragmentation trending up to %.2f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Redis fragmentation %.2f within normal range", current)
}

type OomKillTrendRule struct {
	Threshold float64
	MinKills  float64
}

func (r *OomKillTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *OomKillTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.OomKills
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.OomKills

	if current < r.MinKills {
		return true, fmt.Sprintf("OOM kills %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinKills {
			return false, fmt.Sprintf("OOM kills spiked from 0 to %.0f", current)
		}
		return true, "OOM kills stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("OOM kills trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("OOM kills %.0f within normal range", current)
}

type FileSystemUsageTrendRule struct {
	Threshold float64
	MinUsage  float64
}

func (r *FileSystemUsageTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *FileSystemUsageTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.DiskUsage
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.DiskUsage

	if current < r.MinUsage {
		return true, fmt.Sprintf("File system usage %.2f%% below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinUsage {
			return false, fmt.Sprintf("File system usage spiked from 0%% to %.2f%%", current)
		}
		return true, "File system usage stable at 0%"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("File system usage trending up to %.2f%% (avg: %.2f%%)", current, avg)
	}

	return true, fmt.Sprintf("File system usage %.2f%% within normal range", current)
}

type TcpRetransmitTrendRule struct {
	Threshold      float64
	MinRetransmits float64
}

func (r *TcpRetransmitTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *TcpRetransmitTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.TcpRetransmits
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.TcpRetransmits

	if current < r.MinRetransmits {
		return true, fmt.Sprintf("TCP retransmits %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinRetransmits {
			return false, fmt.Sprintf("TCP retransmits spiked from 0 to %.0f", current)
		}
		return true, "TCP retransmits stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("TCP retransmits trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("TCP retransmits %.0f within normal range", current)
}

type DnsLatencyTrendRule struct {
	Threshold  float64
	MinLatency float64
}

func (r *DnsLatencyTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *DnsLatencyTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.DnsLatency
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.DnsLatency

	if current < r.MinLatency {
		return true, fmt.Sprintf("DNS latency %.2fms below noise threshold", current)
	}

	if avg > 0 && current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("DNS latency trending up to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("DNS latency %.2fms within normal range", current)
}

type KernelPacketDropTrendRule struct {
	Threshold float64
	MinDrops  float64
}

func (r *KernelPacketDropTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *KernelPacketDropTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.KernelPacketDrops
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.KernelPacketDrops

	if current < r.MinDrops {
		return true, fmt.Sprintf("Kernel packet drops %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinDrops {
			return false, fmt.Sprintf("Kernel packet drops spiked from 0 to %.0f", current)
		}
		return true, "Kernel packet drops stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("Kernel packet drops trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Kernel packet drops %.0f within normal range", current)
}

type UdpPacketLossTrendRule struct {
	Threshold float64
	MinLoss   float64
}

func (r *UdpPacketLossTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *UdpPacketLossTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.UdpPacketLoss
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.UdpPacketLoss

	if current < r.MinLoss {
		return true, fmt.Sprintf("UDP packet loss %.0f below noise threshold", current)
	}

	if avg == 0 {
		if current > r.MinLoss {
			return false, fmt.Sprintf("UDP packet loss spiked from 0 to %.0f", current)
		}
		return true, "UDP packet loss stable at 0"
	}

	if current > avg*(1+r.Threshold) {
		return false, fmt.Sprintf("UDP packet loss trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("UDP packet loss %.0f within normal range", current)
}

type PageFaultTrendRule struct {
	Threshold float64
	MinFaults float64
}

func (r *PageFaultTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *PageFaultTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.PageFaults
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.PageFaults

	if current > avg*(1+r.Threshold) && current > r.MinFaults {
		return false, fmt.Sprintf("Page faults trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Page faults %.0f within normal range", current)
}

type ContextSwitchTrendRule struct {
	Threshold   float64
	MinSwitches float64
}

func (r *ContextSwitchTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *ContextSwitchTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.ContextSwitches
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.ContextSwitches

	if current > avg*(1+r.Threshold) && current > r.MinSwitches {
		return false, fmt.Sprintf("Context switches trending up to %.0f (avg: %.2f)", current, avg)
	}

	return true, fmt.Sprintf("Context switches %.0f within normal range", current)
}

type BlockIOLatencyTrendRule struct {
	Threshold  float64
	MinLatency float64
}

func (r *BlockIOLatencyTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *BlockIOLatencyTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.BlockIOLatency
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.BlockIOLatency

	if current > avg*(1+r.Threshold) && current > r.MinLatency {
		return false, fmt.Sprintf("Block I/O latency trending up to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("Block I/O latency %.2fms within normal range", current)
}

type RunQLatencyTrendRule struct {
	Threshold  float64
	MinLatency float64
}

func (r *RunQLatencyTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *RunQLatencyTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.RunQLatency
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.RunQLatency

	if current > avg*(1+r.Threshold) && current > r.MinLatency {
		return false, fmt.Sprintf("Run queue latency trending up to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("Run queue latency %.2fms within normal range", current)
}

type MemoryAllocRateTrendRule struct {
	Threshold float64
	MinRate   float64
}

func (r *MemoryAllocRateTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *MemoryAllocRateTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.MemoryAllocRate
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.MemoryAllocRate

	if current > avg*(1+r.Threshold) && current > r.MinRate {
		return false, fmt.Sprintf("Memory allocation rate trending up to %.2f B/s (avg: %.2f B/s)", current, avg)
	}

	return true, fmt.Sprintf("Memory allocation rate %.2f B/s within normal range", current)
}

type LockContentionTrendRule struct {
	Threshold float64
	MinWait   float64
}

func (r *LockContentionTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *LockContentionTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.LockContention
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.LockContention

	if current > avg*(1+r.Threshold) && current > r.MinWait {
		return false, fmt.Sprintf("Lock contention trending up to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("Lock contention %.2fms within normal range", current)
}

type GCPauseTrendRule struct {
	Threshold float64
	MinPause  float64
}

func (r *GCPauseTrendRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	return true, "Insufficient data"
}

func (r *GCPauseTrendRule) EvaluateTrend(app *servicemap.Application, history []MetricPoint) (bool, string) {
	if len(history) < 5 {
		return true, "Insufficient data for trend analysis"
	}

	lookback := 10
	if len(history)-1 < lookback {
		lookback = len(history) - 1
	}

	var sum float64
	count := 0
	endIndex := len(history) - 1
	startIndex := endIndex - lookback

	for i := startIndex; i < endIndex; i++ {
		sum += history[i].Metrics.GCPause
		count++
	}

	if count == 0 {
		return true, "Insufficient data"
	}

	avg := sum / float64(count)
	current := history[endIndex].Metrics.GCPause

	if current > avg*(1+r.Threshold) && current > r.MinPause {
		return false, fmt.Sprintf("GC pause trending up to %.2fms (avg: %.2fms)", current, avg)
	}

	return true, fmt.Sprintf("GC pause %.2fms within normal range", current)
}
