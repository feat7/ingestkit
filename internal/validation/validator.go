package validation

import (
	"fmt"

	"github.com/yourusername/ingestkit/internal/schema"
)

// Validator validates events against a schema
type Validator struct {
	schema *schema.Schema
}

// NewValidator creates a validator from a schema file
func NewValidator(schemaPath string) (*Validator, error) {
	// Parse schema file (includes validation)
	s, err := schema.ParseSchemaFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	return &Validator{schema: s}, nil
}

// ValidateEvent validates an event payload against the schema
func (v *Validator) ValidateEvent(eventType string, payload map[string]interface{}) error {
	// Check if event type exists
	event, exists := v.schema.Events[eventType]
	if !exists {
		return fmt.Errorf("unknown event type: %s", eventType)
	}

	// Validate all fields
	for fieldName, fieldDef := range event.Fields {
		value, hasValue := payload[fieldName]

		// Check required fields
		if fieldDef.Required && !hasValue {
			return fmt.Errorf("missing required field: %s", fieldName)
		}

		// Skip validation if field is not present and not required
		if !hasValue {
			continue
		}

		// Validate field type and constraints
		if err := v.validateField(fieldName, value, fieldDef); err != nil {
			return err
		}
	}

	return nil
}

// validateField validates a single field value against its definition
func (v *Validator) validateField(fieldName string, value interface{}, fieldDef *schema.Field) error {
	// Nil check
	if value == nil {
		if fieldDef.Required {
			return fmt.Errorf("field '%s' cannot be null", fieldName)
		}
		return nil
	}

	// Type validation
	switch fieldDef.Type {
	case "string":
		strValue, ok := value.(string)
		if !ok {
			return fmt.Errorf("field '%s' must be a string, got %T", fieldName, value)
		}

		// Validate enum values
		if len(fieldDef.Values) > 0 {
			valid := false
			for _, allowedValue := range fieldDef.Values {
				if strValue == allowedValue {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("field '%s' has invalid value '%s', must be one of: %v",
					fieldName, strValue, fieldDef.Values)
			}
		}

	case "integer":
		// JSON unmarshaling can give us float64 for numbers
		switch v := value.(type) {
		case int, int32, int64:
			// Valid integer
		case float64:
			// Check if it's a whole number
			if v != float64(int64(v)) {
				return fmt.Errorf("field '%s' must be an integer, got float", fieldName)
			}
		default:
			return fmt.Errorf("field '%s' must be an integer, got %T", fieldName, value)
		}

	case "decimal":
		// Accept both int and float for decimal
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid number
		default:
			return fmt.Errorf("field '%s' must be a number, got %T", fieldName, value)
		}

	case "boolean":
		_, ok := value.(bool)
		if !ok {
			return fmt.Errorf("field '%s' must be a boolean, got %T", fieldName, value)
		}

	case "timestamp":
		// Timestamps can be strings (ISO 8601) or Unix timestamps
		switch value.(type) {
		case string, int, int64, float64:
			// Valid timestamp format (detailed parsing would be done at storage layer)
		default:
			return fmt.Errorf("field '%s' must be a timestamp (string or number), got %T", fieldName, value)
		}

	case "jsonb":
		// JSONB can be any valid JSON type (object, array, etc.)
		// Just ensure it's not nil
		if value == nil {
			return fmt.Errorf("field '%s' cannot be null", fieldName)
		}

	default:
		return fmt.Errorf("unknown field type '%s' for field '%s'", fieldDef.Type, fieldName)
	}

	return nil
}

// EventTypeExists checks if an event type is defined in the schema
func (v *Validator) EventTypeExists(eventType string) bool {
	_, exists := v.schema.Events[eventType]
	return exists
}

// GetEventTypes returns all event types defined in the schema
func (v *Validator) GetEventTypes() []string {
	types := make([]string, 0, len(v.schema.Events))
	for eventType := range v.schema.Events {
		types = append(types, eventType)
	}
	return types
}
