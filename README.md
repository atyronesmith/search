# File Search System

A modern, high-performance file search system with semantic understanding, real-time indexing, and a native desktop interface.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)
![Node Version](https://img.shields.io/badge/node-16+-green.svg)
![Wails](https://img.shields.io/badge/wails-v2-purple.svg)

## 🚀 Quick Start

### Prerequisites

- **Go 1.21+**: Backend development
- **Node.js 16+**: Frontend development  
- **Wails v2**: Desktop application framework
- **Podman** (preferred) or **Docker**: Database container

### Installation

```bash
# 1. Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 2. Clone and setup
git clone <repository-url>
cd file-search

# 3. Start everything
make install    # Install dependencies
make run-all    # Start database, backend, and desktop app
```

The desktop application will open automatically and connect to the backend service running on `localhost:8080`.

## 🎯 Application Components

```
┌─────────────────────┐    HTTP API    ┌─────────────────────────┐
│                     │                │                         │
│  Desktop App        │◄──────────────►│   Backend Service       │
│  (Wails + React)    │                │   (Go + PostgreSQL)     │
│                     │                │                         │
└─────────────────────┘                └─────────────────────────┘
        Native UI                           API Server + Database
```

### Backend Service
- **Location**: `file-search-system/`
- **Technology**: Go + PostgreSQL + pgVector
- **Features**: File indexing, semantic search, real-time monitoring
- **API**: REST endpoints on `localhost:8080`

### Desktop Application  
- **Location**: `file-search-desktop/`
- **Technology**: Wails + React + TypeScript
- **Features**: Native UI, cross-platform, real-time updates
- **Connection**: HTTP client to backend API

## 📋 Available Commands

### Quick Commands

```bash
make install      # Install all dependencies
make run-all      # Start database, backend, and desktop app
make stop-all     # Stop all services
make clean-all    # Clean all build artifacts
make status       # Show status of all services
```

### Backend Service

```bash
make run-backend     # Start backend service (with database)
make stop-backend    # Stop backend service and database
make build-backend   # Build backend binary
make logs-backend    # Show backend logs
```

### Desktop Application

```bash
make run-frontend    # Build and run desktop app
make dev-frontend    # Run desktop app in development mode
make build-frontend  # Build desktop app for production
make clean-frontend  # Clean frontend build artifacts
```

### Database Management

```bash
make db-start       # Start PostgreSQL database
make db-stop        # Stop database
make db-init        # Initialize database schema
make db-reset       # Reset database (WARNING: destroys data)
```

### AI Model Management

```bash
make ollama-install    # Install Ollama if not already installed
make ollama-start      # Start Ollama service
make ollama-models     # Pull all required models (nomic-embed-text ~274MB)
make ollama-status     # Check Ollama service and models status
make ollama-list       # List installed models
make ollama-stop       # Stop Ollama service
```

### Development

```bash
make dev-all        # Start all services in development mode
make test           # Run all tests
make lint           # Run linters
make format         # Format code
```

## 🔧 Manual Setup

If you prefer to set up components individually:

### 1. Database Setup

```bash
# Start PostgreSQL with pgVector
cd file-search-system
podman-compose up -d    # or docker-compose up -d

# Initialize database schema
go run cmd/server/main.go -init-db
```

### 2. Backend Service

```bash
# Configure environment
cd file-search-system
cp .env.example .env
# Edit .env with your settings

# Start backend service
go run cmd/server/main.go
# Backend API available at http://localhost:8080
```

### 3. Desktop Application

```bash
# Build and run desktop app
cd file-search-desktop
wails build
open build/bin/file-search-desktop.app  # macOS
```

## ⚡ Features

### 🔍 Advanced Search
- **Hybrid Search**: Vector similarity + full-text search  
- **AI-Powered**: Uses Ollama with nomic-embed-text for semantic understanding
- **Smart Filters**: File type, date, size, path patterns
- **Real-time Results**: Instant search with suggestions
- **Contextual Search**: Understands meaning, not just keywords

### 📁 Intelligent Indexing  
- **Real-time Monitoring**: Automatic file system watching
- **Incremental Updates**: Only processes changed files
- **Multi-format Support**: 25+ programming languages and document types
- **Smart Chunking**: Document structure-aware processing

### 🖥️ Native Desktop Interface
- **Cross-platform**: Native apps for macOS, Windows, Linux
- **Modern UI**: React with Material Design components
- **Live Dashboard**: Real-time system metrics and controls
- **Offline Capable**: Graceful degradation when backend unavailable

### ⚙️ System Management
- **Resource Monitoring**: CPU, memory, disk usage tracking
- **Auto-throttling**: Adaptive rate limiting based on system load
- **Configuration**: Comprehensive settings management
- **Logging**: Structured logging with multiple levels

## 🏗️ Project Structure

```
file-search/
├── README.md                    # This file
├── Makefile                     # Build and run automation
├── ARCHITECTURE.md              # Detailed architecture documentation
├── IMPLEMENTATION_STATUS.md     # Development progress
│
├── file-search-system/          # Backend API service
│   ├── cmd/server/             # Application entry point
│   ├── internal/               # Core business logic
│   │   ├── api/               # REST API handlers
│   │   ├── search/            # Hybrid search engine
│   │   ├── service/           # Background services
│   │   └── ...
│   ├── pkg/                   # Reusable packages
│   ├── scripts/               # Database schema
│   ├── docker-compose.yml     # Database container
│   └── .env.example           # Configuration template
│
└── file-search-desktop/         # Desktop application
    ├── app.go                  # Wails app backend
    ├── api_client.go           # HTTP client for backend
    ├── frontend/               # React frontend
    ├── build/                  # Built application
    └── wails.json              # Wails configuration
```

## 🔧 Configuration

### Backend Configuration

Edit `file-search-system/.env`:

```bash
# Database
DATABASE_URL=postgresql://file_search_user:file_search_password@localhost:5432/file_search_db?sslmode=disable

# Indexing
INDEXING_PATHS=/Users,/Documents,/Projects
INDEXING_IGNORE_PATTERNS=*.tmp,node_modules/**,.git/**

# Performance  
PERFORMANCE_CPU_THRESHOLD=90
PERFORMANCE_FILES_PER_MINUTE=60

# Search Weights
SEARCH_VECTOR_WEIGHT=0.6
SEARCH_BM25_WEIGHT=0.3

# Ollama (for embeddings)
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=nomic-embed-text
```

### Desktop App Configuration

The desktop app automatically connects to `localhost:8080`. To change this, modify `file-search-desktop/app.go`:

```go
func NewApp() *App {
    return &App{
        apiClient: NewAPIClient("http://your-backend-url:8080"),
    }
}
```

## 🚨 Troubleshooting

### Application Won't Start

```bash
# Check prerequisites
go version              # Should be 1.21+
node --version          # Should be 16+
wails doctor           # Check Wails installation

# Check services
make status            # Show status of all services
```

### Backend Issues

```bash
# Check database
make db-start          # Ensure database is running
make db-init           # Reinitialize if needed

# Check backend logs
make logs-backend      # View backend service logs

# Test API
curl http://localhost:8080/api/v1/status
```

### Desktop App Issues

```bash
# Rebuild desktop app
make clean-frontend
make build-frontend

# Check backend connection
make status            # Ensure backend is running
```

### Search Not Working

```bash
# Check Ollama service and models
make ollama-status     # Check if Ollama and models are available
make ollama-models     # Install required embedding models
make ollama-logs       # Check Ollama logs for errors

# Test search manually
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query":"test","limit":5}'
```

### Performance Issues

```bash
# Check system resources
make status

# Adjust performance settings in file-search-system/.env:
PERFORMANCE_CPU_THRESHOLD=70
PERFORMANCE_FILES_PER_MINUTE=30

# Restart services
make restart-all
```

### Database Issues

```bash
# Reset database (WARNING: destroys all data)
make db-reset
make db-init

# Check database connection
make db-status
```

## 📊 System Requirements

### Minimum Requirements
- **OS**: macOS 10.13+, Windows 10+, or modern Linux
- **CPU**: 2 cores, 2.0 GHz
- **RAM**: 4GB (8GB recommended)
- **Disk**: 2GB free space + storage for indexed files
- **Network**: Internet connection for initial setup

### Recommended Requirements
- **OS**: Latest macOS, Windows 11, or Ubuntu 22.04+
- **CPU**: 4+ cores, 3.0 GHz+
- **RAM**: 8GB+ 
- **Disk**: SSD with 10GB+ free space
- **Network**: High-speed internet for Ollama model downloads

## 🤝 Development

### Setting up Development Environment

```bash
# Install development dependencies
make dev-deps

# Start development environment
make dev-all           # All services in development mode

# Run tests
make test              # All tests
make test-backend      # Backend tests only
make test-frontend     # Frontend tests only

# Code quality
make lint              # Run all linters
make format            # Format all code
```

### Adding New Features

1. **Backend Changes**: Modify files in `file-search-system/internal/`
2. **Frontend Changes**: Modify files in `file-search-desktop/frontend/src/`
3. **API Changes**: Update both backend handlers and frontend API client
4. **Database Changes**: Add migrations to `file-search-system/scripts/`

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **pgVector**: PostgreSQL extension for vector similarity search
- **Ollama**: Local LLM inference for embeddings  
- **Wails**: Native cross-platform desktop framework
- **Go**: Efficient backend development language
- **React**: Modern frontend framework

## 📚 Documentation

- [Backend API Documentation](file-search-system/README.md)
- [Desktop App Documentation](file-search-desktop/README.md) 
- [Architecture Overview](ARCHITECTURE.md)
- [Implementation Status](IMPLEMENTATION_STATUS.md)

---

**Happy searching!** 🔍