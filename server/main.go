package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ravicb765/rca-app/server/inspections"
	"github.com/ravicb765/rca-app/server/pkg/metrics"
	"github.com/ravicb765/rca-app/server/servicemap"
	"github.com/ravicb765/rca-app/server/slo"
)

var kvRe = regexp.MustCompile(`(src|source|dst|dest|proto)=([^\s]+)`)

// MetadataCache implements servicemap.K8sMetadataCache
type MetadataCache struct {
	mu   sync.RWMutex
	pods map[string]*servicemap.Instance
}

func NewMetadataCache() *MetadataCache {
	return &MetadataCache{
		pods: make(map[string]*servicemap.Instance),
	}
}

func (m *MetadataCache) LookupPod(ip string) *servicemap.Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pods[ip]
}

type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	IP        string `json:"ip"`
	Node      string `json:"node"`
}

func (m *MetadataCache) UpdatePods(pods []PodInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Rebuild cache to avoid stale IPs
	m.pods = make(map[string]*servicemap.Instance)
	for _, p := range pods {
		if p.IP != "" {
			m.pods[p.IP] = &servicemap.Instance{
				ID:        p.Namespace + "/" + p.Name,
				PodName:   p.Name,
				Namespace: p.Namespace,
				NodeName:  p.Node,
			}
		}
	}
}

func (m *MetadataCache) RegisterPods(pods []PodInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range pods {
		if p.IP != "" {
			m.pods[p.IP] = &servicemap.Instance{
				ID:        p.Namespace + "/" + p.Name,
				PodName:   p.Name,
				Namespace: p.Namespace,
				NodeName:  p.Node,
			}
		}
	}
}

// decodePerfRawToEvents tries to interpret raw bytes as one of: JSON array/object, ascii key=value, or a binary conn_event
func decodePerfRawToEvents(b []byte) ([]servicemap.TelemetryEvent, error) {
	// Try JSON
	var events []servicemap.TelemetryEvent
	if err := json.Unmarshal(b, &events); err == nil {
		return events, nil
	}
	var event servicemap.TelemetryEvent
	if err := json.Unmarshal(b, &event); err == nil {
		return []servicemap.TelemetryEvent{event}, nil
	}
	// Try ascii key=value parsing
	sb := string(b)
	matches := kvRe.FindAllStringSubmatch(sb, -1)
	m := map[string]string{}
	for _, mm := range matches {
		if len(mm) == 3 {
			m[strings.ToLower(mm[1])] = mm[2]
		}
	}
	if src, ok := m["src"]; ok {
		dst := m["dst"]
		if dst == "" {
			dst = m["dest"]
		}
		proto := m["proto"]
		// For KV parsing, we treat src/dst as IPs if possible, or the builder will handle them as external
		e := servicemap.TelemetryEvent{SrcIP: src, DstIP: dst, Protocol: proto}
		return []servicemap.TelemetryEvent{e}, nil
	}
	// Try binary layout: IPv6 first: struct { u8 saddr[16]; u8 daddr[16]; u16 sport; u16 dport; u64 ts_ns }
	if len(b) >= 44 {
		srcIP := net.IP(b[0:16]).String()
		dstIP := net.IP(b[16:32]).String()
		sport := binary.LittleEndian.Uint16(b[32:34])
		dport := binary.LittleEndian.Uint16(b[34:36])

		e6 := servicemap.TelemetryEvent{SrcIP: srcIP, DstIP: dstIP, SrcPort: sport, DstPort: dport, Protocol: "tcp"}
		return []servicemap.TelemetryEvent{e6}, nil
	}

	// IPv4 layout: struct { u32 saddr; u32 daddr; u16 sport; u16 dport; u64 ts_ns }
	if len(b) >= 20 {
		srcIP := net.IP(b[0:4]).String()
		dstIP := net.IP(b[4:8]).String()
		sport := binary.LittleEndian.Uint16(b[8:10])
		dport := binary.LittleEndian.Uint16(b[10:12])

		e2 := servicemap.TelemetryEvent{SrcIP: srcIP, DstIP: dstIP, SrcPort: sport, DstPort: dport, Protocol: "tcp"}
		return []servicemap.TelemetryEvent{e2}, nil
	}
	return nil, nil
}

