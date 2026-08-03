.PHONY: help build run test clean up down logs

help:
	@echo "TrustGraph Makefile commands:"
	@echo "  make build        - Build Docker containers"
	@echo "  make up           - Start services (docker-compose up)"
	@echo "  make down         - Stop services (docker-compose down)"
	@echo "  make logs         - View service logs"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean up containers and volumes"
	@echo "  make dev          - Start local development server (requires Go 1.23)"

build:
	docker-compose build

up:
	docker-compose up -d
	@echo "TrustGraph API is running on http://localhost:8080"
	@echo "PostgreSQL is running on localhost:5432"

down:
	docker-compose down

logs:
	docker-compose logs -f

clean:
	docker-compose down -v
	rm -f trustgraph

dev:
	@if ! command -v go &> /dev/null; then \
		echo "Go is not installed. Please install Go 1.23+"; \
		exit 1; \
	fi
	@echo "Starting local TrustGraph development server..."
	@echo "Database: postgres://trustgraph:trustgraph_dev_password@localhost:5432/trustgraph"
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
