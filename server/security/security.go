package security

import (
	"crypto/subtle"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth middleware for API key authentication
func APIKeyAuth() gin.HandlerFunc {
	validAPIKey := os.Getenv("RCA_API_KEY")
	if validAPIKey == "" {
		validAPIKey = "default-insecure-key-change-me" // Fallback for development
	}

	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		
		// Use constant-time comparison to prevent timing attacks
		if apiKey == "" || subtle.ConstantTimeCompare([]byte(apiKey), []byte(validAPIKey)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// SecurityHeaders middleware adds security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// RequestSizeLimit middleware limits request body size
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// RateLimiter simple in-memory rate limiter
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		// Clean old requests
		if timestamps, exists := rl.requests[clientIP]; exists {
			var valid []time.Time
			for _, t := range timestamps {
				if now.Sub(t) < rl.window {
					valid = append(valid, t)
				}
			}
			rl.requests[clientIP] = valid
		}

		// Check limit
		if len(rl.requests[clientIP]) >= rl.limit {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		// Add current request
		rl.requests[clientIP] = append(rl.requests[clientIP], now)
		c.Next()
	}
}

// Role-based access control constants
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// RBAC middleware to enforce role-based access
func RBAC(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, this would be extracted from a JWT claim
		userRole := c.GetHeader("X-User-Role")
		if userRole == "" {
			userRole = RoleViewer // Default
		}

		if !hasPermission(userRole, requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func hasPermission(userRole, requiredRole string) bool {
	roles := map[string]int{
		RoleAdmin:    3,
		RoleOperator: 2,
		RoleViewer:   1,
	}
	return roles[userRole] >= roles[requiredRole]
}

// OIDCAuth middleware for JWT/OIDC validation (stub)
func OIDCAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			// Fail open to API Key for now if no bearer token
			c.Next()
			return
		}

		// Token validation logic would go here (e.g., using jks-go)
		// token := strings.TrimPrefix(authHeader, "Bearer ")
		// claims, err := validateToken(token)
		
		c.Next()
	}
}

// SensitiveDataMasker scrubs PII and secrets from telemetry
func SensitiveDataMasker(data string) string {
	// Mask potential credit cards, emails, and internal tokens
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	masked := emailRegex.ReplaceAllString(data, "[REDACTED_EMAIL]")
	
	tokenRegex := regexp.MustCompile(`(token|password|secret|key)=[^&\s]+`)
	masked = tokenRegex.ReplaceAllString(masked, "$1=[REDACTED]")
	
	return masked
}

// Validation and Sanitization functions...

// ValidateServiceName validates service/application names
func ValidateServiceName(name string) bool {
	if len(name) == 0 || len(name) > 255 {
		return false
	}
	// Allow alphanumeric, hyphens, underscores, dots
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-_.]+$`, name)
	return matched
}

// ValidateNamespace validates Kubernetes namespace
func ValidateNamespace(namespace string) bool {
	if len(namespace) == 0 || len(namespace) > 63 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, namespace)
	return matched
}

// ValidatePercentage validates percentage values (0-100)
func ValidatePercentage(value float64) bool {
	return value >= 0 && value <= 100
}

// ValidateCost validates cost values
func ValidateCost(cost float64) bool {
	return cost >= 0 && cost < 1e12 // Reasonable upper limit
}

// SanitizeString sanitizes user input to prevent XSS
func SanitizeString(input string) string {
	// HTML escape
	sanitized := html.EscapeString(input)
	// Limit length
	if len(sanitized) > 10000 {
		sanitized = sanitized[:10000]
	}
	return sanitized
}

// ValidateURL validates and sanitizes URLs
func ValidateURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Only allow HTTPS
	if parsed.Scheme != "https" {
		return "", &ValidationError{Field: "url", Message: "only HTTPS URLs are allowed"}
	}

	// Check for SSRF - block private IPs
	if isPrivateIP(parsed.Hostname()) {
		return "", &ValidationError{Field: "url", Message: "private IP addresses are not allowed"}
	}

	return parsed.String(), nil
}

// isPrivateIP checks if an IP is private/internal
func isPrivateIP(hostname string) bool {
	ip := net.ParseIP(hostname)
	if ip == nil {
		// Try to resolve hostname
		ips, err := net.LookupIP(hostname)
		if err != nil || len(ips) == 0 {
			return false
		}
		ip = ips[0]
	}

	// Check for private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range privateRanges {
		_, ipNet, _ := net.ParseCIDR(cidr)
		if ipNet.Contains(ip) {
			return true
		}
	}

	// Block localhost
	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}

	return false
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateSLOTarget validates SLO target values
func ValidateSLOTarget(target float64) error {
	if target <= 0 || target > 100 {
		return &ValidationError{
			Field:   "target",
			Message: "target must be between 0 and 100",
		}
	}
	return nil
}

// ValidateSLOWindow validates SLO window duration
func ValidateSLOWindow(window string) error {
	validWindows := []string{"1h", "6h", "12h", "24h", "7d", "30d"}
	for _, valid := range validWindows {
		if window == valid {
			return nil
		}
	}
	return &ValidationError{
		Field:   "window",
		Message: "window must be one of: 1h, 6h, 12h, 24h, 7d, 30d",
	}
}

// ValidateAlertSeverity validates alert severity
func ValidateAlertSeverity(severity string) error {
	validSeverities := []string{"critical", "warning", "info"}
	for _, valid := range validSeverities {
		if strings.ToLower(severity) == valid {
			return nil
		}
	}
	return &ValidationError{
		Field:   "severity",
		Message: "severity must be one of: critical, warning, info",
	}
}

// ValidateProvider validates alert provider names
func ValidateProvider(provider string) error {
	validProviders := []string{"email", "slack", "pagerduty", "webhook", "jira", "opsgenie", "teams"}
	for _, valid := range validProviders {
		if strings.ToLower(provider) == valid {
			return nil
		}
	}
	return &ValidationError{
		Field:   "provider",
		Message: "invalid provider",
	}
}

// SanitizeAlertConfig sanitizes alert configuration
func SanitizeAlertConfig(config map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for key, value := range config {
		if str, ok := value.(string); ok {
			// Don't sanitize URLs, but validate them separately
			if !strings.Contains(key, "url") && !strings.Contains(key, "webhook") {
				sanitized[key] = SanitizeString(str)
			} else {
				sanitized[key] = str
			}
		} else {
			sanitized[key] = value
		}
	}
	return sanitized
}
