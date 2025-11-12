package schema

import (
	"fmt"
	"os"

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

// ParseSchemaFile reads and parses a YAML schema file
func ParseSchemaFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	return ParseSchema(data)
}

// ParseSchema parses YAML schema data
func ParseSchema(data []byte) (*Schema, error) {
	var schema Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema YAML: %w", err)
	}

	// Validate schema
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
