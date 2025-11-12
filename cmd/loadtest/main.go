package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultAPIURL = "http://localhost:8080"
	defaultAPIKey = "dev_key_1234567890"
)

// Test scenario types
type ScenarioType string

const (
	ScenarioConstant ScenarioType = "constant" // Constant RPS
	ScenarioRamp     ScenarioType = "ramp"     // Ramp up to target RPS
	ScenarioSpike    ScenarioType = "spike"    // Sudden spike in traffic
	ScenarioBurst    ScenarioType = "burst"    // Periodic bursts
)

// Config holds load test configuration
type Config struct {
	APIURL      string
	APIKey      string
	Scenario    ScenarioType
	TargetRPS   int
	Duration    time.Duration
	Workers     int
	EventTypes  []string
	BatchSize   int
	WarmupTime  time.Duration
	ReportEvery time.Duration
}

// Stats tracks test statistics
type Stats struct {
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	totalEvents     int64
	latencies       []time.Duration
	latenciesMu     sync.Mutex
	errors          map[string]int64
	errorsMu        sync.Mutex
	startTime       time.Time
}

// EventPayload represents different event types
type UserSignupEvent struct {
	UserId       string                 `json:"user_id"`
	Email        string                 `json:"email"`
	SignupSource string                 `json:"signup_source"`
	UtmCampaign  *string                `json:"utm_campaign,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type PurchaseEvent struct {
	UserId        string  `json:"user_id"`
	OrderId       string  `json:"order_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	PaymentMethod string  `json:"payment_method"`
	Items         []struct {
		ProductId string  `json:"product_id"`
		Quantity  int     `json:"quantity"`
		Price     float64 `json:"price"`
	} `json:"items"`
}

