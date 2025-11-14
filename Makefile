.PHONY: help setup up down restart logs clean test build run dev

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "$(BLUE)IngestKit - Development Commands$(NC)"
	@echo ""
	@echo "$(GREEN)Available commands:$(NC)"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

# =============================================================================
# Environment Setup
# =============================================================================

setup: ## Initial setup - create .env file and initialize project
	@echo "$(BLUE)Setting up IngestKit development environment...$(NC)"
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "$(GREEN)Created .env file from .env.example$(NC)"; \
	else \
		echo "$(YELLOW).env file already exists$(NC)"; \
	fi
	@mkdir -p init-db generated/sql generated/models generated/sdks
	@echo "$(GREEN)Setup complete!$(NC)"

check-env: ## Check if .env file exists
	@if [ ! -f .env ]; then \
		echo "$(RED)Error: .env file not found. Run 'make setup' first.$(NC)"; \
		exit 1; \
	fi

# =============================================================================
# Docker Compose - Infrastructure
# =============================================================================

up: check-env ## Start all services (postgres + redpanda)
	@echo "$(BLUE)Starting IngestKit services...$(NC)"
	docker-compose up -d postgres redpanda redpanda-console
	@echo "$(GREEN)Services started!$(NC)"
	@echo "$(YELLOW)PostgreSQL:$(NC)      localhost:5432"
	@echo "$(YELLOW)Redpanda (Kafka):$(NC) localhost:19092"
	@echo "$(YELLOW)Redpanda Console:$(NC) http://localhost:8090"

up-full: check-env ## Start all services including Redis and pgAdmin
	@echo "$(BLUE)Starting all IngestKit services (full stack)...$(NC)"
	docker-compose --profile full up -d
	@echo "$(GREEN)All services started!$(NC)"
	@echo "$(YELLOW)PostgreSQL:$(NC)      localhost:5432"
	@echo "$(YELLOW)Redpanda (Kafka):$(NC) localhost:19092"
	@echo "$(YELLOW)Redpanda Console:$(NC) http://localhost:8090"
	@echo "$(YELLOW)Redis:$(NC)            localhost:6379"
	@echo "$(YELLOW)pgAdmin:$(NC)          http://localhost:5050"

down: ## Stop all services
	@echo "$(BLUE)Stopping all services...$(NC)"
	docker-compose --profile full down
	@echo "$(GREEN)Services stopped!$(NC)"

restart: down up ## Restart all services

logs: ## Tail logs from all services
	docker-compose logs -f

logs-postgres: ## Tail PostgreSQL logs
	docker-compose logs -f postgres

logs-redpanda: ## Tail Redpanda logs
	docker-compose logs -f redpanda

# =============================================================================
# Database Operations
# =============================================================================

db-connect: ## Connect to PostgreSQL with psql
	@echo "$(BLUE)Connecting to PostgreSQL...$(NC)"
	docker-compose exec postgres psql -U ingestkit -d ingestkit

db-create: ## Create database schema
	@echo "$(BLUE)Creating database schema...$(NC)"
	@if [ -f generated/sql/schema.sql ]; then \
		docker-compose exec -T postgres psql -U ingestkit -d ingestkit < generated/sql/schema.sql; \
		echo "$(GREEN)Schema created!$(NC)"; \
	else \
		echo "$(RED)No schema.sql found. Run 'make generate' first.$(NC)"; \
	fi

db-drop: ## Drop all tables (DANGEROUS)
	@echo "$(RED)WARNING: This will drop all tables!$(NC)"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose exec postgres psql -U ingestkit -d ingestkit -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"; \
		echo "$(GREEN)Database reset!$(NC)"; \
	fi

db-reset: db-drop db-create ## Drop and recreate database

db-migrate: ## Run database migrations
	@echo "$(BLUE)Running migrations...$(NC)"
	@echo "$(YELLOW)Migrations not implemented yet$(NC)"

# =============================================================================
# Redpanda Operations
# =============================================================================

redpanda-topics: ## List Redpanda topics
	docker-compose exec redpanda rpk topic list

redpanda-create-topic: ## Create ingestkit.events topic
	@echo "$(BLUE)Creating topic: ingestkit.events$(NC)"
	docker-compose exec redpanda rpk topic create ingestkit.events -p 3 -r 1
	@echo "$(GREEN)Topic created!$(NC)"

redpanda-consume: ## Consume messages from ingestkit.events topic
	docker-compose exec redpanda rpk topic consume ingestkit.events --format '%v\n'

redpanda-info: ## Show Redpanda cluster info
	docker-compose exec redpanda rpk cluster info

# =============================================================================
# Application Development
# =============================================================================

