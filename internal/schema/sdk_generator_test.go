package schema

import (
	"strings"
	"testing"
)

// Test helper: create a schema with multiple field types for SDK testing
func createSDKTestSchema(t *testing.T) *Schema {
	return &Schema{
		Version: "1.0",
		Events: map[string]*Event{
			"user_signup": {
				Description: "User signed up",
				Fields: map[string]*Field{
					"user_id": {
						Type:        "string",
						Required:    true,
						Description: "User identifier",
					},
					"email": {
						Type:        "string",
						Required:    true,
						Description: "User email address",
					},
					"age": {
						Type:        "integer",
						Required:    false,
						Description: "User age",
					},
					"is_active": {
						Type:        "boolean",
						Required:    false,
						Description: "Whether user is active",
					},
					"metadata": {
						Type:        "jsonb",
						Required:    false,
						Description: "Additional metadata",
					},
					"signup_source": {
						Type:        "string",
						Required:    true,
						Description: "Signup source",
						Values:      []string{"web", "mobile", "api"},
					},
				},
			},
			"article_viewed": {
				Description: "Article was viewed",
				Fields: map[string]*Field{
					"article_id": {
						Type:        "string",
						Required:    true,
						Description: "Article ID",
					},
					"read_time": {
						Type:        "integer",
						Required:    false,
						Description: "Time spent reading in seconds",
					},
				},
			},
		},
	}
}

// TestGenerateSDK_PythonBasicStructure tests Python SDK generation structure
func TestGenerateSDK_PythonBasicStructure(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	// Check all expected files are generated
	expectedFiles := []string{"models.py", "client.py", "__init__.py"}
	for _, expectedFile := range expectedFiles {
		if _, exists := files[expectedFile]; !exists {
			t.Errorf("Expected file %s was not generated", expectedFile)
		}
	}
}

// TestGenerateSDK_PythonModels tests Python model generation
func TestGenerateSDK_PythonModels(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	modelsContent := files["models.py"]
	if modelsContent == "" {
		t.Fatal("models.py is empty")
	}

	t.Logf("Generated Python models:\n%s\n", modelsContent)

	// Check for essential Python model elements
	essentials := []string{
		"from pydantic import BaseModel",
		"from typing import Optional",
		"class UserSignup(BaseModel):",
		"class ArticleViewed(BaseModel):",
		"user_id: str",
		"email: str",
		"age: Optional[int]",
		"is_active: Optional[bool]",
		"metadata: Optional[dict]",
		"Field(...)",  // Required fields use Field(...)
		"#",           // Descriptions as inline comments
	}

	for _, essential := range essentials {
		if !strings.Contains(modelsContent, essential) {
			t.Errorf("Python models missing essential element: %s", essential)
		}
	}

	// Check for enum validation if values are specified
	if !strings.Contains(modelsContent, "signup_source") {
		t.Error("Python models should include signup_source field")
	}
}

// TestGenerateSDK_PythonClient tests Python client generation
func TestGenerateSDK_PythonClient(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	clientContent := files["client.py"]
	if clientContent == "" {
		t.Fatal("client.py is empty")
	}

	t.Logf("Generated Python client:\n%s\n", clientContent)

	// Check for essential Python client elements
	essentials := []string{
		"import requests",
		"import json",
		"import os",
		"import uuid",       // For request ID generation
		"import time",       // For retry backoff
		"import logging",    // For debug mode
		"import queue",      // For non-blocking queue
		"import threading",  // For background worker
		"import atexit",     // For graceful shutdown
		"from concurrent.futures import Future", // Kafka-style Futures
		"class IngestKitClient:",
		"class ClientError",  // Error classification
		"class ServerError",
		"class NetworkError",
		"class QueueFullError",  // Queue full error
		"def __init__",
		"api_url",
		"api_key",
		"tenant_id",   // Tenant ID is passed in event data, not as header
		"max_retries", // Retry configuration
		"background",  // Background mode flag
		"on_success",  // Success callback
		"on_failure",  // Failure callback
		"pending_futures", // Future tracking
		"futures_lock",    // Thread-safe Future access
		"self.session = requests.Session()",
		"def send_user_signup",
		"def send_user_signup_batch",
		"def send_article_viewed",
		"def send_article_viewed_batch",
		"wait: bool = False", // Wait parameter for guaranteed delivery
		"timeout: Optional[float]", // Timeout for wait
		"Union[Future, Dict", // Return type can be Future or Dict
		"Authorization",
		"X-Request-ID",          // Request ID tracking
		"response.ok",           // New error handling
		"health_check",          // Health check method
		"def flush",             // Flush method
		"def close",             // Close method
		"_background_worker",    // Background worker
		"event_queue",           // Event queue
		"atexit.register",       // Auto cleanup
		"GUARANTEED",            // Documentation mentions guaranteed delivery
		"def __enter__",         // Context manager
		"def __exit__",          // Context manager
	}

	for _, essential := range essentials {
		if !strings.Contains(clientContent, essential) {
			t.Errorf("Python client missing essential element: %s", essential)
		}
	}

	// Check for config file loading
	if !strings.Contains(clientContent, "ingestkit.config.json") {
		t.Error("Python client should support config file loading")
	}

	// Check for environment variable substitution
	if !strings.Contains(clientContent, "${") {
		t.Error("Python client should support environment variable substitution")
	}
}

