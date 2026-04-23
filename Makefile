.PHONY: help build build-dashboard deploy dev kill test lint fmt clean deps \
        docker-build docker-up docker-down docker-logs docker-clean

SERVER_BINARY=x-extract-server
CLI_BINARY=x-extract-cli
DOCKER_IMAGE=x-extract:latest
BIN_DIR=$(HOME)/bin

DASH_DIR=web-dashboard
DASH_BUILD=$(DASH_DIR)/build
DASH_SOURCES=$(shell find $(DASH_DIR)/src -type f 2>/dev/null) \
             $(DASH_DIR)/package.json $(DASH_DIR)/bun.lock \
             $(DASH_DIR)/next.config.ts $(DASH_DIR)/tailwind.config.ts

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

$(DASH_BUILD): $(DASH_SOURCES)
	cd $(DASH_DIR) && bun run build
	@touch $(DASH_BUILD)

build-dashboard: $(DASH_BUILD) ## Build Next.js dashboard (incremental)

build: build-dashboard ## Build Go binaries into ./bin (rebuilds dashboard if sources changed)
	go build -o bin/$(SERVER_BINARY) ./cmd/server
	go build -o bin/$(CLI_BINARY) ./cmd/cli

deploy: build ## Build and copy binaries to ~/bin
	cp -f bin/$(SERVER_BINARY) $(BIN_DIR)/
	cp -f bin/$(CLI_BINARY) $(BIN_DIR)/

kill: ## Stop the running server
	@pkill -9 -f $(SERVER_BINARY) || true

install-service: deploy ## Install x-extract-server as a macOS LaunchAgent (auto-starts on login)
	@./scripts/install-service.sh

uninstall-service: ## Remove the macOS LaunchAgent
	@./scripts/install-service.sh --uninstall

dev: deploy kill ## Rebuild, deploy to ~/bin, and restart the server
	@echo "Starting server from $(BIN_DIR)..."
	$(BIN_DIR)/$(SERVER_BINARY)

test: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

lint: ## Run go vet and check formatting
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Code not formatted. Run 'make fmt'" && exit 1)

fmt: ## Format code
	gofmt -w .

clean: ## Clean build artifacts
	rm -rf bin/ dist/
	rm -f coverage.txt coverage.html *.log *.db

deps: ## Download and tidy Go dependencies
	go mod download
	go mod tidy

docker-build: ## Build Docker image (multi-arch, use LOCAL=1 for local platform)
	@if [ "$(LOCAL)" = "1" ]; then \
		docker build -t $(DOCKER_IMAGE):local -f deployments/docker/Dockerfile . ; \
	else \
		docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) -f deployments/docker/Dockerfile --load . ; \
	fi

docker-up: ## Start Docker Compose services (use BUILD=1 to rebuild)
	@cd deployments/docker && [ -f .env ] || cp .env.example .env
	cd deployments/docker && docker-compose up -d $(if $(BUILD),--build,)

docker-down: ## Stop Docker Compose services
	cd deployments/docker && docker-compose down

docker-logs: ## View Docker logs
	cd deployments/docker && docker-compose logs -f

docker-clean: ## Clean Docker resources
	cd deployments/docker && docker-compose down -v
	docker rmi $(DOCKER_IMAGE) $(DOCKER_IMAGE):local 2>/dev/null || true

.DEFAULT_GOAL := help
