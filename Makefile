.PHONY: help build run test clean up down logs dev deps fmt lint db-init

help:
	@echo "TrustGraph Makefile commands:"
	@echo "  make build        - Build Docker containers"
	@echo "  make up           - Start services (requires ConnectionSphere Postgres running)"
	@echo "  make down         - Stop TrustGraph services"
	@echo "  make logs         - View service logs"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean up TrustGraph containers"
	@echo "  make dev          - Start local dev server (requires ConnectionSphere Postgres)"
	@echo "  make db-init      - Create trustgraph database in ConnectionSphere Postgres"
	@echo "  make deps         - Download and tidy Go dependencies"

build:
	docker compose build

up:
	@docker network inspect connectionsphere_default >/dev/null 2>&1 || \
		(echo "ERROR: ConnectionSphere network not found. Run 'docker compose up -d postgres' in ConnectionSphere first." && exit 1)
	docker compose up -d
	@echo "TrustGraph API running on http://localhost:8081"
	@echo "Using ConnectionSphere Postgres (connectsphere-postgres:5432/trustgraph)"

down:
	docker compose down

logs:
	docker compose logs -f

clean:
	docker compose down
	rm -f trustgraph

db-init:
	@docker exec connectsphere-postgres psql -U connectsphere -tc \
		"SELECT 1 FROM pg_database WHERE datname = 'trustgraph'" | grep -q 1 || \
		docker exec connectsphere-postgres psql -U connectsphere -c "CREATE DATABASE trustgraph"
	@echo "trustgraph database ready on connectsphere-postgres"

dev:
	@if ! command -v go &> /dev/null; then \
		echo "Go is not installed. Please install Go 1.23+"; \
		exit 1; \
	fi
	@echo "Starting TrustGraph dev server..."
	@echo "Database: ConnectionSphere Postgres → trustgraph"
	go run ./cmd/trustgraph-api/main.go

test:
	go test -v ./...

deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...

lint:
	@command -v golangci-lint >/dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...
