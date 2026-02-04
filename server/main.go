package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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

// newRouter creates the HTTP handlers. Exported for testing.
func newRouter(store *serviceStore) *gin.Engine {
	r := gin.Default()

	r.GET("/api/v1/servicemap", func(c *gin.Context) {
		// merge local static and cluster-agent-sourced services
		svc := store.list()
		if len(svc) == 0 {
			svc = []string{"service-a", "service-b"}
		}
		c.JSON(http.StatusOK, gin.H{"services": svc})
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

	// Agent event endpoint used by perf readers
	r.POST("/api/v1/agent/event", func(c *gin.Context) {
		var payload map[string]any
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"received": true})
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
