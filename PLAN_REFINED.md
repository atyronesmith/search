# macOS File Search System - Refined Implementation Plan

## System Overview

A background service that indexes documents hierarchically, monitors file changes in real-time, and provides hybrid semantic/keyword search through an Electron+React UI.

## Supported File Types

### Primary Documents (Priority 1)
- PDF files (`.pdf`)
- Microsoft Word (`.docx`, `.doc`)
- Excel spreadsheets (`.xlsx`, `.xls`, `.csv`)
- Plain text files (`.txt`, `.md`, `.rtf`)

### Code Files (Priority 2)
- Source code (`.py`, `.js`, `.ts`, `.jsx`, `.tsx`, `.java`, `.cpp`, `.c`, `.go`, `.rs`, `.swift`)
- Configuration files (`.json`, `.yaml`, `.yml`, `.toml`, `.ini`, `.env`)
- Scripts (`.sh`, `.bash`, `.zsh`, `.ps1`)

### Excluded
- Binary files
- Media files (images, audio, video)
- System files
- Archive files

## Architecture

### Technology Stack
- **Backend**: Python with FastAPI
- **Database**: PostgreSQL with pgVector extension
- **Embeddings**: Ollama (nomic-embed-text model)
- **Frontend**: Electron + React + TypeScript
- **File Monitoring**: watchdog with FSEvents backend
- **Document Processing**: Docling + custom extractors
- **Process Management**: Python daemon with resource monitoring

### System Architecture

```
┌─────────────────────────────────────────────────────┐
│          Electron + React Frontend                   │
│  - Search interface                                  │
│  - Results display with highlighting                 │
│  - Indexing status & controls                       │
└────────────────────┬────────────────────────────────┘
                     │ IPC Bridge / REST API
┌────────────────────▼────────────────────────────────┐
│             Background Service (Python)              │
├──────────────────────────────────────────────────────┤
│  API Layer (FastAPI)                                 │
│  - /search (hybrid search endpoint)                  │
│  - /status (indexing status)                        │
│  - /control (pause/resume/reindex)                  │
├──────────────────────────────────────────────────────┤
│  Indexing Engine                     Search Engine   │
│  - File scanner                      - Vector search │
│  - Change monitor (FSEvents)         - BM25 search   │
│  - Content extractors                - Hybrid ranker │
│  - Smart chunker                     - Query parser  │
│  - Embedding generator                               │
├──────────────────────────────────────────────────────┤
│  Resource Monitor                                    │
│  - CPU/Memory tracking                              │
│  - Auto-pause on high load                          │
│  - Rate limiting                                    │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│              PostgreSQL Database                     │
│  - pgVector for embeddings                          │
│  - Full-text search with GIN indexes                │
│  - File metadata and change tracking                │
└──────────────────────────────────────────────────────┘
```

## Database Schema for Hybrid Search

