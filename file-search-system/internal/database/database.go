package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

func New(connectionString string) (*DB, error) {
	conn, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Register pgvector types - not needed for newer versions
	// The pgvector-go package handles this automatically now

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) InitSchema() error {
	ctx := context.Background()
	
	// Read and execute schema SQL
	schema := getSchemaSQL()
	
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return tx.Commit()
}

func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, nil)
}

func (db *DB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

func (db *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

func getSchemaSQL() string {
	return `
-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Drop existing tables if they exist
DROP TABLE IF EXISTS search_cache CASCADE;
DROP TABLE IF EXISTS file_changes CASCADE;
DROP TABLE IF EXISTS text_search CASCADE;
DROP TABLE IF EXISTS chunks CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS indexing_rules CASCADE;
DROP TABLE IF EXISTS indexing_stats CASCADE;
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
    content_hash TEXT,
    indexing_status TEXT DEFAULT 'pending' CHECK (indexing_status IN ('pending', 'processing', 'completed', 'error', 'skipped')),
    error_message TEXT,
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Document chunks with embeddings
CREATE TABLE chunks (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768),
    start_page INTEGER,
    start_line INTEGER,
    char_start INTEGER NOT NULL,
    char_end INTEGER NOT NULL,
    chunk_type TEXT CHECK (chunk_type IN ('semantic', 'code', 'table', 'list', 'sliding')),
    metadata JSONB DEFAULT '{}'::jsonb,
    UNIQUE(file_id, chunk_index)
);

-- Full-text search index
CREATE TABLE text_search (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    chunk_id BIGINT REFERENCES chunks(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    tsv_content tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    title_tsv tsvector,
    language TEXT DEFAULT 'english'
);

-- File change tracking
CREATE TABLE file_changes (
    id BIGSERIAL PRIMARY KEY,
    file_path TEXT NOT NULL,
    change_type TEXT NOT NULL CHECK (change_type IN ('created', 'modified', 'deleted', 'renamed')),
    old_path TEXT,
    detected_at TIMESTAMP DEFAULT NOW(),
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP,
    error_message TEXT
);

-- Search cache
CREATE TABLE search_cache (
    id BIGSERIAL PRIMARY KEY,
    query_hash TEXT UNIQUE NOT NULL,
    query_text TEXT NOT NULL,
    results JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP DEFAULT NOW(),
    access_count INTEGER DEFAULT 1,
    last_accessed TIMESTAMP DEFAULT NOW(),
    ttl_minutes INTEGER DEFAULT 15
);

-- Indexing rules
CREATE TABLE indexing_rules (
    id SERIAL PRIMARY KEY,
    path_pattern TEXT UNIQUE NOT NULL,
    priority INTEGER DEFAULT 5 CHECK (priority >= 1 AND priority <= 10),
    enabled BOOLEAN DEFAULT TRUE,
    recursive BOOLEAN DEFAULT TRUE,
    file_patterns TEXT[] DEFAULT ARRAY[]::TEXT[],
    exclude_patterns TEXT[] DEFAULT ARRAY[]::TEXT[],
    max_file_size_mb INTEGER DEFAULT 100,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexing statistics
CREATE TABLE indexing_stats (
    id SERIAL PRIMARY KEY,
    total_files BIGINT DEFAULT 0,
    indexed_files BIGINT DEFAULT 0,
    failed_files BIGINT DEFAULT 0,
    total_chunks BIGINT DEFAULT 0,
    total_size_bytes BIGINT DEFAULT 0,
    last_updated TIMESTAMP DEFAULT NOW()
);

-- Create indexes
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

-- Functions
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

-- Insert default data
INSERT INTO indexing_rules (path_pattern, priority, file_patterns, exclude_patterns) VALUES
    ('~/Documents', 1, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt', '*.md'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Desktop', 2, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt', '*.md'], ARRAY['~*', '.*', '*.tmp']),
    ('~/Downloads', 3, ARRAY['*.pdf', '*.docx', '*.doc', '*.txt'], ARRAY['~*', '.*', '*.tmp']);

INSERT INTO indexing_stats (total_files, indexed_files) VALUES (0, 0);
`
}