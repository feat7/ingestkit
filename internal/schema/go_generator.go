package schema

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

//go:embed templates/go/header.tmpl
var goHeaderTemplate string

//go:embed templates/go/model.tmpl
var goModelTemplate string

// GenerateGo generates Go structs for all events in the schema
func GenerateGo(schema *Schema) (string, error) {
	var builder strings.Builder

	// Render header using template
	tmpl, err := template.New("header").Parse(goHeaderTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse header template: %w", err)
	}

	var headerBuf bytes.Buffer
	if err := tmpl.Execute(&headerBuf, nil); err != nil {
		return "", fmt.Errorf("failed to execute header template: %w", err)
	}
	builder.WriteString(headerBuf.String())

	// Add schema version constant
	builder.WriteString("// SchemaVersion is the version of the event schema this code was generated from\n")
	builder.WriteString(fmt.Sprintf("const SchemaVersion = %q\n\n", schema.Version))

	// Generate struct for each event
	for eventName, event := range schema.Events {
		goStruct, err := generateEventStruct(eventName, event)
		if err != nil {
			return "", fmt.Errorf("failed to generate Go struct for event '%s': %w", eventName, err)
		}
		builder.WriteString(goStruct)
		builder.WriteString("\n\n")
	}

	return builder.String(), nil
}

// goModelData holds the data for rendering the Go model template
type goModelData struct {
	StructName  string
	Description string
	Fields      []goFieldData
}

// goFieldData holds field information for Go generation
type goFieldData struct {
	GoName      string
	GoType      string
	JSONName    string
	ValidateTag string
	Description string
}

// generateEventStruct generates a Go struct for a single event using templates
func generateEventStruct(eventName string, event *Event) (string, error) {
	structName := toPascalCase(eventName)

	// Prepare template data
	data := goModelData{
		StructName:  structName,
		Description: event.Description,
		Fields:      make([]goFieldData, 0, len(event.Fields)),
	}

	// Collect field data
	for fieldName, field := range event.Fields {
		fieldData := goFieldData{
			GoName:      toPascalCase(fieldName),
			GoType:      mapFieldTypeToGo(field.Type),
			JSONName:    fieldName,
			ValidateTag: buildValidateTag(field),
			Description: field.Description,
		}
		data.Fields = append(data.Fields, fieldData)
	}

	// Render struct using template
	tmpl, err := template.New("model").Parse(goModelTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse model template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute model template: %w", err)
	}

	return buf.String(), nil
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// toSnakeCase converts PascalCase to snake_case (for reverse mapping)
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

// mapFieldTypeToGo maps schema field types to Go types
func mapFieldTypeToGo(fieldType string) string {
	switch fieldType {
	case "string":
		return "string"
	case "integer":
		return "int64"
	case "decimal":
		return "float64"
	case "boolean":
		return "bool"
	case "jsonb":
		return "json.RawMessage"
	case "timestamp":
		return "time.Time"
	default:
		return "interface{}"
	}
}

// buildValidateTag builds the validate tag based on field constraints
func buildValidateTag(field *Field) string {
	var validators []string

	if field.Required {
		validators = append(validators, "required")
	}

	if len(field.Values) > 0 {
		// Enum validation - go-playground/validator expects space-separated values
		enumValues := strings.Join(field.Values, " ")
		validators = append(validators, fmt.Sprintf("oneof=%s", enumValues))
	}

	// Type-specific validators
	switch field.Type {
	case "string":
		// Could add max length, etc.
	case "integer":
		// Could add min/max
	case "decimal":
		// Could add min/max
	}

	if len(validators) == 0 {
		return ""
	}

	return strings.Join(validators, ",")
}