```sql
-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm; -- For fuzzy text matching

-- File hierarchy and metadata
CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    parent_path TEXT,
    filename TEXT NOT NULL,
    extension TEXT,
    file_type TEXT, -- 'document', 'code', 'text'
    size_bytes BIGINT,
    created_at TIMESTAMP,
    modified_at TIMESTAMP,
    last_indexed TIMESTAMP DEFAULT NOW(),
    content_hash TEXT, -- SHA-256 for change detection
    indexing_status TEXT DEFAULT 'pending', -- pending, processing, completed, error
    error_message TEXT,
    metadata JSONB -- Store document-specific metadata
);

-- Document chunks with embeddings for vector search
CREATE TABLE chunks (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768), -- nomic-embed-text dimension
    start_page INTEGER, -- For PDFs
    start_line INTEGER, -- For code files
    char_start INTEGER,
    char_end INTEGER,
    chunk_type TEXT, -- 'semantic', 'code', 'table', 'list'
    metadata JSONB
);

-- Full-text search index (for BM25)
CREATE TABLE text_search (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    chunk_id BIGINT REFERENCES chunks(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    tsv_content tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    title_tsv tsvector, -- Separate index for filenames/titles
    language TEXT DEFAULT 'english'
);

-- File change tracking for incremental indexing
CREATE TABLE file_changes (
    id BIGSERIAL PRIMARY KEY,
    file_path TEXT NOT NULL,
    change_type TEXT NOT NULL, -- 'created', 'modified', 'deleted'
    detected_at TIMESTAMP DEFAULT NOW(),
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP
);

-- Search history and cache
CREATE TABLE search_cache (
    id BIGSERIAL PRIMARY KEY,
    query_hash TEXT UNIQUE NOT NULL,
    query_text TEXT NOT NULL,
    results JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    access_count INTEGER DEFAULT 1,
    last_accessed TIMESTAMP DEFAULT NOW()
);

-- Indexing configuration per directory
CREATE TABLE indexing_rules (
    id SERIAL PRIMARY KEY,
    path_pattern TEXT UNIQUE NOT NULL, -- e.g., '/Users/*/Documents/*'
    priority INTEGER DEFAULT 5, -- 1-10, lower is higher priority
    enabled BOOLEAN DEFAULT TRUE,
    recursive BOOLEAN DEFAULT TRUE,
    file_patterns TEXT[], -- e.g., ['*.pdf', '*.docx']
    exclude_patterns TEXT[] -- e.g., ['*.tmp', '~*']
);

-- Create indexes for performance
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_parent ON files(parent_path);
CREATE INDEX idx_files_modified ON files(modified_at DESC);
CREATE INDEX idx_files_status ON files(indexing_status);
CREATE INDEX idx_chunks_file ON chunks(file_id);
CREATE INDEX idx_chunks_embedding ON chunks USING ivfflat (embedding vector_cosine_ops);
CREATE INDEX idx_text_search_content ON text_search USING GIN (tsv_content);
CREATE INDEX idx_text_search_title ON text_search USING GIN (title_tsv);
CREATE INDEX idx_file_changes_unprocessed ON file_changes(processed) WHERE processed = FALSE;
CREATE INDEX idx_search_cache_query ON search_cache(query_hash);

-- Create materialized view for document hierarchy
CREATE MATERIALIZED VIEW document_hierarchy AS
WITH RECURSIVE hierarchy AS (
    SELECT 
        id, path, parent_path, filename, 
        0 as level,
        path as root_path
    FROM files 
    WHERE parent_path IS NULL OR parent_path = ''
    
    UNION ALL
    
    SELECT 
        f.id, f.path, f.parent_path, f.filename,
        h.level + 1,
        h.root_path
    FROM files f
    INNER JOIN hierarchy h ON f.parent_path = h.path
)
SELECT * FROM hierarchy;

CREATE INDEX idx_hierarchy_root ON document_hierarchy(root_path);
CREATE INDEX idx_hierarchy_level ON document_hierarchy(level);
```

## Incremental Indexing System

### File Change Detection

```python
# File monitoring configuration
MONITOR_CONFIG = {
    "watch_paths": [
        "~/Documents",
        "~/Desktop",
        "~/Downloads",
    ],
    "file_extensions": [
        # Documents
        ".pdf", ".doc", ".docx", 
        ".xls", ".xlsx", ".csv",
        ".txt", ".md", ".rtf",
        # Code
        ".py", ".js", ".ts", ".jsx", ".tsx",
        ".java", ".cpp", ".c", ".go", ".rs",
        ".json", ".yaml", ".yml"
    ],
    "ignore_patterns": [
        ".*",  # Hidden files
        "~*",  # Temp files
        "*.tmp",
        "__pycache__",
        "node_modules",
        ".git"
    ]
}

# Change detection strategy
class IncrementalIndexer:
    def detect_changes(self, file_path):
        # 1. Check if file exists in database
        # 2. Compare file modification time
        # 3. If newer, compute content hash
        # 4. If hash differs, mark for reindexing
        # 5. Track in file_changes table
        pass
    
    def process_change(self, change_event):
        if change_event.type == 'created':
            self.index_new_file(change_event.path)
        elif change_event.type == 'modified':
            self.reindex_file(change_event.path)
        elif change_event.type == 'deleted':
            self.remove_from_index(change_event.path)
```