// TestGenerateSDK_PythonInit tests Python __init__.py generation
func TestGenerateSDK_PythonInit(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	initContent := files["__init__.py"]
	if initContent == "" {
		t.Fatal("__init__.py is empty")
	}

	// Check for clean exports
	essentials := []string{
		"from .client import IngestKitClient",
		"from .models import",
		"UserSignup",
		"ArticleViewed",
		"Client = IngestKitClient",
		"__all__",
	}

	for _, essential := range essentials {
		if !strings.Contains(initContent, essential) {
			t.Errorf("Python __init__.py missing essential element: %s", essential)
		}
	}
}

// TestGenerateSDK_TypeScriptBasicStructure tests TypeScript SDK generation structure
func TestGenerateSDK_TypeScriptBasicStructure(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguageTypeScript, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	// Check all expected files are generated
	expectedFiles := []string{"models.ts", "client.ts", "index.ts"}
	for _, expectedFile := range expectedFiles {
		if _, exists := files[expectedFile]; !exists {
			t.Errorf("Expected file %s was not generated", expectedFile)
		}
	}
}

// TestGenerateSDK_TypeScriptModels tests TypeScript model generation
func TestGenerateSDK_TypeScriptModels(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguageTypeScript, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	modelsContent := files["models.ts"]
	if modelsContent == "" {
		t.Fatal("models.ts is empty")
	}

	t.Logf("Generated TypeScript models:\n%s\n", modelsContent)

	// Check for essential TypeScript model elements
	essentials := []string{
		"export interface UserSignup",
		"export interface ArticleViewed",
		"user_id: string",
		"email: string",
		"age?: number",
		"is_active?: boolean",
		"metadata?: Record<string, any>",
		"article_id: string",
		"read_time?: number",
	}

	for _, essential := range essentials {
		if !strings.Contains(modelsContent, essential) {
			t.Errorf("TypeScript models missing essential element: %s", essential)
		}
	}

	// Check for JSDoc comments
	if !strings.Contains(modelsContent, "/**") {
		t.Error("TypeScript models should have JSDoc comments")
	}
}

// TestGenerateSDK_TypeScriptClient tests TypeScript client generation
func TestGenerateSDK_TypeScriptClient(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguageTypeScript, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	clientContent := files["client.ts"]
	if clientContent == "" {
		t.Fatal("client.ts is empty")
	}

	t.Logf("Generated TypeScript client:\n%s\n", clientContent)

	// Check for essential TypeScript client elements
	essentials := []string{
		"export class IngestKitClient",
		"private apiUrl: string",
		"private apiKey: string",
		"private tenantId: string",  // Tenant ID is passed in event data, not as header
		"private onSuccess",         // Success callback
		"private onFailure",         // Failure callback
		"constructor",
		"async send", // Methods are async sendUserSignup, sendArticleViewed, etc.
		"Batch",      // Batch methods
		"Authorization",
		"fetch(",
		"AbortController",                  // New implementation uses AbortController with setTimeout
		"GUARANTEED DELIVERY",              // Documentation mentions guaranteed delivery
		"Promise-based confirmation",       // Documentation mentions promises
		"onSuccess?: (eventData: any",     // Callback type definition
		"onFailure?: (eventData: any",     // Callback type definition
		"this.onSuccess(event, result)",   // Success callback call
		"this.onFailure(event, error",     // Failure callback call
		"try {",                            // Try-catch for callback handling
	}

	for _, essential := range essentials {
		if !strings.Contains(clientContent, essential) {
			t.Errorf("TypeScript client missing essential element: %s", essential)
		}
	}

	// Check for config file loading
	if !strings.Contains(clientContent, "ingestkit.config.json") {
		t.Error("TypeScript client should support config file loading")
	}

	// Check for environment variable substitution
	if !strings.Contains(clientContent, "${") {
		t.Error("TypeScript client should support environment variable substitution")
	}
}

