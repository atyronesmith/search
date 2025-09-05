# macOS File Search System - Implementation Plan

## Architecture Critique

### Strengths
- **Docling** for document analysis is excellent - handles PDFs, Word docs, presentations with layout understanding
- **Ollama** provides local LLM inference avoiding API costs and privacy concerns
- **pgVector** is battle-tested for vector similarity search with PostgreSQL's robustness
- Local processing ensures data privacy

### Critical Issues & Improvements

1. **Performance Bottleneck**: Indexing entire disk with embeddings will be:
   - Extremely slow (days for typical Mac with 500GB+ data)
   - Resource intensive (CPU/GPU for embeddings)
   - Storage heavy (vectors for millions of files)
   
   **Solution**: Implement incremental indexing with file watching, priority queues, and selective indexing

2. **File Type Handling**: Docling doesn't cover all formats
   
   **Solution**: Add fallback extractors:
   - Plain text files: Direct reading
   - Code files: Tree-sitter for AST parsing
   - Images: OCR with Tesseract/EasyOCR
   - Audio/Video: Metadata extraction only
   - Archives: Recursive extraction

3. **Chunking Strategy**: Critical for search quality
   
   **Solution**: Hybrid approach:
   - Semantic chunking for documents (paragraph/section boundaries)
   - Sliding window with overlap for code
   - Sentence-based for short texts

4. **Search Quality**: Pure vector search has limitations
   
   **Solution**: Hybrid search combining:
   - Vector similarity (semantic)
   - BM25 full-text search (keyword)
   - Metadata filtering (date, type, location)
   - Re-ranking with cross-encoder

5. **System Resource Management**
   
   **Solution**:
   - Background worker with nice priority
   - Configurable resource limits
   - Pause during high system load
   - Batch processing with checkpoints

## Detailed Implementation Plan

### Technology Stack
- **Backend**: Python with FastAPI
- **Database**: PostgreSQL + pgVector + TimescaleDB (for time-series metadata)
- **Queue**: Redis + Celery for distributed task processing
- **Embeddings**: Ollama with nomic-embed-text or all-minilm
- **GUI**: Electron + React or native Swift/AppKit
- **File Monitoring**: FSEvents API (macOS native)
- **Document Processing**: Docling + additional extractors

### Architecture Components

```
┌─────────────────────────────────────────────────────┐
│                    GUI Application                   │
│              (Electron/React or Swift)               │
└────────────────────┬────────────────────────────────┘
                     │ WebSocket/REST API
┌────────────────────▼────────────────────────────────┐
│                   API Server                         │
│                   (FastAPI)                          │
│  - Search endpoint with hybrid ranking               │
│  - Indexing status/control                          │
│  - File preview/navigation                          │
└────────┬───────────────────────┬────────────────────┘
         │                       │
┌────────▼──────────┐   ┌───────▼──────────────────┐
│   Search Engine   │   │   Indexing Pipeline      │
│  - Vector search  │   │  - File scanner          │
│  - BM25 search    │   │  - Content extractor     │
│  - Re-ranking     │   │  - Chunker               │
└───────┬───────────┘   │  - Embedding generator   │
        │               └───────┬─────────────────┘
        │                       │
┌───────▼───────────────────────▼─────────────────┐
│              PostgreSQL + pgVector               │
│   Tables:                                        │
│   - files (metadata, path, timestamps)           │
│   - chunks (content, vectors, file_id)           │
│   - search_index (FTS)                          │
└──────────────────────────────────────────────────┘
```

### Project Structure

```
macos-file-search/
├── backend/
│   ├── api/
│   │   ├── routes/
│   │   │   ├── search.py
│   │   │   ├── indexing.py
│   │   │   └── files.py
│   │   └── main.py
│   ├── indexing/
│   │   ├── scanner.py         # File system traversal
│   │   ├── extractors/
│   │   │   ├── docling_extractor.py
│   │   │   ├── text_extractor.py
│   │   │   ├── code_extractor.py
│   │   │   └── media_extractor.py
│   │   ├── chunker.py         # Smart chunking
│   │   ├── embedder.py        # Ollama interface
│   │   └── pipeline.py        # Orchestration
│   ├── search/
│   │   ├── vector_search.py
│   │   ├── text_search.py
│   │   ├── hybrid_ranker.py
│   │   └── query_processor.py
│   ├── database/
│   │   ├── models.py
│   │   ├── migrations/
│   │   └── connection.py
│   ├── monitoring/
│   │   ├── fs_monitor.py      # FSEvents watcher
│   │   └── metrics.py
│   └── config.py
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── SearchBar.tsx
│   │   │   ├── ResultsList.tsx
│   │   │   ├── FilePreview.tsx
│   │   │   └── IndexingStatus.tsx
│   │   ├── services/
│   │   └── App.tsx
│   └── electron/
│       └── main.js
├── scripts/
│   ├── setup_db.sql
│   ├── install_models.sh
│   └── benchmark.py
└── docker-compose.yml
```

### Database Schema

