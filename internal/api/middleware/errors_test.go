package middleware

import (
	"encoding/json"
	"errors"
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

func TestErrorHandler_CatchesErrors(t *testing.T) {
	app := fiber.New()
	app.Use(ErrorHandler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return errors.New("something went wrong")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 500 {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Parse response body
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Error != ErrCodeInternal {
		t.Errorf("Expected error code %s, got %s", ErrCodeInternal, errResp.Error)
	}
	if errResp.Message != "something went wrong" {
		t.Errorf("Expected message 'something went wrong', got %s", errResp.Message)
	}
}

func TestErrorHandler_HandlesFiberError(t *testing.T) {
	app := fiber.New()
	app.Use(ErrorHandler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusTeapot, "I'm a teapot")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusTeapot {
		t.Errorf("Expected status 418, got %d", resp.StatusCode)
	}

	// Parse response body
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Message != "I'm a teapot" {
		t.Errorf("Expected message 'I'm a teapot', got %s", errResp.Message)
	}
}

func TestErrorHandler_PassesThroughSuccess(t *testing.T) {
	app := fiber.New()
	app.Use(ErrorHandler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "success" {
		t.Errorf("Expected body 'success', got %s", string(body))
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
