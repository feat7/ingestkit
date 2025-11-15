package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	limiters   map[string]*bucket // map[tenant_id]*bucket
	mu         sync.RWMutex
	rps        int           // requests per second
	bucketTTL  time.Duration // time to keep idle buckets in memory
	stopClean  chan struct{} // signal to stop cleanup goroutine
}

// bucket represents a token bucket for a single tenant
type bucket struct {
	tokens     float64
	lastUpdate time.Time
	lastAccess time.Time // track last access for eviction
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter with automatic cleanup
// Buckets are evicted after 5 minutes of inactivity
func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	return NewRateLimiterWithTTL(requestsPerSecond, 5*time.Minute)
}

// NewRateLimiterWithTTL creates a new rate limiter with custom TTL
func NewRateLimiterWithTTL(requestsPerSecond int, bucketTTL time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters:  make(map[string]*bucket),
		rps:       requestsPerSecond,
		bucketTTL: bucketTTL,
		stopClean: make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes stale buckets
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.bucketTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopClean:
			return
		}
	}
}

// cleanup removes buckets that haven't been accessed within the TTL
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for tenantID, b := range rl.limiters {
		b.mu.Lock()
		idle := now.Sub(b.lastAccess)
		b.mu.Unlock()

		if idle > rl.bucketTTL {
			delete(rl.limiters, tenantID)
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopClean)
}

// allow checks if a request is allowed for the given tenant
func (rl *RateLimiter) allow(tenantID string) (bool, int, int) {
	now := time.Now()

	rl.mu.Lock()
	b, exists := rl.limiters[tenantID]
	if !exists {
		b = &bucket{
			tokens:     float64(rl.rps),
			lastUpdate: now,
			lastAccess: now,
		}
		rl.limiters[tenantID] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Update last access time
	b.lastAccess = now

	// Refill tokens based on time elapsed
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
