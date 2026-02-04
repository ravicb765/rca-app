package servicemap

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// classifyAppName returns a simple service type based on common name hints
func classifyAppName(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "postgres") || strings.Contains(n, "postgresql"):
		return "postgres"
	case strings.Contains(n, "mysql"):
		return "mysql"
	case strings.Contains(n, "redis"):
		return "redis"
	case strings.Contains(n, "mongo"):
		return "mongodb"
	case strings.Contains(n, "kafka"):
		return "kafka"
	case strings.Contains(n, "rabbit"):
		return "rabbitmq"
	case strings.Contains(n, "api") || strings.Contains(n, "http") || strings.Contains(n, "web") || strings.Contains(n, "frontend") || strings.Contains(n, "backend"):
		return "http"
	default:
		return ""
	}
}

// ServiceMap represents a dependency graph of applications
type ServiceMap struct {
	Applications         map[string]*Application `json:"applications"`
	Connections          []Connection            `json:"connections"`
	LastUpdated          time.Time               `json:"last_updated"`
	CircularDependencies [][]string              `json:"circular_dependencies,omitempty"`
	mu                   sync.RWMutex
}

// Application represents a service or application in the map
type Application struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type,omitempty"`
	Instances   []Instance        `json:"instances,omitempty"`
	Upstreams   []string          `json:"upstreams,omitempty"`
	Downstreams []string          `json:"downstreams,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Instance represents a running instance of an application
type Instance struct {
	ID          string            `json:"id"`
	NodeName    string            `json:"node_name,omitempty"`
	ContainerID string            `json:"container_id,omitempty"`
	PodName     string            `json:"pod_name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Connection represents an observed edge between two applications
type Connection struct {
	SourceApp         string  `json:"source_app"`
	DestApp           string  `json:"dest_app"`
	Protocol          string  `json:"protocol,omitempty"`
	RequestRate       float64 `json:"request_rate,omitempty"`
	ErrorRate         float64 `json:"error_rate,omitempty"`
	Latency           float64 `json:"latency,omitempty"`
	MemoryUsage       float64 `json:"memory_usage,omitempty"`
	CPUUsage          float64 `json:"cpu_usage,omitempty"`
	DiskUsage         float64 `json:"disk_usage,omitempty"`
	IOLoad            float64 `json:"io_load,omitempty"`
	ActiveConnections float64 `json:"active_connections,omitempty"`
	PacketLoss        float64 `json:"packet_loss,omitempty"`
	Http5xxRate       float64 `json:"http_5xx_rate,omitempty"`
	IOWait            float64 `json:"io_wait,omitempty"`
	SwapUsage         float64 `json:"swap_usage,omitempty"`
	RestartCount      float64 `json:"restart_count,omitempty"`
	CPUThrottling     float64 `json:"cpu_throttling,omitempty"`
	GoroutineCount    float64 `json:"goroutine_count,omitempty"`
	OpenFDs           float64 `json:"open_fds,omitempty"`
	ThreadCount       float64 `json:"thread_count,omitempty"`
}

// TelemetryEvent represents a raw network event received from an agent
type TelemetryEvent struct {
	SrcIP             string  `json:"src_ip"`
	DstIP             string  `json:"dst_ip"`
	SrcPort           uint16  `json:"src_port"`
	DstPort           uint16  `json:"dst_port"`
	Protocol          string  `json:"protocol"`
	RequestRate       float64 `json:"request_rate"`
	ErrorRate         float64 `json:"error_rate"`
	Latency           float64 `json:"latency"`
	MemoryUsage       float64 `json:"memory_usage"`
	CPUUsage          float64 `json:"cpu_usage"`
	DiskUsage         float64 `json:"disk_usage"`
	IOLoad            float64 `json:"io_load"`
	ActiveConnections float64 `json:"active_connections"`
	PacketLoss        float64 `json:"packet_loss"`
	Http5xxRate       float64 `json:"http_5xx_rate"`
	IOWait            float64 `json:"io_wait"`
	SwapUsage         float64 `json:"swap_usage"`
	RestartCount      float64 `json:"restart_count"`
	CPUThrottling     float64 `json:"cpu_throttling"`
	GoroutineCount    float64 `json:"goroutine_count"`
	OpenFDs           float64 `json:"open_fds"`
	ThreadCount       float64 `json:"thread_count"`
}

// NewServiceMap creates an empty ServiceMap
func NewServiceMap() *ServiceMap {
	return &ServiceMap{
		Applications: make(map[string]*Application),
		Connections:  []Connection{},
		LastUpdated:  time.Now(),
	}
}

// AddService adds or updates an application in the map
func (sm *ServiceMap) AddService(app *Application) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if existing, ok := sm.Applications[app.ID]; ok {
		// merge minimal fields
		existing.Name = app.Name
		if existing.Metadata == nil {
			existing.Metadata = app.Metadata
		}
	} else {
		sm.Applications[app.ID] = app
	}
	sm.LastUpdated = time.Now()
}