```sql
-- Main file metadata
CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    filename TEXT NOT NULL,
    extension TEXT,
    size_bytes BIGINT,
    mime_type TEXT,
    created_at TIMESTAMP,
    modified_at TIMESTAMP,
    indexed_at TIMESTAMP DEFAULT NOW(),
    content_hash TEXT,
    parent_dir TEXT,
    is_hidden BOOLEAN DEFAULT FALSE
);

-- Document chunks with embeddings
CREATE TABLE chunks (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    chunk_index INTEGER,
    content TEXT,
    embedding vector(384),  -- Adjust dimension based on model
    char_start INTEGER,
    char_end INTEGER,
    metadata JSONB
);

-- Full-text search index
CREATE TABLE search_index (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    content tsvector,
    language TEXT DEFAULT 'english'
);

-- Indexing queue and status
CREATE TABLE indexing_queue (
    id BIGSERIAL PRIMARY KEY,
    file_path TEXT UNIQUE NOT NULL,
    priority INTEGER DEFAULT 5,
    status TEXT DEFAULT 'pending',
    error_message TEXT,
    attempts INTEGER DEFAULT 0,
    queued_at TIMESTAMP DEFAULT NOW()
);

-- Create indices
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_extension ON files(extension);
CREATE INDEX idx_chunks_embedding ON chunks USING ivfflat (embedding vector_cosine_ops);
CREATE INDEX idx_search_content ON search_index USING gin(content);
```

### Development Phases

#### Phase 1: Core Infrastructure (Week 1-2)
1. Set up PostgreSQL with pgVector extension
2. Install and configure Ollama with embedding model
3. Create database schema and migrations
4. Build basic file scanner with metadata extraction
5. Implement simple text extraction for common formats

#### Phase 2: Document Processing Pipeline (Week 3-4)
1. Integrate Docling for document parsing
2. Implement smart chunking strategies
3. Build embedding generation with batching
4. Create indexing queue system with Celery
5. Add progress tracking and error handling

#### Phase 3: Search Engine (Week 5-6)
1. Implement vector similarity search
2. Add BM25 full-text search
3. Build hybrid ranking algorithm
4. Create query processor for natural language
5. Implement result caching

#### Phase 4: GUI Development (Week 7-8)
1. Design search interface
2. Build result display with snippets
3. Add file preview capabilities
4. Implement indexing status dashboard
5. Create settings/configuration panel

#### Phase 5: Optimization & Monitoring (Week 9-10)
1. Add FSEvents monitoring for real-time updates
2. Implement incremental indexing
3. Optimize database queries and indices
4. Add resource usage controls
5. Build performance metrics dashboard

#### Phase 6: Advanced Features (Week 11-12)
1. Add support for more file types
2. Implement search filters and facets
3. Add export/backup functionality
4. Create search history and bookmarks
5. Build similar file detection

### Key Implementation Details

#### Indexing Strategy
```python
# Priority-based indexing
PRIORITY_RULES = {
    "Documents": 1,  # User documents highest priority
    "Desktop": 2,
    "Downloads": 2,
    "Code": 3,
    "Pictures": 4,
    "System": 9     # Lowest priority
}

# Selective indexing
SKIP_PATTERNS = [
    "*/node_modules/*",
    "*/venv/*",
    "*/.git/*",
    "*/Library/Caches/*",
    "*.app/Contents/*"
]

# File size limits
MAX_FILE_SIZE = 100 * 1024 * 1024  # 100MB
```

#### Chunking Configuration
```python
CHUNK_STRATEGIES = {
    "markdown": "semantic",    # Respect headers
    "code": "ast_based",       # Function/class boundaries
    "pdf": "page_aware",       # Keep page context
    "text": "sliding_window"   # Fixed size with overlap
}

CHUNK_SIZES = {
    "target": 512,      # Tokens
    "max": 1024,
    "overlap": 64
}
```

#### Search Configuration
```python
SEARCH_WEIGHTS = {
    "vector_similarity": 0.6,
    "text_relevance": 0.3,
    "recency": 0.05,
    "file_importance": 0.05
}
```

### Performance Considerations

1. **Incremental Indexing**: Only process new/modified files
2. **Batch Processing**: Process embeddings in batches of 32-64
3. **Async Operations**: Use async/await for I/O operations
4. **Connection Pooling**: Maintain DB connection pool
5. **Result Caching**: Cache frequent queries with Redis
6. **Lazy Loading**: Load file content on-demand
7. **Index Optimization**: Use IVFFlat or HNSW for large vector indices

### Security & Privacy

1. **Permissions**: Respect file system permissions
2. **Exclusions**: Allow user-defined exclusion patterns
3. **Encryption**: Encrypt sensitive metadata
4. **Local Processing**: All processing stays on device
5. **Access Control**: Optional password protection

### Monitoring & Maintenance

1. **Health Checks**: Monitor Ollama, PostgreSQL status
2. **Storage Alerts**: Warn when DB size exceeds limits
3. **Error Recovery**: Automatic retry with exponential backoff
4. **Cleanup**: Remove entries for deleted files
5. **Backup**: Regular database backups

## Summary

This comprehensive plan addresses the main challenges of building a macOS file search system:
- Handles performance bottlenecks through incremental indexing and priority queues
- Provides comprehensive file type support beyond Docling's capabilities
- Implements hybrid search for better accuracy than pure vector search
- Manages system resources effectively to avoid impacting normal computer use
- Includes monitoring and maintenance features for long-term reliability

The phased development approach allows for iterative improvements and testing at each stage, ensuring a robust and user-friendly final product.