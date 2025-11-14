package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequestID_GeneratesID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		requestID := c.Locals("request_id")
		if requestID == nil {
			t.Error("Expected request_id to be set in locals")
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Check response header has request ID
	responseID := resp.Header.Get("X-Request-ID")
	if responseID == "" {
		t.Error("Expected X-Request-ID header in response")
	}
	if !strings.HasPrefix(responseID, "req_") {
		t.Errorf("Expected request ID to start with 'req_', got %s", responseID)
	}
}

func TestRequestID_PreservesHeaderID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())

	var capturedID string
	app.Get("/test", func(c *fiber.Ctx) error {
		capturedID = c.Locals("request_id").(string)
		return c.SendString("ok")
	})

	providedID := "client-request-123"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", providedID)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Check that the provided ID is preserved
	if capturedID != providedID {
		t.Errorf("Expected request ID %s, got %s", providedID, capturedID)
	}

	// Check response header matches provided ID
	responseID := resp.Header.Get("X-Request-ID")
	if responseID != providedID {
		t.Errorf("Expected response header %s, got %s", providedID, responseID)
	}
}

func TestRequestID_StoredInLocals(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())

	var requestID interface{}
	app.Get("/test", func(c *fiber.Ctx) error {
		requestID = c.Locals("request_id")
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if requestID == nil {
		t.Error("Expected request_id to be stored in locals")
	}

	if _, ok := requestID.(string); !ok {
		t.Error("Expected request_id to be a string")
	}
}

func TestRequestID_AddedToResponse(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	responseID := resp.Header.Get("X-Request-ID")
	if responseID == "" {
		t.Error("Expected X-Request-ID header in response")
	}

	// Read body to ensure handler was executed
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Error("Handler was not executed properly")
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Make multiple requests
	var ids []string
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		ids = append(ids, resp.Header.Get("X-Request-ID"))
	}

	// Check all IDs are unique
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("Duplicate request ID found: %s", id)
		}
		seen[id] = true
	}
}