type PageViewEvent struct {
	UserId     string                 `json:"user_id"`
	SessionId  string                 `json:"session_id"`
	PageUrl    string                 `json:"page_url"`
	PageTitle  string                 `json:"page_title"`
	Referrer   *string                `json:"referrer,omitempty"`
	DurationMs *int                   `json:"duration_ms,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type EventRequest struct {
	EventType string      `json:"event_type"`
	Payload   interface{} `json:"payload"`
}

type BatchRequest struct {
	Events []EventRequest `json:"events"`
}

func main() {
	// Parse flags
	apiURL := flag.String("url", defaultAPIURL, "API URL")
	apiKey := flag.String("key", defaultAPIKey, "API Key")
	scenario := flag.String("scenario", "constant", "Test scenario: constant, ramp, spike, burst")
	targetRPS := flag.Int("rps", 1000, "Target requests per second")
	duration := flag.Duration("duration", 30*time.Second, "Test duration")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	batchSize := flag.Int("batch", 1, "Events per batch (1 = single events)")
	warmup := flag.Duration("warmup", 5*time.Second, "Warmup time before starting measurements")
	reportEvery := flag.Duration("report", 5*time.Second, "Report interval")
	flag.Parse()

	config := &Config{
		APIURL:      *apiURL,
		APIKey:      *apiKey,
		Scenario:    ScenarioType(*scenario),
		TargetRPS:   *targetRPS,
		Duration:    *duration,
		Workers:     *workers,
		EventTypes:  []string{"user_signup", "purchase", "page_view"},
		BatchSize:   *batchSize,
		WarmupTime:  *warmup,
		ReportEvery: *reportEvery,
	}

	log.Println("🔥 IngestKit Load Test")
	log.Printf("Scenario:     %s", config.Scenario)
	log.Printf("Target RPS:   %d", config.TargetRPS)
	log.Printf("Duration:     %v", config.Duration)
	log.Printf("Workers:      %d", config.Workers)
	log.Printf("Batch Size:   %d", config.BatchSize)
	log.Printf("Warmup:       %v", config.WarmupTime)
	log.Printf("API URL:      %s", config.APIURL)
	log.Println()

	// Initialize stats
	stats := &Stats{
		latencies: make([]time.Duration, 0, 100000),
		errors:    make(map[string]int64),
		startTime: time.Now(),
	}

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Warmup phase
	if config.WarmupTime > 0 {
		log.Printf("🔥 Warmup phase: %v", config.WarmupTime)
		time.Sleep(config.WarmupTime)
		stats.startTime = time.Now() // Reset start time after warmup
		log.Println("✅ Warmup complete - starting measurements")
	}

	// Start workers
	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	rateLimiter := make(chan time.Time, config.TargetRPS)

	// Rate limiter goroutine
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(config.TargetRPS))
		defer ticker.Stop()
		for {
			select {
			case <-stopChan:
				return
			case t := <-ticker.C:
				select {
				case rateLimiter <- t:
				default:
				}
			}
		}
	}()

	// Start workers
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go worker(i, config, stats, rateLimiter, stopChan, &wg)
	}

	// Progress reporter
	reportTicker := time.NewTicker(config.ReportEvery)
	defer reportTicker.Stop()
	go func() {
		for {
			select {
			case <-stopChan:
				return
			case <-reportTicker.C:
				printProgress(config, stats)
			}
		}
	}()

	// Wait for duration or interrupt
	select {
	case <-ctx.Done():
		log.Println("\n⏹️  Interrupted by user")
	case <-time.After(config.Duration):
		log.Println("\n✅ Test duration completed")
	}

	// Stop workers
	close(stopChan)
	wg.Wait()

	// Final report
	printFinalReport(config, stats)
}

func worker(id int, config *Config, stats *Stats, rateLimiter <-chan time.Time, stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for {
		select {
		case <-stop:
			return
		case <-rateLimiter:
			// Send request
			start := time.Now()
			err := sendRequest(client, config)
			latency := time.Since(start)

			// Record stats
			atomic.AddInt64(&stats.totalRequests, 1)
			if config.BatchSize > 1 {
				atomic.AddInt64(&stats.totalEvents, int64(config.BatchSize))
			} else {
				atomic.AddInt64(&stats.totalEvents, 1)
			}

			if err != nil {
				atomic.AddInt64(&stats.failedRequests, 1)
				stats.errorsMu.Lock()
				stats.errors[err.Error()]++
				stats.errorsMu.Unlock()
			} else {
				atomic.AddInt64(&stats.successRequests, 1)
				stats.latenciesMu.Lock()
				stats.latencies = append(stats.latencies, latency)
				stats.latenciesMu.Unlock()
			}
		}
	}
}

func sendRequest(client *http.Client, config *Config) error {
	var reqBody []byte
	var err error
	var endpoint string

	if config.BatchSize > 1 {
		// Batch request
		batch := BatchRequest{
			Events: make([]EventRequest, config.BatchSize),
		}
		for i := 0; i < config.BatchSize; i++ {
			batch.Events[i] = generateEvent(config.EventTypes)
		}
		reqBody, err = json.Marshal(batch)
		endpoint = "/v1/events/batch"
	} else {
		// Single event
		event := generateEvent(config.EventTypes)
		reqBody, err = json.Marshal(event)
		endpoint = "/v1/events"
	}

	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", config.APIURL+endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	// Read and discard body
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func generateEvent(eventTypes []string) EventRequest {
	eventType := eventTypes[rand.Intn(len(eventTypes))]

	switch eventType {
	case "user_signup":
		return EventRequest{
			EventType: "user_signup",
			Payload: UserSignupEvent{
				UserId:       fmt.Sprintf("user_%d", rand.Intn(100000)),
				Email:        fmt.Sprintf("user%d@example.com", rand.Intn(100000)),
				SignupSource: []string{"web", "mobile", "api"}[rand.Intn(3)],
				Metadata: map[string]interface{}{
					"ip":         fmt.Sprintf("192.168.1.%d", rand.Intn(255)),
					"user_agent": "LoadTest/1.0",
				},
			},
		}

	case "purchase":
		return EventRequest{
			EventType: "purchase",
			Payload: PurchaseEvent{
				UserId:        fmt.Sprintf("user_%d", rand.Intn(100000)),
				OrderId:       fmt.Sprintf("order_%d", rand.Intn(1000000)),
				Amount:        float64(rand.Intn(50000)) / 100.0,
				Currency:      "USD",
				PaymentMethod: []string{"card", "paypal", "bank_transfer"}[rand.Intn(3)],
				Items: []struct {
					ProductId string  `json:"product_id"`
					Quantity  int     `json:"quantity"`
					Price     float64 `json:"price"`
				}{
					{
						ProductId: fmt.Sprintf("prod_%d", rand.Intn(1000)),
						Quantity:  rand.Intn(5) + 1,
						Price:     float64(rand.Intn(10000)) / 100.0,
					},
				},
			},
		}

	case "page_view":
		durationMs := rand.Intn(60000)
		return EventRequest{
			EventType: "page_view",
			Payload: PageViewEvent{
				UserId:     fmt.Sprintf("user_%d", rand.Intn(100000)),
				SessionId:  fmt.Sprintf("session_%d", rand.Intn(10000)),
				PageUrl:    fmt.Sprintf("/page/%d", rand.Intn(100)),
				PageTitle:  fmt.Sprintf("Page %d", rand.Intn(100)),
				DurationMs: &durationMs,
				Metadata: map[string]interface{}{
					"viewport_width":  1920,
					"viewport_height": 1080,
				},
			},
		}

	default:
		return EventRequest{
			EventType: "page_view",
			Payload:   PageViewEvent{},
		}
	}
}

func printProgress(config *Config, stats *Stats) {
	elapsed := time.Since(stats.startTime)
	total := atomic.LoadInt64(&stats.totalRequests)
	success := atomic.LoadInt64(&stats.successRequests)
	totalEvents := atomic.LoadInt64(&stats.totalEvents)

	currentRPS := float64(total) / elapsed.Seconds()
	currentEPS := float64(totalEvents) / elapsed.Seconds()
	successRate := 0.0
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}

	stats.latenciesMu.Lock()
	latencyCount := len(stats.latencies)
	var p50, p95, p99 time.Duration
	if latencyCount > 0 {
		sortedLatencies := make([]time.Duration, latencyCount)
		copy(sortedLatencies, stats.latencies)
		sort.Slice(sortedLatencies, func(i, j int) bool {
			return sortedLatencies[i] < sortedLatencies[j]
		})
		p50 = sortedLatencies[latencyCount*50/100]
		p95 = sortedLatencies[latencyCount*95/100]
		p99 = sortedLatencies[latencyCount*99/100]
	}
	stats.latenciesMu.Unlock()

	log.Printf("📊 [%6.1fs] Requests: %d | Events: %d | RPS: %.0f | EPS: %.0f | Success: %.1f%% | Latency p50/p95/p99: %v/%v/%v",
		elapsed.Seconds(), total, totalEvents, currentRPS, currentEPS, successRate, p50, p95, p99)
}

func printFinalReport(config *Config, stats *Stats) {
	elapsed := time.Since(stats.startTime)
	total := atomic.LoadInt64(&stats.totalRequests)
	success := atomic.LoadInt64(&stats.successRequests)
	failed := atomic.LoadInt64(&stats.failedRequests)
	totalEvents := atomic.LoadInt64(&stats.totalEvents)

	log.Println()
	log.Println("═══════════════════════════════════════════════════════════════")
	log.Println("📊 FINAL RESULTS")
	log.Println("═══════════════════════════════════════════════════════════════")
	log.Printf("Duration:          %v", elapsed)
	log.Printf("Total Requests:    %d", total)
	log.Printf("Total Events:      %d", totalEvents)
	log.Printf("Successful:        %d (%.2f%%)", success, float64(success)/float64(total)*100)
	log.Printf("Failed:            %d (%.2f%%)", failed, float64(failed)/float64(total)*100)
	log.Println()
	log.Printf("Requests/sec:      %.2f", float64(total)/elapsed.Seconds())
	log.Printf("Events/sec:        %.2f", float64(totalEvents)/elapsed.Seconds())
	log.Println()

	// Latency percentiles
	stats.latenciesMu.Lock()
	if len(stats.latencies) > 0 {
		sort.Slice(stats.latencies, func(i, j int) bool {
			return stats.latencies[i] < stats.latencies[j]
		})

		log.Println("Latency Distribution:")
		log.Printf("  min:    %v", stats.latencies[0])
		log.Printf("  p50:    %v", stats.latencies[len(stats.latencies)*50/100])
		log.Printf("  p75:    %v", stats.latencies[len(stats.latencies)*75/100])
		log.Printf("  p90:    %v", stats.latencies[len(stats.latencies)*90/100])
		log.Printf("  p95:    %v", stats.latencies[len(stats.latencies)*95/100])
		log.Printf("  p99:    %v", stats.latencies[len(stats.latencies)*99/100])
		log.Printf("  max:    %v", stats.latencies[len(stats.latencies)-1])

		// Calculate average
		var sum time.Duration
		for _, lat := range stats.latencies {
			sum += lat
		}
		log.Printf("  avg:    %v", sum/time.Duration(len(stats.latencies)))
	}
	stats.latenciesMu.Unlock()

	// Error breakdown
	stats.errorsMu.Lock()
	if len(stats.errors) > 0 {
		log.Println()
		log.Println("Error Breakdown:")
		for errMsg, count := range stats.errors {
			log.Printf("  %s: %d", errMsg, count)
		}
	}
	stats.errorsMu.Unlock()

	log.Println("═══════════════════════════════════════════════════════════════")
}
