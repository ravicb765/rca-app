package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"context"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ravicb765/rca-app/server/inspections"
	"github.com/ravicb765/rca-app/server/alerts"
	"github.com/ravicb765/rca-app/server/deployment"
	"github.com/ravicb765/rca-app/server/cost"
	"github.com/ravicb765/rca-app/server/security"
	"github.com/ravicb765/rca-app/server/pkg/cache"
	"github.com/ravicb765/rca-app/server/pkg/integrations"
	"github.com/ravicb765/rca-app/server/pkg/metrics"
	"github.com/ravicb765/rca-app/server/pkg/store"
	"github.com/ravicb765/rca-app/server/pkg/stream"
	"github.com/ravicb765/rca-app/server/pkg/telemetry"
	"github.com/ravicb765/rca-app/server/servicemap"
	"github.com/ravicb765/rca-app/server/slo"
)

var kvRe = regexp.MustCompile(`(src|source|dst|dest|proto)=([^\s]+)`)

type MetadataCache struct {
	mu   sync.RWMutex
	pods map[string]*servicemap.Instance
}

type MockAggregator struct{}
func (m *MockAggregator) Process(data map[string]float64) {
	log.Printf("Processed stream metrics: %v", data)
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

	// Apply security middleware
	r.Use(security.SecurityHeaders())
	r.Use(security.RequestSizeLimit(10 << 20)) // 10 MB limit
	
	// Rate limiter: 100 requests per minute per IP
	rateLimiter := security.NewRateLimiter(100, time.Minute)
	r.Use(rateLimiter.Middleware())
	
	// API key authentication for all routes except health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	
	// Protected routes
	protected := r.Group("/")
	protected.Use(security.APIKeyAuth())

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

	// Create new SLO
	r.POST("/api/v1/slos", func(c *gin.Context) {
		var slo slo.SLO
		if err := c.BindJSON(&slo); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		
		// Validate inputs
		if !security.ValidateServiceName(slo.Name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid SLO name"})
			return
		}
		if err := security.ValidateSLOTarget(slo.Target); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := security.ValidateSLOWindow(slo.Window); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		// Sanitize string fields
		slo.Name = security.SanitizeString(slo.Name)
		
		if err := sloTracker.AddSLO(&slo); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create SLO"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"message": "SLO created", "slo": slo})
	})

	// Get specific SLO configuration
	r.GET("/api/v1/slos/:name", func(c *gin.Context) {
		name := c.Param("name")
		slo, err := sloTracker.GetSLO(name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"slo": slo})
	})

	// Update SLO
	r.PUT("/api/v1/slos/:name", func(c *gin.Context) {
		name := c.Param("name")
		var updated slo.SLO
		if err := c.BindJSON(&updated); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := sloTracker.UpdateSLO(name, &updated); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "SLO updated", "slo": updated})
	})

	// Delete SLO
	r.DELETE("/api/v1/slos/:name", func(c *gin.Context) {
		name := c.Param("name")
		if err := sloTracker.RemoveSLO(name); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "SLO deleted"})
	})

	// Get SLO status (current compliance)
	r.GET("/api/v1/slos/:name/status", func(c *gin.Context) {
		name := c.Param("name")
		slo, err := sloTracker.GetSLO(name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		status, err := sloTracker.CheckSLO(c.Request.Context(), slo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": status})
	})

	// List all SLO configurations
	r.GET("/api/v1/slos/list", func(c *gin.Context) {
		slos := sloTracker.ListSLOs()
		c.JSON(http.StatusOK, gin.H{"slos": slos})
	})

	// Initialize Alert Manager
	alertManager := alerts.NewAlertManager()

	// Configure alert provider
	r.POST("/api/v1/alerts/config", func(c *gin.Context) {
		var config alerts.AlertConfig
		if err := c.BindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		
		// Validate provider
		if err := security.ValidateProvider(config.Provider); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		// Validate webhook URLs (SSRF protection)
		if webhookURL, ok := config.Config["webhook_url"].(string); ok {
			validatedURL, err := security.ValidateURL(webhookURL)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook URL: " + err.Error()})
				return
			}
			config.Config["webhook_url"] = validatedURL
		}
		
		// Sanitize config
		config.Config = security.SanitizeAlertConfig(config.Config)
		
		if err := alertManager.ConfigureProvider(config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to configure provider"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"message": "Alert provider configured"})
	})

	// List alert configurations
	r.GET("/api/v1/alerts/config", func(c *gin.Context) {
		configs := alertManager.GetConfigs()
		c.JSON(http.StatusOK, gin.H{"configs": configs})
	})

	// Update alert provider configuration
	r.PUT("/api/v1/alerts/config/:provider", func(c *gin.Context) {
		provider := c.Param("provider")
		var config alerts.AlertConfig
		if err := c.BindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.Provider = provider
		if err := alertManager.ConfigureProvider(config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Alert provider updated"})
	})

	// Delete alert provider
	r.DELETE("/api/v1/alerts/config/:provider", func(c *gin.Context) {
		provider := c.Param("provider")
		if err := alertManager.RemoveProvider(provider); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Alert provider removed"})
	})

	// Test alert delivery
	r.POST("/api/v1/alerts/test", func(c *gin.Context) {
		var testAlert alerts.Alert
		if err := c.BindJSON(&testAlert); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		
		// Validate severity
		if err := security.ValidateAlertSeverity(string(testAlert.Severity)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		// Sanitize inputs
		testAlert.Title = security.SanitizeString(testAlert.Title)
		testAlert.Description = security.SanitizeString(testAlert.Description)
		testAlert.Source = security.SanitizeString(testAlert.Source)
		
		if testAlert.Timestamp.IsZero() {
			testAlert.Timestamp = time.Now()
		}
		if err := alertManager.SendAlert(testAlert); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send alert"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Test alert sent successfully"})
	})

	// Initialize Deployment Tracker
	deploymentTracker, err := deployment.NewDeploymentTracker()
	if err != nil {
		log.Printf("Warning: failed to create deployment tracker: %v", err)
	} else {
		go deploymentTracker.Start(context.Background())
	}

	// Get all recent deployments
	r.GET("/api/v1/deployments", func(c *gin.Context) {
		if deploymentTracker == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "deployment tracker not available"})
			return
		}
		limit := 50
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil {
				limit = l
			}
		}
		deployments := deploymentTracker.GetAllDeployments(limit)
		c.JSON(http.StatusOK, gin.H{"deployments": deployments})
	})

	// Get deployments for a specific service
	r.GET("/api/v1/deployments/:namespace/:name", func(c *gin.Context) {
		if deploymentTracker == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "deployment tracker not available"})
			return
		}
		namespace := c.Param("namespace")
		name := c.Param("name")
		
		// Validate inputs
		if !security.ValidateNamespace(namespace) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid namespace"})
			return
		}
		if !security.ValidateServiceName(name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service name"})
			return
		}
		
		limit := 10
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
				limit = l
			}
		}
		history := deploymentTracker.GetDeploymentHistory(namespace, name, limit)
		c.JSON(http.StatusOK, gin.H{"history": history})
	})

	// Get latest deployment for a service
	r.GET("/api/v1/deployments/:namespace/:name/latest", func(c *gin.Context) {
		if deploymentTracker == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "deployment tracker not available"})
			return
		}
		namespace := c.Param("namespace")
		name := c.Param("name")
		latest := deploymentTracker.GetLatestDeployment(namespace, name)
		if latest == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no deployment found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deployment": latest})
	})

	// Initialize Cost Tracker
	costTracker := cost.NewCostTracker()

	// Record cost data
	r.POST("/api/v1/costs", func(c *gin.Context) {
		var costData cost.CostData
		if err := c.BindJSON(&costData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		
		// Validate inputs
		if !security.ValidateServiceName(costData.Service) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service name"})
			return
		}
		if !security.ValidateCost(costData.Cost) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cost value"})
			return
		}
		
		// Sanitize string fields
		costData.Service = security.SanitizeString(costData.Service)
		costData.Period = security.SanitizeString(costData.Period)
		
		costTracker.TrackCost(costData)
		c.JSON(http.StatusCreated, gin.H{"message": "Cost data recorded"})
	})

	// Get all costs
	r.GET("/api/v1/costs", func(c *gin.Context) {
		period := c.Query("period")
		services := costTracker.GetAllServices()
		allCosts := make(map[string][]cost.CostData)
		for _, service := range services {
			allCosts[service] = costTracker.GetCostByService(service, period)
		}
		c.JSON(http.StatusOK, gin.H{"costs": allCosts})
	})

	// Get costs for a specific service
	r.GET("/api/v1/costs/:service", func(c *gin.Context) {
		service := c.Param("service")
		period := c.Query("period")
		costs := costTracker.GetCostByService(service, period)
		c.JSON(http.StatusOK, gin.H{"service": service, "costs": costs})
	})

	// Get cost trend for a service
	r.GET("/api/v1/costs/:service/trend", func(c *gin.Context) {
		service := c.Param("service")
		days := 30
		if daysStr := c.Query("days"); daysStr != "" {
			if d, err := strconv.Atoi(daysStr); err == nil {
				days = d
			}
		}
		trend := costTracker.GetCostTrend(service, days)
		c.JSON(http.StatusOK, gin.H{"trend": trend})
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

	// Phase 6: Metrics Cache
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	metricsCache := cache.NewMetricsCache(redisAddr)

	// Metrics Query API (Prompt 2.3)
	metricsEngine := metrics.NewQueryEngine(v1api, metricsCache)

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

	// Phase 6: OpenTelemetry
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "localhost:4317"
	}
	tp, err := telemetry.InitTracer(context.Background(), otelEndpoint)
	if err != nil {
		log.Printf("Failed to init OTel: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
	}

	// Phase 6: ClickHouse
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	chStore, err := store.NewClickHouseStore(chAddr)
	if err != nil {
		log.Printf("Failed to connect to ClickHouse: %v", err)
	} else {
		log.Println("Connected to ClickHouse")
	}
	_ = chStore // Use it in handlers later

	// Phase 6: Integrations
	intManager := integrations.NewIntegrationManager()

	// Initialize ServiceMapBuilder with metadata cache
	metaCache := NewMetadataCache()
	builder := servicemap.NewServiceMapBuilder(metaCache)
	inspectionEngine := inspections.NewInspectionEngine(reg)

	// Phase 6: Kafka Stream Processor
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers != "" {
		// Mock aggregator for now, implemented in earlier phase
		agg := &MockAggregator{} 
		processor := stream.NewStreamProcessor(agg, strings.Split(kafkaBrokers, ","), "rca-metrics")
		go processor.Start(context.Background())
		defer processor.Close()
	}

	// Initialize SLO Tracker

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
