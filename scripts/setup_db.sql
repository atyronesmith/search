-- Database setup script for File Search System
-- Requires PostgreSQL with pgVector extension

-- Create database (run as superuser)
-- CREATE DATABASE file_search_db;

-- Connect to the database
-- \c file_search_db;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm; -- For fuzzy text matching

-- Drop existing tables if they exist (for clean setup)
DROP TABLE IF EXISTS search_cache CASCADE;
DROP TABLE IF EXISTS file_changes CASCADE;
DROP TABLE IF EXISTS text_search CASCADE;
DROP TABLE IF EXISTS chunks CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS indexing_rules CASCADE;
DROP MATERIALIZED VIEW IF EXISTS document_hierarchy CASCADE;

-- File hierarchy and metadata
CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    parent_path TEXT,
    filename TEXT NOT NULL,
    extension TEXT,
    file_type TEXT CHECK (file_type IN ('document', 'code', 'text', 'spreadsheet')),
    size_bytes BIGINT,
    created_at TIMESTAMP,
    modified_at TIMESTAMP,
    last_indexed TIMESTAMP DEFAULT NOW(),
    content_hash TEXT, -- SHA-256 for change detection
    indexing_status TEXT DEFAULT 'pending' CHECK (indexing_status IN ('pending', 'processing', 'completed', 'error', 'skipped')),
    error_message TEXT,
    metadata JSONB DEFAULT '{}'::jsonb -- Store document-specific metadata
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
    char_start INTEGER NOT NULL,
    char_end INTEGER NOT NULL,
    chunk_type TEXT CHECK (chunk_type IN ('semantic', 'code', 'table', 'list', 'sliding')),
    metadata JSONB DEFAULT '{}'::jsonb,
    UNIQUE(file_id, chunk_index)
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
    change_type TEXT NOT NULL CHECK (change_type IN ('created', 'modified', 'deleted', 'renamed')),
    old_path TEXT, -- For rename operations
    detected_at TIMESTAMP DEFAULT NOW(),
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP,
    error_message TEXT
);

-- Search history and cache
CREATE TABLE search_cache (
    id BIGSERIAL PRIMARY KEY,
    query_hash TEXT UNIQUE NOT NULL,
    query_text TEXT NOT NULL,
    results JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP DEFAULT NOW(),
    access_count INTEGER DEFAULT 1,
    last_accessed TIMESTAMP DEFAULT NOW(),
    ttl_minutes INTEGER DEFAULT 15 -- Cache time-to-live
);

