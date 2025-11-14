package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	limiters map[string]*bucket // map[tenant_id]*bucket
	mu       sync.RWMutex
	rps      int // requests per second
}

// bucket represents a token bucket for a single tenant
type bucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*bucket),
		rps:      requestsPerSecond,
	}
}

// allow checks if a request is allowed for the given tenant
func (rl *RateLimiter) allow(tenantID string) (bool, int, int) {
	rl.mu.Lock()
	b, exists := rl.limiters[tenantID]
	if !exists {
		b = &bucket{
			tokens:     float64(rl.rps),
			lastUpdate: time.Now(),
		}
		rl.limiters[tenantID] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens += elapsed * float64(rl.rps)

	// Cap at max tokens (burst allowance)
	maxTokens := float64(rl.rps)
	if b.tokens > maxTokens {
		b.tokens = maxTokens
	}

	b.lastUpdate = now

	// Try to consume a token
	if b.tokens >= 1 {
		b.tokens--
		return true, rl.rps, int(b.tokens)
	}

	return false, rl.rps, 0
}

// RateLimit creates a rate limiting middleware
func RateLimit(limiter *RateLimiter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get tenant_id from context (set by auth middleware)
		tenantIDRaw := c.Locals("tenant_id")
		tenantID := "default" // Default value

		if tenantIDRaw != nil {
			// Try to convert to string
			if tid, ok := tenantIDRaw.(string); ok {
				tenantID = tid
			}
			// If not a string, use default (shouldn't happen if middleware is configured correctly)
		}

		allowed, limit, remaining := limiter.allow(tenantID)

		// Set rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			return SendError(c, fiber.StatusTooManyRequests, ErrCodeRateLimit,
				"rate limit exceeded, please try again later")
		}

		return c.Next()
	}
}