### Initial Indexing Strategy

```python
# Hierarchical indexing with priorities
INDEXING_HIERARCHY = {
    "tier_1": {  # Immediate indexing
        "paths": ["~/Documents", "~/Desktop"],
        "max_depth": 5,
        "priority": 1
    },
    "tier_2": {  # Secondary indexing
        "paths": ["~/Downloads", "~/Projects"],
        "max_depth": 3,
        "priority": 5
    },
    "tier_3": {  # Background indexing
        "paths": ["~/Library/CloudStorage"],
        "max_depth": 2,
        "priority": 8
    }
}
```

## Content Extraction Pipeline

### Extractor Architecture

```python
from abc import ABC, abstractmethod

class BaseExtractor(ABC):
    @abstractmethod
    def can_extract(self, file_path: str) -> bool:
        pass
    
    @abstractmethod
    def extract(self, file_path: str) -> dict:
        pass

class DoclingExtractor(BaseExtractor):
    """For PDF, Word, Excel files"""
    def can_extract(self, file_path):
        return file_path.endswith(('.pdf', '.docx', '.doc', '.xlsx', '.xls'))
    
    def extract(self, file_path):
        # Use Docling API
        # Return structured content with metadata
        pass

class PlainTextExtractor(BaseExtractor):
    """For text, markdown, config files"""
    def can_extract(self, file_path):
        return file_path.endswith(('.txt', '.md', '.rtf', '.csv'))
    
    def extract(self, file_path):
        # Direct file reading with encoding detection
        pass

class CodeExtractor(BaseExtractor):
    """For source code files"""
    def can_extract(self, file_path):
        code_extensions = {'.py', '.js', '.ts', '.java', '.cpp', '.c', '.go'}
        return any(file_path.endswith(ext) for ext in code_extensions)
    
    def extract(self, file_path):
        # Extract with syntax awareness
        # Preserve structure (functions, classes)
        pass

# Extractor selection
class ExtractorPipeline:
    def __init__(self):
        self.extractors = [
            DoclingExtractor(),
            CodeExtractor(),
            PlainTextExtractor()  # Fallback
        ]
    
    def extract(self, file_path):
        for extractor in self.extractors:
            if extractor.can_extract(file_path):
                return extractor.extract(file_path)
        raise ValueError(f"No extractor for {file_path}")
```

## Hybrid Chunking Strategy

```python
class HybridChunker:
    def __init__(self):
        self.strategies = {
            'document': SemanticChunker(),
            'code': ASTChunker(),
            'text': SlidingWindowChunker()
        }
    
    def chunk(self, content, file_type, metadata):
        strategy = self.strategies.get(file_type, self.strategies['text'])
        return strategy.chunk(content, metadata)

class SemanticChunker:
    """For documents - respects paragraph/section boundaries"""
    def chunk(self, content, metadata):
        chunks = []
        # Split by paragraphs, headers
        # Keep semantic units together
        # Target ~512 tokens per chunk
        # Add 10% overlap between chunks
        return chunks

class ASTChunker:
    """For code - respects function/class boundaries"""
    def chunk(self, content, metadata):
        chunks = []
        # Parse AST if possible
        # Keep functions/classes intact
        # Include context (imports, class definition)
        return chunks

class SlidingWindowChunker:
    """Fallback - fixed size with overlap"""
    def chunk(self, content, metadata):
        chunks = []
        window_size = 512  # tokens
        overlap = 64  # tokens
        # Slide window across content
        return chunks
```

## Hybrid Search Implementation

