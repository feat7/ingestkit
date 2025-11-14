package schema

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/sql/header.tmpl
var sqlHeaderTemplate string

//go:embed templates/sql/table.tmpl
var sqlTableTemplate string

// GenerateSQL generates PostgreSQL DDL for all events in the schema
func GenerateSQL(schema *Schema) (string, error) {
	var builder strings.Builder

	// Render header using template
	tmpl, err := template.New("header").Parse(sqlHeaderTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse header template: %w", err)
	}

	var headerBuf bytes.Buffer
	if err := tmpl.Execute(&headerBuf, schema); err != nil {
		return "", fmt.Errorf("failed to execute header template: %w", err)
	}
	builder.WriteString(headerBuf.String())

	// Generate table for each event
	for eventName, event := range schema.Events {
		sql, err := generateEventTable(eventName, event)
		if err != nil {
			return "", fmt.Errorf("failed to generate SQL for event '%s': %w", eventName, err)
		}
		builder.WriteString(sql)
		builder.WriteString("\n\n")
	}

	return builder.String(), nil
}

// sqlTableData holds the data for rendering the SQL table template
type sqlTableData struct {
	EventName     string
	TableName     string
	Description   string
	Fields        []sqlFieldData
	IndexedFields []sqlFieldData
}

// sqlFieldData holds field information for SQL generation
type sqlFieldData struct {
	Name             string
	SQLType          string
	Required         bool
	Default          string
	DefaultFormatted string
	Description      string
}

// generateEventTable generates SQL DDL for a single event table using templates
func generateEventTable(eventName string, event *Event) (string, error) {
	tableName := fmt.Sprintf("events_%s", eventName)

	// Prepare template data
	data := sqlTableData{
		EventName:   eventName,
		TableName:   tableName,
		Description: event.Description,
		Fields:      make([]sqlFieldData, 0, len(event.Fields)),
		IndexedFields: make([]sqlFieldData, 0),
	}

	// Collect field data
	for fieldName, field := range event.Fields {
		fieldData := sqlFieldData{
			Name:        fieldName,
			SQLType:     mapFieldTypeToSQL(field.Type),
			Required:    field.Required,
			Default:     field.Default,
			Description: field.Description,
		}

		// Format default value if present
		if field.Default != "" {
			fieldData.DefaultFormatted = formatDefaultValue(field.Type, field.Default)
		}

		data.Fields = append(data.Fields, fieldData)

		// Collect indexed fields
		if field.Indexed {
			data.IndexedFields = append(data.IndexedFields, fieldData)
		}
	}

	// Render table using template
	tmpl, err := template.New("table").Parse(sqlTableTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse table template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute table template: %w", err)
	}

	return buf.String(), nil
}

// mapFieldTypeToSQL maps schema field types to PostgreSQL types
func mapFieldTypeToSQL(fieldType string) string {
	switch fieldType {
	case "string":
		return "VARCHAR(255)"
	case "integer":
		return "BIGINT"
	case "decimal":
		return "DECIMAL(20, 2)"
	case "boolean":
		return "BOOLEAN"
	case "jsonb":
		return "JSONB"
	case "timestamp":
		return "TIMESTAMPTZ"
	default:
		return "TEXT" // Fallback
	}
}

// formatDefaultValue formats a default value for SQL based on field type
// This prevents SQL injection by properly escaping/formatting values
func formatDefaultValue(fieldType, value string) string {
	switch fieldType {
	case "string":
		// Escape single quotes by doubling them (PostgreSQL standard)
		escaped := strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case "integer", "decimal":
		// Numeric values should not be quoted
		// TODO: Could add validation that value is actually numeric
		return value
	case "boolean":
		// Boolean values should not be quoted
		if value == "true" || value == "false" {
			return value
		}
		// Default to false if invalid
		return "false"
	case "timestamp":
		// Timestamps should be quoted
		escaped := strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case "jsonb":
		// JSON should be quoted
		escaped := strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	default:
		// For unknown types, quote and escape
		escaped := strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	}
}
