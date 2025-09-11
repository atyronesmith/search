# Alias to run the desktop frontend
.PHONY: run-front
run-front: run-frontend ## Alias to run the desktop frontend
# File Search System - Main Makefile
# Manages both backend service and desktop application

# Configuration
BACKEND_DIR = file-search-system
FRONTEND_DIR = file-search-desktop
CONTAINER_RUNTIME ?= $(shell command -v podman >/dev/null 2>&1 && echo podman || echo docker)
COMPOSE_CMD = $(CONTAINER_RUNTIME)-compose

# Ensure Go tools are in PATH
export PATH := $(PATH):$(HOME)/go/bin

# Colors for output
RED = \033[31m
GREEN = \033[32m
YELLOW = \033[33m
BLUE = \033[34m
MAGENTA = \033[35m
CYAN = \033[36m
RESET = \033[0m

# Default target
.PHONY: help
help: ## Show this help message
	@echo "$(CYAN)File Search System$(RESET)"
	@echo "$(YELLOW)Available commands:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'

# ==============================================================================
# Quick Start Commands
# ==============================================================================

.PHONY: install
install: ## Install all dependencies
	@echo "$(BLUE)Installing dependencies...$(RESET)"
	@echo "$(YELLOW)Installing Wails...$(RESET)"
	@go install github.com/wailsapp/wails/v2/cmd/wails@latest || echo "$(RED)Warning: Could not install Wails. Please install manually.$(RESET)"
	@echo "$(YELLOW)Installing backend dependencies...$(RESET)"
	@cd $(BACKEND_DIR) && go mod download
	@echo "$(YELLOW)Installing frontend dependencies...$(RESET)"
	@cd $(FRONTEND_DIR)/frontend && npm install
	@echo "$(YELLOW)Installing Ollama...$(RESET)"
	@$(MAKE) ollama-install
	@echo "$(GREEN)Dependencies installed successfully!$(RESET)"

.PHONY: run-all
run-all: db-start ollama-start backend-daemon frontend ## Start database, Ollama, backend, and desktop app
	@echo "$(GREEN)All services started!$(RESET)"
	@echo "$(CYAN)Backend API:$(RESET) http://localhost:8080"
	@echo "$(CYAN)Desktop App:$(RESET) Should open automatically"
	@echo ""
	@# Check if models are installed
	@if ! ollama list 2>/dev/null | grep -q "nomic-embed-text"; then \
		echo "$(YELLOW)⚠ Required Ollama models not found$(RESET)"; \
		echo "$(CYAN)Run 'make ollama-models' to install them$(RESET)"; \
	else \
		echo "$(GREEN)✓ All required models are installed$(RESET)"; \
	fi

.PHONY: stop-all
stop-all: stop-backend stop-frontend ollama-stop ## Stop all services
	@echo "$(GREEN)All services stopped!$(RESET)"

.PHONY: status
status: ## Show status of all services
	@echo "$(CYAN)=== Service Status ===$(RESET)"
	@echo "$(YELLOW)Database:$(RESET)"
	@$(COMPOSE_CMD) -f $(BACKEND_DIR)/$(CONTAINER_RUNTIME)-compose.yml ps 2>/dev/null || echo "  Database not running"
	@echo ""
	@echo "$(YELLOW)Ollama Service:$(RESET)"
	@if command -v ollama >/dev/null 2>&1; then \
		if pgrep -x "ollama" >/dev/null 2>&1; then \
			echo "  Ollama running on http://localhost:11434"; \
			if ollama list 2>/dev/null | grep -q "nomic-embed-text"; then \
				echo "  ✓ Required models installed"; \
			else \
				echo "  ⚠ Required models missing"; \
			fi; \
		else \
			echo "  Ollama not running"; \
		fi; \
	else \
		echo "  Ollama not installed"; \
	fi
	@echo ""
	@echo "$(YELLOW)Backend Service:$(RESET)"
	@lsof -i :8080 >/dev/null 2>&1 && echo "  Backend running on port 8080" || echo "  Backend not running"
	@echo ""
	@echo "$(YELLOW)Desktop Application:$(RESET)"
	@pgrep -f "file-search-desktop" >/dev/null 2>&1 && echo "  Desktop app running" || echo "  Desktop app not running"

.PHONY: clean-all
clean-all: clean-backend clean-frontend ## Clean all build artifacts
	@echo "$(GREEN)All artifacts cleaned!$(RESET)"

# ==============================================================================
# Ollama Model Management
# ==============================================================================

.PHONY: ollama-install
ollama-install: ## Install Ollama if not already installed
	@echo "$(BLUE)Checking Ollama installation...$(RESET)"
	@if ! command -v ollama >/dev/null 2>&1; then \
		echo "$(YELLOW)Ollama not found. Installing...$(RESET)"; \
		if [[ "$$OSTYPE" == "darwin"* ]]; then \
			echo "$(CYAN)Installing Ollama for macOS...$(RESET)"; \
			curl -fsSL https://ollama.com/install.sh | sh; \
		elif [[ "$$OSTYPE" == "linux-gnu"* ]]; then \
			echo "$(CYAN)Installing Ollama for Linux...$(RESET)"; \
			curl -fsSL https://ollama.com/install.sh | sh; \
		else \
			echo "$(RED)Please install Ollama manually from https://ollama.com$(RESET)"; \
			exit 1; \
		fi; \
		echo "$(GREEN)Ollama installed successfully!$(RESET)"; \
	else \
		echo "$(GREEN)Ollama is already installed$(RESET)"; \
	fi

.PHONY: ollama-start
ollama-start: ## Start Ollama service
	@echo "$(BLUE)Starting Ollama service...$(RESET)"
	@if pgrep -x "ollama" >/dev/null 2>&1; then \
		echo "$(GREEN)Ollama is already running$(RESET)"; \
	else \
		echo "$(YELLOW)Starting Ollama in background...$(RESET)"; \
		nohup ollama serve > /tmp/ollama.log 2>&1 & \
		sleep 2; \
		if pgrep -x "ollama" >/dev/null 2>&1; then \
			echo "$(GREEN)Ollama started successfully!$(RESET)"; \
			echo "$(CYAN)Logs: tail -f /tmp/ollama.log$(RESET)"; \
		else \
			echo "$(RED)Failed to start Ollama. Check /tmp/ollama.log for details$(RESET)"; \
			exit 1; \
		fi; \
	fi

.PHONY: ollama-stop
ollama-stop: ## Stop Ollama service
	@echo "$(BLUE)Stopping Ollama service...$(RESET)"
	@pkill -x ollama 2>/dev/null || true
	@echo "$(GREEN)Ollama service stopped$(RESET)"

.PHONY: ollama-models
ollama-models: ollama-start ## Pull all required Ollama models
	@echo "$(BLUE)Pulling required Ollama models...$(RESET)"
	@echo "$(YELLOW)This may take several minutes depending on your internet connection$(RESET)"
	@echo ""
	@# Pull embedding model (required for search)
	@echo "$(CYAN)[1/2] Pulling embedding model: nomic-embed-text$(RESET)"
	@echo "$(YELLOW)  Size: ~274MB$(RESET)"
	@ollama pull nomic-embed-text || { echo "$(RED)Failed to pull nomic-embed-text model$(RESET)"; exit 1; }
	@echo "$(GREEN)  ✓ nomic-embed-text model ready$(RESET)"
	@echo ""
	@# Pull optional LLM for enhanced features (you can add more models here)
	@echo "$(CYAN)[2/2] Checking for optional models...$(RESET)"
	@echo "$(YELLOW)  You can optionally pull larger models for enhanced features:$(RESET)"
	@echo "$(CYAN)    - llama2 (3.8GB): ollama pull llama2$(RESET)"
	@echo "$(CYAN)    - mistral (4.1GB): ollama pull mistral$(RESET)"
	@echo "$(CYAN)    - phi (1.6GB): ollama pull phi$(RESET)"
	@echo ""
	@echo "$(GREEN)Required models successfully installed!$(RESET)"
	@echo "$(CYAN)The File Search System is now ready to use.$(RESET)"

.PHONY: ollama-list
ollama-list: ## List installed Ollama models
	@echo "$(BLUE)Installed Ollama models:$(RESET)"
	@ollama list 2>/dev/null || echo "$(YELLOW)Ollama is not running or not installed$(RESET)"

.PHONY: ollama-status
ollama-status: ## Check Ollama service status
	@echo "$(BLUE)Ollama Status:$(RESET)"
	@if command -v ollama >/dev/null 2>&1; then \
		echo "$(GREEN)  ✓ Ollama installed$(RESET)"; \
		if pgrep -x "ollama" >/dev/null 2>&1; then \
			echo "$(GREEN)  ✓ Ollama service running$(RESET)"; \
			echo ""; \
			echo "$(CYAN)Installed models:$(RESET)"; \
			ollama list 2>/dev/null | tail -n +2 | while read line; do \
				echo "    $$line"; \
			done; \
			if ! ollama list 2>/dev/null | grep -q "nomic-embed-text"; then \
				echo ""; \
				echo "$(YELLOW)  ⚠ Required model 'nomic-embed-text' not found$(RESET)"; \
				echo "$(CYAN)  Run 'make ollama-models' to install it$(RESET)"; \
			fi; \
		else \
			echo "$(YELLOW)  ✗ Ollama service not running$(RESET)"; \
			echo "$(CYAN)  Run 'make ollama-start' to start it$(RESET)"; \
		fi; \
	else \
		echo "$(RED)  ✗ Ollama not installed$(RESET)"; \
		echo "$(CYAN)  Run 'make ollama-install' to install it$(RESET)"; \
	fi

.PHONY: ollama-logs
ollama-logs: ## Show Ollama service logs
	@if [ -f /tmp/ollama.log ]; then \
		echo "$(BLUE)Ollama logs:$(RESET)"; \
		tail -f /tmp/ollama.log; \
	else \
		echo "$(YELLOW)No Ollama log file found. Ollama may not be running.$(RESET)"; \
	fi

# ==============================================================================
# Database Management
# ==============================================================================

.PHONY: db-start
db-start: ## Start PostgreSQL database
	@echo "$(BLUE)Starting database...$(RESET)"
	@# Check if port 5432 is already in use
	@if lsof -i :5432 >/dev/null 2>&1; then \
		echo "$(YELLOW)Warning: Port 5432 is already in use$(RESET)"; \
		echo "$(YELLOW)Checking for existing containers...$(RESET)"; \
		$(CONTAINER_RUNTIME) ps --format "table {{.Names}}\t{{.Status}}" | grep -E "(postgres|pgvector|file-search-db|file_search_db)" || true; \
		echo "$(CYAN)You may need to run 'make db-cleanup' first$(RESET)"; \
	fi
	@cd $(BACKEND_DIR) && $(COMPOSE_CMD) up -d
	@echo "$(GREEN)Database started!$(RESET)"

.PHONY: db-stop
db-stop: ## Stop database
	@echo "$(BLUE)Stopping database...$(RESET)"
	@cd $(BACKEND_DIR) && $(COMPOSE_CMD) down
	@echo "$(GREEN)Database stopped!$(RESET)"

.PHONY: db-init
db-init: ## Initialize database schema
	@echo "$(BLUE)Initializing database schema...$(RESET)"
	@cd $(BACKEND_DIR) && go run cmd/server/main.go -init-db
	@echo "$(GREEN)Database schema initialized!$(RESET)"

.PHONY: db-reset
db-reset: ## Reset database (WARNING: destroys all data)
	@echo "$(RED)WARNING: This will destroy all data!$(RESET)"
	@read -p "Are you sure? [y/N] " -n 1 -r; echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "$(BLUE)Resetting database...$(RESET)"; \
		cd $(BACKEND_DIR) && $(COMPOSE_CMD) down -v; \
		cd $(BACKEND_DIR) && $(COMPOSE_CMD) up -d; \
		sleep 5; \
		cd $(BACKEND_DIR) && go run cmd/server/main.go -init-db; \
		echo "$(GREEN)Database reset complete!$(RESET)"; \
	else \
		echo "$(YELLOW)Database reset cancelled.$(RESET)"; \
	fi

.PHONY: db-status
db-status: ## Show database status
	@echo "$(BLUE)Database Status:$(RESET)"
	@cd $(BACKEND_DIR) && $(COMPOSE_CMD) ps

.PHONY: db-cleanup
db-cleanup: ## Clean up any conflicting database containers
	@echo "$(BLUE)Cleaning up database containers...$(RESET)"
	@# Stop and remove any containers that might be using port 5432
	@$(CONTAINER_RUNTIME) ps -a --format "{{.Names}}" | grep -E "(postgres|pgvector|file-search-db|file_search_db)" | while read name; do \
		if [ ! -z "$$name" ]; then \
			echo "$(YELLOW)Stopping container: $$name$(RESET)"; \
			$(CONTAINER_RUNTIME) stop $$name 2>/dev/null || true; \
			$(CONTAINER_RUNTIME) rm $$name 2>/dev/null || true; \
		fi \
	done
	@# Also try to stop via compose
	@cd $(BACKEND_DIR) && $(COMPOSE_CMD) down 2>/dev/null || true
	@# Kill any remaining processes on port 5432
	@lsof -ti :5432 | xargs kill -9 2>/dev/null || true
	@echo "$(GREEN)Database cleanup complete!$(RESET)"

# ==============================================================================
# Backend Service
# ==============================================================================

.PHONY: run-backend
run-backend: ## Start backend service (with database)
	@echo "$(BLUE)Starting backend service...$(RESET)"
	@# Ensure database is running
	@if ! $(CONTAINER_RUNTIME) ps | grep -q file-search-db; then \
		echo "$(YELLOW)Database not running, starting it first...$(RESET)"; \
		$(MAKE) db-start; \
		sleep 3; \
	fi
	@cd $(BACKEND_DIR) && go run cmd/server/main.go

.PHONY: backend-daemon
backend-daemon: db-start ## Start backend service in background
	@echo "$(BLUE)Starting backend service in background...$(RESET)"
	@sleep 2  # Wait for database to be ready
	@cd $(BACKEND_DIR) && nohup go run cmd/server/main.go > /tmp/file-search-backend.log 2>&1 & echo $$! > /tmp/file-search-backend.pid
	@echo "$(GREEN)Backend service started! PID: $$(cat /tmp/file-search-backend.pid)$(RESET)"
	@echo "$(CYAN)Logs: tail -f /tmp/file-search-backend.log$(RESET)"

.PHONY: stop-backend
stop-backend: db-stop ## Stop backend service and database
	@echo "$(BLUE)Stopping backend service...$(RESET)"
	@if [ -f /tmp/file-search-backend.pid ]; then \
		kill $$(cat /tmp/file-search-backend.pid) 2>/dev/null || true; \
		rm -f /tmp/file-search-backend.pid; \
	fi
	@pkill -f "go run cmd/server/main.go" 2>/dev/null || true
	@echo "$(GREEN)Backend service stopped!$(RESET)"

.PHONY: build-backend
build-backend: ## Build backend binary
	@echo "$(BLUE)Building backend...$(RESET)"
	@cd $(BACKEND_DIR) && go build -o bin/file-search-server cmd/server/main.go
	@echo "$(GREEN)Backend built: $(BACKEND_DIR)/bin/file-search-server$(RESET)"

.PHONY: test-backend
test-backend: ## Run backend tests
	@echo "$(BLUE)Running backend tests...$(RESET)"
	@cd $(BACKEND_DIR) && go test ./...

.PHONY: logs-backend
logs-backend: ## Show backend logs
	@if [ -f /tmp/file-search-backend.log ]; then \
		echo "$(BLUE)Backend logs:$(RESET)"; \
		tail -f /tmp/file-search-backend.log; \
	else \
		echo "$(YELLOW)No backend log file found. Backend may not be running in daemon mode.$(RESET)"; \
	fi

.PHONY: clean-backend
clean-backend: ## Clean backend build artifacts
	@echo "$(BLUE)Cleaning backend artifacts...$(RESET)"
	@cd $(BACKEND_DIR) && rm -rf bin/
	@echo "$(GREEN)Backend artifacts cleaned!$(RESET)"

# ==============================================================================
# Desktop Application
# ==============================================================================

.PHONY: run-frontend
run-frontend: build-frontend ## Build and run desktop app
	@echo "$(BLUE)Starting desktop application...$(RESET)"
	@cd $(FRONTEND_DIR) && open build/bin/file-search-desktop.app 2>/dev/null || \
		./build/bin/file-search-desktop.exe 2>/dev/null || \
		./build/bin/file-search-desktop 2>/dev/null || \
		echo "$(RED)Could not start desktop app. Please run manually from $(FRONTEND_DIR)/build/bin/$(RESET)"

.PHONY: dev-frontend
dev-frontend: ## Run desktop app in development mode
	@echo "$(BLUE)Starting desktop app in development mode...$(RESET)"
	@cd $(FRONTEND_DIR) && wails dev

.PHONY: build-frontend
build-frontend: ## Build desktop app for production
	@echo "$(BLUE)Building desktop application...$(RESET)"
	@cd $(FRONTEND_DIR) && wails build
	@echo "$(GREEN)Desktop app built: $(FRONTEND_DIR)/build/bin/$(RESET)"

.PHONY: frontend
frontend: build-frontend run-frontend ## Build and run desktop app

.PHONY: dashboard
dashboard: frontend ## Start the dashboard UI (alias for frontend)

.PHONY: stop-frontend
stop-frontend: ## Stop desktop application
	@echo "$(BLUE)Stopping desktop application...$(RESET)"
	@pkill -f "file-search-desktop" 2>/dev/null || true
	@echo "$(GREEN)Desktop application stopped!$(RESET)"

.PHONY: test-frontend
test-frontend: ## Run frontend tests
	@echo "$(BLUE)Running frontend tests...$(RESET)"
	@cd $(FRONTEND_DIR)/frontend && npm test

.PHONY: clean-frontend
clean-frontend: ## Clean frontend build artifacts
	@echo "$(BLUE)Cleaning frontend artifacts...$(RESET)"
	@cd $(FRONTEND_DIR) && rm -rf build/
	@cd $(FRONTEND_DIR)/frontend && rm -rf dist/
	@echo "$(GREEN)Frontend artifacts cleaned!$(RESET)"

# ==============================================================================
# Development Commands
# ==============================================================================

.PHONY: dev-all
dev-all: ## Start all services in development mode
	@echo "$(BLUE)Starting development environment...$(RESET)"
	@echo "$(YELLOW)This will start:$(RESET)"
	@echo "  - Database container"
	@echo "  - Backend service in background"
	@echo "  - Desktop app in development mode"
	@echo ""
	@make backend-daemon
	@sleep 3
	@make dev-frontend

.PHONY: restart-all
restart-all: stop-all run-all ## Restart all services

.PHONY: restart-dev
restart-dev: ## Restart backend and frontend to pick up code changes
	@echo "$(BLUE)Restarting development services...$(RESET)"
	@echo "$(YELLOW)Stopping backend and frontend...$(RESET)"
	@# Stop backend process
	@if [ -f /tmp/file-search-backend.pid ]; then \
		kill $$(cat /tmp/file-search-backend.pid) 2>/dev/null || true; \
		rm -f /tmp/file-search-backend.pid; \
	fi
	@pkill -f "go run cmd/server/main.go" 2>/dev/null || true
	@# Stop frontend process
	@pkill -f "wails dev" 2>/dev/null || true
	@pkill -f "file-search-desktop" 2>/dev/null || true
	@sleep 2
	@echo "$(YELLOW)Starting backend in background...$(RESET)"
	@# Ensure database is running
	@if ! $(CONTAINER_RUNTIME) ps | grep -q file-search-db; then \
		echo "$(YELLOW)Database not running, starting it first...$(RESET)"; \
		$(MAKE) db-start; \
		sleep 3; \
	fi
	@cd $(BACKEND_DIR) && nohup go run cmd/server/main.go > /tmp/file-search-backend.log 2>&1 & echo $$! > /tmp/file-search-backend.pid
	@echo "$(GREEN)Backend restarted! PID: $$(cat /tmp/file-search-backend.pid)$(RESET)"
	@sleep 2
	@echo "$(YELLOW)Starting frontend in development mode...$(RESET)"
	@cd $(FRONTEND_DIR) && nohup wails dev > /tmp/file-search-frontend.log 2>&1 &
	@sleep 3
	@echo "$(GREEN)Development services restarted!$(RESET)"
	@echo "$(CYAN)Backend logs: tail -f /tmp/file-search-backend.log$(RESET)"
	@echo "$(CYAN)Frontend logs: tail -f /tmp/file-search-frontend.log$(RESET)"

.PHONY: dev-deps
dev-deps: install ## Install development dependencies
	@echo "$(BLUE)Installing development tools...$(RESET)"
	@go install github.com/cosmtrek/air@latest || echo "$(YELLOW)Warning: Could not install air (live reload)$(RESET)"
	@echo "$(GREEN)Development dependencies installed!$(RESET)"

.PHONY: test
test: test-backend test-frontend ## Run all tests

.PHONY: lint
lint: ## Run all linters
	@echo "$(BLUE)Running linters...$(RESET)"
	@cd $(BACKEND_DIR) && go fmt ./... && go vet ./...
	@cd $(FRONTEND_DIR)/frontend && npm run lint 2>/dev/null || echo "$(YELLOW)No lint script found in frontend$(RESET)"
	@echo "$(GREEN)Linting complete!$(RESET)"

.PHONY: format
format: ## Format all code
	@echo "$(BLUE)Formatting code...$(RESET)"
	@cd $(BACKEND_DIR) && go fmt ./...
	@cd $(FRONTEND_DIR)/frontend && npm run format 2>/dev/null || echo "$(YELLOW)No format script found in frontend$(RESET)"
	@echo "$(GREEN)Code formatting complete!$(RESET)"

# ==============================================================================
# Utility Commands
# ==============================================================================

.PHONY: check-deps
check-deps: ## Check if all dependencies are installed
	@echo "$(BLUE)Checking dependencies...$(RESET)"
	@echo "$(YELLOW)Go:$(RESET)"
	@go version 2>/dev/null || echo "$(RED)  Go not installed$(RESET)"
	@echo "$(YELLOW)Node.js:$(RESET)"
	@node --version 2>/dev/null || echo "$(RED)  Node.js not installed$(RESET)"
	@echo "$(YELLOW)Wails:$(RESET)"
	@wails version 2>/dev/null || echo "$(RED)  Wails not installed$(RESET)"
	@echo "$(YELLOW)Container Runtime:$(RESET)"
	@$(CONTAINER_RUNTIME) --version 2>/dev/null || echo "$(RED)  No container runtime found$(RESET)"

.PHONY: api-test
api-test: ## Test backend API endpoints
	@echo "$(BLUE)Testing API endpoints...$(RESET)"
	@echo "$(YELLOW)Health check:$(RESET)"
	@curl -s http://localhost:8080/api/v1/status | head -1 || echo "$(RED)API not responding$(RESET)"
	@echo ""
	@echo "$(YELLOW)Search test:$(RESET)"
	@curl -s -X POST http://localhost:8080/api/v1/search \
		-H "Content-Type: application/json" \
		-d '{"query":"test","limit":5}' | head -1 || echo "$(RED)Search API not working$(RESET)"

.PHONY: docker-up
docker-up: db-start ## Legacy alias for db-start

.PHONY: docker-down
docker-down: db-stop ## Legacy alias for db-stop

.PHONY: logs
logs: logs-backend ## Show logs (alias for logs-backend)

# ==============================================================================
# Docker/Podman Detection and Setup
# ==============================================================================

.PHONY: setup-podman
setup-podman: ## Setup Podman as container runtime
	@echo "$(BLUE)Setting up Podman...$(RESET)"
	@command -v podman >/dev/null 2>&1 || { echo "$(RED)Podman not installed. Please install Podman first.$(RESET)"; exit 1; }
	@cd $(BACKEND_DIR) && ln -sf podman-compose.yml docker-compose.yml
	@echo "$(GREEN)Podman setup complete!$(RESET)"

.PHONY: setup-docker
setup-docker: ## Setup Docker as container runtime
	@echo "$(BLUE)Setting up Docker...$(RESET)"
	@command -v docker >/dev/null 2>&1 || { echo "$(RED)Docker not installed. Please install Docker first.$(RESET)"; exit 1; }
	@cd $(BACKEND_DIR) && ln -sf docker-compose.yml docker-compose.yml
	@echo "$(GREEN)Docker setup complete!$(RESET)"

# ==============================================================================
# Advanced Commands
# ==============================================================================

.PHONY: benchmark
benchmark: ## Run performance benchmarks
	@echo "$(BLUE)Running benchmarks...$(RESET)"
	@cd $(BACKEND_DIR) && go test -bench=. ./...

.PHONY: profile
profile: ## Start backend with profiling enabled
	@echo "$(BLUE)Starting backend with profiling...$(RESET)"
	@echo "$(CYAN)Profiling will be available at http://localhost:6060/debug/pprof/$(RESET)"
	@cd $(BACKEND_DIR) && go run cmd/server/main.go -profile

.PHONY: release
release: clean-all build-backend build-frontend ## Build release versions
	@echo "$(GREEN)Release builds complete!$(RESET)"
	@echo "$(CYAN)Backend binary:$(RESET) $(BACKEND_DIR)/bin/file-search-server"
	@echo "$(CYAN)Desktop app:$(RESET) $(FRONTEND_DIR)/build/bin/"

# Make sure intermediate files are not deleted
.PRECIOUS: $(BACKEND_DIR)/bin/ $(FRONTEND_DIR)/build/

# Default to help if no target specified
.DEFAULT_GOAL := help