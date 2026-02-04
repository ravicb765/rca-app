package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// newRouter creates the HTTP handlers. Exported for testing.
func newRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/api/v1/servicemap", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"services": []string{"service-a", "service-b"}})
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

func main() {
	r := newRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}
