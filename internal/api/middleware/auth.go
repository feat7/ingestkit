package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// APIKeyConfig holds the API key to tenant_id mapping
type APIKeyConfig struct {
	Keys map[string]string // map[api_key]tenant_id
}

// NewAPIKeyConfig creates a new API key configuration
func NewAPIKeyConfig() *APIKeyConfig {
	return &APIKeyConfig{
		Keys: make(map[string]string),
	}
}

// AddKey adds an API key with its associated tenant_id
func (c *APIKeyConfig) AddKey(apiKey, tenantID string) {
	c.Keys[apiKey] = tenantID
}

// ValidateKey checks if an API key is valid and returns the tenant_id
func (c *APIKeyConfig) ValidateKey(apiKey string) (string, bool) {
	tenantID, exists := c.Keys[apiKey]
	return tenantID, exists
}

// APIKeyAuth creates an authentication middleware
func APIKeyAuth(config *APIKeyConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract API key from Authorization header
		// Expected format: "Bearer <api_key>" or just "<api_key>"
		authHeader := c.Get("Authorization")

		if authHeader == "" {
			return SendError(c, fiber.StatusUnauthorized, ErrCodeAuth, "Missing Authorization header")
		}

		// Extract the key (handle both "Bearer <key>" and plain "<key>")
		var apiKey string
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			apiKey = authHeader
		}

		// Validate the API key
		tenantID, valid := config.ValidateKey(apiKey)
		if !valid {
			return SendError(c, fiber.StatusUnauthorized, ErrCodeAuth, "Invalid API key")
		}

		// Store tenant_id in context for use by handlers
		c.Locals("tenant_id", tenantID)

		return c.Next()
	}
}
