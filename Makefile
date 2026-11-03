# Supported replacement workflow; legacy helpers are removed after recovery verification.
.PHONY: help up down logs test test-integration
help:
	@echo "make up | down | logs | test | test-integration"
up:
	docker compose up -d --build --wait
down:
	docker compose down
logs:
	docker compose logs --tail=100 -f api consumer
test:
	go test -race ./...
test-integration:
	scripts/test-integration.sh
