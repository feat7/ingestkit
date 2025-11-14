// Package schema provides event schema parsing and code generation.
//
// It reads YAML schema definitions and generates:
//   - SQL DDL for PostgreSQL tables
//   - Go models with validation tags
//   - Storage layer code using pgx COPY protocol
//
// The schema-first approach ensures consistency between database,
// application code, and validation rules.
package schema

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema represents the entire event schema definition
type Schema struct {
	Version string             `yaml:"version"`
	Events  map[string]*Event  `yaml:"events"`
}

// Event represents a single event type definition
type Event struct {
	Description string              `yaml:"description"`
	Fields      map[string]*Field   `yaml:"fields"`
}

// Field represents a field in an event
type Field struct {
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Indexed     bool     `yaml:"indexed"`
	Description string   `yaml:"description"`
	Default     string   `yaml:"default"`
	Values      []string `yaml:"values"`
}

// ParseSchemaFile reads and parses a YAML schema file.
// The schema is automatically validated during parsing, so callers
// do not need to call Validate() separately.
func ParseSchemaFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	return ParseSchema(data)
}

// ParseSchema parses YAML schema data.
// The schema is automatically validated during parsing, so callers
// do not need to call Validate() separately. Returns a validated
// Schema or an error if parsing or validation fails.
func ParseSchema(data []byte) (*Schema, error) {
	var schema Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema YAML: %w", err)
	}

	// Validate schema (automatic validation - callers don't need to validate again)
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	return &schema, nil
}

// Validate performs validation on the schema
func (s *Schema) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("schema version is required")
	}

	// Validate version format (semantic versioning: major.minor or major.minor.patch)
	if err := validateSchemaVersion(s.Version); err != nil {
		return err
	}

	if len(s.Events) == 0 {
		return fmt.Errorf("schema must define at least one event")
	}

	for eventName, event := range s.Events {
		if err := event.Validate(eventName); err != nil {
			return err
		}
	}

	return nil
}

// validateSchemaVersion validates the schema version format
// Accepts semantic versioning formats: "1.0", "1.0.0", "2.1", "2.1.5"
func validateSchemaVersion(version string) error {
	// Simple validation: version must be major.minor or major.minor.patch
	// where major, minor, patch are numeric
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("invalid schema version format '%s': must be 'major.minor' or 'major.minor.patch'", version)
	}

	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("invalid schema version format '%s': empty version component", version)
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return fmt.Errorf("invalid schema version format '%s': version component %d contains non-numeric character", version, i+1)
			}
		}
	}

	return nil
}

// Validate performs validation on an event
func (e *Event) Validate(eventName string) error {
	if len(e.Fields) == 0 {
		return fmt.Errorf("event '%s' must have at least one field", eventName)
	}

	for fieldName, field := range e.Fields {
		if err := field.Validate(eventName, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// Validate performs validation on a field
func (f *Field) Validate(eventName, fieldName string) error {
	validTypes := map[string]bool{
		"string":  true,
		"integer": true,
		"decimal": true,
		"boolean": true,
		"jsonb":   true,
		"timestamp": true,
	}

	if !validTypes[f.Type] {
		return fmt.Errorf("event '%s', field '%s': invalid type '%s'", eventName, fieldName, f.Type)
	}

	return nil
}

// GetEventNames returns a sorted list of event names
func (s *Schema) GetEventNames() []string {
	names := make([]string, 0, len(s.Events))
	for name := range s.Events {
		names = append(names, name)
	}
	return names
}
