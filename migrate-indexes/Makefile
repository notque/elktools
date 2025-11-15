.PHONY: help build test test-unit test-integration docker-up docker-down docker-logs clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the migration tool
	go build -o migrate-indexes main.go

test: docker-up test-unit test-integration ## Run all tests (starts Docker if needed)

test-unit: ## Run unit tests only
	go test -v -run "^TestExtractTenantID$$" -count=1

test-integration: ## Run integration tests (requires OpenSearch running)
	@echo "Waiting for OpenSearch to be ready..."
	@until curl -s http://localhost:9200/_cluster/health > /dev/null 2>&1; do \
		echo "Waiting for OpenSearch..."; \
		sleep 2; \
	done
	@echo "OpenSearch is ready!"
	go test -v -run "^TestIntegration$$" -count=1 -timeout 5m

docker-up: ## Start OpenSearch in Docker
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker is not installed"; exit 1; }
	docker compose up -d
	@echo "Waiting for OpenSearch to start..."
	@sleep 5
	@until curl -s http://localhost:9200/_cluster/health > /dev/null 2>&1; do \
		echo "Waiting for OpenSearch to be ready..."; \
		sleep 3; \
	done
	@echo "OpenSearch is ready at http://localhost:9200"

docker-down: ## Stop and remove OpenSearch container
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker is not installed"; exit 1; }
	docker compose down -v

docker-logs: ## Show OpenSearch logs
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker is not installed"; exit 1; }
	docker compose logs -f opensearch

clean: docker-down ## Clean up test artifacts and Docker containers
	rm -f migrate-indexes
	go clean -testcache

# Quick test cycle: rebuild and test
retest: build docker-up test-integration ## Rebuild and run integration tests
