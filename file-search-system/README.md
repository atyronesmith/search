# File Search System

A modern, high-performance file search system with semantic understanding, real-time indexing, and a beautiful desktop interface.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)
![Node Version](https://img.shields.io/badge/node-16+-green.svg)

## Features

### 🔍 Advanced Search
- **Hybrid Search**: Combines vector similarity search with BM25 full-text search
- **Semantic Understanding**: Uses embeddings for context-aware search results
- **Real-time Results**: Instant search with query suggestions and autocomplete
- **Advanced Filters**: Filter by file type, date, size, and path patterns
- **Smart Ranking**: Multi-factor scoring with recency and relevance boosting

### 📁 Intelligent Indexing
- **Real-time Monitoring**: Automatic file system watching with fsnotify/FSEvents
- **Incremental Updates**: Only processes new and changed files
- **Smart Chunking**: Document structure-aware chunking for better search results
- **Multi-format Support**: 25+ programming languages, documents, and text files
- **Resource Management**: Auto-pause during high system load

### 🖥️ Modern Desktop Interface
- **Wails + React**: Native cross-platform desktop application
- **Service Architecture**: Backend API server with desktop client
- **Live Dashboard**: Real-time indexing status and system metrics
- **File Management**: Browse, preview, and manage indexed files
- **Settings Panel**: Comprehensive configuration management
- **Native Performance**: Smaller app size and better performance than Electron

### ⚡ Performance & Scalability
- **PostgreSQL + pgVector**: Efficient vector storage and similarity search
- **Concurrent Processing**: Parallel file processing and embedding generation
- **Smart Caching**: Multi-level caching for optimal performance
- **Resource Monitoring**: CPU, memory, and disk usage monitoring
- **Adaptive Rate Limiting**: Dynamic throttling based on system load

## Quick Start

### Prerequisites
- **Go 1.21+**: Backend and desktop app development
- **Node.js 16+**: React frontend development  
- **Wails v2**: Desktop application framework
- **Podman** (preferred) or **Docker**: Container runtime for database

### Setup
```bash
# 1. Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 2. Start backend service
cd file-search-system
cp .env.example .env
podman-compose up -d  # or docker-compose up -d
go run cmd/server/main.go -init-db
go run cmd/server/main.go

# 3. Build and run desktop app (in new terminal)
cd ../file-search-desktop
wails build
open build/bin/file-search-desktop.app  # macOS
```

## Usage

### Backend Service Commands

```bash
# Start backend service
cd file-search-system
go run cmd/server/main.go               # Production mode
go run cmd/server/main.go -init-db      # Initialize database first

# Development with auto-reload
go install github.com/cosmtrek/air@latest
air                                     # Auto-reloading development server
```

### Desktop Application Commands

```bash
# Development mode
cd file-search-desktop
wails dev                              # Development with hot reload

# Production build
wails build                            # Build native app
wails build -debug                     # Build with debug info

# Cross-platform builds
wails build -platform darwin/amd64    # Intel macOS
wails build -platform windows/amd64   # Windows
wails build -platform linux/amd64     # Linux
```

### Database Management

```bash
# Start database
podman-compose up -d              # Using Podman (preferred)
docker-compose up -d              # Using Docker

# Initialize database
go run cmd/server/main.go -init-db

# Stop database
podman-compose down               # Using Podman
docker-compose down               # Using Docker
```

## Project Structure

```
file-search-system/
├── cmd/server/                 # Backend server entry point
├── internal/
│   ├── api/                   # REST API server
│   ├── config/                # Configuration management
│   ├── database/              # Database layer
│   ├── indexing/              # File scanning and monitoring
│   ├── embeddings/            # Ollama integration
│   ├── search/                # Hybrid search engine
│   └── service/               # Background service orchestration
├── pkg/
│   ├── extractor/             # Content extraction
│   └── chunker/               # Text chunking strategies
├── scripts/                   # Database schema and utilities
├── docker-compose.yml         # Database container (Docker)
├── podman-compose.yml         # Database container (Podman)
└── .env.example               # Configuration template

file-search-desktop/            # Wails desktop application
├── app.go                     # Wails app backend
├── api_client.go              # HTTP client for backend API
├── wails.json                 # Wails configuration
├── frontend/                  # React frontend
│   ├── src/                   # React application
│   └── package.json           # Node.js dependencies
└── build/                     # Built application files
```

## Architecture

### Backend Service (Go)
- **Modular Design**: Clean architecture with separated concerns
- **RESTful API**: Comprehensive endpoints for all operations
- **WebSocket Support**: Real-time updates for indexing status
- **Background Service**: Orchestrates indexing, monitoring, and search
- **Resource Management**: Adaptive throttling and auto-pause
- **Standalone Service**: Runs independently on localhost:8080

### Desktop Application (Wails + React)
- **Native Performance**: True native app using Wails framework
- **Cross-platform**: Native experience on macOS, Windows, Linux
- **Service Integration**: HTTP client connects to backend API
- **Modern UI**: React frontend with Material-UI components
- **Offline Capability**: Graceful fallback when backend unavailable
- **Smaller Footprint**: More efficient than Electron-based solutions

### Database (PostgreSQL + pgVector)
- **Vector Storage**: Efficient embedding storage and similarity search
- **Full-text Search**: Built-in PostgreSQL text search capabilities
- **Hybrid Queries**: Combined vector and text search in single queries
- **Optimized Indexes**: Strategic indexing for performance

## Configuration

The system uses environment-based configuration. Copy `.env.example` to `.env` and modify as needed:

### Key Settings
- **Indexing Paths**: Directories to scan and index
- **Ignore Patterns**: Files and directories to exclude
- **Performance Limits**: CPU/memory thresholds and rate limits
- **Search Weights**: Balance between vector and text search
- **Database Settings**: Connection and optimization parameters

### Example Configuration
```bash
# Indexing
INDEXING_PATHS=/Users,/Documents,/Projects
INDEXING_IGNORE_PATTERNS=*.tmp,node_modules/**,.git/**

# Performance  
PERFORMANCE_CPU_THRESHOLD=90
PERFORMANCE_FILES_PER_MINUTE=60

# Search
SEARCH_VECTOR_WEIGHT=0.6
SEARCH_BM25_WEIGHT=0.3
```

## API Documentation

The system provides a comprehensive REST API:

### Search Endpoints
- `GET /api/v1/search` - Search files with filters
- `GET /api/v1/search/suggest` - Get search suggestions
- `GET /api/v1/search/history` - Search history

### File Management
- `GET /api/v1/files` - List indexed files
- `GET /api/v1/files/{id}` - Get file details
- `POST /api/v1/files/{id}/reindex` - Reindex specific file

### System Control
- `GET /api/v1/status` - System status
- `POST /api/v1/indexing/start` - Start indexing
- `POST /api/v1/indexing/stop` - Stop indexing
- `GET /api/v1/metrics` - System metrics

### WebSocket
- `WS /api/v1/ws` - Real-time status updates

## Development

### Adding New File Types
1. Implement extractor in `pkg/extractor/`
2. Add file type detection logic
3. Update chunking strategy if needed
4. Add tests and documentation

### Extending Search Features
1. Modify search engine in `internal/search/`
2. Update API handlers in `internal/api/`
3. Add frontend components
4. Update configuration options

### Contributing
1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run `make test lint format`
5. Submit a pull request

## Troubleshooting

### Common Issues

**Backend service won't start**
```bash
# Check database connection
podman-compose ps                        # Verify database is running
go run cmd/server/main.go -init-db      # Reinitialize database schema

# Check configuration
cat .env                                # Verify DATABASE_URL is set
```

**Desktop app build fails**
```bash
cd file-search-desktop
rm -rf frontend/node_modules
cd frontend && npm install
cd .. && wails build
```

**Desktop app can't connect to backend**
```bash
# Verify backend is running
curl http://localhost:8080/api/v1/status

# Check backend logs
go run cmd/server/main.go               # Start with logs visible
```

**Database connection issues**
```bash
podman-compose down && podman-compose up -d  # Restart database
go run cmd/server/main.go -init-db           # Reinitialize schema
```

## Container Runtime Support

### Podman (Preferred)
The system preferentially uses **Podman** as the container runtime for better security and rootless operation:

- **Rootless Containers**: Run containers without root privileges
- **Daemonless**: No background daemon required
- **Docker Compatible**: Drop-in replacement for Docker commands
- **Better Security**: Built-in SELinux/AppArmor support

### Docker (Fallback)
Docker is fully supported as a fallback when Podman is not available:

- **Wide Adoption**: Most common container runtime
- **Docker Desktop**: Easy setup on macOS/Windows
- **Compose Support**: Native docker-compose integration

### Configuration
The Makefile automatically detects the available runtime:

```bash
# Use Podman explicitly
CONTAINER_RUNTIME=podman make container-up

# Use Docker explicitly  
CONTAINER_RUNTIME=docker make container-up

# Auto-detect (Podman preferred)
make container-up
```

## Performance

### Benchmarks
- **Indexing Speed**: 60+ files/minute (configurable)
- **Search Latency**: <100ms for most queries
- **Memory Usage**: <500MB for 100k files
- **Storage**: ~10MB per 10k files (including embeddings)

### Optimization Tips
1. **SSD Storage**: Use SSD for database and index storage
2. **Memory**: 8GB+ RAM recommended for large collections
3. **CPU**: Multi-core CPU benefits parallel processing
4. **Network**: Local Ollama instance for best embedding performance

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- **pgVector**: PostgreSQL extension for vector similarity search
- **Ollama**: Local LLM inference for embeddings
- **Wails**: Native cross-platform desktop framework
- **Go**: Efficient backend development
- **React**: Modern frontend framework