// AddConnection adds a connection and updates upstream/downstream lists
func (sm *ServiceMap) AddConnection(conn Connection) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	// add connection
	sm.Connections = append(sm.Connections, conn)
	// ensure apps exist
	if _, ok := sm.Applications[conn.SourceApp]; !ok {
		sm.Applications[conn.SourceApp] = &Application{ID: conn.SourceApp, Name: conn.SourceApp}
	}
	if _, ok := sm.Applications[conn.DestApp]; !ok {
		sm.Applications[conn.DestApp] = &Application{ID: conn.DestApp, Name: conn.DestApp}
	}
	// update upstream / downstream
	s := sm.Applications[conn.SourceApp]
	d := sm.Applications[conn.DestApp]
	// add dest to source downstreams
	if !contains(s.Downstreams, conn.DestApp) {
		s.Downstreams = append(s.Downstreams, conn.DestApp)
	}
	// add source to dest upstreams
	if !contains(d.Upstreams, conn.SourceApp) {
		d.Upstreams = append(d.Upstreams, conn.SourceApp)
	}
	sm.LastUpdated = time.Now()
}

func contains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

// HasDependency returns true if a depends on b (i.e., a -> b)
func (sm *ServiceMap) HasDependency(a, b string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if app, ok := sm.Applications[a]; ok {
		for _, d := range app.Downstreams {
			if d == b {
				return true
			}
		}
	}
	return false
}

// K8sMetadataCache is a placeholder interface for Kubernetes metadata lookups.
type K8sMetadataCache interface {
	LookupPod(ip string) *Instance
}

// ServiceMapBuilder maintains the state of the service graph over time.
// It handles incremental updates and metadata correlation.
type ServiceMapBuilder struct {
	// State
	applications map[string]*Application
	connections  map[string]*Connection

	// Metadata cache
	k8sMetadata K8sMetadataCache

	mu sync.RWMutex
}

// NewServiceMapBuilder creates a new stateful builder.
func NewServiceMapBuilder(metadataCache K8sMetadataCache) *ServiceMapBuilder {
	return &ServiceMapBuilder{
		applications: make(map[string]*Application),
		connections:  make(map[string]*Connection),
		k8sMetadata:  metadataCache,
	}
}

// Update processes a batch of observed connections and returns the current graph snapshot.
func (b *ServiceMapBuilder) Update(events []TelemetryEvent) *ServiceMap {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, event := range events {
		b.processEvent(event)
	}

	return b.buildGraph()
}

func (b *ServiceMapBuilder) processEvent(event TelemetryEvent) {
	// Resolve IPs to Applications
	srcInstance := b.k8sMetadata.LookupPod(event.SrcIP)
	dstInstance := b.k8sMetadata.LookupPod(event.DstIP)

	var srcApp, dstApp string

	if srcInstance != nil {
		srcApp = srcInstance.ID // Assuming ID maps to App Name or similar identifier
	} else {
		srcApp = "external-" + event.SrcIP
	}

	if dstInstance != nil {
		dstApp = dstInstance.ID
	} else {
		dstApp = "external-" + event.DstIP
	}

	// Don't track external-to-external connections to reduce noise
	if strings.HasPrefix(srcApp, "external-") && strings.HasPrefix(dstApp, "external-") {
		return
	}

	// 1. Update Applications
	b.ensureApp(srcApp)
	b.ensureApp(dstApp)

	// 2. Update Connection State
	key := fmt.Sprintf("%s->%s", srcApp, dstApp)
	if existing, ok := b.connections[key]; ok {
		// Simple moving average for stats
		alpha := 0.3
		existing.RequestRate = existing.RequestRate*(1-alpha) + event.RequestRate*alpha
		existing.ErrorRate = existing.ErrorRate*(1-alpha) + event.ErrorRate*alpha
		existing.Latency = existing.Latency*(1-alpha) + event.Latency*alpha
		existing.MemoryUsage = existing.MemoryUsage*(1-alpha) + event.MemoryUsage*alpha
		existing.CPUUsage = existing.CPUUsage*(1-alpha) + event.CPUUsage*alpha
		existing.DiskUsage = existing.DiskUsage*(1-alpha) + event.DiskUsage*alpha
		existing.IOLoad = existing.IOLoad*(1-alpha) + event.IOLoad*alpha
		existing.ActiveConnections = existing.ActiveConnections*(1-alpha) + event.ActiveConnections*alpha
		existing.PacketLoss = existing.PacketLoss*(1-alpha) + event.PacketLoss*alpha
		existing.Http5xxRate = existing.Http5xxRate*(1-alpha) + event.Http5xxRate*alpha
		existing.IOWait = existing.IOWait*(1-alpha) + event.IOWait*alpha
		existing.SwapUsage = existing.SwapUsage*(1-alpha) + event.SwapUsage*alpha
		existing.RestartCount = existing.RestartCount*(1-alpha) + event.RestartCount*alpha
		existing.CPUThrottling = existing.CPUThrottling*(1-alpha) + event.CPUThrottling*alpha
		existing.GoroutineCount = existing.GoroutineCount*(1-alpha) + event.GoroutineCount*alpha
		existing.OpenFDs = existing.OpenFDs*(1-alpha) + event.OpenFDs*alpha
		existing.ThreadCount = existing.ThreadCount*(1-alpha) + event.ThreadCount*alpha
		if existing.Protocol == "" && event.Protocol != "" {
			existing.Protocol = event.Protocol
		}
	} else {
		b.connections[key] = &Connection{
			SourceApp:         srcApp,
			DestApp:           dstApp,
			Protocol:          event.Protocol,
			RequestRate:       event.RequestRate,
			ErrorRate:         event.ErrorRate,
			Latency:           event.Latency,
			MemoryUsage:       event.MemoryUsage,
			CPUUsage:          event.CPUUsage,
			DiskUsage:         event.DiskUsage,
			IOLoad:            event.IOLoad,
			ActiveConnections: event.ActiveConnections,
			PacketLoss:        event.PacketLoss,
			Http5xxRate:       event.Http5xxRate,
			IOWait:            event.IOWait,
			SwapUsage:         event.SwapUsage,
			RestartCount:      event.RestartCount,
			CPUThrottling:     event.CPUThrottling,
			GoroutineCount:    event.GoroutineCount,
			OpenFDs:           event.OpenFDs,
			ThreadCount:       event.ThreadCount,
		}
	}

	// 3. Update Topology
	src := b.applications[srcApp]
	dst := b.applications[dstApp]

	if !contains(src.Downstreams, dstApp) {
		src.Downstreams = append(src.Downstreams, dstApp)
	}
	if !contains(dst.Upstreams, srcApp) {
		dst.Upstreams = append(dst.Upstreams, srcApp)
	}
}

