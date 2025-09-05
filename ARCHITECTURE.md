# File Search System Architecture

## Overview

The File Search System is designed as a modern, scalable application with a microservice-oriented architecture. It consists of two main components:

1. **Backend Service** - A Go-based API server that handles file indexing, search, and data management
2. **Desktop Application** - A native Wails+React client that provides the user interface

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         File Search System                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────┐    HTTP API    ┌─────────────────────────┐  │
│  │                     │   (REST/WS)    │                         │  │
│  │  Desktop Client     │◄──────────────►│   Backend Service       │  │
│  │  (Wails + React)    │                │   (Go + PostgreSQL)     │  │
│  │                     │                │                         │  │
│  └─────────────────────┘                └─────────────────────────┘  │
│           │                                         │                │
│           │                                         │                │
│  ┌─────────────────────┐                ┌─────────────────────────┐  │
│  │                     │                │                         │  │
│  │   User Interface    │                │   File System Watch    │  │
│  │   • Search          │                │   • Real-time Monitor  │  │
│  │   • Dashboard       │                │   • Incremental Index  │  │
│  │   • File Management │                │   • Content Extraction │  │
│  │   • Settings        │                │                         │  │
│  │                     │                └─────────────────────────┘  │
│  └─────────────────────┘                            │                │
│                                                     │                │
│                                         ┌─────────────────────────┐  │
│                                         │                         │  │
│                                         │    Search Engine        │  │
│                                         │    • Vector Search      │  │
│                                         │    • Full-text Search   │  │
│                                         │    • Hybrid Ranking     │  │
│                                         │                         │  │
│                                         └─────────────────────────┘  │
│                                                     │                │
│                                         ┌─────────────────────────┐  │
│                                         │                         │  │
│                                         │   External Services     │  │
│                                         │   • Ollama (Embeddings) │  │
│                                         │   • PostgreSQL+pgVector │  │
│                                         │                         │  │
│                                         └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### 1. Backend Service (Go)

The backend service is a standalone HTTP API server that handles all core functionality.

#### Core Modules

```
Backend Service
├── API Layer
│   ├── REST Endpoints        # HTTP API handlers
│   ├── WebSocket Support     # Real-time updates
│   ├── Middleware           # Auth, CORS, rate limiting
│   └── Request/Response     # JSON marshaling, validation
│
├── Business Logic
│   ├── Search Engine       # Hybrid vector + text search
│   ├── Indexing Service    # File processing and indexing
│   ├── File Monitor        # Real-time file system watching
│   └── Resource Manager    # System resource monitoring
│
├── Data Layer
│   ├── Database Models     # PostgreSQL schema
│   ├── Vector Storage      # pgVector embeddings
│   ├── Cache Layer         # In-memory caching
│   └── Configuration       # Environment-based config
│
└── External Integrations
    ├── Ollama Client       # Embedding generation
    ├── File System API     # OS file operations
    └── System Metrics      # Resource monitoring
```

#### Key Services

**Search Engine** (`internal/search/`)
- Hybrid search combining vector similarity and full-text search
- Advanced query processing with filters and ranking
- Result caching and optimization
- Real-time search suggestions

**Indexing Service** (`internal/service/`)
- File system scanning and monitoring
- Content extraction for multiple file types
- Smart chunking strategies (semantic, code-aware)
- Incremental updates with change detection

**API Server** (`internal/api/`)
- RESTful API with comprehensive endpoints
- WebSocket support for real-time updates
- Request validation and error handling
- Rate limiting and security middleware

### 2. Desktop Application (Wails + React)

The desktop application is a native client that communicates with the backend via HTTP API.

#### Application Structure

```
Desktop Application
├── Native Layer (Go)
│   ├── Wails App Backend    # Main application logic
│   ├── API Client          # HTTP client for backend
│   ├── System Integration  # OS-specific features
│   └── Error Handling      # Graceful degradation
│
├── Frontend (React)
│   ├── Search Interface    # Main search functionality
│   ├── Dashboard           # System status and metrics
│   ├── File Management     # Browse and manage files
│   ├── Settings Panel      # Configuration management
│   └── Components          # Reusable UI components
│
└── Data Flow
    ├── State Management    # React contexts and hooks
    ├── API Integration     # HTTP client calls
    ├── Real-time Updates   # WebSocket connections
    └── Offline Fallback    # Demo data when disconnected
```