// decodePerfData attempts to decode hex-encoded perf/map data into connections.
// It decodes the hex string and calls decodePerfRaw on the bytes.
func decodePerfData(hexStr string) ([]servicemap.TelemetryEvent, error) {
	s := strings.TrimPrefix(hexStr, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return decodePerfRawToEvents(b)
}

// decodePerfBase64 decodes base64 payloads and calls decodePerfRaw
func decodePerfBase64(b64 string) ([]servicemap.TelemetryEvent, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return decodePerfRawToEvents(b)
}

// Simple in-memory service map store
type serviceStore struct {
	services []string
	// protect concurrent access
	mu sync.RWMutex
}

func (s *serviceStore) set(services []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = services
}

func (s *serviceStore) list() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.services...)
}

// AgentInfo represents a connected node agent
type AgentInfo struct {
	Hostname string    `json:"hostname"`
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}

// AgentStore tracks active agents
type AgentStore struct {
	agents map[string]AgentInfo
	mu     sync.RWMutex
}

func NewAgentStore() *AgentStore {
	return &AgentStore{
		agents: make(map[string]AgentInfo),
	}
}

func (s *AgentStore) Touch(hostname, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[hostname] = AgentInfo{Hostname: hostname, IP: ip, LastSeen: time.Now()}
}

func (s *AgentStore) ListActive() []AgentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var active []AgentInfo
	cutoff := time.Now().Add(-5 * time.Minute)
	for _, a := range s.agents {
		if a.LastSeen.After(cutoff) {
			active = append(active, a)
		}
	}
	return active
}