-- Indexing configuration per directory
CREATE TABLE indexing_rules (
    id SERIAL PRIMARY KEY,
    path_pattern TEXT UNIQUE NOT NULL, -- e.g., '/Users/*/Documents/*'
    priority INTEGER DEFAULT 5 CHECK (priority >= 1 AND priority <= 10), -- 1-10, lower is higher priority
    enabled BOOLEAN DEFAULT TRUE,
    recursive BOOLEAN DEFAULT TRUE,
    file_patterns TEXT[] DEFAULT ARRAY[]::TEXT[], -- e.g., ['*.pdf', '*.docx']
    exclude_patterns TEXT[] DEFAULT ARRAY[]::TEXT[], -- e.g., ['*.tmp', '~*']
    max_file_size_mb INTEGER DEFAULT 100,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_parent ON files(parent_path);
CREATE INDEX idx_files_extension ON files(extension);
CREATE INDEX idx_files_modified ON files(modified_at DESC);
CREATE INDEX idx_files_status ON files(indexing_status);
CREATE INDEX idx_files_type ON files(file_type);
CREATE INDEX idx_files_hash ON files(content_hash);

CREATE INDEX idx_chunks_file ON chunks(file_id);
CREATE INDEX idx_chunks_embedding ON chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_text_search_content ON text_search USING GIN (tsv_content);
CREATE INDEX idx_text_search_title ON text_search USING GIN (title_tsv);
CREATE INDEX idx_text_search_file ON text_search(file_id);
CREATE INDEX idx_text_search_chunk ON text_search(chunk_id);

CREATE INDEX idx_file_changes_unprocessed ON file_changes(processed) WHERE processed = FALSE;
CREATE INDEX idx_file_changes_path ON file_changes(file_path);
CREATE INDEX idx_file_changes_detected ON file_changes(detected_at DESC);

CREATE INDEX idx_search_cache_query ON search_cache(query_hash);
CREATE INDEX idx_search_cache_accessed ON search_cache(last_accessed DESC);

CREATE INDEX idx_indexing_rules_pattern ON indexing_rules(path_pattern);
CREATE INDEX idx_indexing_rules_priority ON indexing_rules(priority) WHERE enabled = TRUE;

-- Create materialized view for document hierarchy
CREATE MATERIALIZED VIEW document_hierarchy AS
WITH RECURSIVE hierarchy AS (
    SELECT 
        id, 
        path, 
        parent_path, 
        filename,
        file_type,
        size_bytes,
        0 as level,
        path as root_path,
        ARRAY[filename] as path_array
    FROM files 
    WHERE parent_path IS NULL OR parent_path = ''
    
    UNION ALL
    
    SELECT 
        f.id, 
        f.path, 
        f.parent_path, 
        f.filename,
        f.file_type,
        f.size_bytes,
        h.level + 1,
        h.root_path,
        h.path_array || f.filename
    FROM files f
    INNER JOIN hierarchy h ON f.parent_path = h.path
    WHERE h.level < 10 -- Prevent infinite recursion
)
SELECT * FROM hierarchy;

CREATE INDEX idx_hierarchy_root ON document_hierarchy(root_path);
CREATE INDEX idx_hierarchy_level ON document_hierarchy(level);
CREATE INDEX idx_hierarchy_path ON document_hierarchy(path);

-- Function to clean old cache entries
CREATE OR REPLACE FUNCTION clean_old_cache() RETURNS void AS $$
BEGIN
    DELETE FROM search_cache 
    WHERE last_accessed < NOW() - INTERVAL '1 minute' * ttl_minutes;
END;
$$ LANGUAGE plpgsql;

-- Function to update file hierarchy on insert/update
CREATE OR REPLACE FUNCTION update_parent_path() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.path IS NOT NULL THEN
        NEW.parent_path := regexp_replace(NEW.path, '/[^/]+$', '');
        IF NEW.parent_path = NEW.path THEN
            NEW.parent_path := NULL;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER files_update_parent_path
    BEFORE INSERT OR UPDATE ON files
    FOR EACH ROW
    EXECUTE FUNCTION update_parent_path();

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER indexing_rules_updated_at
    BEFORE UPDATE ON indexing_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Insert default indexing rules
INSERT INTO indexing_rules (path_pattern, priority, file_patterns, exclude_patterns) VALUES
    ('~/Documents', 1, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt', '*.md'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Desktop', 2, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt', '*.md'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Downloads', 3, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Projects', 4, ARRAY['*.py', '*.js', '*.ts', '*.jsx', '*.tsx', '*.md', '*.json'], ARRAY['node_modules/*', '__pycache__/*', '.git/*']);

-- Create initial statistics table for monitoring
CREATE TABLE IF NOT EXISTS indexing_stats (
    id SERIAL PRIMARY KEY,
    total_files BIGINT DEFAULT 0,
    indexed_files BIGINT DEFAULT 0,
    failed_files BIGINT DEFAULT 0,
    total_chunks BIGINT DEFAULT 0,
    total_size_bytes BIGINT DEFAULT 0,
    last_updated TIMESTAMP DEFAULT NOW()
);

-- Initialize stats
INSERT INTO indexing_stats (total_files, indexed_files) VALUES (0, 0);

-- Function to update statistics
CREATE OR REPLACE FUNCTION update_indexing_stats() RETURNS void AS $$
BEGIN
    UPDATE indexing_stats SET
        total_files = (SELECT COUNT(*) FROM files),
        indexed_files = (SELECT COUNT(*) FROM files WHERE indexing_status = 'completed'),
        failed_files = (SELECT COUNT(*) FROM files WHERE indexing_status = 'error'),
        total_chunks = (SELECT COUNT(*) FROM chunks),
        total_size_bytes = (SELECT COALESCE(SUM(size_bytes), 0) FROM files),
        last_updated = NOW()
    WHERE id = 1;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust user as needed)
-- GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO file_search_user;
-- GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO file_search_user;
-- GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO file_search_user;

COMMENT ON TABLE files IS 'Main file metadata and indexing status';
COMMENT ON TABLE chunks IS 'Document chunks with vector embeddings for similarity search';
COMMENT ON TABLE text_search IS 'Full-text search index for BM25 ranking';
COMMENT ON TABLE file_changes IS 'Tracks file system changes for incremental indexing';
COMMENT ON TABLE search_cache IS 'Caches search results for performance';
COMMENT ON TABLE indexing_rules IS 'Configurable rules for directory indexing priorities';
COMMENT ON MATERIALIZED VIEW document_hierarchy IS 'Hierarchical view of indexed documents';

-- Success message
DO $$
BEGIN
    RAISE NOTICE 'Database setup completed successfully!';
    RAISE NOTICE 'Tables created: files, chunks, text_search, file_changes, search_cache, indexing_rules';
    RAISE NOTICE 'Indexes and triggers configured';
    RAISE NOTICE 'Default indexing rules inserted';
END $$;