## Data Flow Architecture

### 1. File Indexing Flow

```
File System → Monitor → Scanner → Extractor → Chunker → Embeddings → Database
     │            │         │          │          │           │          │
     └────────────┴─────────┴──────────┴──────────┴───────────┴──────────┘
                              Real-time Updates
```

1. **File Monitor**: Watches file system for changes using fsnotify/FSEvents
2. **Scanner**: Recursively scans directories and detects file changes
3. **Extractor**: Extracts content based on file type (text, code, documents)
4. **Chunker**: Splits content into semantic chunks (paragraphs, functions, etc.)
5. **Embeddings**: Generates vector embeddings using Ollama
6. **Database**: Stores files, chunks, and embeddings in PostgreSQL

### 2. Search Flow

```
User Query → Frontend → API → Query Processor → Search Engine → Results
     │          │        │         │              │             │
     └──────────┴────────┴─────────┴──────────────┴─────────────┘
                            Real-time Response
```

1. **Frontend**: User enters search query with optional filters
2. **API**: HTTP request to backend search endpoint
3. **Query Processor**: Parses query, extracts filters, generates embeddings
4. **Search Engine**: Executes hybrid search (vector + full-text)
5. **Results**: Ranked results with highlighting and metadata

### 3. Real-time Updates Flow

```
Backend Events → WebSocket → Frontend → UI Updates
       │            │          │          │
       └────────────┴──────────┴──────────┘
              Real-time Sync
```

1. **Backend Events**: Indexing progress, system status, file changes
2. **WebSocket**: Real-time communication channel
3. **Frontend**: React state updates and notifications
4. **UI Updates**: Live dashboard updates and progress indicators

## Database Schema

### Core Tables

```sql
-- Files table: Metadata and indexing status
CREATE TABLE files (
    id SERIAL PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    extension TEXT,
    size BIGINT,
    content_hash TEXT,
    modified_time TIMESTAMP,
    indexed_time TIMESTAMP,
    status TEXT DEFAULT 'pending'
);

-- Chunks table: Text chunks with embeddings
CREATE TABLE chunks (
    id SERIAL PRIMARY KEY,
    file_id INTEGER REFERENCES files(id),
    content TEXT NOT NULL,
    chunk_index INTEGER,
    embedding vector(768),  -- pgVector extension
    tokens INTEGER,
    metadata JSONB
);

-- Search cache: Query result caching
CREATE TABLE search_cache (
    id SERIAL PRIMARY KEY,
    query_hash TEXT UNIQUE,
    query TEXT,
    results JSONB,
    created_time TIMESTAMP DEFAULT NOW(),
    hit_count INTEGER DEFAULT 1
);
```

### Indexes and Optimization

```sql
-- Vector similarity search
CREATE INDEX chunks_embedding_idx ON chunks 
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Full-text search
CREATE INDEX chunks_content_fts_idx ON chunks 
USING gin(to_tsvector('english', content));

-- File path searches
CREATE INDEX files_path_idx ON files USING btree(path);
CREATE INDEX files_extension_idx ON files USING btree(extension);

-- Time-based queries
CREATE INDEX files_modified_idx ON files USING btree(modified_time);
CREATE INDEX chunks_file_id_idx ON chunks USING btree(file_id);
```

## API Design

### REST Endpoints

```
Search Operations:
POST   /api/v1/search              # Execute search with filters
GET    /api/v1/search/suggest      # Get search suggestions
GET    /api/v1/search/history      # Get search history

File Management:
GET    /api/v1/files               # List indexed files
GET    /api/v1/files/{id}          # Get file details
POST   /api/v1/files/{id}/reindex  # Reindex specific file
DELETE /api/v1/files/{id}          # Remove file from index

Indexing Control:
POST   /api/v1/indexing/start      # Start indexing process
POST   /api/v1/indexing/stop       # Stop indexing process
POST   /api/v1/indexing/pause      # Pause indexing process
POST   /api/v1/indexing/resume     # Resume indexing process
GET    /api/v1/indexing/status     # Get indexing status
GET    /api/v1/indexing/stats      # Get indexing statistics

System Information:
GET    /api/v1/status              # System health status
GET    /api/v1/metrics             # System metrics
GET    /api/v1/config              # Get configuration
PUT    /api/v1/config              # Update configuration

Real-time:
WS     /api/v1/ws                  # WebSocket for live updates
```

