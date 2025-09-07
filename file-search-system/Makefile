# File Search System - Makefile
# Comprehensive build system for Go backend + Electron frontend

# Variables
GO_MODULE = file-search-system
BACKEND_DIR = .
FRONTEND_DIR = web/frontend
BINARY_NAME = file-search-server
BINARY_PATH = ./cmd/server
BUILD_DIR = build
DIST_DIR = dist

# Container runtime - Podman by default, can override with CONTAINER_RUNTIME=docker
CONTAINER_RUNTIME ?= podman
COMPOSE_CMD ?= podman-compose

# Database configuration
DB_NAME = file_search
DB_USER = postgres
DB_PASSWORD = CHANGE_ME
DB_HOST = localhost
DB_PORT = 5432

# Docling service configuration
DOCLING_HOST = localhost
DOCLING_PORT = 8082
DOCLING_SERVICE_DIR = ../docling-service

# Colors for output
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[0;33m
BLUE = \033[0;34m
PURPLE = \033[0;35m
CYAN = \033[0;36m
NC = \033[0m # No Color

# Default target
.DEFAULT_GOAL := help

.PHONY: help status install build clean start stop restart logs test lint format container-up container-down frontend-dev frontend-build electron-dev electron-build package deps-go deps-frontend deps setup dev prod docling-up docling-down docling-status docling-logs run-all stop-all

## Help
help: ## Show this help message
	@echo "$(CYAN)File Search System - Available Commands$(NC)"
	@echo "$(BLUE)===========================================$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)Quick Start:$(NC)"
	@echo "  make setup      - Install all dependencies and setup database + docling"
	@echo "  make run-all    - Start all services (database + docling)"
	@echo "  make dev        - Start development environment"
	@echo "  make status     - Check status of all services"
	@echo "  make stop-all   - Stop all services"

## Status and Information
status: ## Show system status
	@echo "$(CYAN)=== File Search System Status ===$(NC)"
	@echo ""
	@echo "$(BLUE)Go Environment:$(NC)"
	@go version 2>/dev/null || echo "$(RED)Go not installed$(NC)"
	@echo "Module: $(GO_MODULE)"
	@echo "Backend Directory: $(BACKEND_DIR)"
	@echo ""
	@echo "$(BLUE)Node.js Environment:$(NC)"
	@node --version 2>/dev/null || echo "$(RED)Node.js not installed$(NC)"
	@npm --version 2>/dev/null || echo "$(RED)npm not installed$(NC)"
	@echo "Frontend Directory: $(FRONTEND_DIR)"
	@echo ""
	@echo "$(BLUE)Database Status:$(NC)"
	@if command -v psql >/dev/null 2>&1; then \
		if pg_isready -h $(DB_HOST) -p $(DB_PORT) >/dev/null 2>&1; then \
			echo "$(GREEN)PostgreSQL is running$(NC)"; \
		else \
			echo "$(RED)PostgreSQL is not running$(NC)"; \
		fi; \
	else \
		echo "$(RED)PostgreSQL client not installed$(NC)"; \
	fi
	@echo ""
	@echo "$(BLUE)Container Runtime Status:$(NC)"
	@if command -v $(CONTAINER_RUNTIME) >/dev/null 2>&1; then \
		echo "$(GREEN)$(CONTAINER_RUNTIME) is installed$(NC)"; \
		if $(CONTAINER_RUNTIME) ps >/dev/null 2>&1; then \
			echo "$(GREEN)$(CONTAINER_RUNTIME) is running$(NC)"; \
			$(CONTAINER_RUNTIME) ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" --filter "name=file-search" 2>/dev/null || echo "No file-search containers running"; \
		else \
			echo "$(YELLOW)$(CONTAINER_RUNTIME) is installed but not running$(NC)"; \
		fi; \
	else \
		echo "$(RED)$(CONTAINER_RUNTIME) not installed$(NC)"; \
	fi
	@echo ""
	@echo "$(BLUE)Docling Service Status:$(NC)"
	@if curl -s http://$(DOCLING_HOST):$(DOCLING_PORT)/health >/dev/null 2>&1; then \
		echo "$(GREEN)Docling service is running on $(DOCLING_HOST):$(DOCLING_PORT)$(NC)"; \
	else \
		echo "$(RED)Docling service is not running$(NC)"; \
		if [ -d "$(DOCLING_SERVICE_DIR)" ]; then \
			echo "  Service directory: $(DOCLING_SERVICE_DIR)"; \
		else \
			echo "  $(RED)Service directory not found: $(DOCLING_SERVICE_DIR)$(NC)"; \
		fi; \
	fi
	@echo ""
	@echo "$(BLUE)Build Artifacts:$(NC)"
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then echo "$(GREEN)Backend binary exists$(NC)"; else echo "$(RED)Backend binary missing$(NC)"; fi
	@if [ -d "$(FRONTEND_DIR)/build" ]; then echo "$(GREEN)Frontend build exists$(NC)"; else echo "$(RED)Frontend build missing$(NC)"; fi
	@if [ -d "$(FRONTEND_DIR)/dist" ]; then echo "$(GREEN)Electron dist exists$(NC)"; else echo "$(RED)Electron dist missing$(NC)"; fi

## Dependencies
deps: deps-go deps-frontend ## Install all dependencies
	@echo "$(GREEN)All dependencies installed$(NC)"

deps-go: ## Install Go dependencies
	@echo "$(BLUE)Installing Go dependencies...$(NC)"
	@go mod tidy
	@go mod download
	@echo "$(GREEN)Go dependencies installed$(NC)"

deps-frontend: ## Install Node.js dependencies
	@echo "$(BLUE)Installing Node.js dependencies...$(NC)"
	@cd $(FRONTEND_DIR) && npm install
	@echo "$(GREEN)Node.js dependencies installed$(NC)"

## Setup
setup: deps container-up docling-up db-setup ## Complete setup for development
	@echo "$(GREEN)Setup complete! Run 'make dev' to start development environment$(NC)"

db-setup: ## Setup database schema
	@echo "$(BLUE)Setting up database schema...$(NC)"
	@sleep 3  # Wait for database to be ready
	@if [ -f "scripts/setup_db.sql" ]; then \
		if command -v psql >/dev/null 2>&1; then \
			PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f scripts/setup_db.sql; \
		else \
			echo "$(YELLOW)psql not found locally, using $(CONTAINER_RUNTIME) exec$(NC)"; \
			$(CONTAINER_RUNTIME) exec -i file-search-db psql -U $(DB_USER) -d $(DB_NAME) < scripts/setup_db.sql; \
		fi; \
		echo "$(GREEN)Database schema setup complete$(NC)"; \
	else \
		echo "$(RED)Database schema file not found$(NC)"; \
	fi

## Build
build: build-backend build-frontend ## Build both backend and frontend
	@echo "$(GREEN)Build complete$(NC)"

build-backend: ## Build Go backend
	@echo "$(BLUE)Building Go backend...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(BINARY_PATH)
	@echo "$(GREEN)Backend built: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

build-frontend: ## Build React frontend
	@echo "$(BLUE)Building React frontend...$(NC)"
	@cd $(FRONTEND_DIR) && npm run build
	@echo "$(GREEN)Frontend built: $(FRONTEND_DIR)/build$(NC)"

## Development
dev: ## Start development environment
	@echo "$(BLUE)Starting development environment...$(NC)"
	@$(MAKE) container-up
	@$(MAKE) docling-up
	@sleep 2
	@echo "$(YELLOW)Starting backend in background...$(NC)"
	@cd $(BACKEND_DIR) && go run $(BINARY_PATH) &
	@echo $$! > .backend.pid
	@sleep 2
	@echo "$(YELLOW)Starting frontend development server...$(NC)"
	@cd $(FRONTEND_DIR) && npm run electron-dev

dev-backend: ## Start only backend development server
	@echo "$(BLUE)Starting backend development server...$(NC)"
	@cd $(BACKEND_DIR) && go run $(BINARY_PATH)

dev-frontend: ## Start only frontend development server
	@echo "$(BLUE)Starting frontend development server...$(NC)"
	@cd $(FRONTEND_DIR) && npm run electron-dev

## Production
prod: build ## Start production servers
	@echo "$(BLUE)Starting production environment...$(NC)"
	@$(MAKE) container-up
	@$(MAKE) docling-up
	@sleep 2
	@echo "$(YELLOW)Starting backend server...$(NC)"
	@$(BUILD_DIR)/$(BINARY_NAME) &
	@echo $$! > .backend.pid
	@echo "$(GREEN)Production environment started$(NC)"
	@echo "$(YELLOW)Frontend can be started with: make electron$(NC)"

## Process Management
start: prod ## Start production environment

stop: ## Stop all services
	@echo "$(BLUE)Stopping services...$(NC)"
	@if [ -f .backend.pid ]; then \
		kill `cat .backend.pid` 2>/dev/null || true; \
		rm -f .backend.pid; \
		echo "$(GREEN)Backend stopped$(NC)"; \
	fi
	@pkill -f "electron.*file-search" 2>/dev/null || true
	@pkill -f "file-search-server" 2>/dev/null || true
	@$(MAKE) container-down
	@$(MAKE) docling-down
	@echo "$(GREEN)All services stopped$(NC)"

restart: stop start ## Restart all services

## Logs
logs: ## Show backend logs (if running with systemd or docker)
	@if [ -f .backend.pid ] && kill -0 `cat .backend.pid` 2>/dev/null; then \
		echo "$(BLUE)Backend is running (PID: `cat .backend.pid`)$(NC)"; \
	else \
		echo "$(RED)Backend is not running$(NC)"; \
	fi
	@$(CONTAINER_RUNTIME) logs file-search-db 2>/dev/null || echo "$(RED)Database container not running$(NC)"

logs-backend: ## Show backend logs
	@tail -f logs/app.log 2>/dev/null || echo "$(RED)No backend logs found$(NC)"

logs-db: ## Show database logs
	@$(CONTAINER_RUNTIME) logs file-search-db 2>/dev/null || echo "$(RED)Database container not running$(NC)"

## Testing
test: test-backend test-frontend ## Run all tests

test-backend: ## Run Go tests
	@echo "$(BLUE)Running Go tests...$(NC)"
	@go test -v ./...

test-frontend: ## Run React tests
	@echo "$(BLUE)Running React tests...$(NC)"
	@cd $(FRONTEND_DIR) && npm test -- --watchAll=false

test-coverage: ## Run tests with coverage
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

## Code Quality
lint: lint-backend lint-frontend ## Run all linters

lint-backend: ## Run Go linter
	@echo "$(BLUE)Running Go linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not installed, using go vet$(NC)"; \
		go vet ./...; \
	fi

lint-frontend: ## Run React linter
	@echo "$(BLUE)Running React linter...$(NC)"
	@cd $(FRONTEND_DIR) && npm run lint 2>/dev/null || echo "$(YELLOW)Frontend linting not configured$(NC)"

format: format-backend format-frontend ## Format all code

format-backend: ## Format Go code
	@echo "$(BLUE)Formatting Go code...$(NC)"
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

format-frontend: ## Format React code
	@echo "$(BLUE)Formatting React code...$(NC)"
	@cd $(FRONTEND_DIR) && npx prettier --write src/ 2>/dev/null || echo "$(YELLOW)Prettier not configured$(NC)"

## Container Management
container-up: ## Start database container
	@echo "$(BLUE)Starting database container with $(CONTAINER_RUNTIME)...$(NC)"
	@$(COMPOSE_CMD) up -d
	@echo "$(GREEN)Database container started$(NC)"

container-down: ## Stop database container
	@echo "$(BLUE)Stopping database container...$(NC)"
	@$(COMPOSE_CMD) down
	@echo "$(GREEN)Database container stopped$(NC)"

container-logs: ## Show database container logs
	@$(COMPOSE_CMD) logs -f

container-reset: ## Reset database (WARNING: destroys data)
	@echo "$(RED)WARNING: This will destroy all data!$(NC)"
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@$(COMPOSE_CMD) down -v
	@$(COMPOSE_CMD) up -d
	@sleep 3
	@$(MAKE) db-setup

## Docling Service Management
docling-up: ## Start Docling service
	@echo "$(BLUE)Starting Docling service...$(NC)"
	@if [ -d "$(DOCLING_SERVICE_DIR)" ]; then \
		cd $(DOCLING_SERVICE_DIR) && $(COMPOSE_CMD) up -d; \
		echo "$(GREEN)Docling service started$(NC)"; \
		echo "$(YELLOW)Waiting for service to be ready...$(NC)"; \
		for i in 1 2 3 4 5 6; do \
			sleep 5; \
			if curl -s http://$(DOCLING_HOST):$(DOCLING_PORT)/health >/dev/null 2>&1; then \
				echo "$(GREEN)Docling service is ready!$(NC)"; \
				break; \
			fi; \
			echo "$(YELLOW)Still waiting... (attempt $$i/6)$(NC)"; \
		done; \
	else \
		echo "$(RED)Docling service directory not found: $(DOCLING_SERVICE_DIR)$(NC)"; \
		exit 1; \
	fi

docling-down: ## Stop Docling service
	@echo "$(BLUE)Stopping Docling service...$(NC)"
	@if [ -d "$(DOCLING_SERVICE_DIR)" ]; then \
		cd $(DOCLING_SERVICE_DIR) && $(COMPOSE_CMD) down; \
		echo "$(GREEN)Docling service stopped$(NC)"; \
	else \
		echo "$(YELLOW)Docling service directory not found: $(DOCLING_SERVICE_DIR)$(NC)"; \
	fi

docling-restart: docling-down docling-up ## Restart Docling service

docling-status: ## Check Docling service status
	@echo "$(BLUE)Checking Docling service status...$(NC)"
	@if curl -s http://$(DOCLING_HOST):$(DOCLING_PORT)/health >/dev/null 2>&1; then \
		echo "$(GREEN)✓ Docling service is running$(NC)"; \
		curl -s http://$(DOCLING_HOST):$(DOCLING_PORT)/ | python3 -m json.tool 2>/dev/null || echo "Service info unavailable"; \
	else \
		echo "$(RED)✗ Docling service is not running$(NC)"; \
	fi
	@if [ -d "$(DOCLING_SERVICE_DIR)" ]; then \
		echo "$(BLUE)Container status:$(NC)"; \
		cd $(DOCLING_SERVICE_DIR) && $(COMPOSE_CMD) ps 2>/dev/null || echo "No containers found"; \
	fi

docling-logs: ## Show Docling service logs
	@if [ -d "$(DOCLING_SERVICE_DIR)" ]; then \
		cd $(DOCLING_SERVICE_DIR) && $(COMPOSE_CMD) logs -f; \
	else \
		echo "$(RED)Docling service directory not found: $(DOCLING_SERVICE_DIR)$(NC)"; \
	fi

# Aliases for backward compatibility
docker-up: container-up
docker-down: container-down
docker-logs: container-logs
docker-reset: container-reset

## Frontend Specific
frontend-dev: ## Start frontend development server only
	@echo "$(BLUE)Starting React development server...$(NC)"
	@cd $(FRONTEND_DIR) && npm start

frontend-build: build-frontend ## Build React frontend

electron-dev: ## Start Electron in development mode
	@echo "$(BLUE)Starting Electron development...$(NC)"
	@cd $(FRONTEND_DIR) && npm run electron-dev

electron-build: ## Build Electron app
	@echo "$(BLUE)Building Electron app...$(NC)"
	@cd $(FRONTEND_DIR) && npm run build-electron

electron: ## Start Electron in production mode
	@echo "$(BLUE)Starting Electron...$(NC)"
	@cd $(FRONTEND_DIR) && npm run electron

## Packaging
package: build package-frontend ## Create distribution packages

package-frontend: ## Package Electron app for distribution
	@echo "$(BLUE)Packaging Electron app...$(NC)"
	@cd $(FRONTEND_DIR) && npm run dist
	@echo "$(GREEN)Electron app packaged in $(FRONTEND_DIR)/dist$(NC)"

package-backend: build-backend ## Package backend binary
	@echo "$(BLUE)Packaging backend...$(NC)"
	@mkdir -p $(DIST_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/
	@cp -r scripts $(DIST_DIR)/ 2>/dev/null || true
	@cp docker-compose.yml $(DIST_DIR)/ 2>/dev/null || true
	@cp .env.example $(DIST_DIR)/.env 2>/dev/null || true
	@echo "$(GREEN)Backend packaged in $(DIST_DIR)/$(NC)"

## Cleanup
clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@rm -rf $(FRONTEND_DIR)/build
	@rm -rf $(FRONTEND_DIR)/dist
	@rm -f $(BINARY_NAME)
	@rm -f .backend.pid
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)Clean complete$(NC)"

clean-deps: ## Clean dependencies (node_modules, go mod cache)
	@echo "$(BLUE)Cleaning dependencies...$(NC)"
	@rm -rf $(FRONTEND_DIR)/node_modules
	@go clean -modcache
	@echo "$(GREEN)Dependencies cleaned$(NC)"

clean-all: clean clean-deps container-down ## Clean everything including dependencies and containers
	@echo "$(GREEN)Everything cleaned$(NC)"

## Utilities
check-deps: ## Check if required tools are installed
	@echo "$(BLUE)Checking dependencies...$(NC)"
	@echo -n "Go: "; go version 2>/dev/null || echo "$(RED)Missing$(NC)"
	@echo -n "Node.js: "; node --version 2>/dev/null || echo "$(RED)Missing$(NC)"
	@echo -n "npm: "; npm --version 2>/dev/null || echo "$(RED)Missing$(NC)"
	@echo -n "Podman: "; podman --version 2>/dev/null || echo "$(RED)Missing$(NC)"
	@echo -n "Docker: "; docker --version 2>/dev/null || echo "$(YELLOW)Not installed (using Podman)$(NC)"
	@echo -n "PostgreSQL: "; psql --version 2>/dev/null || echo "$(RED)Missing$(NC)"

install-tools: ## Install development tools
	@echo "$(BLUE)Installing development tools...$(NC)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)Development tools installed$(NC)"

env: ## Show environment variables
	@echo "$(BLUE)Environment Variables:$(NC)"
	@echo "GO_MODULE=$(GO_MODULE)"
	@echo "BACKEND_DIR=$(BACKEND_DIR)"
	@echo "FRONTEND_DIR=$(FRONTEND_DIR)"
	@echo "BINARY_NAME=$(BINARY_NAME)"
	@echo "DB_NAME=$(DB_NAME)"
	@echo "DB_HOST=$(DB_HOST)"
	@echo "DB_PORT=$(DB_PORT)"
	@echo "CONTAINER_RUNTIME=$(CONTAINER_RUNTIME)"
	@echo "COMPOSE_CMD=$(COMPOSE_CMD)"
	@echo "DOCLING_HOST=$(DOCLING_HOST)"
	@echo "DOCLING_PORT=$(DOCLING_PORT)"
	@echo "DOCLING_SERVICE_DIR=$(DOCLING_SERVICE_DIR)"

## Convenience Commands
run-all: ## Start all services (database + docling + backend)
	@echo "$(BLUE)Starting all services...$(NC)"
	@$(MAKE) container-up
	@$(MAKE) docling-up
	@sleep 3
	@echo "$(YELLOW)Services started. Backend can be started with 'make dev-backend'$(NC)"
	@$(MAKE) status

stop-all: ## Stop all services
	@echo "$(BLUE)Stopping all services...$(NC)"
	@$(MAKE) stop
	@echo "$(GREEN)All services stopped$(NC)"