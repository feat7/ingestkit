package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// checkDockerAvailable verifies Docker is installed and the daemon is running
func checkDockerAvailable() error {
	// Check if docker command exists
	_, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker not found. Install from: https://docker.com/get-started")
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker daemon not running. Start Docker Desktop or run: sudo systemctl start docker")
	}

	return nil
}

// checkDockerComposeAvailable verifies docker compose is available
func checkDockerComposeAvailable() error {
	// Try docker compose (v2) first
	cmd := exec.Command("docker", "compose", "version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		// Try docker-compose (v1)
		_, err := exec.LookPath("docker-compose")
		if err != nil {
			return fmt.Errorf("Docker Compose not found. Update Docker or install docker-compose")
		}
	}
	return nil
}

// dockerCompose runs a docker compose command with the server compose file
func dockerCompose(args ...string) error {
	composePath := ".ingestkit/docker-compose.yaml"
	fullArgs := append([]string{"compose", "-f", composePath}, args...)

	cmd := exec.Command("docker", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// dockerComposeOutput runs docker compose and returns output
func dockerComposeOutput(args ...string) (string, error) {
	composePath := ".ingestkit/docker-compose.yaml"
	fullArgs := append([]string{"compose", "-f", composePath}, args...)

	cmd := exec.Command("docker", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// waitForHealth polls health endpoints until services are ready
func waitForHealth(timeout time.Duration) error {
	endpoints := map[string]string{
		"API":      "http://localhost:8080/health",
		"Consumer": "http://localhost:8081/health",
	}

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for name, url := range endpoints {
		fmt.Printf("%s  Waiting for %s...%s", colorYellow, name, colorReset)

		for time.Now().Before(deadline) {
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				fmt.Printf("\r%s  %s is ready%s\n", colorGreen, name, colorReset)
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}

		// Check if we timed out
		if time.Now().After(deadline) {
			fmt.Printf("\n")
			return fmt.Errorf("%s failed to become healthy within %v", name, timeout)
		}
	}

	return nil
}

// getContainerStatus returns the status of IngestKit containers
func getContainerStatus() (map[string]string, error) {
	output, err := dockerComposeOutput("ps", "--format", "{{.Service}}:{{.State}}")
	if err != nil {
		return nil, err
	}

	status := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			status[parts[0]] = parts[1]
		}
	}
	return status, nil
}

// isServerInitialized checks if the server has been initialized
func isServerInitialized() bool {
	_, err := os.Stat(".ingestkit/docker-compose.yaml")
	return err == nil
}