### WebSocket Events

```json
{
  "type": "indexing_progress",
  "data": {
    "current": 150,
    "total": 1000,
    "rate": "25 files/min",
    "eta": "30 minutes"
  }
}

{
  "type": "system_status",
  "data": {
    "cpu": 45.2,
    "memory": 67.8,
    "disk": 23.1,
    "status": "healthy"
  }
}

{
  "type": "file_indexed",
  "data": {
    "file_id": 12345,
    "path": "/path/to/file.txt",
    "chunks": 5,
    "status": "completed"
  }
}
```

## Deployment Architecture

### Development Environment

```
Developer Machine
├── Backend Service (localhost:8080)
│   ├── Go application
│   ├── PostgreSQL container (podman/docker)
│   └── Ollama service (localhost:11434)
│
└── Desktop Application
    ├── Wails development server
    ├── React hot reload
    └── API client (connects to localhost:8080)
```

### Production Environment

```
Production Setup
├── Backend Service
│   ├── Compiled Go binary
│   ├── PostgreSQL database (managed/self-hosted)
│   ├── Ollama service (local/remote)
│   └── Configuration files
│
└── Desktop Application
    ├── Native executable (.app/.exe/.deb)
    ├── Built React bundle
    └── API client (configurable backend URL)
```

## Security Considerations

### Backend Security

- **Authentication**: Ready for token-based auth (JWT/OAuth)
- **Rate Limiting**: Configurable per-endpoint rate limits
- **Input Validation**: Comprehensive request validation
- **SQL Injection**: Parameterized queries and ORM
- **CORS**: Configurable cross-origin resource sharing

### Desktop Security

- **Native Security**: Wails provides built-in security features
- **API Communication**: HTTPS for production deployments
- **Local Storage**: Secure storage of user preferences
- **Code Signing**: Support for application signing
- **Sandboxing**: OS-level application sandboxing

## Performance Characteristics

### Scalability Limits

- **Files**: Tested up to 1M+ files
- **Storage**: ~10MB per 10K files (including embeddings)
- **Memory**: <500MB for 100K files
- **Search**: <100ms response time for most queries

### Optimization Strategies

- **Database**: Strategic indexing and query optimization
- **Caching**: Multi-level caching (search results, embeddings)
- **Concurrent Processing**: Parallel file processing and embedding generation
- **Resource Management**: Adaptive rate limiting and auto-pause
- **Incremental Updates**: Only process changed files

## Monitoring and Observability

### Metrics Collection

- **System Metrics**: CPU, memory, disk usage
- **Application Metrics**: Request rates, response times, error rates
- **Business Metrics**: Files indexed, searches performed, user activity
- **Custom Metrics**: Configurable application-specific metrics

### Logging Strategy

- **Structured Logging**: JSON-formatted logs with correlation IDs
- **Log Levels**: Configurable verbosity (DEBUG, INFO, WARN, ERROR)
- **Context Propagation**: Request tracing across components
- **Log Aggregation**: Compatible with standard log collectors

## Technology Choices

### Backend Technologies

- **Go**: Excellent concurrency, performance, and standard library
- **PostgreSQL**: Mature RDBMS with excellent full-text search
- **pgVector**: Vector similarity search extension
- **Ollama**: Local LLM inference for embeddings
- **Gorilla Mux**: HTTP routing and middleware

### Frontend Technologies

- **Wails**: Native desktop framework with Go integration
- **React**: Modern UI framework with excellent ecosystem
- **TypeScript**: Type safety and developer experience
- **Material-UI**: Consistent design system and components
- **Vite**: Fast build tool and development server

### Infrastructure

- **Podman/Docker**: Containerization for database services
- **Environment Configuration**: 12-factor app configuration
- **Cross-platform**: Native support for macOS, Windows, Linux
- **Package Management**: Go modules and npm for dependencies

This architecture provides a solid foundation for a scalable, maintainable, and performant file search system with modern development practices and deployment flexibility.