func (b *ServiceMapBuilder) ensureApp(id string) {
	if _, ok := b.applications[id]; !ok {
		b.applications[id] = &Application{
			ID:   id,
			Name: id,
			Type: classifyAppName(id),
		}
	}
}

func (b *ServiceMapBuilder) buildGraph() *ServiceMap {
	sm := NewServiceMap()

	for id, app := range b.applications {
		// Deep copy to avoid race conditions on the returned map
		val := *app
		val.Upstreams = make([]string, len(app.Upstreams))
		copy(val.Upstreams, app.Upstreams)
		val.Downstreams = make([]string, len(app.Downstreams))
		copy(val.Downstreams, app.Downstreams)
		sm.Applications[id] = &val
	}

	for _, conn := range b.connections {
		sm.Connections = append(sm.Connections, *conn)
	}

	sm.LastUpdated = time.Now()
	sm.CircularDependencies = sm.DetectCycles()
	return sm
}

// BuildServiceMap constructs a ServiceMap from a set of observed connections
func BuildServiceMap(conns []Connection) *ServiceMap {
	sm := NewServiceMap()
	for _, c := range conns {
		if c.SourceApp == "" || c.DestApp == "" {
			continue
		}
		// ensure basic apps
		if _, ok := sm.Applications[c.SourceApp]; !ok {
			sm.Applications[c.SourceApp] = &Application{ID: c.SourceApp, Name: c.SourceApp}
		}
		if _, ok := sm.Applications[c.DestApp]; !ok {
			sm.Applications[c.DestApp] = &Application{ID: c.DestApp, Name: c.DestApp}
		}
		sm.AddConnection(c)
	}

	// classify applications using simple heuristics
	for _, app := range sm.Applications {
		// protocol-based detection: if any incoming connection is http, set type
		for _, conn := range sm.Connections {
			if conn.DestApp == app.ID && strings.EqualFold(conn.Protocol, "http") {
				app.Type = "http"
				break
			}
		}
		if app.Type == "" {
			app.Type = classifyAppName(app.ID)
		}
	}

	return sm
}

// DetectCycles identifies circular dependencies in the service graph
func (sm *ServiceMap) DetectCycles() [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for id := range sm.Applications {
		if !visited[id] {
			sm.dfsCycle(id, visited, recStack, []string{}, &cycles)
		}
	}
	return cycles
}

func (sm *ServiceMap) dfsCycle(curr string, visited, recStack map[string]bool, path []string, cycles *[][]string) {
	visited[curr] = true
	recStack[curr] = true
	path = append(path, curr)

	if app, ok := sm.Applications[curr]; ok {
		for _, neighbor := range app.Downstreams {
			if !visited[neighbor] {
				sm.dfsCycle(neighbor, visited, recStack, path, cycles)
			} else if recStack[neighbor] {
				// Cycle detected
				// Extract cycle from path
				cycle := make([]string, 0)
				startIdx := -1
				for i, node := range path {
					if node == neighbor {
						startIdx = i
						break
					}
				}
				if startIdx != -1 {
					cycle = append(cycle, path[startIdx:]...)
					*cycles = append(*cycles, cycle)
				}
			}
		}
	}
	recStack[curr] = false
}