build: ## Build Go applications
	@echo "$(BLUE)Building IngestKit...$(NC)"
	@if [ -f go.mod ]; then \
		go build -o bin/ingestkit ./cmd/cli; \
		go build -o bin/api ./cmd/api; \
		go build -o bin/consumer ./cmd/consumer; \
		echo "$(GREEN)Build complete!$(NC)"; \
	else \
		echo "$(RED)No go.mod found. Initialize Go project first.$(NC)"; \
	fi

run-api: ## Run API server locally
	@echo "$(BLUE)Starting API server...$(NC)"
	@if [ -f bin/api ]; then \
		./bin/api; \
	else \
		echo "$(RED)Binary not found. Run 'make build' first.$(NC)"; \
	fi

run-consumer: ## Run consumer worker locally
	@echo "$(BLUE)Starting consumer worker...$(NC)"
	@if [ -f bin/consumer ]; then \
		./bin/consumer; \
	else \
		echo "$(RED)Binary not found. Run 'make build' first.$(NC)"; \
	fi

dev: up ## Start infrastructure and run in dev mode
	@echo "$(GREEN)Infrastructure is running. Ready for development!$(NC)"

# =============================================================================
# Code Generation
# =============================================================================

generate: ## Generate code from schema
	@echo "$(BLUE)Generating code from schema...$(NC)"
	@if [ -f bin/ingestkit ]; then \
		./bin/ingestkit schema compile; \
	else \
		@echo "$(YELLOW)Building CLI tool...$(NC)"; \
		go build -o bin/ingestkit ./cmd/cli; \
		./bin/ingestkit schema compile; \
	fi

# =============================================================================
# Testing
# =============================================================================

test: ## Run unit tests
	@echo "$(BLUE)Running tests...$(NC)"
	@if [ -f go.mod ]; then \
		go test ./... -v; \
	else \
		echo "$(YELLOW)No Go project initialized yet$(NC)"; \
	fi

test-integration: up ## Run integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	@echo "$(YELLOW)Integration tests not implemented yet$(NC)"

# =============================================================================
# Load Testing (k6)
# =============================================================================

loadtest: loadtest-baseline ## Run default load test (baseline scenario)

loadtest-quick: ## Quick validation test - 100 RPS for 10s
	@echo "$(BLUE)Running quick load test with k6...$(NC)"
	@if ! command -v k6 > /dev/null; then \
		echo "$(RED)k6 not installed. Install with: brew install k6$(NC)"; \
		exit 1; \
	fi
	k6 run loadtest/quick.js

loadtest-baseline: ## Baseline test - 500 RPS for 30s
	@echo "$(BLUE)Running baseline load test with k6...$(NC)"
	@if ! command -v k6 > /dev/null; then \
		echo "$(RED)k6 not installed. Install with: brew install k6$(NC)"; \
		exit 1; \
	fi
	k6 run loadtest/baseline.js

loadtest-production: ## Production simulation - 1000 RPS for 1 min
	@echo "$(BLUE)Running production load test with k6...$(NC)"
	@if ! command -v k6 > /dev/null; then \
		echo "$(RED)k6 not installed. Install with: brew install k6$(NC)"; \
		exit 1; \
	fi
	k6 run loadtest/production.js

loadtest-batch: ## Batch test - 100 RPS with 10 events/batch
	@echo "$(BLUE)Running batch load test with k6...$(NC)"
	@if ! command -v k6 > /dev/null; then \
		echo "$(RED)k6 not installed. Install with: brew install k6$(NC)"; \
		exit 1; \
	fi
	k6 run loadtest/batch.js

loadtest-high: ## High throughput - 5000 RPS for 30s (requires RATE_LIMIT_RPS=10000)
	@echo "$(BLUE)Running high throughput load test with k6...$(NC)"
	@echo "$(YELLOW)Note: Ensure RATE_LIMIT_RPS is set to 10000 or higher$(NC)"
	@if ! command -v k6 > /dev/null; then \
		echo "$(RED)k6 not installed. Install with: brew install k6$(NC)"; \
		exit 1; \
	fi
	k6 run loadtest/high-throughput.js

loadtest-custom: ## Custom load test - set API_URL, API_KEY env vars
	@echo "$(BLUE)Running custom load test with k6...$(NC)"
	@SCRIPT=$${SCRIPT:-loadtest/baseline.js}; \
	echo "Using script: $$SCRIPT"; \
	k6 run $$SCRIPT

# =============================================================================
# Utilities
# =============================================================================

metrics: ## Check consumer metrics endpoint
	@echo "$(BLUE)Consumer Metrics:$(NC)"
	@curl -s http://localhost:8081/metrics | grep -E "^(ingestkit_|# )" || echo "$(RED)Metrics endpoint not available$(NC)"

metrics-health: ## Check consumer health endpoint
	@echo "$(BLUE)Consumer Health:$(NC)"
	@curl -s http://localhost:8081/health | jq . || echo "$(RED)Health endpoint not available$(NC)"

