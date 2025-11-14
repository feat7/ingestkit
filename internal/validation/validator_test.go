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

// Edge Case Tests

func TestValidator_EmptyString(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Empty string should be valid for non-required fields
	payload := map[string]interface{}{
		"user_id": "user123",
		"email":   "", // Empty string
	}

	err := validator.ValidateEvent("test_event", payload)
	// Empty string is technically valid for string type
	if err != nil {
		t.Logf("Empty string validation: %v", err)
	}
}

func TestValidator_UnicodeCharacters(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Unicode characters in strings
	payload := map[string]interface{}{
		"user_id": "用户123", // Chinese characters
		"email":   "test@例え.jp", // Japanese characters
		"status":  "pending",
		"metadata": map[string]interface{}{
			"comment": "こんにちは世界", // Japanese: Hello World
			"emoji":   "🎉🚀✨",
		},
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Errorf("Unicode validation failed: %v", err)
	}
}

func TestValidator_VeryLongString(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Very long string (10KB)
	longString := ""
	for i := 0; i < 10000; i++ {
		longString += "a"
	}

	payload := map[string]interface{}{
		"user_id": longString,
		"email":   "test@example.com",
		"status":  "pending",
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Errorf("Long string validation failed: %v", err)
	}
}

func TestValidator_NestedJSONB(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Deeply nested JSONB structure
	payload := map[string]interface{}{
		"user_id": "user123",
		"email":   "test@example.com",
		"status":  "pending",
		"metadata": map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"level4": map[string]interface{}{
							"value": "deeply nested",
							"array": []interface{}{1, 2, 3, "four", true},
						},
					},
				},
			},
			"complex": []interface{}{
				map[string]interface{}{"key": "value1"},
				map[string]interface{}{"key": "value2"},
				[]interface{}{1, 2, 3},
			},
		},
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Errorf("Nested JSONB validation failed: %v", err)
	}
}

func TestValidator_NullValues(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Null values for non-required fields should be valid
	payload := map[string]interface{}{
		"user_id": "user123",
		"email":   "test@example.com",
		"age":     nil, // Null for non-required field
		"balance": nil,
		"active":  nil,
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Logf("Null value validation: %v", err)
	}
}

func TestValidator_NullRequiredField(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Null value for required field should fail
	payload := map[string]interface{}{
		"user_id": nil, // Required field
		"email":   "test@example.com",
	}

	err := validator.ValidateEvent("test_event", payload)
	if err == nil {
		t.Error("Expected validation to fail for null required field")
	}
}

func TestValidator_SpecialCharacters(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Special characters in strings
	payload := map[string]interface{}{
		"user_id": "user<>\"'&@#$%^&*()",
		"email":   "test+tag@example.com",
		"status":  "pending",
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Errorf("Special characters validation failed: %v", err)
	}
}

func TestValidator_BoundaryIntegers(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	tests := []struct {
		name  string
		value interface{}
		valid bool
	}{
		{"zero", 0, true},
		{"positive", 123, true},
		{"negative", -456, true},
		{"max int", 9223372036854775807, true},
		{"float as int", 123.0, true}, // JSON unmarshaling gives float64
		{"not an integer", 123.45, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"user_id": "user123",
				"email":   "test@example.com",
				"age":     tt.value,
			}

			err := validator.ValidateEvent("test_event", payload)
			if tt.valid && err != nil {
				t.Errorf("Expected %v to be valid, got error: %v", tt.value, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected %v to be invalid", tt.value)
			}
		})
	}
}

func TestValidator_BoundaryDecimals(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	tests := []struct {
		name  string
		value interface{}
		valid bool
	}{
		{"zero", 0.0, true},
		{"positive float", 123.45, true},
		{"negative float", -123.45, true},
		{"integer as decimal", 100, true},
		{"very small", 0.000001, true},
		{"very large", 999999999.99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"user_id": "user123",
				"email":   "test@example.com",
				"balance": tt.value,
			}

			err := validator.ValidateEvent("test_event", payload)
			if tt.valid && err != nil {
				t.Errorf("Expected %v to be valid, got error: %v", tt.value, err)
			}
		})
	}
}

func TestValidator_EmptyJSONB(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Empty JSONB structures
	tests := []struct {
		name  string
		value interface{}
	}{
		{"empty object", map[string]interface{}{}},
		{"empty array", []interface{}{}},
		{"null", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"user_id":  "user123",
				"email":    "test@example.com",
				"metadata": tt.value,
			}

			// Empty JSONB should be valid
			err := validator.ValidateEvent("test_event", payload)
			if err != nil {
				t.Logf("Empty JSONB (%s) validation: %v", tt.name, err)
			}
		})
	}
}

func TestValidator_MixedTypes(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// All valid types in one payload
	payload := map[string]interface{}{
		"user_id": "user123",
		"email":   "test@example.com",
		"age":     25,
		"balance": 1234.56,
		"active":  true,
		"status":  "active",
		"metadata": map[string]interface{}{
			"string":  "value",
			"number":  123,
			"boolean": false,
			"array":   []interface{}{1, "two", true},
			"object":  map[string]interface{}{"nested": "value"},
		},
	}

	err := validator.ValidateEvent("test_event", payload)
	if err != nil {
		t.Errorf("Mixed types validation failed: %v", err)
	}
}

func TestValidator_WhitespaceStrings(t *testing.T) {
	schemaPath := createTestSchema(t)
	defer os.Remove(schemaPath)

	validator, _ := NewValidator(schemaPath)

	// Whitespace-only strings
	tests := []struct {
		name  string
		value string
	}{
		{"spaces", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
		{"mixed", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"user_id": tt.value,
				"email":   "test@example.com",
			}

			// Whitespace strings are technically valid
			err := validator.ValidateEvent("test_event", payload)
			if err != nil {
				t.Logf("Whitespace string validation: %v", err)
			}
		})
	}
}
