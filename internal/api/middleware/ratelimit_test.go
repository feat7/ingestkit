package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 requests per second
	app := fiber.New()

	// Set tenant_id manually
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", "test_tenant")
		return c.Next()
	})
	app.Use(RateLimit(limiter))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Make requests within limit
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Request %d: expected status 200, got %d", i, resp.StatusCode)
		}
	}
}

func TestRateLimit_BlocksWhenExceeded(t *testing.T) {
	limiter := NewRateLimiter(5) // 5 requests per second
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", "test_tenant")
		return c.Next()
	})
	app.Use(RateLimit(limiter))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Request %d: expected status 200, got %d", i, resp.StatusCode)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 429 {
		t.Errorf("Expected status 429, got %d", resp.StatusCode)
	}

	// Verify error response
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}
	if errResp.Error != ErrCodeRateLimit {
		t.Errorf("Expected error code %s, got %s", ErrCodeRateLimit, errResp.Error)
	}
}

func TestRateLimit_RefillsOverTime(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 requests per second
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", "test_tenant")
		return c.Next()
	})
	app.Use(RateLimit(limiter))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Use up all tokens
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		app.Test(req)
	}

	// Next request should fail
	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 429 {
		t.Errorf("Expected rate limit, got status %d", resp.StatusCode)
	}

	// Wait for tokens to refill (100ms = 1 token at 10 RPS)
	time.Sleep(150 * time.Millisecond)

	// Request should succeed now
	req = httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200 after refill, got %d", resp.StatusCode)
	}
}

func TestRateLimit_PerTenant(t *testing.T) {
	limiter := NewRateLimiter(5) // 5 requests per second per tenant

	// Create separate apps for each tenant to avoid middleware order issues
	app1 := fiber.New()
	app1.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", "tenant1")
		return c.Next()
	})
	app1.Use(RateLimit(limiter))
	app1.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	app2 := fiber.New()
	app2.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", "tenant2")
		return c.Next()
	})
	app2.Use(RateLimit(limiter))
	app2.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Tenant 1: use up all tokens
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app1.Test(req)
		if resp.StatusCode != 200 {
			t.Errorf("Tenant1 request %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	// Tenant 1: next request should fail
	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app1.Test(req)
	if resp.StatusCode != 429 {
		t.Errorf("Tenant1: expected rate limit (429), got %d", resp.StatusCode)
	}

	// Tenant 2: should still have tokens (separate bucket)
	req = httptest.NewRequest("GET", "/test", nil)
	resp, err := app2.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Tenant2: expected status 200, got %d", resp.StatusCode)
	}
}

func TestRateLimit_SetsHeaders(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 requests per second
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", "test_tenant")
		return c.Next()
	})
	app.Use(RateLimit(limiter))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Check rate limit headers
	limitHeader := resp.Header.Get("X-RateLimit-Limit")
	if limitHeader != "10" {
		t.Errorf("Expected X-RateLimit-Limit to be '10', got '%s'", limitHeader)
	}

	remainingHeader := resp.Header.Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Error("Expected X-RateLimit-Remaining header to be set")
	}
}

func TestRateLimit_DefaultTenant(t *testing.T) {
	limiter := NewRateLimiter(5)
	app := fiber.New()

	// Don't set tenant_id, should use "default"
	app.Use(RateLimit(limiter))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Should allow requests for default tenant
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestNewRateLimiter_InitializesCorrectly(t *testing.T) {
	limiter := NewRateLimiter(100)

	if limiter.rps != 100 {
		t.Errorf("Expected RPS to be 100, got %d", limiter.rps)
	}

	if limiter.limiters == nil {
		t.Error("Expected limiters map to be initialized")
	}
}
