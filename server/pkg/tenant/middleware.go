package tenant

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	HeaderTenantID = "X-Tenant-ID"
	KeyTenantID    = "tenant_id"
)

// Middleware extracts the X-Tenant-ID header and sets it in the context
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader(HeaderTenantID)
		if tenantID == "" {
			// In a strict implementation, we might abort here.
			// c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-Tenant-ID required"})
			// return
			
			// For now, default to "default"
			tenantID = "default"
		}

		// Set in Gin context
		c.Set(KeyTenantID, tenantID)

		// Also set in Request context for compatibility with stdlib/grpc
		ctx := context.WithValue(c.Request.Context(), KeyTenantID, tenantID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// FromContext retrieves the tenant ID from a context
func FromContext(ctx context.Context) (string, error) {
	val := ctx.Value(KeyTenantID)
	if val == nil {
		return "", errors.New("tenant id not found in context")
	}
	id, ok := val.(string)
	if !ok {
		return "", errors.New("tenant id is not a string")
	}
	return id, nil
}