// TestGenerateSDK_TypeScriptIndex tests TypeScript index.ts generation
func TestGenerateSDK_TypeScriptIndex(t *testing.T) {
	schema := createSDKTestSchema(t)
	files, err := GenerateSDK(schema, SDKLanguageTypeScript, "http://localhost:8080")
	if err != nil {
		t.Fatalf("GenerateSDK failed: %v", err)
	}

	indexContent := files["index.ts"]
	if indexContent == "" {
		t.Fatal("index.ts is empty")
	}

	// Check for clean exports
	essentials := []string{
		"export { IngestKitClient",
		"export type {",
		"UserSignup",
		"ArticleViewed",
		"export { IngestKitClient as Client }",
	}

	for _, essential := range essentials {
		if !strings.Contains(indexContent, essential) {
			t.Errorf("TypeScript index.ts missing essential element: %s", essential)
		}
	}
}

// TestGenerateSDK_TypeMapping tests type mapping for different languages
func TestGenerateSDK_TypeMapping(t *testing.T) {
	tests := []struct {
		schemaType     string
		pythonType     string
		typescriptType string
		javaType       string
	}{
		{"string", "str", "string", "String"},
		{"integer", "int", "number", "Long"},
		{"decimal", "Decimal", "number", "BigDecimal"},
		{"boolean", "bool", "boolean", "Boolean"},
		{"jsonb", "dict", "Record<string, any>", "Map<String, Object>"},
		{"timestamp", "datetime", "Date", "Instant"},
	}

	for _, tt := range tests {
		t.Run(tt.schemaType, func(t *testing.T) {
			// Test Python type mapping
			pythonType := mapFieldTypeToPython(tt.schemaType)
			if pythonType != tt.pythonType {
				t.Errorf("Python type mapping failed: expected %s, got %s", tt.pythonType, pythonType)
			}

			// Test TypeScript type mapping
			tsType := mapFieldTypeToTypeScript(tt.schemaType)
			if tsType != tt.typescriptType {
				t.Errorf("TypeScript type mapping failed: expected %s, got %s", tt.typescriptType, tsType)
			}

			// Test Java type mapping
			javaType := mapFieldTypeToJava(tt.schemaType)
			if javaType != tt.javaType {
				t.Errorf("Java type mapping failed: expected %s, got %s", tt.javaType, javaType)
			}
		})
	}
}

