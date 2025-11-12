package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RequestID adds a unique request ID to each request
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if request ID already exists in header
		requestID := c.Get("X-Request-ID")

		// Generate new ID if not provided
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Store in context for use by other middleware/handlers
		c.Locals("request_id", requestID)

		// Add to response header
		c.Set("X-Request-ID", requestID)

		return c.Next()
	}
}

// generateRequestID creates a unique request ID
// Format: req_<unix_nano>_<random>
func generateRequestID() string {
	// Using timestamp for uniqueness (good enough for MVP)
	// For production, consider using UUID or more sophisticated ID generation
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
