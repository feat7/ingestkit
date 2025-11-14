package validation

import (
	"os"
	"testing"

	"github.com/feat7/ingestkit/internal/schema"
)

// Create a test schema file for testing
func createTestSchema(t *testing.T) string {
	schemaContent := `version: "1.0"

events:
  test_event:
    fields:
      user_id:
        type: string
        required: true
        indexed: true
      email:
        type: string
        required: true
      age:
        type: integer
        required: false
      balance:
        type: decimal
        required: false
      active:
        type: boolean
        required: false
      status:
        type: string
        values: [pending, active, inactive]
      metadata:
        type: jsonb
`

	tmpFile := "/tmp/test_schema.yaml"
	err := os.WriteFile(tmpFile, []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	return tmpFile
}

func TestNewValidator(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, err := NewValidator(schemaPath)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	if validator == nil {
		t.Fatal("Expected validator to be non-nil")
	}

	if !validator.EventTypeExists("test_event") {
		t.Error("Expected test_event to exist in schema")
	}
}

func TestValidator_InvalidSchemaPath(t *testing.T) {
	_, err := NewValidator("/nonexistent/schema.yaml")
	if err == nil {
		t.Error("Expected error for non-existent schema file")
	}
}

func TestValidator_ValidateEvent_Success(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	payload := map[string]interface{}{
		"user_id": "usr_123",
		"email":   "test@example.com",
		"age":     25,
		"balance": 100.50,
		"active":  true,
		"status":  "active",
		"metadata": map[string]interface{}{
			"source": "web",
		},
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestValidator_ValidateEvent_MissingRequiredField(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	payload := map[string]interface{}{
		"user_id": "usr_123",
		// Missing required "email" field
	}

	err := validator.ValidateEvent("test_event", payload)
	if err == nil {
		t.Error("Expected validation to fail for missing required field")
	}
}

func TestValidator_ValidateEvent_InvalidType(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	payload := map[string]interface{}{
		"user_id": "usr_123",
		"email":   "test@example.com",
		"age":     "not_a_number", // Should be integer
	}

	err := validator.ValidateEvent("test_event", payload)
	if err == nil {
		t.Error("Expected validation to fail for invalid type")
	}
}

func TestValidator_ValidateEvent_InvalidEnumValue(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	payload := map[string]interface{}{
		"user_id": "usr_123",
		"email":   "test@example.com",
		"status":  "invalid_status", // Not in enum values
	}

	err := validator.ValidateEvent("test_event", payload)
	if err == nil {
		t.Error("Expected validation to fail for invalid enum value")
	}
}

func TestValidator_ValidateEvent_UnknownEventType(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	payload := map[string]interface{}{
		"field": "value",
	}

	err := validator.ValidateEvent("unknown_event", payload)
	if err == nil {
		t.Error("Expected validation to fail for unknown event type")
	}
}

func TestValidator_ValidateField_Integer(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Test valid integer
	fieldDef := &schema.Field{Type: "integer", Required: false}
	err := validator.validateField("test_field", 42, fieldDef)
	if err != nil {
		t.Errorf("Expected integer validation to pass: %v", err)
	}

	// Test float64 whole number (JSON unmarshaling)
	err = validator.validateField("test_field", float64(42), fieldDef)
	if err != nil {
		t.Errorf("Expected float64 whole number validation to pass: %v", err)
	}

	// Test invalid (float with decimal)
	err = validator.validateField("test_field", 42.5, fieldDef)
	if err == nil {
		t.Error("Expected validation to fail for float with decimal")
	}
}

func TestValidator_ValidateField_Boolean(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	fieldDef := &schema.Field{Type: "boolean", Required: false}

	// Valid boolean
	err := validator.validateField("test_field", true, fieldDef)
	if err != nil {
		t.Errorf("Expected boolean validation to pass: %v", err)
	}

	// Invalid type
	err = validator.validateField("test_field", "true", fieldDef)
	if err == nil {
		t.Error("Expected validation to fail for string instead of boolean")
	}
}

func TestValidator_ValidateField_Decimal(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	fieldDef := &schema.Field{Type: "decimal", Required: false}

	// Valid decimal
	err := validator.validateField("test_field", 42.5, fieldDef)
	if err != nil {
		t.Errorf("Expected decimal validation to pass: %v", err)
	}

	// Integer is also valid for decimal
	err = validator.validateField("test_field", 42, fieldDef)
	if err != nil {
		t.Errorf("Expected integer as decimal validation to pass: %v", err)
	}

	// Invalid type
	err = validator.validateField("test_field", "42.5", fieldDef)
	if err == nil {
		t.Error("Expected validation to fail for string instead of decimal")
	}
}

func TestValidator_EventTypeExists(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	if !validator.EventTypeExists("test_event") {
		t.Error("Expected test_event to exist")
	}

	if validator.EventTypeExists("nonexistent_event") {
		t.Error("Expected nonexistent_event to not exist")
	}
}

func TestValidator_GetEventTypes(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	types := validator.GetEventTypes()
	if len(types) == 0 {
		t.Error("Expected at least one event type")
	}

	found := false
	for _, eventType := range types {
		if eventType == "test_event" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find test_event in event types")
	}
}
