package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ravicb765/rca-app/server/servicemap"
)

var kvRe = regexp.MustCompile(`(src|source|dst|dest|proto)=([^\s]+)`) 

// decodePerfRaw tries to interpret raw bytes as one of: JSON array/object, ascii key=value, or a binary conn_event
func decodePerfRaw(b []byte) ([]servicemap.Connection, error) {
	// Try JSON
	var conns []servicemap.Connection
	if err := json.Unmarshal(b, &conns); err == nil {
		return conns, nil
	}
	var conn servicemap.Connection
	if err := json.Unmarshal(b, &conn); err == nil {
		return []servicemap.Connection{conn}, nil
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
		c := servicemap.Connection{SourceApp: src, DestApp: dst, Protocol: proto}
		return []servicemap.Connection{c}, nil
	}
	// Try binary layout: IPv6 first: struct { u8 saddr[16]; u8 daddr[16]; u16 sport; u16 dport; u64 ts_ns }
	if len(b) >= 44 {
		srcIP := net.IP(b[0:16]).String()
		dstIP := net.IP(b[16:32]).String()
		sport := binary.LittleEndian.Uint16(b[32:34])
		dport := binary.LittleEndian.Uint16(b[34:36])
		src := fmt.Sprintf("%s:%d", srcIP, sport)
		dst := fmt.Sprintf("%s:%d", dstIP, dport)
		c6 := servicemap.Connection{SourceApp: src, DestApp: dst, Protocol: "tcp", Latency: 0}
		return []servicemap.Connection{c6}, nil
	}

	// IPv4 layout: struct { u32 saddr; u32 daddr; u16 sport; u16 dport; u64 ts_ns }
	if len(b) >= 20 {
		srcIP := net.IP(b[0:4]).String()
		dstIP := net.IP(b[4:8]).String()
		sport := binary.LittleEndian.Uint16(b[8:10])
		dport := binary.LittleEndian.Uint16(b[10:12])
		src := fmt.Sprintf("%s:%d", srcIP, sport)
		dst := fmt.Sprintf("%s:%d", dstIP, dport)
		c2 := servicemap.Connection{SourceApp: src, DestApp: dst, Protocol: "tcp", Latency: 0}
		return []servicemap.Connection{c2}, nil
	}
	return nil, nil
}

// decodePerfData attempts to decode hex-encoded perf/map data into connections.
// It decodes the hex string and calls decodePerfRaw on the bytes.
func decodePerfData(hexStr string) ([]servicemap.Connection, error) {
	s := strings.TrimPrefix(hexStr, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return decodePerfRaw(b)
}

// decodePerfBase64 decodes base64 payloads and calls decodePerfRaw
func decodePerfBase64(b64 string) ([]servicemap.Connection, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return decodePerfRaw(b)
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

// connStore holds observed connections from agents
type connStore struct {
	conns []servicemap.Connection
	mu    sync.RWMutex
}

func (c *connStore) add(conn servicemap.Connection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conns = append(c.conns, conn)
}

func (c *connStore) addAll(conns []servicemap.Connection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conns = append(c.conns, conns...)
}

func (c *connStore) list() []servicemap.Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]servicemap.Connection, len(c.conns))
	copy(out, c.conns)
	return out
}

// newRouter creates the HTTP handlers. Exported for testing.
func newRouter(store *serviceStore) *gin.Engine {
	r := gin.Default()

	// connection store used to build service map from agent events
	cstore := &connStore{}

	r.GET("/api/v1/servicemap", func(c *gin.Context) {
		// build a service map from observed connections
		sm := servicemap.BuildServiceMap(cstore.list())

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

	// Agent heartbeat endpoint used by node-agent
	r.POST("/api/v1/agent/heartbeat", func(c *gin.Context) {
		var payload map[string]any
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
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
			Connections []servicemap.Connection `json:"connections"`
			Map         string                  `json:"map"`
			Data        string                  `json:"data"`
			DataBase64  string                  `json:"data_base64"`
		}
		if err := json.Unmarshal(raw, &env); err == nil {
			if len(env.Connections) > 0 {
				cstore.addAll(env.Connections)
				c.JSON(http.StatusOK, gin.H{"received": true, "added": len(env.Connections)})
				return
			}
			// prefer base64 field if present (common from perf-consumer)
			if env.Map != "" && env.DataBase64 != "" {
				if conns, err := decodePerfBase64(env.DataBase64); err == nil && len(conns) > 0 {
					cstore.addAll(conns)
					c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map, "added": len(conns)})
					return
				}
				c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map})
				return
			}
			if env.Map != "" && env.Data != "" {
				// Attempt best-effort decoding of perf/map payloads (hex or raw)
				// try hex first
				if strings.HasPrefix(env.Data, "0x") {
					if conns, err := decodePerfData(env.Data); err == nil && len(conns) > 0 {
						cstore.addAll(conns)
						c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map, "added": len(conns)})
						return
					}
				} else {
					// raw ASCII or JSON bytes
					if conns, err := decodePerfRaw([]byte(env.Data)); err == nil && len(conns) > 0 {
						cstore.addAll(conns)
						c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map, "added": len(conns)})
						return
					}
				}
				// Accept perf/map event formats; decoding may be done by specialized consumers
				c.JSON(http.StatusOK, gin.H{"received": true, "map": env.Map})
				return
			}
		}

		// Backwards compatible: array of connections
		var conns []servicemap.Connection
		if err := json.Unmarshal(raw, &conns); err == nil {
			cstore.addAll(conns)
			c.JSON(http.StatusOK, gin.H{"received": true, "added": len(conns)})
			return
		}

		// Single connection
		var conn servicemap.Connection
		if err := json.Unmarshal(raw, &conn); err == nil {
			cstore.add(conn)
			c.JSON(http.StatusOK, gin.H{"received": true, "added": 1})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
	})

	return r
}

// fetchOnce performs a single cluster-agent fetch and updates metrics and store.
func fetchOnce(client *http.Client, clusterAgent string, store *serviceStore, success, errors, total prometheus.Counter) error {
	if clusterAgent == "" {
		clusterAgent = "http://cluster-agent:9100"
	}
	total.Inc()
	resp, err := client.Get(clusterAgent + "/api/v1/cluster/services")
	if err != nil {
		errors.Inc()
		log.Printf("cluster fetch error: %v", err)
		return err
	}
	defer resp.Body.Close()
	var payload map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		errors.Inc()
		log.Printf("cluster fetch decode error: %v", err)
		return err
	}
	if s, ok := payload["services"]; ok {
		store.set(s)
		success.Inc()
	}
	return nil
}

func startClusterFetcher(store *serviceStore, stopCh <-chan struct{}, success, errors, total prometheus.Counter) {
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
			if err := fetchOnce(client, clusterAgent, store, success, errors, total); err != nil {
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

	// Prometheus metrics for server-side ingestion
	reg := prometheus.NewRegistry()
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

	r := newRouter(store)
	// add server metrics endpoint bound to registry
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	stopCh := make(chan struct{})
	go startClusterFetcher(store, stopCh, fetchSuccess, fetchErrors, fetchTotal)

	// shutdown handling omitted for brevity
	r.Run(":" + port)
}