// TestGenerateSDK_UnsupportedLanguage tests error handling for unsupported languages
func TestGenerateSDK_UnsupportedLanguage(t *testing.T) {
	schema := createSDKTestSchema(t)
	_, err := GenerateSDK(schema, "ruby", "http://localhost:8080")
	if err == nil {
		t.Error("Expected error for unsupported language, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported SDK language") {
		t.Errorf("Expected 'unsupported SDK language' error, got: %v", err)
	}
}

// TestGenerateSDK_GoNotImplemented tests that Go SDK returns appropriate error
func TestGenerateSDK_GoNotImplemented(t *testing.T) {
	schema := createSDKTestSchema(t)
	_, err := GenerateSDK(schema, SDKLanguageGo, "http://localhost:8080")
	if err == nil {
		t.Error("Expected error for Go SDK (not implemented), got nil")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

// TestGenerateSDK_JavaNotImplemented tests that Java SDK returns appropriate error
func TestGenerateSDK_JavaNotImplemented(t *testing.T) {
	schema := createSDKTestSchema(t)
	_, err := GenerateSDK(schema, SDKLanguageJava, "http://localhost:8080")
	if err == nil {
		t.Error("Expected error for Java SDK (not implemented), got nil")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

// TestGenerateSDK_MultipleEvents tests SDK generation with multiple events
func TestGenerateSDK_MultipleEvents(t *testing.T) {
	schema := createSDKTestSchema(t)

	// Test Python SDK
	pythonFiles, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("Python SDK generation failed: %v", err)
	}

	// Check models contain both events
	modelsContent := pythonFiles["models.py"]
	if !strings.Contains(modelsContent, "class UserSignup") || !strings.Contains(modelsContent, "class ArticleViewed") {
		t.Error("Python models should contain both UserSignup and ArticleViewed classes")
	}

	// Check client has methods for both events
	clientContent := pythonFiles["client.py"]
	if !strings.Contains(clientContent, "send_user_signup") || !strings.Contains(clientContent, "send_article_viewed") {
		t.Error("Python client should have methods for both events")
	}

	// Test TypeScript SDK
	tsFiles, err := GenerateSDK(schema, SDKLanguageTypeScript, "http://localhost:8080")
	if err != nil {
		t.Fatalf("TypeScript SDK generation failed: %v", err)
	}

	// Check models contain both events
	tsModelsContent := tsFiles["models.ts"]
	if !strings.Contains(tsModelsContent, "interface UserSignup") || !strings.Contains(tsModelsContent, "interface ArticleViewed") {
		t.Error("TypeScript models should contain both UserSignup and ArticleViewed interfaces")
	}

	// Check client has methods for both events
	tsClientContent := tsFiles["client.ts"]
	if !strings.Contains(tsClientContent, "sendUserSignup") || !strings.Contains(tsClientContent, "sendArticleViewed") {
		t.Error("TypeScript client should have methods for both events")
	}
}

// TestGenerateSDK_CustomAPIURL tests SDK generation with custom API URL
func TestGenerateSDK_CustomAPIURL(t *testing.T) {
	schema := createSDKTestSchema(t)
	customURL := "https://api.example.com:3000"

	pythonFiles, err := GenerateSDK(schema, SDKLanguagePython, customURL)
	if err != nil {
		t.Fatalf("SDK generation failed: %v", err)
	}

	// Python client should still have the default localhost:8080 since customURL is passed at runtime
	// The templates use a fixed default value, not the apiURL parameter
	clientContent := pythonFiles["client.py"]
	if !strings.Contains(clientContent, "http://localhost:8080") {
		t.Error("Python client should contain default API URL")
	}

	tsFiles, err := GenerateSDK(schema, SDKLanguageTypeScript, customURL)
	if err != nil {
		t.Fatalf("TypeScript SDK generation failed: %v", err)
	}

	// TypeScript client should also have the default localhost:8080
	tsClientContent := tsFiles["client.ts"]
	if !strings.Contains(tsClientContent, "http://localhost:8080") {
		t.Error("TypeScript client should contain default API URL")
	}
}

// TestGenerateSDK_RequiredVsOptionalFields tests handling of required and optional fields
func TestGenerateSDK_RequiredVsOptionalFields(t *testing.T) {
	schema := createSDKTestSchema(t)

	// Test Python SDK
	pythonFiles, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("Python SDK generation failed: %v", err)
	}

	modelsContent := pythonFiles["models.py"]

	// Required fields should not be Optional
	if strings.Contains(modelsContent, "user_id: Optional[str]") {
		t.Error("Required field user_id should not be Optional in Python")
	}
	if strings.Contains(modelsContent, "email: Optional[str]") {
		t.Error("Required field email should not be Optional in Python")
	}

	// Optional fields should be Optional
	if !strings.Contains(modelsContent, "age: Optional[int]") {
		t.Error("Optional field age should be Optional[int] in Python")
	}

	// Test TypeScript SDK
	tsFiles, err := GenerateSDK(schema, SDKLanguageTypeScript, "http://localhost:8080")
	if err != nil {
		t.Fatalf("TypeScript SDK generation failed: %v", err)
	}

	tsModelsContent := tsFiles["models.ts"]

	// Required fields should not have '?'
	lines := strings.Split(tsModelsContent, "\n")
	for _, line := range lines {
		if strings.Contains(line, "user_id") && strings.Contains(line, "?:") {
			t.Error("Required field user_id should not be optional in TypeScript")
		}
		if strings.Contains(line, "email") && strings.Contains(line, "?:") && !strings.Contains(line, "@") {
			t.Error("Required field email should not be optional in TypeScript")
		}
	}

	// Optional fields should have '?'
	if !strings.Contains(tsModelsContent, "age?: number") {
		t.Error("Optional field age should have '?' in TypeScript")
	}
}

// TestGenerateSDK_EnumFields tests handling of enum fields with values
func TestGenerateSDK_EnumFields(t *testing.T) {
	schema := createSDKTestSchema(t)

	// Test Python SDK
	pythonFiles, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("Python SDK generation failed: %v", err)
	}

	modelsContent := pythonFiles["models.py"]

	// Should include signup_source field
	if !strings.Contains(modelsContent, "signup_source") {
		t.Error("Python models should include signup_source field")
	}

	// Check if enum values are documented or validated
	// (Implementation may vary - check for at least the field name and type)
	if !strings.Contains(modelsContent, "str") {
		t.Error("Python models should use str type for enum fields")
	}
}

// TestGenerateSDK_EmptySchema tests handling of schema with no events
func TestGenerateSDK_EmptySchema(t *testing.T) {
	schema := &Schema{
		Version: "1.0",
		Events:  map[string]*Event{},
	}

	// Should still generate valid files, just with no event-specific code
	pythonFiles, err := GenerateSDK(schema, SDKLanguagePython, "http://localhost:8080")
	if err != nil {
		t.Fatalf("Python SDK generation failed for empty schema: %v", err)
	}

	if len(pythonFiles) == 0 {
		t.Error("Should generate files even for empty schema")
	}

	// Client should still be generated
	if _, exists := pythonFiles["client.py"]; !exists {
		t.Error("Should generate client.py even for empty schema")
	}
}

// TestPrepareSDKData tests the SDK data preparation function
func TestPrepareSDKData(t *testing.T) {
	schema := createSDKTestSchema(t)
	data := prepareSDKData(schema, "http://localhost:8080", mapFieldTypeToPython)

	// Check schema version
	if data.SchemaVersion != "1.0" {
		t.Errorf("Expected schema version 1.0, got %s", data.SchemaVersion)
	}

	// Check API URL
	if data.APIURL != "http://localhost:8080" {
		t.Errorf("Expected API URL http://localhost:8080, got %s", data.APIURL)
	}

	// Check events are prepared
	if len(data.Events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(data.Events))
	}

	// Find user_signup event
	var userSignupEvent *sdkModelData
	for i := range data.Events {
		if data.Events[i].EventName == "user_signup" {
			userSignupEvent = &data.Events[i]
			break
		}
	}

	if userSignupEvent == nil {
		t.Fatal("user_signup event not found in prepared data")
	}

	// Check event data
	if userSignupEvent.ClassName != "UserSignup" {
		t.Errorf("Expected class name UserSignup, got %s", userSignupEvent.ClassName)
	}

	if userSignupEvent.SnakeName != "user_signup" {
		t.Errorf("Expected snake name user_signup, got %s", userSignupEvent.SnakeName)
	}

	// Check fields are prepared
	if len(userSignupEvent.Fields) != 6 {
		t.Errorf("Expected 6 fields, got %d", len(userSignupEvent.Fields))
	}

	// Check field with enum values
	var signupSourceField *sdkFieldData
	for i := range userSignupEvent.Fields {
		if userSignupEvent.Fields[i].Name == "signup_source" {
			signupSourceField = &userSignupEvent.Fields[i]
			break
		}
	}

	if signupSourceField == nil {
		t.Fatal("signup_source field not found")
	}

	if len(signupSourceField.Values) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(signupSourceField.Values))
	}

	expectedValues := []string{"web", "mobile", "api"}
	for _, expected := range expectedValues {
		found := false
		for _, value := range signupSourceField.Values {
			if value == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected enum value %s not found", expected)
		}
	}
}
