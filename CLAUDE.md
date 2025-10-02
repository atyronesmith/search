# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# ✅✅✅ INDEXING SCOPE BUG - FIXED 2025-01-09 ✅✅✅

**CRITICAL FIX APPLIED**: The indexing scope bug has been FIXED. The system was indexing 100,000+ files because:
1. **DashboardPage.tsx** was hardcoded to use user home directory instead of configured paths
2. **api_client.go** had fallback logic to use `/Users` directory
3. **Backend** wasn't using configured `WATCH_PATHS` when no paths provided

**FIXES APPLIED**:
- DashboardPage.tsx: Line 31 changed to use empty string `StartIndexing('')`
- api_client.go: Line 172 removed `/Users` fallback, now sends empty array
- handlers.go: Lines 242-245 added logic to use `s.config.WatchPaths` when `len(paths) == 0`

**RESULT**: System now correctly indexes ONLY ~/Documents and ~/Downloads as configured in .env WATCH_PATHS.

**⚠️ DO NOT REVERT THESE CHANGES OR ADD HARDCODED PATHS TO DESKTOP APP! ⚠️**

# ✅✅✅ INDEXING SCOPE - PROBLEM SOLVED ✅✅✅

## Project Overview

This is a hybrid file search system with both desktop and web interfaces. The **desktop application** (`file-search-desktop/`) is the primary interface, built with Wails (Go + React). The web frontend (`web/frontend/`) is secondary.

**Current Status**: ✅ PDF support implemented, search capabilities fixed, **Docling integration complete**, **temporal filesystem date queries implemented**, **✅ INDEXING SCOPE BUG FIXED**, **✅ AUTOMATIC FILE MONITORING IMPLEMENTED**, ready for production use.

## Architecture

