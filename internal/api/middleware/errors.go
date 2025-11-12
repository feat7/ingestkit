package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// ErrorResponse represents a standardized API error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorCode constants for common errors
const (
	ErrCodeValidation    = "validation_failed"
	ErrCodeAuth          = "authentication_failed"
	ErrCodeRateLimit     = "rate_limit_exceeded"
	ErrCodeInternal      = "internal_error"
	ErrCodeBadRequest    = "bad_request"
	ErrCodeNotFound      = "not_found"
	ErrCodeUnknownEvent  = "unknown_event_type"
)

// SendError sends a standardized error response
func SendError(c *fiber.Ctx, status int, code string, message string) error {
	requestID := c.Locals("request_id")
	var reqID string
	if requestID != nil {
		reqID = requestID.(string)
	}

	return c.Status(status).JSON(ErrorResponse{
		Error:     code,
		Message:   message,
		RequestID: reqID,
		Timestamp: time.Now(),
	})
}

// ErrorHandler is a middleware that catches panics and converts them to error responses
func ErrorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		if err != nil {
			// Check if it's a Fiber error
			if e, ok := err.(*fiber.Error); ok {
				return SendError(c, e.Code, ErrCodeInternal, e.Message)
			}

			// Default to 500 internal server error
			return SendError(c, fiber.StatusInternalServerError, ErrCodeInternal, err.Error())
		}

		return nil
	}
}
