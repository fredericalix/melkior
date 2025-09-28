.PHONY: help gen build test run clean docker-build docker-run cli deps

BINARY_SERVER := server
BINARY_CLI := nodectl
GO := go
DOCKER_COMPOSE := docker-compose

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

deps: ## Install dependencies
	$(GO) mod download
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	$(GO) install github.com/bufbuild/buf/cmd/buf@latest

gen: ## Generate Go code from proto files
	buf dep update
	buf generate

build: ## Build server and CLI binaries
	$(GO) build -o $(BINARY_SERVER) ./cmd/server
	$(GO) build -o $(BINARY_CLI) ./cmd/nodectl

test: ## Run tests
	$(GO) test -v -cover ./...

run: ## Run server and Redis with docker-compose
	$(DOCKER_COMPOSE) up

run-server: ## Run server locally (requires Redis)
	ADMIN_TOKEN=testtoken $(GO) run ./cmd/server

run-cli: ## Run CLI watch mode
	BACKEND_TOKEN=testtoken $(GO) run ./cmd/nodectl watch

docker-build: ## Build Docker images
	docker build -f Dockerfile.server -t nodestatus-server:latest .
	docker build -f Dockerfile.cli -t nodestatus-cli:latest .

docker-run: ## Run with docker-compose
	ADMIN_TOKEN=supersecrettoken $(DOCKER_COMPOSE) up

docker-stop: ## Stop docker-compose services
	$(DOCKER_COMPOSE) down

clean: ## Clean build artifacts
	rm -f $(BINARY_SERVER) $(BINARY_CLI)
	rm -rf gen/

fmt: ## Format Go code
	$(GO) fmt ./...

lint: ## Run linter
	golangci-lint run ./...

cli: ## Build and install CLI
	$(GO) install ./cmd/nodectl

build-sim: ## Build demo-sim CLI
	$(GO) build -o demo-sim ./cmd/demo-sim

run-sim-seed: ## Seed 300 nodes
	BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=testtoken ./demo-sim seed --total 300

run-sim: ## Run simulation for 6 hours
	BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=testtoken ./demo-sim run --duration 6h --update-qps 15

run-sim-stats: ## Show simulation stats
	BACKEND_ADDR=localhost:50051 ./demo-sim stats

run-sim-cleanup: ## Cleanup simulation nodes
	BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=testtoken ./demo-sim cleanup --force

sim-demo: build-sim ## Complete simulation demo
	@echo "Starting complete simulation demo..."
	@$(MAKE) run-sim-seed
	@sleep 2
	@$(MAKE) run-sim-stats
	@echo "Run 'make run-sim' in another terminal to start simulation"