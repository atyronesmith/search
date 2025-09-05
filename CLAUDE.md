# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a hybrid file search system with both desktop and web interfaces. The **desktop application** (`file-search-desktop/`) is the primary interface, built with Wails (Go + React). The web frontend (`web/frontend/`) is secondary.

## Architecture

### Backend Service (`file-search-system/`)
- **Go HTTP API server** with PostgreSQL + pgVector for hybrid search
- **Vector embeddings** via Ollama (nomic-embed-text model)
- **Real-time monitoring** with WebSocket updates for indexing progress
- **Content processing pipeline**: File extraction → Chunking → Embeddings → Storage

### Desktop App (`file-search-desktop/`)
- **Wails v2 framework** (Go backend + React frontend)
- **Primary interface** - this is what users primarily interact with
- **TypeScript definitions** in `frontend/src/wails.d.ts` must be manually updated when adding new Go methods
- **API communication** via HTTP client to backend service

### Database
- **PostgreSQL with pgVector** extension for vector similarity search
- **Key tables**: files, chunks, text_search, indexing_stats
- **Hybrid search**: Vector (60%) + BM25 (30%) + metadata (10%)

## Common Commands

### Quick Start
```bash
make install      # Install all dependencies (Go, Node, Wails, Ollama)
make run-all     # Start database + backend + desktop app
make status      # Show status of all services
make stop-all    # Stop all services
```

### Backend Development
```bash
make run-backend     # Start backend API server (port 8080)
make backend-daemon  # Run backend in background
make test-backend    # Run Go tests
make logs-backend    # View backend logs
```

### Desktop App Development  
```bash
make dev-frontend      # Run desktop app in development mode (hot reload)
make build-frontend    # Build production desktop app
make run-frontend      # Run built desktop app
```

### Database Management
```bash
make db-start        # Start PostgreSQL container
make db-stop         # Stop database
make db-reset        # Reset database (destroys all data)
make db-status       # Check database status
```

### AI/Embeddings
```bash
make ollama-start    # Start Ollama service
make ollama-models   # Download required embedding models
make ollama-status   # Check Ollama and model status
```

## Development Guidelines

### Adding New API Methods
When adding methods to the desktop app:
1. Add method to `app.go` (Go backend)
2. Add corresponding method to `api_client.go`
3. **Manually update** `frontend/src/wails.d.ts` with TypeScript definition
4. Build with `wails build` to regenerate bindings

### File Processing
- **Supported formats**: 25+ languages including Go, JS, Python, Java, C++
- **Chunking strategies**: Semantic (default), code-aware, sliding window
- **Size limits**: 10MB default per file, configurable via settings
- **Error handling**: Files are categorized (skipped vs failed) based on error type

### Database Schema
- **Files table**: Metadata, indexing status, content hashes
- **Chunks table**: Text segments with vector embeddings
- **Text_search table**: Full-text search optimization
- **Foreign key constraints**: Maintain referential integrity

### API Patterns
- **RESTful endpoints** with consistent JSON responses
- **WebSocket endpoint** `/api/v1/ws` for real-time updates
- **Rate limiting**: 100 requests/minute default
- **Error responses**: Standard format with error codes

### Configuration
- **Environment-based**: Settings via config files and env vars
- **Runtime updates**: Configuration can be updated via API
- **Default paths**: `~/Documents`, `~/Desktop`, `~/Downloads` for indexing

## Key File Locations

- `file-search-system/internal/api/` - HTTP handlers and server logic
- `file-search-system/internal/service/` - Core business logic and orchestration  
- `file-search-system/internal/search/` - Hybrid search engine implementation
- `file-search-desktop/app.go` - Wails app methods (exposed to frontend)
- `file-search-desktop/frontend/src/wails.d.ts` - TypeScript interface definitions
- `file-search-system/scripts/setup_db.sql` - Database schema initialization

## Testing

- **Backend tests**: `make test-backend` runs Go unit tests
- **Frontend tests**: `make test-frontend` runs React component tests  
- **API testing**: `make api-test` validates endpoint connectivity
- **Database**: Test data reset via `/api/v1/database/reset` endpoint

## Dependencies

### Required External Services
- **PostgreSQL**: Database with pgVector extension
- **Ollama**: Local AI service for embeddings (requires nomic-embed-text model)

### Build Tools
- **Go 1.23+** for backend
- **Node.js 18+** for frontend builds
- **Wails CLI** for desktop app compilation
- **Podman/Docker** for containerized database

The system requires all services (database, Ollama, backend) to be running for full functionality. The desktop app can operate in degraded mode if backend is unavailable.