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

build-dashboard: ## Build Next.js dashboard
	@echo "Building Next.js dashboard..."
	cd web-dashboard && bun run build
	@echo "Dashboard build complete!"

build: build-dashboard ## Build the application
	@echo "Building server..."
	$(GO) build -o bin/$(SERVER_BINARY) ./cmd/server
	@echo "Building CLI..."
	$(GO) build -o bin/$(CLI_BINARY) ./cmd/cli
	@echo "Build complete!"

deploy: build ## Deploy the application
	@echo "Deploying application..."
	cp -f bin/$(SERVER_BINARY) ~/bin/$(SERVER_BINARY)
	cp -f bin/$(CLI_BINARY) ~/bin/$(CLI_BINARY)
	@echo "Deployment complete!"

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

kill-server: ## Kill the running server
	@echo "Killing server..."
	@pkill -f $(SERVER_BINARY) || echo "No server process found"
	@echo "Server killed!"

restart-server: kill-server build ## Kill and restart the server
	@echo "Starting server..."
	./bin/$(SERVER_BINARY)

docker-build: ## Build Docker image
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE) \
		-f deployments/docker/Dockerfile \
		--load \
		.

docker-build-local: ## Build Docker image for local platform only
	docker build \
		-t $(DOCKER_IMAGE):local \
		-f deployments/docker/Dockerfile \
		.

docker-up: ## Start Docker Compose services
	@cd deployments/docker && \
	if [ ! -f .env ]; then cp .env.example .env; fi && \
	docker-compose up -d

docker-up-build: ## Rebuild and start Docker Compose services
	@cd deployments/docker && \
	if [ ! -f .env ]; then cp .env.example .env; fi && \
	docker-compose up -d --build

docker-down: ## Stop Docker Compose services
	cd deployments/docker && docker-compose down

docker-logs: ## View Docker logs
	cd deployments/docker && docker-compose logs -f

docker-clean: ## Clean Docker resources
	cd deployments/docker && docker-compose down -v
	docker rmi $(DOCKER_IMAGE) $(DOCKER_IMAGE):local 2>/dev/null || true

docker-status: ## Show Docker container status
	cd deployments/docker && docker-compose ps

install-tools: ## Install development tools
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

deps: ## Download dependencies
	$(GO) mod download
	$(GO) mod tidy

.DEFAULT_GOAL := help

