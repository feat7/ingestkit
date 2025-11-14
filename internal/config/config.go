package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the IngestKit configuration file
type Config struct {
	Version   string            `json:"version"`
	APIURL    string            `json:"apiUrl"`
	APIKey    string            `json:"apiKey"`
	TenantID  string            `json:"tenantId"`
	Generator GeneratorConfig   `json:"generator"`
}

// GeneratorConfig holds SDK generation settings
type GeneratorConfig struct {
	Language string `json:"language"` // python, typescript, go
	Output   string `json:"output"`   // ./ingestkit
}

const (
	ConfigFileName = "ingestkit.config.json"
	SchemaDir      = "ingestkit"
	SchemaFileName = "schema.yaml"
)

// DefaultConfig returns a new config with sensible defaults
func DefaultConfig(language, tenantID string) *Config {
	return &Config{
		Version:  "1.0",
		APIURL:   "http://localhost:8080",
		APIKey:   "${INGESTKIT_API_KEY}",
		TenantID: tenantID,
		Generator: GeneratorConfig{
			Language: language,
			Output:   "./" + SchemaDir,
		},
	}
}

// LoadConfig reads the config file from the current directory
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig writes the config file to the current directory
func SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(ConfigFileName, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// FindConfig searches for config file in current and parent directories
func FindConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("config file not found (run 'ingestkit init' first)")
}

// GetSchemaPath returns the path to the schema file
func GetSchemaPath() string {
	return filepath.Join(SchemaDir, SchemaFileName)
}

// DetectLanguage tries to detect the project language from context
func DetectLanguage() string {
	// Check for package.json (Node.js/TypeScript)
	if _, err := os.Stat("package.json"); err == nil {
		return "typescript"
	}

	// Check for requirements.txt or setup.py (Python)
	if _, err := os.Stat("requirements.txt"); err == nil {
		return "python"
	}
	if _, err := os.Stat("setup.py"); err == nil {
		return "python"
	}

	// Check for go.mod (Go)
	if _, err := os.Stat("go.mod"); err == nil {
		return "go"
	}

	// Default to Python
	return "python"
}
