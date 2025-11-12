package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "github.com/lib/pq"
)

const (
	dbHost     = "localhost"
	dbPort     = 5433
	dbUser     = "ingestkit"
	dbPassword = "ingestkit_dev"
	dbName     = "ingestkit"

	// Benchmark parameters
	numEvents = 100000
	batchSize = 1000
	tenantID  = "benchmark_tenant"
)

var (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

func main() {
	fmt.Printf("%s╔═══════════════════════════════════════════════════╗%s\n", colorBlue, colorReset)
	fmt.Printf("%s║   IngestKit Performance Benchmark                ║%s\n", colorBlue, colorReset)
	fmt.Printf("%s║   Normalized Schema vs JSONB                     ║%s\n", colorBlue, colorReset)
	fmt.Printf("%s╚═══════════════════════════════════════════════════╝%s\n\n", colorBlue, colorReset)

	// Connect to database
	db, err := connectDB()
	if err != nil {
		fmt.Printf("%s✗ Failed to connect to database: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Printf("%s✓ Connected to PostgreSQL%s\n\n", colorGreen, colorReset)

	// Setup schemas
	fmt.Printf("%s→ Setting up schemas...%s\n", colorYellow, colorReset)
	if err := setupSchemas(db); err != nil {
		fmt.Printf("%s✗ Failed to setup schemas: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Schemas ready%s\n\n", colorGreen, colorReset)

	// Clean existing data
	fmt.Printf("%s→ Cleaning existing data...%s\n", colorYellow, colorReset)
	if err := cleanData(db); err != nil {
		fmt.Printf("%s✗ Failed to clean data: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Data cleaned%s\n\n", colorGreen, colorReset)

	// Run benchmarks
	fmt.Printf("%s════════════════════════════════════════════════════%s\n", colorCyan, colorReset)
	fmt.Printf("%s  BENCHMARK 1: INSERT PERFORMANCE%s\n", colorCyan, colorReset)
	fmt.Printf("%s════════════════════════════════════════════════════%s\n\n", colorCyan, colorReset)

	normalizedInsertTime := benchmarkNormalizedInserts(db)
	jsonbInsertTime := benchmarkJSONBInserts(db)

	fmt.Printf("\n%s════════════════════════════════════════════════════%s\n", colorCyan, colorReset)
	fmt.Printf("%s  BENCHMARK 2: QUERY PERFORMANCE%s\n", colorCyan, colorReset)
	fmt.Printf("%s════════════════════════════════════════════════════%s\n\n", colorCyan, colorReset)

	runQueryBenchmarks(db)

	// Summary
	fmt.Printf("\n%s════════════════════════════════════════════════════%s\n", colorGreen, colorReset)
	fmt.Printf("%s  SUMMARY%s\n", colorGreen, colorReset)
	fmt.Printf("%s════════════════════════════════════════════════════%s\n\n", colorGreen, colorReset)

	fmt.Printf("Insert Performance:\n")
	fmt.Printf("  Normalized: %s%s%s\n", colorCyan, normalizedInsertTime, colorReset)
	fmt.Printf("  JSONB:      %s%s%s\n", colorCyan, jsonbInsertTime, colorReset)

	speedup := float64(jsonbInsertTime) / float64(normalizedInsertTime)
	if speedup > 1 {
		fmt.Printf("  %s→ Normalized is %.2fx faster%s\n\n", colorGreen, speedup, colorReset)
	} else {
		fmt.Printf("  %s→ JSONB is %.2fx faster%s\n\n", colorYellow, 1/speedup, colorReset)
	}
}

func connectDB() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	return sql.Open("postgres", connStr)
}

func setupSchemas(db *sql.DB) error {
	// Read and execute JSONB schema
	jsonbSchema, err := os.ReadFile("jsonb_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read jsonb_schema.sql: %w", err)
	}
	if _, err := db.Exec(string(jsonbSchema)); err != nil {
		return fmt.Errorf("failed to create JSONB schema: %w", err)
	}

	// Read and execute partition setup
	partitions, err := os.ReadFile("setup_partitions.sql")
	if err != nil {
		return fmt.Errorf("failed to read setup_partitions.sql: %w", err)
	}
	if _, err := db.Exec(string(partitions)); err != nil {
		return fmt.Errorf("failed to create partitions: %w", err)
	}

	return nil
}

func cleanData(db *sql.DB) error {
	tables := []string{
		"events_user_signup",
		"events_purchase",
		"events_page_view",
		"events_jsonb",
	}

	for _, table := range tables {
		if _, err := db.Exec(fmt.Sprintf("TRUNCATE %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
	}

	return nil
}

func benchmarkNormalizedInserts(db *sql.DB) time.Duration {
	fmt.Printf("%s1. Normalized Schema Insert (%d events)%s\n", colorYellow, numEvents, colorReset)

	start := time.Now()

	// Generate and insert user_signup events
	for i := 0; i < numEvents/3; i++ {
		_, err := db.Exec(`
			INSERT INTO events_user_signup (tenant_id, user_id, email, signup_source, utm_campaign)
			VALUES ($1, $2, $3, $4, $5)
		`, tenantID, fmt.Sprintf("user_%d", i), fmt.Sprintf("user%d@example.com", i),
			randomChoice([]string{"web", "mobile", "api"}), randomChoice([]string{"campaign_1", "campaign_2", "campaign_3"}))

		if err != nil {
			fmt.Printf("%s✗ Insert failed: %v%s\n", colorRed, err, colorReset)
			return 0
		}
	}

	// Generate and insert purchase events
	for i := 0; i < numEvents/3; i++ {
		_, err := db.Exec(`
			INSERT INTO events_purchase (tenant_id, user_id, order_id, amount, currency, payment_method)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, tenantID, fmt.Sprintf("user_%d", rand.Intn(numEvents/3)), fmt.Sprintf("order_%d", i),
			rand.Float64()*1000, "USD", randomChoice([]string{"card", "paypal", "stripe"}))

		if err != nil {
			fmt.Printf("%s✗ Insert failed: %v%s\n", colorRed, err, colorReset)
			return 0
		}
	}

	// Generate and insert page_view events
	for i := 0; i < numEvents/3; i++ {
		_, err := db.Exec(`
			INSERT INTO events_page_view (tenant_id, session_id, page_url, duration_ms)
			VALUES ($1, $2, $3, $4)
		`, tenantID, fmt.Sprintf("session_%d", rand.Intn(10000)),
			fmt.Sprintf("https://example.com/page_%d", rand.Intn(100)),
			rand.Intn(60000))

		if err != nil {
			fmt.Printf("%s✗ Insert failed: %v%s\n", colorRed, err, colorReset)
			return 0
		}
	}

	duration := time.Since(start)
	fmt.Printf("   %s✓ Completed in: %s%s\n", colorGreen, duration, colorReset)
	fmt.Printf("   %s→ %s events/sec%s\n", colorCyan, fmt.Sprintf("%.0f", float64(numEvents)/duration.Seconds()), colorReset)

	return duration
}

func benchmarkJSONBInserts(db *sql.DB) time.Duration {
	fmt.Printf("\n%s2. JSONB Schema Insert (%d events)%s\n", colorYellow, numEvents, colorReset)

	start := time.Now()

	// Generate and insert user_signup events as JSONB
	for i := 0; i < numEvents/3; i++ {
		payload := map[string]interface{}{
			"user_id":       fmt.Sprintf("user_%d", i),
			"email":         fmt.Sprintf("user%d@example.com", i),
			"signup_source": randomChoice([]string{"web", "mobile", "api"}),
			"utm_campaign":  randomChoice([]string{"campaign_1", "campaign_2", "campaign_3"}),
		}
		payloadJSON, _ := json.Marshal(payload)

		_, err := db.Exec(`
			INSERT INTO events_jsonb (tenant_id, event_type, payload)
			VALUES ($1, $2, $3)
		`, tenantID, "user_signup", payloadJSON)

		if err != nil {
			fmt.Printf("%s✗ Insert failed: %v%s\n", colorRed, err, colorReset)
			return 0
		}
	}

	// Generate and insert purchase events as JSONB
	for i := 0; i < numEvents/3; i++ {
		payload := map[string]interface{}{
			"user_id":        fmt.Sprintf("user_%d", rand.Intn(numEvents/3)),
			"order_id":       fmt.Sprintf("order_%d", i),
			"amount":         rand.Float64() * 1000,
			"currency":       "USD",
			"payment_method": randomChoice([]string{"card", "paypal", "stripe"}),
		}
		payloadJSON, _ := json.Marshal(payload)

		_, err := db.Exec(`
			INSERT INTO events_jsonb (tenant_id, event_type, payload)
			VALUES ($1, $2, $3)
		`, tenantID, "purchase", payloadJSON)

		if err != nil {
			fmt.Printf("%s✗ Insert failed: %v%s\n", colorRed, err, colorReset)
			return 0
		}
	}

	// Generate and insert page_view events as JSONB
	for i := 0; i < numEvents/3; i++ {
		payload := map[string]interface{}{
			"session_id":  fmt.Sprintf("session_%d", rand.Intn(10000)),
			"page_url":    fmt.Sprintf("https://example.com/page_%d", rand.Intn(100)),
			"duration_ms": rand.Intn(60000),
		}
		payloadJSON, _ := json.Marshal(payload)

		_, err := db.Exec(`
			INSERT INTO events_jsonb (tenant_id, event_type, payload)
			VALUES ($1, $2, $3)
		`, tenantID, "page_view", payloadJSON)

		if err != nil {
			fmt.Printf("%s✗ Insert failed: %v%s\n", colorRed, err, colorReset)
			return 0
		}
	}

	duration := time.Since(start)
	fmt.Printf("   %s✓ Completed in: %s%s\n", colorGreen, duration, colorReset)
	fmt.Printf("   %s→ %s events/sec%s\n", colorCyan, fmt.Sprintf("%.0f", float64(numEvents)/duration.Seconds()), colorReset)

	return duration
}

func runQueryBenchmarks(db *sql.DB) {
	// Query 1: Filter by user_id
	fmt.Printf("%s1. Query: Filter by user_id%s\n", colorYellow, colorReset)

	// Normalized
	start := time.Now()
	var count int
	db.QueryRow("SELECT COUNT(*) FROM events_user_signup WHERE user_id = $1", "user_1000").Scan(&count)
	normalizedTime := time.Since(start)
	fmt.Printf("   Normalized: %s%s%s (%d rows)\n", colorCyan, normalizedTime, colorReset, count)

	// JSONB
	start = time.Now()
	db.QueryRow("SELECT COUNT(*) FROM events_jsonb WHERE event_type = 'user_signup' AND payload->>'user_id' = $1", "user_1000").Scan(&count)
	jsonbTime := time.Since(start)
	fmt.Printf("   JSONB:      %s%s%s (%d rows)\n", colorCyan, jsonbTime, colorReset, count)

	speedup := float64(jsonbTime) / float64(normalizedTime)
	fmt.Printf("   %s→ Normalized is %.2fx faster%s\n\n", colorGreen, speedup, colorReset)

	// Query 2: Time range query
	fmt.Printf("%s2. Query: Events in last hour%s\n", colorYellow, colorReset)
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	// Normalized
	start = time.Now()
	db.QueryRow("SELECT COUNT(*) FROM events_purchase WHERE timestamp > $1", oneHourAgo).Scan(&count)
	normalizedTime = time.Since(start)
	fmt.Printf("   Normalized: %s%s%s (%d rows)\n", colorCyan, normalizedTime, colorReset, count)

	// JSONB
	start = time.Now()
	db.QueryRow("SELECT COUNT(*) FROM events_jsonb WHERE event_type = 'purchase' AND timestamp > $1", oneHourAgo).Scan(&count)
	jsonbTime = time.Since(start)
	fmt.Printf("   JSONB:      %s%s%s (%d rows)\n", colorCyan, jsonbTime, colorReset, count)

	speedup = float64(jsonbTime) / float64(normalizedTime)
	fmt.Printf("   %s→ Normalized is %.2fx faster%s\n\n", colorGreen, speedup, colorReset)

	// Query 3: Aggregation (SUM)
	fmt.Printf("%s3. Query: Total purchase amount%s\n", colorYellow, colorReset)

	// Normalized
	start = time.Now()
	var total float64
	db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM events_purchase").Scan(&total)
	normalizedTime = time.Since(start)
	fmt.Printf("   Normalized: %s%s%s (total: $%.2f)\n", colorCyan, normalizedTime, colorReset, total)

	// JSONB
	start = time.Now()
	db.QueryRow("SELECT COALESCE(SUM((payload->>'amount')::decimal), 0) FROM events_jsonb WHERE event_type = 'purchase'").Scan(&total)
	jsonbTime = time.Since(start)
	fmt.Printf("   JSONB:      %s%s%s (total: $%.2f)\n", colorCyan, jsonbTime, colorReset, total)

	speedup = float64(jsonbTime) / float64(normalizedTime)
	fmt.Printf("   %s→ Normalized is %.2fx faster%s\n\n", colorGreen, speedup, colorReset)
}

func randomChoice(options []string) string {
	return options[rand.Intn(len(options))]
}
