package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	// Setup
	config := NewAPIKeyConfig()
	config.AddKey("test_key_123", "test_tenant")

	app := fiber.New()
	app.Use(APIKeyAuth(config))
	app.Get("/test", func(c *fiber.Ctx) error {
		tenantID := c.Locals("tenant_id")
		return c.SendString("tenant:" + tenantID.(string))
	})

	// Test with valid API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test_key_123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	// Setup
	config := NewAPIKeyConfig()
	config.AddKey("valid_key", "tenant1")

	app := fiber.New()
	app.Use(APIKeyAuth(config))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("success")
	})

	// Test with invalid API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid_key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_MissingHeader(t *testing.T) {
	// Setup
	config := NewAPIKeyConfig()
	config.AddKey("test_key", "tenant1")

	app := fiber.New()
	app.Use(APIKeyAuth(config))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("success")
	})

	// Test without Authorization header
	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_BearerPrefix(t *testing.T) {
	// Setup
	config := NewAPIKeyConfig()
	config.AddKey("test_key_456", "tenant2")

	app := fiber.New()
	app.Use(APIKeyAuth(config))
	app.Get("/test", func(c *fiber.Ctx) error {
		tenantID := c.Locals("tenant_id")
		return c.SendString("tenant:" + tenantID.(string))
	})

	// Test with Bearer prefix
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test_key_456")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_WithoutBearerPrefix(t *testing.T) {
	// Setup
	config := NewAPIKeyConfig()
	config.AddKey("test_key_789", "tenant3")

	app := fiber.New()
	app.Use(APIKeyAuth(config))
	app.Get("/test", func(c *fiber.Ctx) error {
		tenantID := c.Locals("tenant_id")
		return c.SendString("tenant:" + tenantID.(string))
	})

	// Test without Bearer prefix (plain key)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "test_key_789")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIKeyConfig_AddAndValidateKey(t *testing.T) {
	config := NewAPIKeyConfig()

	// Add keys
	config.AddKey("key1", "tenant1")
	config.AddKey("key2", "tenant2")

	// Validate existing keys
	tenant, valid := config.ValidateKey("key1")
	if !valid {
		t.Error("Expected key1 to be valid")
	}
	if tenant != "tenant1" {
		t.Errorf("Expected tenant1, got %s", tenant)
	}

	tenant, valid = config.ValidateKey("key2")
	if !valid {
		t.Error("Expected key2 to be valid")
	}
	if tenant != "tenant2" {
		t.Errorf("Expected tenant2, got %s", tenant)
	}

	// Validate non-existent key
	_, valid = config.ValidateKey("invalid_key")
	if valid {
		t.Error("Expected invalid_key to be invalid")
	}
}