```python
class HybridSearchEngine:
    def __init__(self, db_connection, embedder):
        self.db = db_connection
        self.embedder = embedder
        self.weights = {
            'vector': 0.6,
            'bm25': 0.3,
            'metadata': 0.1
        }
    
    async def search(self, query, limit=20):
        # 1. Generate query embedding
        query_embedding = await self.embedder.embed(query)
        
        # 2. Vector similarity search
        vector_results = await self.vector_search(query_embedding, limit * 2)
        
        # 3. BM25 full-text search
        bm25_results = await self.bm25_search(query, limit * 2)
        
        # 4. Metadata boost (recency, file type, location)
        metadata_scores = self.calculate_metadata_scores(
            vector_results + bm25_results
        )
        
        # 5. Hybrid ranking
        final_results = self.hybrid_rank(
            vector_results, 
            bm25_results, 
            metadata_scores
        )
        
        return final_results[:limit]
    
    async def vector_search(self, embedding, limit):
        query = """
        SELECT 
            c.id, c.file_id, c.content,
            1 - (c.embedding <=> %s) as similarity
        FROM chunks c
        ORDER BY c.embedding <=> %s
        LIMIT %s
        """
        return await self.db.fetch(query, embedding, embedding, limit)
    
    async def bm25_search(self, query_text, limit):
        query = """
        SELECT 
            ts.chunk_id, ts.file_id, ts.content,
            ts_rank(ts.tsv_content, plainto_tsquery('english', %s)) as rank
        FROM text_search ts
        WHERE ts.tsv_content @@ plainto_tsquery('english', %s)
        ORDER BY rank DESC
        LIMIT %s
        """
        return await self.db.fetch(query, query_text, query_text, limit)
    
    def hybrid_rank(self, vector_results, bm25_results, metadata_scores):
        # Normalize scores
        # Apply weights
        # Combine and re-rank
        pass
```

## Resource Management

```python
import psutil
import asyncio

class ResourceMonitor:
    def __init__(self):
        self.cpu_threshold = 70  # percent
        self.memory_threshold = 80  # percent
        self.is_paused = False
        self.check_interval = 5  # seconds
    
    async def start_monitoring(self):
        while True:
            cpu_percent = psutil.cpu_percent(interval=1)
            memory_percent = psutil.virtual_memory().percent
            
            if cpu_percent > self.cpu_threshold or memory_percent > self.memory_threshold:
                if not self.is_paused:
                    await self.pause_indexing()
            else:
                if self.is_paused:
                    await self.resume_indexing()
            
            await asyncio.sleep(self.check_interval)
    
    async def pause_indexing(self):
        self.is_paused = True
        # Signal indexer to pause
        # Save current state
        
    async def resume_indexing(self):
        self.is_paused = False
        # Signal indexer to resume
        # Restore state

class BackgroundService:
    def __init__(self):
        self.indexer = IncrementalIndexer()
        self.monitor = ResourceMonitor()
        self.rate_limiter = RateLimiter(
            max_files_per_minute=60,
            max_embeddings_per_minute=120
        )
    
    async def run(self):
        # Start resource monitor
        asyncio.create_task(self.monitor.start_monitoring())
        
        # Start file watcher
        asyncio.create_task(self.watch_filesystem())
        
        # Start indexing loop
        await self.indexing_loop()
    
    async def indexing_loop(self):
        while True:
            if not self.monitor.is_paused:
                # Get next file to index
                file = await self.get_next_file()
                if file:
                    await self.rate_limiter.acquire()
                    await self.indexer.index_file(file)
            else:
                await asyncio.sleep(1)
```

## Project Structure

