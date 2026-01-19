.PHONY: help build test clean run docker-build docker-up docker-down lint coverage

# Variables
APP_NAME=x-extract
SERVER_BINARY=x-extract-server
CLI_BINARY=x-extract-cli
DOCKER_IMAGE=x-extract:latest
GO=go
GOTEST=$(GO) test
GOVET=$(GO) vet
GOFMT=gofmt

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	@echo "Building server..."
	$(GO) build -o bin/$(SERVER_BINARY) ./cmd/server
	@echo "Building CLI..."
	$(GO) build -o bin/$(CLI_BINARY) ./cmd/cli
	@echo "Build complete!"

test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...

test-coverage: test ## Run tests with coverage report
	$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linters
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Checking formatting..."
	@test -z "$$($(GOFMT) -l .)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

fmt: ## Format code
	$(GOFMT) -w .

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.txt coverage.html
	rm -f *.log *.db
	@echo "Clean complete!"

run-server: build ## Run the server
	./bin/$(SERVER_BINARY)

run-cli: build ## Run the CLI
	./bin/$(CLI_BINARY)

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) -f deployments/docker/Dockerfile .

docker-up: ## Start Docker Compose services
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-down: ## Stop Docker Compose services
	docker-compose -f deployments/docker/docker-compose.yml down

docker-logs: ## View Docker logs
	docker-compose -f deployments/docker/docker-compose.yml logs -f

install-tools: ## Install development tools
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

deps: ## Download dependencies
	$(GO) mod download
	$(GO) mod tidy

.DEFAULT_GOAL := help