// newRouter creates the HTTP handlers. Exported for testing.
func newRouter(store *serviceStore, agentStore *AgentStore, builder *servicemap.ServiceMapBuilder, inspectionEngine *inspections.InspectionEngine, sloTracker *slo.SLOTracker, metaCache *MetadataCache) *gin.Engine {
	r := gin.Default()

	r.GET("/api/v1/servicemap", func(c *gin.Context) {
		// Get current graph snapshot (Update with nil events)
		sm := builder.GetServiceMap()

		// retain backward compatible services list (cluster-agent or static)
		svc := store.list()
		if len(svc) == 0 {
			// if we have a generated service map, derive a list
			if len(sm.Applications) > 0 {
				for id := range sm.Applications {
					svc = append(svc, id)
				}
			} else {
				svc = []string{"service-a", "service-b"}
			}
		}

		c.JSON(http.StatusOK, gin.H{"services": svc, "servicemap": sm})
	})

	r.GET("/api/v1/applications", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"applications": []string{"app-1", "app-2"}})
	})

	r.GET("/api/v1/applications/:id/inspections", func(c *gin.Context) {
		id := c.Param("id")
		results := inspectionEngine.GetResults(id)
		c.JSON(http.StatusOK, gin.H{"inspections": results})
	})

	r.GET("/api/v1/applications/:id/inspections/history", func(c *gin.Context) {
		id := c.Param("id")
		results := inspectionEngine.GetHistory(id)
		c.JSON(http.StatusOK, gin.H{"history": results})
	})

	r.GET("/api/v1/inspections/rules", func(c *gin.Context) {
		rules := inspectionEngine.GetRules()
		c.JSON(http.StatusOK, gin.H{"rules": rules})
	})

	r.GET("/api/v1/slos", func(c *gin.Context) {
		status, err := sloTracker.CheckAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"slos": status})
	})

	r.GET("/api/v1/agents", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"agents": agentStore.ListActive()})
	})

	r.POST("/api/v1/metadata", func(c *gin.Context) {
		var payload struct {
			Pods []PodInfo `json:"pods"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		metaCache.RegisterPods(payload.Pods)
		c.JSON(http.StatusOK, gin.H{"registered": len(payload.Pods)})
	})

	// Agent heartbeat endpoint used by node-agent
	r.POST("/api/v1/agent/heartbeat", func(c *gin.Context) {
		var payload struct {
			Hostname string `json:"hostname"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update agent store
		agentStore.Touch(payload.Hostname, c.ClientIP())

		// For now, just acknowledge receipt
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	// Agent event endpoint used by perf readers - accepts several payload forms:
	//  - top-level envelope {"connections": [...]}
	//  - an array of connections
	//  - a single connection
	//  - perf/map events {"map": "mymap", "data": "0x..."}
	r.POST("/api/v1/agent/event", func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Top-level envelope support
		var env struct {
			Connections []servicemap.TelemetryEvent `json:"connections"`
			Map         string                      `json:"map"`
			Data        string                      `json:"data"`
			DataBase64  string                      `json:"data_base64"`
		}
		if err := json.Unmarshal(raw, &env); err == nil {
			if len(env.Connections) > 0 {
				builder.Update(env.Connections)
				c.JSON(http.StatusOK, gin.H{"received": true, "added": len(env.Connections)})
				return
			}
			// prefer base64 field if present (common from perf-consumer)
			if env.Map != "" && env.DataBase64 != "" {
				if events, err := decodePerfBase64(env.DataBase64); err == nil && len(events) > 0 {
					builder.Update(events)
					c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map, "added": len(events)})
					return
				}
				c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map})
				return
			}
			if env.Map != "" && env.Data != "" {
				// Attempt best-effort decoding of perf/map payloads (hex or raw)
				// try hex first
				if strings.HasPrefix(env.Data, "0x") {
					if events, err := decodePerfData(env.Data); err == nil && len(events) > 0 {
						builder.Update(events)
						c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map, "added": len(events)})
						return
					}
				} else {
					// raw ASCII or JSON bytes
					if events, err := decodePerfRawToEvents([]byte(env.Data)); err == nil && len(events) > 0 {
						builder.Update(events)
						c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map, "added": len(events)})
						return
					}
				}
				// Accept perf/map event formats; decoding may be done by specialized consumers
				c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map})
				return
			}
		}

		// Backwards compatible: array of connections
		var events []servicemap.TelemetryEvent
		if err := json.Unmarshal(raw, &events); err == nil {
			builder.Update(events)
			c.JSON(http.StatusOK, gin.H{"received": true, "added": len(events)})
			return
		}

		// Single connection
		var event servicemap.TelemetryEvent
		if err := json.Unmarshal(raw, &event); err == nil {
			builder.Update([]servicemap.TelemetryEvent{event})
			c.JSON(http.StatusOK, gin.H{"received": true, "added": 1})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
	})

	// Process metrics endpoint
	r.POST("/api/v1/agent/processes", func(c *gin.Context) {
		var processes []struct {
			PID      int     `json:"pid"`
			CPU      float64 `json:"cpu"`
			Memory   uint64  `json:"memory"`
			Hostname string  `json:"hostname"`
		}
		if err := c.BindJSON(&processes); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("Received metrics for %d processes", len(processes))
		c.JSON(http.StatusOK, gin.H{"received": true, "count": len(processes)})
	})

	// Metrics Query API (Prompt 2.3)
	metricsEngine := metrics.NewQueryEngine(v1api)

	r.GET("/api/v1/metrics/query", func(c *gin.Context) {
		q := c.Query("query")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter required"})
			return
		}
		// Default to now
		ts := time.Now()
		if tStr := c.Query("time"); tStr != "" {
			if parsed, err := time.Parse(time.RFC3339, tStr); err == nil {
				ts = parsed
			}
		}
		
		val, err := metricsEngine.Query(c.Request.Context(), q, ts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": val})
	})

	r.GET("/api/v1/metrics/query_range", func(c *gin.Context) {
		q := c.Query("query")
		startStr := c.Query("start")
		endStr := c.Query("end")
		stepStr := c.Query("step")
		
		if q == "" || startStr == "" || endStr == "" || stepStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query, start, end, step parameters required"})
			return
		}
		
		start, err1 := time.Parse(time.RFC3339, startStr)
		end, err2 := time.Parse(time.RFC3339, endStr)
		step, err3 := time.ParseDuration(stepStr)
		
		if err1 != nil || err2 != nil || err3 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time format"})
			return
		}

		val, err := metricsEngine.QueryRange(c.Request.Context(), q, v1.Range{Start: start, End: end, Step: step})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": val})
	})

	return r
}