```
file-search-system/
├── backend/
│   ├── src/
│   │   ├── api/
│   │   │   ├── __init__.py
│   │   │   ├── main.py              # FastAPI app
│   │   │   ├── routes/
│   │   │   │   ├── search.py        # Search endpoints
│   │   │   │   ├── status.py        # System status
│   │   │   │   └── control.py       # Start/stop/pause
│   │   │   └── models.py            # Pydantic models
│   │   ├── indexing/
│   │   │   ├── __init__.py
│   │   │   ├── scanner.py           # File system scanner
│   │   │   ├── monitor.py           # FSEvents watcher
│   │   │   ├── indexer.py           # Incremental indexer
│   │   │   ├── extractors/
│   │   │   │   ├── __init__.py
│   │   │   │   ├── base.py          # Base extractor class
│   │   │   │   ├── docling.py       # Docling integration
│   │   │   │   ├── text.py          # Plain text extractor
│   │   │   │   └── code.py          # Code file extractor
│   │   │   └── chunkers/
│   │   │       ├── __init__.py
│   │   │       ├── semantic.py      # Document chunking
│   │   │       ├── ast_based.py     # Code chunking
│   │   │       └── sliding.py       # Window chunking
│   │   ├── search/
│   │   │   ├── __init__.py
│   │   │   ├── engine.py            # Hybrid search engine
│   │   │   ├── vector.py            # Vector search
│   │   │   ├── text.py              # BM25 search
│   │   │   └── ranker.py            # Result ranking
│   │   ├── embeddings/
│   │   │   ├── __init__.py
│   │   │   └── ollama_client.py     # Ollama integration
│   │   ├── database/
│   │   │   ├── __init__.py
│   │   │   ├── connection.py        # DB connection pool
│   │   │   ├── models.py            # SQLAlchemy models
│   │   │   └── queries.py           # Query builders
│   │   ├── service/
│   │   │   ├── __init__.py
│   │   │   ├── daemon.py            # Background service
│   │   │   ├── resource_monitor.py  # CPU/memory monitoring
│   │   │   └── rate_limiter.py      # Rate limiting
│   │   └── config.py                # Configuration
│   ├── scripts/
│   │   ├── setup_db.sql             # Database setup
│   │   ├── install_ollama.sh        # Ollama installation
│   │   └── init_service.py          # Service initialization
│   ├── tests/
│   │   └── ...
│   ├── requirements.txt
│   └── Dockerfile
├── frontend/
│   ├── public/
│   │   └── index.html
│   ├── src/
│   │   ├── main/
│   │   │   └── main.ts              # Electron main process
│   │   ├── renderer/
│   │   │   ├── App.tsx              # React app root
│   │   │   ├── components/
│   │   │   │   ├── SearchBar.tsx    # Search input
│   │   │   │   ├── ResultsList.tsx  # Search results
│   │   │   │   ├── FilePreview.tsx  # File content preview
│   │   │   │   ├── StatusBar.tsx    # Indexing status
│   │   │   │   └── Settings.tsx     # Configuration UI
│   │   │   ├── hooks/
│   │   │   │   ├── useSearch.ts     # Search API hook
│   │   │   │   └── useStatus.ts     # Status API hook
│   │   │   ├── services/
│   │   │   │   └── api.ts           # API client
│   │   │   └── styles/
│   │   │       └── globals.css
│   │   └── preload/
│   │       └── preload.ts           # Electron preload
│   ├── electron-builder.json
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── docker-compose.yml               # PostgreSQL + pgVector
├── .env.example
└── README.md
```

## Implementation Timeline

### Week 1: Foundation
- Set up PostgreSQL with pgVector
- Install and configure Ollama
- Create database schema
- Basic project structure

### Week 2: Content Extraction
- Implement Docling extractor
- Build text and code extractors
- Create chunking strategies
- Test extraction pipeline

### Week 3: Indexing System
- Build file scanner
- Implement incremental indexer
- Add FSEvents monitoring
- Create content hash checking

### Week 4: Search Engine
- Implement vector search
- Add BM25 text search
- Build hybrid ranking
- Test search accuracy

### Week 5: Background Service
- Create daemon service
- Add resource monitoring
- Implement rate limiting
- Test auto-pause functionality

### Week 6: Frontend Development
- Set up Electron + React
- Build search interface
- Add results display
- Create status dashboard

### Week 7: Integration & Testing
- Connect frontend to backend
- End-to-end testing
- Performance optimization
- Bug fixes

### Week 8: Polish & Deployment
- Add settings UI
- Create installer
- Write documentation
- Final testing

<function_calls>
<invoke name="TodoWrite">
<parameter name="todos">[{"id": "1", "content": "Create refined implementation plan with specified requirements", "status": "completed"}, {"id": "2", "content": "Design incremental indexing system with FSEvents", "status": "in_progress"}, {"id": "3", "content": "Define hybrid search database schema", "status": "pending"}, {"id": "4", "content": "Create project structure and initial setup files", "status": "pending"}, {"id": "5", "content": "Generate database setup scripts", "status": "pending"}]