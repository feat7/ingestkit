package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowOrigins string
	AllowMethods string
	AllowHeaders string
}

// NewCORSConfig creates a default CORS configuration
func NewCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins: "*", // Allow all origins for MVP (can restrict later)
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
	}
}

// CORS creates a CORS middleware with the given configuration
func CORS(config *CORSConfig) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     config.AllowOrigins,
		AllowMethods:     config.AllowMethods,
		AllowHeaders:     config.AllowHeaders,
		AllowCredentials: false,
		ExposeHeaders:    "X-Request-ID,X-RateLimit-Limit,X-RateLimit-Remaining",
	})
}
