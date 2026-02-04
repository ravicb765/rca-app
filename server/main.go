package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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

func startClusterFetcher(store *serviceStore, stopCh <-chan struct{}) {
	clusterAgent := os.Getenv("CLUSTER_AGENT_URL")
	if clusterAgent == "" {
		clusterAgent = "http://cluster-agent:9100"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		select {
		case <-stopCh:
			return
		default:
			resp, err := client.Get(clusterAgent + "/api/v1/cluster/services")
			if err != nil {
				log.Printf("cluster fetch error: %v", err)
				time.Sleep(30 * time.Second)
				continue
			}
			var payload map[string][]string
			if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
				if s, ok := payload["services"]; ok {
					store.set(s)
				}
			}
			_ = resp.Body.Close()
			time.Sleep(30 * time.Second)
		}
	}
}

func main() {
	store := &serviceStore{}
	r := newRouter(store)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	stopCh := make(chan struct{})
	go startClusterFetcher(store, stopCh)

	// shutdown handling omitted for brevity
	r.Run(":" + port)
}
