package schema

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

// SDKLanguage represents a target SDK language
type SDKLanguage string

const (
	SDKLanguagePython     SDKLanguage = "python"
	SDKLanguageTypeScript SDKLanguage = "typescript"
	SDKLanguageGo         SDKLanguage = "go"
	SDKLanguageJava       SDKLanguage = "java"
)

//go:embed templates/sdk/python/models.py.tmpl
var pythonModelsTemplate string

//go:embed templates/sdk/python/client.py.tmpl
var pythonClientTemplate string

//go:embed templates/sdk/python/__init__.py.tmpl
var pythonInitTemplate string

//go:embed templates/sdk/typescript/models.ts.tmpl
var typescriptModelsTemplate string

//go:embed templates/sdk/typescript/client.ts.tmpl
var typescriptClientTemplate string

//go:embed templates/sdk/typescript/index.ts.tmpl
var typescriptIndexTemplate string

// GenerateSDK generates SDK code for the specified language
func GenerateSDK(schema *Schema, language SDKLanguage, apiURL string) (map[string]string, error) {
	switch language {
	case SDKLanguagePython:
		return generatePythonSDK(schema, apiURL)
	case SDKLanguageTypeScript:
		return generateTypeScriptSDK(schema, apiURL)
	case SDKLanguageGo:
		return generateGoSDK(schema, apiURL)
	case SDKLanguageJava:
		return nil, fmt.Errorf("Java SDK generation not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported SDK language: %s", language)
	}
}

// sdkModelData holds data for SDK model generation
type sdkModelData struct {
	EventName   string
	ClassName   string
	SnakeName   string
	Description string
	Fields      []sdkFieldData
}

// sdkFieldData holds field information for SDK generation
type sdkFieldData struct {
	Name        string
	Type        string
	Required    bool
	Description string
	Values      []string
}

// sdkData holds all data needed for SDK generation
type sdkData struct {
	SchemaVersion string
	APIURL        string
	Events        []sdkModelData
}

// prepareSDKData prepares data for SDK template rendering
func prepareSDKData(schema *Schema, apiURL string, typeMapper func(string) string) sdkData {
	data := sdkData{
		SchemaVersion: schema.Version,
		APIURL:        apiURL,
		Events:        make([]sdkModelData, 0, len(schema.Events)),
	}

	for eventName, event := range schema.Events {
		eventData := sdkModelData{
			EventName:   eventName,
			ClassName:   toPascalCase(eventName),
			SnakeName:   toSnakeCase(toPascalCase(eventName)),
			Description: event.Description,
			Fields:      make([]sdkFieldData, 0, len(event.Fields)),
		}

		for fieldName, field := range event.Fields {
			fieldData := sdkFieldData{
				Name:        fieldName,
				Type:        typeMapper(field.Type),
				Required:    field.Required,
				Description: field.Description,
				Values:      field.Values,
			}
			eventData.Fields = append(eventData.Fields, fieldData)
		}

		data.Events = append(data.Events, eventData)
	}

	return data
}

// Type mapping functions for different languages

// mapFieldTypeToPython maps schema field types to Python types
func mapFieldTypeToPython(fieldType string) string {
	switch fieldType {
	case "string":
		return "str"
	case "integer":
		return "int"
	case "decimal":
		return "Decimal"
	case "boolean":
		return "bool"
	case "jsonb":
		return "dict"
	case "timestamp":
		return "datetime"
	default:
		return "Any"
	}
}

// mapFieldTypeToTypeScript maps schema field types to TypeScript types
func mapFieldTypeToTypeScript(fieldType string) string {
	switch fieldType {
	case "string":
		return "string"
	case "integer":
		return "number"
	case "decimal":
		return "number"
	case "boolean":
		return "boolean"
	case "jsonb":
		return "Record<string, any>"
	case "timestamp":
		return "Date"
	default:
		return "unknown"
	}
}

// mapFieldTypeToJava maps schema field types to Java types
func mapFieldTypeToJava(fieldType string) string {
	switch fieldType {
	case "string":
		return "String"
	case "integer":
		return "Long"
	case "decimal":
		return "BigDecimal"
	case "boolean":
		return "Boolean"
	case "jsonb":
		return "Map<String, Object>"
	case "timestamp":
		return "Instant"
	default:
		return "Object"
	}
}

// generatePythonSDK generates Python SDK files
func generatePythonSDK(schema *Schema, apiURL string) (map[string]string, error) {
	files := make(map[string]string)

	// Prepare data
	data := prepareSDKData(schema, apiURL, mapFieldTypeToPython)

	// Generate models.py
	modelsContent, err := renderTemplate("python-models", pythonModelsTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate models.py: %w", err)
	}
	files["models.py"] = modelsContent

	// Generate client.py
	clientContent, err := renderTemplate("python-client", pythonClientTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client.py: %w", err)
	}
	files["client.py"] = clientContent

	// Generate __init__.py
	initContent, err := renderTemplate("python-init", pythonInitTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate __init__.py: %w", err)
	}
	files["__init__.py"] = initContent

	return files, nil
}

// generateTypeScriptSDK generates TypeScript SDK files
func generateTypeScriptSDK(schema *Schema, apiURL string) (map[string]string, error) {
	files := make(map[string]string)

	// Prepare data
	data := prepareSDKData(schema, apiURL, mapFieldTypeToTypeScript)

	// Generate models.ts
	modelsContent, err := renderTemplate("typescript-models", typescriptModelsTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate models.ts: %w", err)
	}
	files["models.ts"] = modelsContent

	// Generate client.ts
	clientContent, err := renderTemplate("typescript-client", typescriptClientTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client.ts: %w", err)
	}
	files["client.ts"] = clientContent

	// Generate index.ts
	indexContent, err := renderTemplate("typescript-index", typescriptIndexTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate index.ts: %w", err)
	}
	files["index.ts"] = indexContent

	return files, nil
}

// generateGoSDK generates Go SDK client (models are already generated)
func generateGoSDK(schema *Schema, apiURL string) (map[string]string, error) {
	// TODO: Generate Go SDK client library
	// The models are already generated by GenerateGo
	// This would generate a client package with helper methods
	return nil, fmt.Errorf("Go SDK client generation not yet implemented")
}

// renderTemplate is a helper function to render templates
func renderTemplate(name, templateContent string, data interface{}) (string, error) {
	tmpl, err := template.New(name).
		Funcs(sdkTemplateFuncs()).
		Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// sdkTemplateFuncs returns custom template functions for SDK generation
func sdkTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"toPascalCase": toPascalCase,
		"toSnakeCase":  toSnakeCase,
		"toCamelCase":  toCamelCase,
		"join":         strings.Join,
	}
}
