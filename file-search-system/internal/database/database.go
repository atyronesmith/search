package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB represents a database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection
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

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// InitSchema initializes the database schema
func (db *DB) InitSchema() error {
	ctx := context.Background()

	// Read and execute schema SQL
	schema := getSchemaSQL()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Rollback error is expected if transaction was committed
			// Only log if it's not a "no transaction" error
			if !strings.Contains(err.Error(), "no transaction") {
				logrus.WithError(err).Error("Failed to rollback transaction")
			}
		}
	}()

	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return tx.Commit()
}

// BeginTx begins a database transaction
func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, nil)
}

// Query executes a query that returns rows
func (db *DB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (db *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning any rows
func (db *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// Ping verifies the database connection is alive
func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// GetFailedFiles returns a list of file paths that have failed or were skipped during indexing
func (db *DB) GetFailedFiles(ctx context.Context) ([]string, error) {
	query := `SELECT path FROM files WHERE indexing_status IN ('failed', 'skipped')`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed files: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close database rows")
		}
	}()

	var failedFiles []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan failed file path: %v", err)
		}
		failedFiles = append(failedFiles, path)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading failed files: %v", err)
	}

	return failedFiles, nil
}

// ResetFileStatus resets a file's indexing status from 'failed' or 'skipped' to 'pending'
func (db *DB) ResetFileStatus(ctx context.Context, filePath string) error {
	query := `UPDATE files SET indexing_status = 'pending', last_indexed = NULL WHERE path = $1 AND indexing_status IN ('failed', 'skipped')`

	result, err := db.Exec(ctx, query, filePath)
	if err != nil {
		return fmt.Errorf("failed to reset file status: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no failed file found with path: %s", filePath)
	}

	return nil
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
DROP TABLE IF EXISTS file_entities CASCADE;
DROP TABLE IF EXISTS file_classification_scores CASCADE;
DROP TABLE IF EXISTS chunks CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS indexing_rules CASCADE;
DROP TABLE IF EXISTS indexing_stats CASCADE;
DROP MATERIALIZED VIEW IF EXISTS document_hierarchy CASCADE;
DROP VIEW IF EXISTS files_with_entities CASCADE;
DROP VIEW IF EXISTS document_type_distribution CASCADE;
DROP VIEW IF EXISTS entity_type_distribution CASCADE;

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
    metadata JSONB DEFAULT '{}'::jsonb,
    -- NLP fields
    document_type VARCHAR(50),
    document_confidence FLOAT,
    nlp_processed_at TIMESTAMPTZ
);

-- NLP Entity extraction table
CREATE TABLE file_entities (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    entity_text TEXT NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    confidence FLOAT NOT NULL,
    start_position INT,
    end_position INT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- NLP Document classification scores
CREATE TABLE file_classification_scores (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    classification_type VARCHAR(50) NOT NULL,
    score FLOAT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(file_id, classification_type)
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
    -- NLP fields
    entities JSONB,
    semantic_category VARCHAR(50),
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

-- NLP indexes
CREATE INDEX idx_file_entities_file_id ON file_entities(file_id);
CREATE INDEX idx_file_entities_type ON file_entities(entity_type);
CREATE INDEX idx_file_entities_confidence ON file_entities(confidence);
CREATE INDEX idx_file_classification_file_id ON file_classification_scores(file_id);
CREATE INDEX idx_file_classification_type ON file_classification_scores(classification_type);
CREATE INDEX idx_chunks_entities ON chunks USING GIN (entities);

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

-- NLP Views
CREATE OR REPLACE VIEW files_with_entities AS
SELECT 
    f.id,
    f.path,
    f.file_type,
    f.document_type,
    f.document_confidence,
    COUNT(DISTINCT fe.id) as entity_count,
    ARRAY_AGG(DISTINCT fe.entity_type) as entity_types,
    MAX(fe.confidence) as max_entity_confidence
FROM files f
LEFT JOIN file_entities fe ON f.id = fe.file_id
GROUP BY f.id, f.path, f.file_type, f.document_type, f.document_confidence;

CREATE OR REPLACE VIEW document_type_distribution AS
SELECT 
    document_type,
    COUNT(*) as file_count,
    AVG(document_confidence) as avg_confidence,
    MIN(document_confidence) as min_confidence,
    MAX(document_confidence) as max_confidence
FROM files
WHERE document_type IS NOT NULL
GROUP BY document_type
ORDER BY file_count DESC;

CREATE OR REPLACE VIEW entity_type_distribution AS
SELECT 
    entity_type,
    COUNT(*) as entity_count,
    COUNT(DISTINCT file_id) as file_count,
    AVG(confidence) as avg_confidence
FROM file_entities
GROUP BY entity_type
ORDER BY entity_count DESC;
`
}
