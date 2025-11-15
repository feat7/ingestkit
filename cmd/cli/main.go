package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/feat7/ingestkit/internal/config"
	"github.com/feat7/ingestkit/internal/schema"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorRed    = "\033[31m"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		initProject()
	case "generate":
		generateClient()
	case "schema":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subcommand := os.Args[2]
		handleSchemaCommand(subcommand)
	case "sdk":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subcommand := os.Args[2]
		handleSDKCommand(subcommand)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("%sError: Unknown command '%s'%s\n", colorRed, command, colorReset)
		printUsage()
		os.Exit(1)
	}
}

func handleSchemaCommand(subcommand string) {
	switch subcommand {
	case "compile":
		compileSchema()
	case "validate":
		validateSchema()
	case "push":
		pushSchema()
	default:
		fmt.Printf("%sError: Unknown schema subcommand '%s'%s\n", colorRed, subcommand, colorReset)
		printUsage()
		os.Exit(1)
	}
}

func compileSchema() {
	fmt.Printf("%s=== IngestKit Schema Compiler ===%s\n\n", colorBlue, colorReset)

	// Parse schema
	fmt.Printf("%s→ Parsing schema/events.yaml...%s\n", colorYellow, colorReset)
	schemaPath := "schema/events.yaml"
	parsedSchema, err := schema.ParseSchemaFile(schemaPath)
	if err != nil {
		fmt.Printf("%s✗ Failed to parse schema: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Schema parsed successfully (version: %s)%s\n", colorGreen, parsedSchema.Version, colorReset)
	fmt.Printf("  Found %d event types: %v\n\n", len(parsedSchema.Events), parsedSchema.GetEventNames())

	// Generate SQL DDL
	fmt.Printf("%s→ Generating SQL DDL...%s\n", colorYellow, colorReset)
	sqlDDL, err := schema.GenerateSQL(parsedSchema)
	if err != nil {
		fmt.Printf("%s✗ Failed to generate SQL: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Write SQL to file
	sqlPath := "generated/sql/schema.sql"
	if err := os.MkdirAll(filepath.Dir(sqlPath), 0755); err != nil {
		fmt.Printf("%s✗ Failed to create SQL directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	if err := os.WriteFile(sqlPath, []byte(sqlDDL), 0644); err != nil {
		fmt.Printf("%s✗ Failed to write SQL file: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ SQL DDL generated: %s%s\n\n", colorGreen, sqlPath, colorReset)

	// Generate Go structs
	fmt.Printf("%s→ Generating Go models...%s\n", colorYellow, colorReset)
	goCode, err := schema.GenerateGo(parsedSchema)
	if err != nil {
		fmt.Printf("%s✗ Failed to generate Go code: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Write Go code to file
	goPath := "generated/models/events.go"
	if err := os.MkdirAll(filepath.Dir(goPath), 0755); err != nil {
		fmt.Printf("%s✗ Failed to create models directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	if err := os.WriteFile(goPath, []byte(goCode), 0644); err != nil {
		fmt.Printf("%s✗ Failed to write Go file: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Go models generated: %s%s\n\n", colorGreen, goPath, colorReset)

	// Generate storage writer
	fmt.Printf("%s→ Generating storage writer...%s\n", colorYellow, colorReset)
	storageCode, err := schema.GenerateStorage(parsedSchema)
	if err != nil {
		fmt.Printf("%s✗ Failed to generate storage code: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Write storage code to file
	storagePath := "generated/storage/writer.go"
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		fmt.Printf("%s✗ Failed to create storage directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	if err := os.WriteFile(storagePath, []byte(storageCode), 0644); err != nil {
		fmt.Printf("%s✗ Failed to write storage file: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Storage writer generated: %s%s\n\n", colorGreen, storagePath, colorReset)

	// Generate consumer handler
	fmt.Printf("%s→ Generating consumer handler...%s\n", colorYellow, colorReset)
	consumerCode, err := schema.GenerateConsumer(parsedSchema)
	if err != nil {
		fmt.Printf("%s✗ Failed to generate consumer code: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Write consumer code to file
	consumerPath := "generated/consumer/handler.go"
	if err := os.MkdirAll(filepath.Dir(consumerPath), 0755); err != nil {
		fmt.Printf("%s✗ Failed to create consumer directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	if err := os.WriteFile(consumerPath, []byte(consumerCode), 0644); err != nil {
		fmt.Printf("%s✗ Failed to write consumer file: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Consumer handler generated: %s%s\n\n", colorGreen, consumerPath, colorReset)

	// Success summary
	fmt.Printf("%s=== Compilation Complete ===%s\n", colorGreen, colorReset)
	fmt.Printf("Generated files:\n")
	fmt.Printf("  • %s\n", sqlPath)
	fmt.Printf("  • %s\n", goPath)
	fmt.Printf("  • %s\n", storagePath)
	fmt.Printf("  • %s\n", consumerPath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Apply schema: make db-create\n")
	fmt.Printf("  2. Build application: make build\n")
	fmt.Printf("  3. Run API server: make run-api\n\n")
}

func initProject() {
	fmt.Printf("%s=== IngestKit Project Initialization ===%s\n\n", colorBlue, colorReset)

	// Parse flags
	var language string
	var tenantID string

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--python":
			language = "python"
		case "--typescript", "--ts":
			language = "typescript"
		case "--go":
			language = "go"
		case "--tenant-id", "-t":
			if i+1 < len(os.Args) {
				tenantID = os.Args[i+1]
				i++
			}
		}
	}

	// Auto-detect language if not specified
	if language == "" {
		language = config.DetectLanguage()
		fmt.Printf("%s→ Auto-detected language: %s%s\n", colorYellow, language, colorReset)
	}

	// Generate tenant ID if not specified
	if tenantID == "" {
		// Use directory name as default tenant ID
		cwd, _ := os.Getwd()
		tenantID = filepath.Base(cwd)
		tenantID = strings.ReplaceAll(tenantID, " ", "-")
		tenantID = strings.ToLower(tenantID)
	}

	// Check if already initialized
	if _, err := os.Stat(config.ConfigFileName); err == nil {
		fmt.Printf("%s✗ Project already initialized (found %s)%s\n", colorRed, config.ConfigFileName, colorReset)
		fmt.Printf("  Use 'ingestkit generate' to regenerate client code\n")
		os.Exit(1)
	}

	// Create ingestkit directory
	fmt.Printf("%s→ Creating %s directory...%s\n", colorYellow, config.SchemaDir, colorReset)
	if err := os.MkdirAll(config.SchemaDir, 0755); err != nil {
		fmt.Printf("%s✗ Failed to create directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Created %s/%s\n", colorGreen, config.SchemaDir, colorReset)

	// Create schema template
	fmt.Printf("%s→ Creating schema template...%s\n", colorYellow, colorReset)
	schemaTemplate := `version: "1.0"

events:
  user_signup:
    description: Fired when a new user signs up
    fields:
      user_id:
        type: string
        required: true
        indexed: true
        description: Unique user identifier
      email:
        type: string
        required: true
        description: User email address
      signup_source:
        type: string
        description: Where the signup originated
        values: [web, mobile, api]

  # Add more events here...
`
	schemaPath := filepath.Join(config.SchemaDir, config.SchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(schemaTemplate), 0644); err != nil {
		fmt.Printf("%s✗ Failed to create schema: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Created %s%s\n", colorGreen, schemaPath, colorReset)

	// Create config file
	fmt.Printf("%s→ Creating config file...%s\n", colorYellow, colorReset)
	cfg := config.DefaultConfig(language, tenantID)
	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("%s✗ Failed to create config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Created %s%s\n", colorGreen, config.ConfigFileName, colorReset)

	// Create .gitignore for ingestkit directory
	gitignorePath := filepath.Join(config.SchemaDir, ".gitignore")
	gitignoreContent := "# Generated files\n*.py\n*.ts\n*.js\n__pycache__/\nnode_modules/\n"
	os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)

	// Success summary
	fmt.Printf("\n%s=== Initialization Complete ===%s\n", colorGreen, colorReset)
	fmt.Printf("Project structure created:\n")
	fmt.Printf("  • %s - Configuration\n", config.ConfigFileName)
	fmt.Printf("  • %s - Event schema definitions\n", schemaPath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit %s to define your events\n", schemaPath)
	fmt.Printf("  2. Run 'ingestkit generate' to create client code\n")
	fmt.Printf("  3. Set INGESTKIT_API_KEY environment variable\n")
	fmt.Printf("  4. Start using: ")
	switch language {
	case "python":
		fmt.Printf("from ingestkit import Client\n")
	case "typescript":
		fmt.Printf("import { Client } from './ingestkit'\n")
	case "go":
		fmt.Printf("import \"your-project/ingestkit\"\n")
	}
	fmt.Println()
}

// fetchSchemaFromURL fetches schema from a remote URL
func fetchSchemaFromURL(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func generateClient() {
	fmt.Printf("%s=== IngestKit Client Generation ===%s\n\n", colorBlue, colorReset)

	// Check for --schema-url flag
	var schemaURL string
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--schema-url" && i+1 < len(os.Args) {
			schemaURL = os.Args[i+1]
			break
		}
	}

	// Load config
	fmt.Printf("%s→ Loading configuration...%s\n", colorYellow, colorReset)
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("%s✗ %v%s\n", colorRed, err, colorReset)
		fmt.Printf("  Run 'ingestkit init' first\n")
		os.Exit(1)
	}
	fmt.Printf("%s✓ Loaded %s%s\n", colorGreen, config.ConfigFileName, colorReset)

	// Parse schema (either from URL or local file)
	var parsedSchema *schema.Schema
	if schemaURL != "" {
		// Fetch schema from URL
		fmt.Printf("%s→ Fetching schema from %s...%s\n", colorYellow, schemaURL, colorReset)
		schemaData, err := fetchSchemaFromURL(schemaURL)
		if err != nil {
			fmt.Printf("%s✗ Failed to fetch schema: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}

		parsedSchema, err = schema.ParseSchema(schemaData)
		if err != nil {
			fmt.Printf("%s✗ Failed to parse schema: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Schema fetched and parsed successfully (version: %s)%s\n", colorGreen, parsedSchema.Version, colorReset)
	} else {
		// Use local schema file
		schemaPath := config.GetSchemaPath()
		fmt.Printf("%s→ Parsing %s...%s\n", colorYellow, schemaPath, colorReset)
		parsedSchema, err = schema.ParseSchemaFile(schemaPath)
		if err != nil {
			fmt.Printf("%s✗ Failed to parse schema: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Schema parsed successfully (version: %s)%s\n", colorGreen, parsedSchema.Version, colorReset)
	}

	fmt.Printf("  Found %d event types: %v\n\n", len(parsedSchema.Events), parsedSchema.GetEventNames())

	// Generate SDK based on config language
	var sdkLang schema.SDKLanguage
	switch cfg.Generator.Language {
	case "python":
		sdkLang = schema.SDKLanguagePython
	case "typescript", "ts":
		sdkLang = schema.SDKLanguageTypeScript
	case "go":
		sdkLang = schema.SDKLanguageGo
	default:
		fmt.Printf("%s✗ Unsupported language: %s%s\n", colorRed, cfg.Generator.Language, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s→ Generating %s client...%s\n", colorYellow, cfg.Generator.Language, colorReset)
	files, err := schema.GenerateSDK(parsedSchema, sdkLang, cfg.APIURL)
	if err != nil {
		fmt.Printf("%s✗ Failed to generate client: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Write files to ingestkit directory
	outputDir := config.SchemaDir
	for filename, content := range files {
		fullPath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Printf("%s✗ Failed to write %s: %v%s\n", colorRed, filename, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Generated: %s%s\n", colorGreen, fullPath, colorReset)
	}

	// Success summary
	fmt.Printf("\n%s=== Client Generation Complete ===%s\n", colorGreen, colorReset)
	fmt.Printf("Location: %s/\n", outputDir)
	fmt.Printf("\nUsage:\n")
	switch cfg.Generator.Language {
	case "python":
		fmt.Printf("  from ingestkit import Client\n")
		fmt.Printf("  client = Client()\n")
		fmt.Printf("  client.user_signup.send(user_id=\"123\", email=\"test@example.com\")\n")
	case "typescript":
		fmt.Printf("  import { Client } from './ingestkit'\n")
		fmt.Printf("  const client = new Client()\n")
		fmt.Printf("  await client.userSignup.send({ userId: \"123\", email: \"test@example.com\" })\n")
	case "go":
		fmt.Printf("  import \"your-project/ingestkit\"\n")
		fmt.Printf("  client := ingestkit.NewClient()\n")
		fmt.Printf("  client.UserSignup.Send(...)\n")
	}
	fmt.Println()
}

func validateSchema() {
	fmt.Printf("%s=== IngestKit Schema Validator ===%s\n\n", colorBlue, colorReset)

	schemaPath := "schema/events.yaml"
	fmt.Printf("%s→ Validating %s...%s\n", colorYellow, schemaPath, colorReset)

	_, err := schema.ParseSchemaFile(schemaPath)
	if err != nil {
		fmt.Printf("%s✗ Schema validation failed: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s✓ Schema is valid!%s\n", colorGreen, colorReset)
}

func pushSchema() {
	fmt.Printf("%s=== IngestKit Schema Push ===%s\n\n", colorBlue, colorReset)

	// Parse flags
	var apiURL string
	var apiKey string
	var schemaPath string

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--api-url", "-u":
			if i+1 < len(os.Args) {
				apiURL = os.Args[i+1]
				i++
			}
		case "--api-key", "-k":
			if i+1 < len(os.Args) {
				apiKey = os.Args[i+1]
				i++
			}
		case "--schema", "-s":
			if i+1 < len(os.Args) {
				schemaPath = os.Args[i+1]
				i++
			}
		}
	}

	// Default values
	if apiURL == "" {
		apiURL = os.Getenv("INGESTKIT_API_URL")
		if apiURL == "" {
			apiURL = "http://localhost:8080"
		}
	}

	if apiKey == "" {
		apiKey = os.Getenv("INGESTKIT_API_KEY")
		if apiKey == "" {
			fmt.Printf("%sError: API key is required%s\n", colorRed, colorReset)
			fmt.Println("Provide via --api-key flag or INGESTKIT_API_KEY environment variable")
			os.Exit(1)
		}
	}

	if schemaPath == "" {
		schemaPath = "schema/events.yaml"
	}

	// Read schema file
	fmt.Printf("%s→ Reading %s...%s\n", colorYellow, schemaPath, colorReset)
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		fmt.Printf("%s✗ Failed to read schema: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Schema loaded (%d bytes)%s\n", colorGreen, len(schemaData), colorReset)

	// Validate schema locally first
	fmt.Printf("%s→ Validating schema...%s\n", colorYellow, colorReset)
	parsedSchema, err := schema.ParseSchema(schemaData)
	if err != nil {
		fmt.Printf("%s✗ Schema validation failed: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Schema is valid (version: %s, events: %d)%s\n",
		colorGreen, parsedSchema.Version, len(parsedSchema.Events), colorReset)

	// Push to server
	url := fmt.Sprintf("%s/v1/schema/push", apiURL)
	fmt.Printf("\n%s→ Pushing schema to %s...%s\n", colorYellow, url, colorReset)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(schemaData)))
	if err != nil {
		fmt.Printf("%s✗ Failed to create request: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/x-yaml")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s✗ Failed to push schema: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s✗ Failed to read response: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s✗ Server returned error (HTTP %d):%s\n", colorRed, resp.StatusCode, colorReset)
		fmt.Printf("  %s\n", string(body))
		os.Exit(1)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("%s✗ Failed to parse response: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s✓ Schema pushed successfully!%s\n\n", colorGreen, colorReset)

	if message, ok := result["message"].(string); ok {
		fmt.Printf("  Message: %s\n", message)
	}

	// Display warning prominently if present
	if warning, ok := result["warning"].(string); ok {
		fmt.Printf("\n%s%s%s\n", colorYellow, warning, colorReset)
	}

	if eventTypes, ok := result["event_types"].([]interface{}); ok {
		fmt.Printf("  Event types: %d\n", len(eventTypes))
	}

	if backup, ok := result["backup"].(string); ok {
		fmt.Printf("  Backup created: %s\n", backup)
	}

	fmt.Println()
}

func handleSDKCommand(subcommand string) {
	switch subcommand {
	case "generate":
		generateSDK()
	default:
		fmt.Printf("%sError: Unknown SDK subcommand '%s'%s\n", colorRed, subcommand, colorReset)
		printUsage()
		os.Exit(1)
	}
}

func generateSDK() {
	// Parse flags
	var language string
	var apiURL string

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--lang", "-l":
			if i+1 < len(os.Args) {
				language = os.Args[i+1]
				i++
			}
		case "--api-url", "-u":
			if i+1 < len(os.Args) {
				apiURL = os.Args[i+1]
				i++
			}
		}
	}

	if language == "" {
		fmt.Printf("%sError: --lang flag is required%s\n", colorRed, colorReset)
		fmt.Println("Supported languages: python, typescript")
		os.Exit(1)
	}

	if apiURL == "" {
		apiURL = "http://localhost:8080"
		fmt.Printf("%sWarning: --api-url not specified, using default: %s%s\n", colorYellow, apiURL, colorReset)
	}

	fmt.Printf("%s=== IngestKit SDK Generator ===%s\n\n", colorBlue, colorReset)

	// Parse schema
	fmt.Printf("%s→ Parsing schema/events.yaml...%s\n", colorYellow, colorReset)
	schemaPath := "schema/events.yaml"
	parsedSchema, err := schema.ParseSchemaFile(schemaPath)
	if err != nil {
		fmt.Printf("%s✗ Failed to parse schema: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Schema parsed successfully (version: %s)%s\n", colorGreen, parsedSchema.Version, colorReset)
	fmt.Printf("  Found %d event types: %v\n\n", len(parsedSchema.Events), parsedSchema.GetEventNames())

	// Generate SDK
	fmt.Printf("%s→ Generating %s SDK...%s\n", colorYellow, language, colorReset)
	var sdkLang schema.SDKLanguage
	switch language {
	case "python":
		sdkLang = schema.SDKLanguagePython
	case "typescript", "ts":
		sdkLang = schema.SDKLanguageTypeScript
	case "go":
		sdkLang = schema.SDKLanguageGo
	case "java":
		sdkLang = schema.SDKLanguageJava
	default:
		fmt.Printf("%s✗ Unsupported language: %s%s\n", colorRed, language, colorReset)
		fmt.Println("Supported languages: python, typescript")
		os.Exit(1)
	}

	files, err := schema.GenerateSDK(parsedSchema, sdkLang, apiURL)
	if err != nil {
		fmt.Printf("%s✗ Failed to generate SDK: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Write SDK files
	sdkPath := fmt.Sprintf("generated/sdk/%s", language)
	if err := os.MkdirAll(sdkPath, 0755); err != nil {
		fmt.Printf("%s✗ Failed to create SDK directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	for filename, content := range files {
		fullPath := filepath.Join(sdkPath, filename)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Printf("%s✗ Failed to write %s: %v%s\n", colorRed, filename, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Generated: %s%s\n", colorGreen, fullPath, colorReset)
	}

	// Success summary
	fmt.Printf("\n%s=== SDK Generation Complete ===%s\n", colorGreen, colorReset)
	fmt.Printf("SDK Location: %s\n", sdkPath)
	fmt.Printf("API URL: %s\n", apiURL)
	fmt.Printf("\nNext steps:\n")
	switch language {
	case "python":
		fmt.Printf("  1. Install dependencies: pip install requests pydantic\n")
		fmt.Printf("  2. Import SDK: from %s import IngestKitClient\n", language)
		fmt.Printf("  3. Create client: client = IngestKitClient('%s', 'your_api_key')\n", apiURL)
	case "typescript":
		fmt.Printf("  1. Install in your project: cp -r %s/* src/\n", sdkPath)
		fmt.Printf("  2. Import SDK: import { IngestKitClient } from './sdk'\n")
		fmt.Printf("  3. Create client: const client = new IngestKitClient({ apiUrl: '%s', apiKey: 'your_api_key' })\n", apiURL)
	}
	fmt.Println("")
}

func printUsage() {
	fmt.Printf("%sIngestKit CLI Tool%s\n\n", colorBlue, colorReset)
	fmt.Println("Usage:")
	fmt.Println("  ingestkit <command> [arguments]")
	fmt.Println("")
	fmt.Println("🚀 Quick Start Commands:")
	fmt.Println("  init [--python|--typescript|--go]")
	fmt.Println("                                Initialize IngestKit in your project")
	fmt.Println("  generate                      Generate type-safe client from schema")
	fmt.Println("")
	fmt.Println("📦 Advanced Commands:")
	fmt.Println("  schema compile                Generate SQL and Go code (server-side)")
	fmt.Println("  schema validate               Validate schema/events.yaml syntax")
	fmt.Println("  schema push [--api-url <url>] [--api-key <key>]")
	fmt.Println("                                Push local schema to IngestKit server")
	fmt.Println("  sdk generate --lang <language> [--api-url <url>]")
	fmt.Println("                                Generate SDK (legacy command)")
	fmt.Println("  help                          Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  # Initialize a new project (auto-detects language)")
	fmt.Println("  ingestkit init")
	fmt.Println("")
	fmt.Println("  # Initialize with specific language")
	fmt.Println("  ingestkit init --python")
	fmt.Println("  ingestkit init --typescript")
	fmt.Println("")
	fmt.Println("  # Generate client after editing schema")
	fmt.Println("  ingestkit generate")
	fmt.Println("")
	fmt.Println("Project Structure After Init:")
	fmt.Println("  my-project/")
	fmt.Println("    ├── ingestkit/")
	fmt.Println("    │   ├── schema.yaml          # Define your events here")
	fmt.Println("    │   ├── client.py|ts         # Generated (gitignored)")
	fmt.Println("    │   └── models.py|ts         # Generated (gitignored)")
	fmt.Println("    └── ingestkit.config.json    # Configuration")
	fmt.Println("")
}
