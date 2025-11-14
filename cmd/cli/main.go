package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	case "schema":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subcommand := os.Args[2]
		handleSchemaCommand(subcommand)
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

	// Success summary
	fmt.Printf("%s=== Compilation Complete ===%s\n", colorGreen, colorReset)
	fmt.Printf("Generated files:\n")
	fmt.Printf("  • %s\n", sqlPath)
	fmt.Printf("  • %s\n", goPath)
	fmt.Printf("  • %s\n", storagePath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Apply schema: make db-create\n")
	fmt.Printf("  2. Build application: make build\n")
	fmt.Printf("  3. Run API server: make run-api\n\n")
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

func printUsage() {
	fmt.Printf("%sIngestKit CLI Tool%s\n\n", colorBlue, colorReset)
	fmt.Println("Usage:")
	fmt.Println("  ingestkit <command> [arguments]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  schema compile   Generate SQL and Go code from schema/events.yaml")
	fmt.Println("  schema validate  Validate schema/events.yaml syntax")
	fmt.Println("  help            Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ingestkit schema compile")
	fmt.Println("  ingestkit schema validate")
	fmt.Println("")
}