metrics-watch: ## Watch consumer metrics in real-time (requires watch command)
	@echo "$(BLUE)Watching consumer metrics (Ctrl+C to stop)...$(NC)"
	@watch -n 2 'curl -s http://localhost:8081/metrics | grep -E "^ingestkit_" | grep -v "# "'

db-stats: ## Show database statistics (table sizes, row counts)
	@echo "$(BLUE)Database Statistics:$(NC)"
	@docker-compose exec -T postgres psql -U ingestkit -d ingestkit -c "\
		SELECT \
			schemaname, \
			tablename, \
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size, \
			n_live_tup AS rows \
		FROM pg_stat_user_tables \
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema') \
		ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;"

db-event-counts: ## Show event counts by type
	@echo "$(BLUE)Event Counts:$(NC)"
	@docker-compose exec -T postgres psql -U ingestkit -d ingestkit -c "\
		SELECT 'user_signup' as event_type, COUNT(*) as count FROM events_user_signup \
		UNION ALL \
		SELECT 'purchase', COUNT(*) FROM events_purchase \
		UNION ALL \
		SELECT 'page_view', COUNT(*) FROM events_page_view \
		ORDER BY count DESC;"

db-dlq-check: ## Check dead letter queue
	@echo "$(BLUE)Dead Letter Queue:$(NC)"
	@docker-compose exec -T postgres psql -U ingestkit -d ingestkit -c "\
		SELECT \
			event_type, \
			COUNT(*) as count, \
			MAX(created_at) as latest_failure \
		FROM ingestkit_meta.dead_letter_queue \
		GROUP BY event_type;"

clean: down ## Clean up containers, volumes, and build artifacts
	@echo "$(BLUE)Cleaning up...$(NC)"
	docker-compose --profile full down -v
	rm -rf bin/ generated/
	@echo "$(GREEN)Cleanup complete!$(NC)"

status: ## Show status of all services
	@echo "$(BLUE)Service Status:$(NC)"
	@docker-compose ps

health: ## Check health of all services
	@echo "$(BLUE)Health Check:$(NC)"
	@echo -n "$(YELLOW)PostgreSQL:$(NC) "
	@docker-compose exec -T postgres pg_isready -U ingestkit && echo "$(GREEN)✓$(NC)" || echo "$(RED)✗$(NC)"
	@echo -n "$(YELLOW)Redpanda:$(NC)   "
	@docker-compose exec -T redpanda rpk cluster health > /dev/null 2>&1 && echo "$(GREEN)✓$(NC)" || echo "$(RED)✗$(NC)"

fmt: ## Format Go code
	@if [ -f go.mod ]; then \
		go fmt ./...; \
		echo "$(GREEN)Code formatted!$(NC)"; \
	fi

lint: ## Lint Go code
	@if [ -f go.mod ]; then \
		if command -v golangci-lint > /dev/null; then \
			golangci-lint run ./...; \
		else \
			echo "$(YELLOW)golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
		fi; \
	fi

# =============================================================================
# Project Initialization
# =============================================================================

init-go: setup ## Initialize Go project
	@echo "$(BLUE)Initializing Go project...$(NC)"
	@if [ ! -f go.mod ]; then \
		go mod init github.com/feat7/ingestkit; \
		go get github.com/gofiber/fiber/v2; \
		go get github.com/twmb/franz-go/pkg/kgo; \
		go get github.com/lib/pq; \
		go get gopkg.in/yaml.v3; \
		mkdir -p cmd/api cmd/consumer cmd/cli internal pkg; \
		echo "$(GREEN)Go project initialized!$(NC)"; \
	else \
		echo "$(YELLOW)go.mod already exists$(NC)"; \
	fi

# =============================================================================
# Quick Start
# =============================================================================

quickstart: setup up redpanda-create-topic ## Complete setup and start infrastructure
	@echo ""
	@echo "$(GREEN)═══════════════════════════════════════════════════$(NC)"
	@echo "$(GREEN)  IngestKit is ready for development!$(NC)"
	@echo "$(GREEN)═══════════════════════════════════════════════════$(NC)"
	@echo ""
	@echo "$(YELLOW)Services running:$(NC)"
	@echo "  • PostgreSQL:      localhost:5432"
	@echo "  • Redpanda:        localhost:19092"
	@echo "  • Redpanda Console: http://localhost:8090"
	@echo ""
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Run 'make init-go' to initialize Go project"
	@echo "  2. Define schemas in schema/events.yaml"
	@echo "  3. Run 'make generate' to generate code"
	@echo "  4. Run 'make build' to build applications"
	@echo "  5. Run 'make run-api' to start the API server"
	@echo ""
	@echo "$(YELLOW)Useful commands:$(NC)"
	@echo "  • make logs          - View all logs"
	@echo "  • make db-connect    - Connect to PostgreSQL"
	@echo "  • make health        - Check service health"
	@echo "  • make help          - Show all commands"
	@echo ""
