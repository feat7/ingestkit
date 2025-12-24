package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/feat7/ingestkit/cmd/cli/assets"
)

// handleServerCommand routes server subcommands
func handleServerCommand(subcommand string) {
	switch subcommand {
	case "start":
		serverStart()
	case "stop":
		serverStop()
	case "restart":
		serverRestart()
	case "logs":
		serverLogs()
	case "status":
		serverStatus()
	default:
		fmt.Printf("%sError: Unknown server subcommand '%s'%s\n", colorRed, subcommand, colorReset)
		printServerUsage()
		os.Exit(1)
	}
}

// printServerUsage prints server command help
func printServerUsage() {
	fmt.Println()
	fmt.Println("Server Commands:")
	fmt.Println("  ingestkit server start     Start IngestKit server (Docker)")
	fmt.Println("  ingestkit server stop      Stop IngestKit server")
	fmt.Println("  ingestkit server restart   Restart IngestKit server")
	fmt.Println("  ingestkit server logs      View server logs")
	fmt.Println("  ingestkit server status    Show server status")
	fmt.Println()
	fmt.Println("Setup:")
	fmt.Println("  ingestkit init --server    Initialize server project")
	fmt.Println()
}

// serverInit initializes a server project
func serverInit() {
	fmt.Printf("%s=== IngestKit Server Init ===%s\n\n", colorBlue, colorReset)

	// Check Docker availability
	fmt.Printf("%s-> Checking Docker...%s\n", colorYellow, colorReset)
	if err := checkDockerAvailable(); err != nil {
		fmt.Printf("%sX %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	if err := checkDockerComposeAvailable(); err != nil {
		fmt.Printf("%sX %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s+ Docker is available%s\n\n", colorGreen, colorReset)

	// Check if already initialized
	if isServerInitialized() {
		fmt.Printf("%s! Server already initialized (.ingestkit/ exists)%s\n", colorYellow, colorReset)
		fmt.Println("  To reinitialize, remove .ingestkit/ directory first")
		os.Exit(1)
	}

	// Create directories
	fmt.Printf("%s-> Creating project structure...%s\n", colorYellow, colorReset)
	dirs := []string{".ingestkit", "schema"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("%sX Failed to create %s: %v%s\n", colorRed, dir, err, colorReset)
			os.Exit(1)
		}
	}
	fmt.Printf("%s+ Created directories%s\n", colorGreen, colorReset)

	// Generate secure password
	password := generateSecurePassword(24)

	// Write docker-compose.yaml
	fmt.Printf("%s-> Creating docker-compose.yaml...%s\n", colorYellow, colorReset)
	composePath := ".ingestkit/docker-compose.yaml"
	if err := os.WriteFile(composePath, []byte(assets.DockerComposeTemplate), 0644); err != nil {
		fmt.Printf("%sX Failed to write docker-compose.yaml: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s+ Created %s%s\n", colorGreen, composePath, colorReset)

	// Write .env file
	fmt.Printf("%s-> Creating .env...%s\n", colorYellow, colorReset)
	envContent := strings.ReplaceAll(assets.EnvTemplate, "{{POSTGRES_PASSWORD}}", password)
	if err := os.WriteFile(".env", []byte(envContent), 0600); err != nil {
		fmt.Printf("%sX Failed to write .env: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s+ Created .env (with secure password)%s\n", colorGreen, colorReset)

	// Write schema template
	fmt.Printf("%s-> Creating schema/events.yaml...%s\n", colorYellow, colorReset)
	schemaPath := "schema/events.yaml"
	if err := os.WriteFile(schemaPath, []byte(assets.SchemaTemplate), 0644); err != nil {
		fmt.Printf("%sX Failed to write schema: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s+ Created %s%s\n", colorGreen, schemaPath, colorReset)

	// Write .gitignore
	gitignore := `.env
.ingestkit/
`
	if err := os.WriteFile(".gitignore", []byte(gitignore), 0644); err != nil {
		// Non-fatal, just warn
		fmt.Printf("%s! Could not create .gitignore%s\n", colorYellow, colorReset)
	}

	fmt.Println()
	fmt.Printf("%s=== Server Initialized ===%s\n\n", colorGreen, colorReset)
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit schema/events.yaml to define your events")
	fmt.Println("  2. Run: ingestkit server start")
	fmt.Println("  3. Send events to: http://localhost:8080/v1/events/{type}")
	fmt.Println()
	fmt.Println("API Key (for testing): dev_key_1234567890")
	fmt.Println()
}

// serverStart starts the IngestKit server
func serverStart() {
	fmt.Printf("%s=== Starting IngestKit Server ===%s\n\n", colorBlue, colorReset)

	// Pre-flight checks
	if err := checkDockerAvailable(); err != nil {
		fmt.Printf("%sX %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	if !isServerInitialized() {
		fmt.Printf("%sX Server not initialized%s\n", colorRed, colorReset)
		fmt.Println("  Run: ingestkit init --server")
		os.Exit(1)
	}

	// Check if .env exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		fmt.Printf("%sX .env file not found%s\n", colorRed, colorReset)
		fmt.Println("  Run: ingestkit init --server")
		os.Exit(1)
	}

	// Start Docker Compose
	fmt.Printf("%s-> Starting containers...%s\n", colorYellow, colorReset)
	if err := dockerCompose("up", "-d"); err != nil {
		fmt.Printf("%sX Failed to start containers%s\n", colorRed, err, colorReset)
		fmt.Println("\nTroubleshooting:")
		fmt.Println("  1. Check Docker is running: docker info")
		fmt.Println("  2. Check port conflicts: lsof -i :5433 :8080 :8081 :19092")
		fmt.Println("  3. View logs: ingestkit server logs")
		os.Exit(1)
	}
	fmt.Printf("%s+ Containers started%s\n\n", colorGreen, colorReset)

	// Wait for health checks
	fmt.Printf("%s-> Waiting for services to be ready...%s\n", colorYellow, colorReset)
	if err := waitForHealth(90 * time.Second); err != nil {
		fmt.Printf("%sX %v%s\n", colorRed, err, colorReset)
		fmt.Println("\nView logs for details:")
		fmt.Println("  ingestkit server logs")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("%s=== IngestKit Server Running ===%s\n\n", colorGreen, colorReset)
	fmt.Println("Services:")
	fmt.Println("  API:        http://localhost:8080")
	fmt.Println("  Consumer:   http://localhost:8081/metrics")
	fmt.Println("  PostgreSQL: localhost:5433")
	fmt.Println("  Redpanda:   localhost:19092")
	fmt.Println()
	fmt.Println("Send events:")
	fmt.Println("  curl -X POST http://localhost:8080/v1/events/user_signup \\")
	fmt.Println("    -H 'Authorization: Bearer dev_key_1234567890' \\")
	fmt.Println("    -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"user_id\": \"123\", \"email\": \"test@example.com\"}'")
	fmt.Println()
}

// serverStop stops the IngestKit server
func serverStop() {
	fmt.Printf("%s=== Stopping IngestKit Server ===%s\n\n", colorBlue, colorReset)

	if !isServerInitialized() {
		fmt.Printf("%sX Server not initialized%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	// Parse flags
	keepData := false
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--keep-data" || os.Args[i] == "-k" {
			keepData = true
		}
	}

	fmt.Printf("%s-> Stopping containers...%s\n", colorYellow, colorReset)

	var err error
	if keepData {
		err = dockerCompose("stop")
	} else {
		err = dockerCompose("down")
	}

	if err != nil {
		fmt.Printf("%sX Failed to stop containers: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s+ Server stopped%s\n", colorGreen, colorReset)
	if keepData {
		fmt.Println("  Data volumes preserved. Use 'docker compose down -v' to remove.")
	}
}

// serverRestart restarts the IngestKit server
func serverRestart() {
	fmt.Printf("%s=== Restarting IngestKit Server ===%s\n\n", colorBlue, colorReset)

	if !isServerInitialized() {
		fmt.Printf("%sX Server not initialized%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s-> Restarting containers...%s\n", colorYellow, colorReset)
	if err := dockerCompose("restart"); err != nil {
		fmt.Printf("%sX Failed to restart: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Wait for health checks
	fmt.Printf("%s-> Waiting for services...%s\n", colorYellow, colorReset)
	if err := waitForHealth(60 * time.Second); err != nil {
		fmt.Printf("%sX %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s+ Server restarted%s\n", colorGreen, colorReset)
}

// serverLogs shows server logs
func serverLogs() {
	if !isServerInitialized() {
		fmt.Printf("%sX Server not initialized%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	// Parse flags
	follow := false
	service := ""
	lines := "100"

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-f", "--follow":
			follow = true
		case "--api":
			service = "api"
		case "--consumer":
			service = "consumer"
		case "-n":
			if i+1 < len(os.Args) {
				lines = os.Args[i+1]
				i++
			}
		}
	}

	args := []string{"logs", "--tail", lines}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	if err := dockerCompose(args...); err != nil {
		fmt.Printf("%sX Failed to get logs: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

// serverStatus shows server status
func serverStatus() {
	fmt.Printf("%s=== IngestKit Server Status ===%s\n\n", colorBlue, colorReset)

	if !isServerInitialized() {
		fmt.Printf("%sX Server not initialized%s\n", colorRed, colorReset)
		fmt.Println("  Run: ingestkit init --server")
		os.Exit(1)
	}

	// Get container status
	status, err := getContainerStatus()
	if err != nil {
		fmt.Printf("%s! Could not get container status%s\n", colorYellow, colorReset)
	} else {
		fmt.Println("Containers:")
		for name, state := range status {
			stateColor := colorRed
			if state == "running" {
				stateColor = colorGreen
			}
			fmt.Printf("  %-12s %s%s%s\n", name+":", stateColor, state, colorReset)
		}
		fmt.Println()
	}

	// Check health endpoints
	fmt.Println("Health:")
	endpoints := map[string]string{
		"API":      "http://localhost:8080/health",
		"Consumer": "http://localhost:8081/health",
	}

	for name, url := range endpoints {
		if checkEndpointHealth(url) {
			fmt.Printf("  %-12s %shealthy%s\n", name+":", colorGreen, colorReset)
		} else {
			fmt.Printf("  %-12s %sunhealthy%s\n", name+":", colorRed, colorReset)
		}
	}
	fmt.Println()
}

// schemaApply applies schema changes
func schemaApply() {
	fmt.Printf("%s=== IngestKit Schema Apply ===%s\n\n", colorBlue, colorReset)

	if !isServerInitialized() {
		fmt.Printf("%sX Server not initialized%s\n", colorRed, colorReset)
		fmt.Println("  Run: ingestkit init --server")
		os.Exit(1)
	}

	// Check schema exists
	schemaPath := "schema/events.yaml"
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		fmt.Printf("%sX Schema file not found: %s%s\n", colorRed, schemaPath, colorReset)
		os.Exit(1)
	}

	// Validate schema
	fmt.Printf("%s-> Validating schema...%s\n", colorYellow, colorReset)
	validateSchema() // This calls the existing validation

	// Restart containers to pick up changes
	// Note: In production, this would rebuild images if schema is embedded
	fmt.Printf("\n%s-> Restarting services...%s\n", colorYellow, colorReset)
	if err := dockerCompose("restart", "api", "consumer"); err != nil {
		fmt.Printf("%sX Failed to restart services: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Wait for health
	if err := waitForHealth(60 * time.Second); err != nil {
		fmt.Printf("%sX %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s=== Schema Applied ===%s\n", colorGreen, colorReset)
}

// Helper functions

func generateSecurePassword(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a default (not great, but better than failing)
		return "ingestkit_secure_password_change_me"
	}
	return hex.EncodeToString(bytes)
}

func checkEndpointHealth(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}

// getAbsPath returns absolute path for a relative path
func getAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