// fetchClusterData performs a single cluster-agent fetch and updates metrics, services, and pod metadata.
func fetchClusterData(client *http.Client, clusterAgent string, store *serviceStore, metaCache *MetadataCache, success, errors, total prometheus.Counter) error {
	if clusterAgent == "" {
		clusterAgent = "http://cluster-agent:9100"
	}
	total.Inc()
	resp, err := client.Get(clusterAgent + "/api/v1/cluster/services")
	if err != nil {
		errors.Inc()
		log.Printf("cluster services fetch error: %v", err)
		return err
	}

	var payload map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		resp.Body.Close()
		errors.Inc()
		log.Printf("cluster services decode error: %v", err)
		return err
	}
	resp.Body.Close()

	if s, ok := payload["services"]; ok {
		store.set(s)
	}

	// Fetch Pods
	resp, err = client.Get(clusterAgent + "/api/v1/cluster/pods")
	if err != nil {
		errors.Inc()
		log.Printf("cluster pods fetch error: %v", err)
		return err
	}
	defer resp.Body.Close()

	var podPayload struct {
		Pods []PodInfo `json:"pods"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&podPayload); err == nil {
		metaCache.UpdatePods(podPayload.Pods)
		success.Inc()
	}

	return nil
}

func startClusterFetcher(store *serviceStore, metaCache *MetadataCache, stopCh <-chan struct{}, success, errors, total prometheus.Counter) {
	clusterAgent := os.Getenv("CLUSTER_AGENT_URL")
	if clusterAgent == "" {
		clusterAgent = "http://cluster-agent:9100"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	baseDelay := 5 * time.Second
	maxDelay := 120 * time.Second
	delay := baseDelay
	for {
		select {
		case <-stopCh:
			return
		default:
			if err := fetchClusterData(client, clusterAgent, store, metaCache, success, errors, total); err != nil {
				// exponential backoff with jitter
				j := time.Duration(rand.Int63n(int64(delay)))
				wait := delay + j/2
				if wait > maxDelay {
					wait = maxDelay
				}
				log.Printf("fetch failed, backing off %s", wait)
				time.Sleep(wait)
				if delay < maxDelay {
					delay *= 2
				}
				continue
			}
			// success -> reset delay and wait a fixed interval
			delay = baseDelay
			time.Sleep(30 * time.Second)
		}
	}
}

func main() {
	store := &serviceStore{}
	agentStore := NewAgentStore()

	// Prometheus metrics for server-side ingestion
	reg := prometheus.NewRegistry()

	// Initialize ServiceMapBuilder with metadata cache
	metaCache := NewMetadataCache()
	builder := servicemap.NewServiceMapBuilder(metaCache)
	inspectionEngine := inspections.NewInspectionEngine(reg)

	// Initialize SLO Tracker
	promURL := os.Getenv("PROMETHEUS_URL")
	if promURL == "" {
		promURL = "http://prometheus:9090"
	}
	client, err := api.NewClient(api.Config{Address: promURL})
	if err != nil {
		log.Printf("Warning: failed to create prometheus client: %v", err)
	}
	v1api := v1.NewAPI(client)
	sloTracker := slo.NewSLOTracker(v1api)

	// Add default SLOs
	sloTracker.AddSLO(&slo.SLO{
		Name:        "API Availability",
		Application: "app-1",
		Type:        slo.SLOTypeAvailability,
		Target:      99.9,
		Window:      "30d",
		Indicator: slo.SLOIndicator{
			Success: "http_requests_total{status!~'5..'}",
			Total:   "http_requests_total",
		},
	})

	fetchTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "server_cluster_fetch_total",
		Help: "Total attempts to fetch services from cluster-agent",
	})
	fetchSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "server_cluster_fetch_success_total",
		Help: "Total successful fetches from cluster-agent",
	})
	fetchErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "server_cluster_fetch_errors_total",
		Help: "Total errors while fetching from cluster-agent",
	})
	reg.MustRegister(fetchTotal, fetchSuccess, fetchErrors)

	r := newRouter(store, agentStore, builder, inspectionEngine, sloTracker, metaCache)
	// add server metrics endpoint bound to registry
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	stopCh := make(chan struct{})

	// Start inspection loop
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Get latest snapshot and run inspections
				builder.Prune(5 * time.Minute)
				sm := builder.GetServiceMap()
				inspectionEngine.Run(sm)
			}
		}
	}()

	go startClusterFetcher(store, metaCache, stopCh, fetchSuccess, fetchErrors, fetchTotal)

	// shutdown handling omitted for brevity
	r.Run(":" + port)
}
