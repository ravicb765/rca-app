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
