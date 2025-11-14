// +build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	apiURL = "http://localhost:8080"
	apiKey = "dev_key_1234567890"
	dbConnStr = "postgres://ingestkit:ingestkit_dev@localhost:5433/ingestkit?sslmode=disable"
)

// TestEndToEnd tests the full pipeline: API → Redpanda → Consumer → Database
func TestEndToEnd(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run")
	}

	// Wait for services to be ready
	if !waitForAPI(t, 30*time.Second) {
		t.Fatal("API did not become ready in time")
	}

	// Connect to database
	db, err := sql.Open("pgx", dbConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Test single event ingestion
	t.Run("SingleEventIngestion", func(t *testing.T) {
		testSingleEventIngestion(t, db)
	})

	// Test batch event ingestion
	t.Run("BatchEventIngestion", func(t *testing.T) {
		testBatchEventIngestion(t, db)
	})

	// Test validation error handling
	t.Run("ValidationErrors", func(t *testing.T) {
		testValidationErrors(t)
	})
}

func testSingleEventIngestion(t *testing.T, db *sql.DB) {
	// Create unique test event
	userID := fmt.Sprintf("test_user_%d", time.Now().UnixNano())
	email := fmt.Sprintf("%s@integration-test.com", userID)

	payload := map[string]interface{}{
		"user_id":        userID,
		"email":         email,
		"signup_source": "web",
		"metadata":      map[string]interface{}{"test": "integration"},
	}

	// Send event to API
	eventID, err := sendEvent(t, "user_signup", payload)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	t.Logf("Sent event with ID: %s", eventID)

	// Wait for event to be processed (consumer batch timeout + DB write)
	time.Sleep(2 * time.Second)

	// Verify event in database
	var count int
	query := `SELECT COUNT(*) FROM events_user_signup WHERE user_id = $1 AND email = $2`
	err = db.QueryRow(query, userID, email).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event in database, got %d", count)
	}

	t.Logf("✓ Event successfully written to database")
}

func testBatchEventIngestion(t *testing.T, db *sql.DB) {
	// Create batch of events
	batchSize := 10
	events := make([]map[string]interface{}, batchSize)

	baseUserID := fmt.Sprintf("batch_user_%d", time.Now().UnixNano())

	for i := 0; i < batchSize; i++ {
		userID := fmt.Sprintf("%s_%d", baseUserID, i)
		events[i] = map[string]interface{}{
			"user_id":        userID,
			"email":         fmt.Sprintf("%s@integration-test.com", userID),
			"signup_source": "api",
			"metadata":      map[string]interface{}{"batch": i},
		}
	}

	// Send batch to API
	eventIDs, err := sendBatch(t, "user_signup", events)
	if err != nil {
		t.Fatalf("Failed to send batch: %v", err)
	}

	if len(eventIDs) != batchSize {
		t.Errorf("Expected %d event IDs, got %d", batchSize, len(eventIDs))
	}

	t.Logf("Sent batch with %d events", len(eventIDs))

	// Wait for batch to be processed
	time.Sleep(3 * time.Second)

	// Verify all events in database
	var count int
	query := `SELECT COUNT(*) FROM events_user_signup WHERE user_id LIKE $1`
	err = db.QueryRow(query, baseUserID+"%").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != batchSize {
		t.Errorf("Expected %d events in database, got %d", batchSize, count)
	}

	t.Logf("✓ Batch of %d events successfully written to database", batchSize)
}

func testValidationErrors(t *testing.T) {
	// Test missing required field
	payload := map[string]interface{}{
		"email": "invalid@test.com",
		// Missing required user_id field
	}

	_, err := sendEvent(t, "user_signup", payload)
	if err == nil {
		t.Error("Expected validation error for missing required field")
	} else {
		t.Logf("✓ Validation error correctly returned: %v", err)
	}

	// Test invalid enum value
	payload = map[string]interface{}{
		"user_id":        "test_invalid_enum",
		"email":         "test@example.com",
		"signup_source": "invalid_source", // Not in enum: web, mobile, api
	}

	_, err = sendEvent(t, "user_signup", payload)
	if err == nil {
		t.Error("Expected validation error for invalid enum value")
	} else {
		t.Logf("✓ Enum validation error correctly returned: %v", err)
	}

	// Test unknown event type
	payload = map[string]interface{}{
		"field": "value",
	}

	_, err = sendEvent(t, "unknown_event_type", payload)
	if err == nil {
		t.Error("Expected error for unknown event type")
	} else {
		t.Logf("✓ Unknown event type error correctly returned: %v", err)
	}
}

func sendEvent(t *testing.T, eventType string, payload map[string]interface{}) (string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/v1/events/%s", apiURL, eventType)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("API returned status %d: %v", resp.StatusCode, errResp)
	}

	var response struct {
		EventID string `json:"event_id"`
		Status  string `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return response.EventID, nil
}

func sendBatch(t *testing.T, eventType string, events []map[string]interface{}) ([]string, error) {
	request := map[string]interface{}{
		"events": events,
	}

	payloadJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch: %w", err)
	}

	url := fmt.Sprintf("%s/v1/events/%s/batch", apiURL, eventType)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API returned status %d: %v", resp.StatusCode, errResp)
	}

	var response struct {
		EventIDs []string `json:"event_ids"`
		Status   string   `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.EventIDs, nil
}

func waitForAPI(t *testing.T, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			resp, err := http.Get(apiURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				t.Log("✓ API is ready")
				return true
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}