### Backend Service (`file-search-system/`)
- **Go HTTP API server** with PostgreSQL + pgVector for hybrid search
- **Vector embeddings** via Ollama (nomic-embed-text model)
- **Real-time file monitoring** with automatic detection of file changes (create/modify/delete)
- **WebSocket updates** for indexing progress
- **Content processing pipeline**: File extraction → Chunking → Embeddings → Storage
- **PDF Support**: Integrated `pdftotext` extraction with fallback mechanisms
- **Enhanced Search**: Fixed query processing with proper file type filtering

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
ollama pull qwen3:4b # Download LLM model for query enhancement (auto-pulled if missing)
```

## Development Guidelines

### Code Quality Requirements

**⚠️ IMPORTANT: ALL GO CODE MUST PASS GOLANGCI-LINT WITHOUT ERRORS! ⚠️**

All Go code in this repository must:
- Pass `golangci-lint` checks without any errors
- Have proper documentation comments on all exported types, functions, and methods
- Follow Go naming conventions (avoid stuttering type names)
- Include justification comments for blank imports
- Use idiomatic Go patterns (simplify if-else blocks where appropriate)

Run `golangci-lint run` before committing any Go code changes. Fix all reported issues.

### Adding New API Methods
When adding methods to the desktop app:
1. Add method to `app.go` (Go backend)
2. Add corresponding method to `api_client.go`
3. **Manually update** `frontend/src/wails.d.ts` with TypeScript definition
4. Build with `wails build` to regenerate bindings

### File Processing
- **Document formats**: PDF, DOC, DOCX, XLS, XLSX, CSV (✅ Recently added)
- **Code formats**: 25+ languages including Go, JS, Python, Java, C++
- **Text formats**: TXT, MD, RTF, YAML, JSON, HTML, CSS
- **Chunking strategies**: Semantic (default), code-aware, sliding window
- **Size limits**: 10MB default per file, configurable via settings
- **Error handling**: Files are categorized (skipped vs failed) based on error type
- **PDF Processing**: Uses `pdftotext` for text extraction with page-based parsing

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
- **Cache management**: `/api/v1/cache/clear` endpoint for testing (✅ Recently added)
- **Enhanced search**: Fixed query processing for `"type: pdf"` style filters

### Configuration
- **Environment-based**: Settings via config files and env vars
- **Runtime updates**: Configuration can be updated via API
- **Default paths**: `~/Documents`, `~/Desktop`, `~/Downloads` for indexing

## Key File Locations

- `file-search-system/internal/api/` - HTTP handlers and server logic
- `file-search-system/internal/service/` - Core business logic and orchestration
- `file-search-system/internal/search/` - Hybrid search engine implementation
- `file-search-system/pkg/extractor/` - Content extraction (text, code, **PDF** ✅)
- `file-search-desktop/app.go` - Wails app methods (exposed to frontend)
- `file-search-desktop/frontend/src/wails.d.ts` - TypeScript interface definitions
- `file-search-system/scripts/setup_db.sql` - Database schema initialization
- `DOCLING.md` - 10-week plan for advanced document processing enhancement

## Testing

- **Backend tests**: `make test-backend` runs Go unit tests
- **Frontend tests**: `make test-frontend` runs React component tests
- **API testing**: `make api-test` validates endpoint connectivity
- **Database**: Test data reset via `/api/v1/database/reset` endpoint

## Dependencies

### Required External Services
- **PostgreSQL**: Database with pgVector extension
- **Ollama**: Local AI service for embeddings (requires nomic-embed-text model) and LLM enhancement (requires qwen3:4b model)

### Build Tools
- **Go 1.23+** for backend
- **Node.js 18+** for frontend builds
- **Wails CLI** for desktop app compilation
- **Podman/Docker** for containerized database
- **pdftotext** (optional) for enhanced PDF extraction

### Recent Enhancements (2025-01-09 & 2025-09-05)
- ✅ **PDF Support**: Complete PDF extraction and indexing pipeline
- ✅ **Search Fixes**: Resolved file type filtering issues (`"type: pdf"` now works correctly)
- ✅ **Enhanced File Support**: Added DOC, DOCX, XLS, XLSX, CSV to supported formats
- ✅ **Cache Management**: Added `/api/v1/cache/clear` endpoint for testing
- ✅ **Query Processing**: Fixed regex patterns for robust filter parsing
- ✅ **Docling Integration**: Complete ML-powered document processing (enabled by default)
- ✅ **Docling Roadmap**: Comprehensive 10-week plan for advanced document processing
- ✅ **Temporal Filesystem Queries** (2025-09-05): Proper creation vs modification date filtering using actual filesystem timestamps
- ✅ **LLM Query Enhancement**: Natural language temporal queries like "files created yesterday" vs "files modified last week"
- ✅ **Cross-Platform Timestamp Extraction**: Added djherbis/times dependency for reliable filesystem date access
- ✅ **Enhanced Settings UI**: Display current indexing configuration with warnings about scope issues
- ⚠️ **CRITICAL BUG**: Scanner may follow symlinks and index beyond configured scope (see emergency procedures below)

## LLM-Enhanced Search

**Updated Model**: Now uses **phi3:mini** for faster performance (3.3s vs 30s+ with qwen3:4b)

The search system includes intelligent query processing using the phi3:mini LLM model. This enables natural language searches such as:

### Supported Query Types
- **Content Pattern Detection**: "Find all files that contain a possible social security number"
- **Semantic Analysis**: "Find files that contain tables with financial information"
- **Count Queries**: "How many files of type PDF are there?"
- **Temporal Queries**: "Find files that were modified on Tuesday of last week"
- **Creation vs Modification**: "Files created yesterday" vs "Files modified this week"
- **Filesystem Date Queries**: Uses actual filesystem timestamps (not processing dates)
- **Document Classification**: "Find files that look like legal documents related to a law case"
- **Communication Analysis**: "Find files that contain correspondence with a doctor"

### Technical Implementation
- **Query Classification**: Automatically detects if LLM enhancement is needed
- **Content Filtering**: Pattern matching for SSNs, credit cards, financial data, tables
- **Semantic Search**: Vector similarity matching for conceptual queries
- **Fallback Support**: Gracefully degrades to traditional search if LLM is unavailable
- **Model Management**: Auto-downloads qwen3:4b model if missing from Ollama

The system requires all services (database, Ollama, backend) to be running for full functionality. The desktop app can operate in degraded mode if backend is unavailable. **Docling service is enabled by default** for enhanced document processing.

## 🚨 **CRITICAL: INDEX SCOPE WARNING**

**NEVER modify the indexing configuration without explicit user permission!**

The system is configured to index ONLY:
- `~/Documents` (priority 1)
- `~/Desktop` (priority 2)
- `~/Downloads` (priority 3)

**DO NOT:**
- Change the default indexing paths in the database
- Modify `indexing_rules` table entries
- Add broad paths like `/Users/`, `/`, or `/usr/`
- Enable recursive indexing of system directories
- Change the `WatchPaths` configuration without user approval

**Current safe configuration** (as per `setup_db.sql`):
```sql
INSERT INTO indexing_rules (path_pattern, priority, file_patterns, exclude_patterns) VALUES
    ('~/Documents', 1, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt', '*.md'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Desktop', 2, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt', '*.md'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Downloads', 3, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt'], ARRAY['~*', '.*', '*.tmp']);
```

Adding system-wide paths will cause the system to index 1.6M+ files, making it unusable. Always check the current indexing scope before making any configuration changes!

**If the system is indexing too many files, check for rogue indexing rules:**
```bash
# Check current indexing rules
podman exec file-search-db psql -U postgres -d file_search -c "SELECT path_pattern, priority, enabled FROM indexing_rules ORDER BY priority;"

# Remove problematic rules (example)
podman exec file-search-db psql -U postgres -d file_search -c "DELETE FROM indexing_rules WHERE path_pattern LIKE '%/*' OR path_pattern = '/';"
```

**ONLY these 3 rules should exist:**
- `~/Documents` (priority 1)
- `~/Desktop` (priority 2)
- `~/Downloads` (priority 3)

## 🚨 **CRITICAL BUG: Scanner Following Symlinks**

**KNOWN ISSUE:** Even with correct configuration, the scanner may follow symbolic links and index unintended directories like:
- `/Users/*/miniconda/` (Python environments)
- `/Users/*/go/pkg/` (Go packages)
- `/Users/*/dev/` (Development directories)

**EMERGENCY FIX if this happens:**

1. **Stop all indexing immediately:**
```bash
pkill -f file-search
```

2. **Clear the database:**
```bash
podman exec file-search-db psql -U postgres -d file_search -c "TRUNCATE files CASCADE;"
```

3. **Check current file count before restarting:**
```bash
podman exec file-search-db psql -U postgres -d file_search -c "SELECT COUNT(*) FROM files;"
# Should return 0 after truncate
```

4. **Before restarting, verify no problematic symlinks:**
```bash
find ~/Documents ~/Downloads -type l -exec readlink {} \; -exec echo " -> {}" \; | grep -E "(miniconda|go/pkg|dev|venv|python)"
```

5. **Monitor file count after restart:**
```bash
# Wait 30 seconds after restart, then check:
podman exec file-search-db psql -U postgres -d file_search -c "SELECT COUNT(*) as total_files FROM files;"
# Should be < 5000 for Documents+Downloads only
```

6. **If file count exceeds 10,000, STOP IMMEDIATELY and check paths:**
```bash
podman exec file-search-db psql -U postgres -d file_search -c "SELECT SUBSTRING(path FROM 1 FOR 50) as path_prefix, COUNT(*) FROM files GROUP BY SUBSTRING(path FROM 1 FOR 50) ORDER BY COUNT(*) DESC LIMIT 5;"
```

**Expected file counts:**
- Documents only: ~1000-3000 files
- Downloads only: ~500-2000 files
- Combined: < 5000 files total

**If you see more than 10,000 files, the system is broken and needs immediate attention.**
- all go code must pass golangci-lint
