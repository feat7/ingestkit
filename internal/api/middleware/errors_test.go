package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSendError_BasicError(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return SendError(c, fiber.StatusBadRequest, ErrCodeValidation, "test error message")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	// Parse response body
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Error != ErrCodeValidation {
		t.Errorf("Expected error code %s, got %s", ErrCodeValidation, errResp.Error)
	}
	if errResp.Message != "test error message" {
		t.Errorf("Expected message 'test error message', got %s", errResp.Message)
	}
}

func TestSendError_WithRequestID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		return SendError(c, fiber.StatusUnauthorized, ErrCodeAuth, "invalid credentials")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}

	// Parse response body
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.RequestID == "" {
		t.Error("Expected request_id to be present in error response")
	}
}

func TestSendError_WithoutRequestID(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return SendError(c, fiber.StatusNotFound, ErrCodeNotFound, "resource not found")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Parse response body
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	// RequestID should be empty string when not set
	if errResp.RequestID != "" {
		t.Errorf("Expected empty request_id, got %s", errResp.RequestID)
	}
}

func TestErrorResponse_HasTimestamp(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return SendError(c, fiber.StatusBadRequest, ErrCodeBadRequest, "test error")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Parse response body
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set in error response")
	}
}
