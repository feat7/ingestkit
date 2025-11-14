package main

import (
	"testing"
	"time"
)

// TestConfigStructure verifies the Config struct can be created
func TestConfigStructure(t *testing.T) {
	config := &Config{
		Scenario:   "quick",
		TargetRPS:  100,
		Duration:   10 * time.Second,
		Workers:    2,
		BatchSize:  10,
		WarmupTime: 1 * time.Second,
		APIURL:     "http://localhost:8080",
	}

	if config.Scenario != "quick" {
		t.Errorf("Expected scenario 'quick', got '%s'", config.Scenario)
	}
	if config.TargetRPS != 100 {
		t.Errorf("Expected TargetRPS 100, got %d", config.TargetRPS)
	}
	if config.Workers != 2 {
		t.Errorf("Expected Workers 2, got %d", config.Workers)
	}
}

// TestStatsStructure verifies the Stats struct can be created and updated
func TestStatsStructure(t *testing.T) {
	stats := &Stats{
		latencies: []time.Duration{},
		errors:    make(map[string]int64),
		startTime: time.Now(),
	}

	// Test adding latencies
	stats.latenciesMu.Lock()
	stats.latencies = append(stats.latencies, 100*time.Millisecond)
	stats.latencies = append(stats.latencies, 200*time.Millisecond)
	stats.latenciesMu.Unlock()

	// Test updating counters (using atomic would be better in real code)
	stats.totalRequests = 1
	stats.successRequests = 1

	if len(stats.latencies) != 2 {
		t.Errorf("Expected 2 latencies, got %d", len(stats.latencies))
	}
	if stats.totalRequests != 1 {
		t.Errorf("Expected totalRequests=1, got %d", stats.totalRequests)
	}
	if stats.successRequests != 1 {
		t.Errorf("Expected successRequests=1, got %d", stats.successRequests)
	}
}

// TestEventGeneration verifies event request structures
func TestEventGeneration(t *testing.T) {
	// Test UserSignupEvent structure
	event := UserSignupEvent{
		UserId:       "test_user_123",
		Email:        "test@example.com",
		SignupSource: "web",
		Metadata:     map[string]interface{}{"test": true},
	}

	if event.UserId != "test_user_123" {
		t.Errorf("Expected UserId 'test_user_123', got '%s'", event.UserId)
	}
	if event.Email != "test@example.com" {
		t.Errorf("Expected Email 'test@example.com', got '%s'", event.Email)
	}
}

// TestBatchRequestStructure verifies batch request structure
func TestBatchRequestStructure(t *testing.T) {
	events := []EventRequest{
		{
			EventType: "user_signup",
			Payload: UserSignupEvent{
				UserId:       "user1",
				Email:        "user1@example.com",
				SignupSource: "web",
				Metadata:     map[string]interface{}{},
			},
		},
		{
			EventType: "user_signup",
			Payload: UserSignupEvent{
				UserId:       "user2",
				Email:        "user2@example.com",
				SignupSource: "mobile",
				Metadata:     map[string]interface{}{},
			},
		},
	}

	batch := BatchRequest{Events: events}

	if len(batch.Events) != 2 {
		t.Errorf("Expected 2 events in batch, got %d", len(batch.Events))
	}
}

// TestScenarioTypes verifies scenario type constants
func TestScenarioTypes(t *testing.T) {
	scenarios := []ScenarioType{"quick", "baseline", "production", "stress"}

	for _, scenario := range scenarios {
		config := &Config{Scenario: scenario}
		if config.Scenario != scenario {
			t.Errorf("Expected scenario '%s', got '%s'", scenario, config.Scenario)
		}
	}
}
