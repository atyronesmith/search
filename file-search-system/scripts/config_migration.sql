-- Configuration management table migration
-- This table stores system configuration and survives database resets

-- Create configuration table if it doesn't exist
CREATE TABLE IF NOT EXISTS system_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(255) NOT NULL UNIQUE,
    config_value TEXT,
    config_type VARCHAR(50) NOT NULL DEFAULT 'string', -- string, number, boolean, json
    description TEXT,
    category VARCHAR(100) NOT NULL DEFAULT 'general',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_system_config_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS update_system_config_updated_at ON system_config;
CREATE TRIGGER update_system_config_updated_at
    BEFORE UPDATE ON system_config
    FOR EACH ROW
    EXECUTE FUNCTION update_system_config_updated_at();

-- Insert default configuration values
INSERT INTO system_config (config_key, config_value, config_type, description, category) VALUES
    -- Indexing Configuration
    ('watch_paths', '/Users/asmith/Documents,/Users/asmith/Downloads', 'json', 'Directories to monitor for file indexing', 'indexing'),
    ('ignore_patterns', '.*,~*,*.tmp,__pycache__,node_modules,.git,*.log,*.photoslibrary/**,**/Spotlight/**,**/Caches/**,**/Cache/**', 'json', 'File patterns to ignore during indexing', 'indexing'),
    ('max_file_size_mb', '100', 'number', 'Maximum file size to process in MB', 'indexing'),
    ('chunk_size', '512', 'number', 'Text chunk size for processing', 'indexing'),
    ('chunk_overlap', '64', 'number', 'Overlap between text chunks', 'indexing'),
    ('batch_size', '32', 'number', 'Number of files to process in each batch', 'indexing'),
    
    -- AI & Embeddings
    ('ollama_host', 'http://localhost:11434', 'string', 'Ollama service URL', 'ai'),
    ('embedding_model', 'nomic-embed-text', 'string', 'Model used for text embeddings', 'ai'),
    ('embedding_dim', '768', 'number', 'Dimension of embedding vectors', 'ai'),
    ('ollama_timeout', '30', 'number', 'Timeout for Ollama requests in seconds', 'ai'),
    
    -- Performance & Limits  
    ('cpu_threshold', '70', 'number', 'CPU threshold percentage for throttling', 'performance'),
    ('memory_threshold', '80', 'number', 'Memory threshold percentage for throttling', 'performance'),
    ('files_per_minute', '60', 'number', 'Maximum files to process per minute', 'performance'),
    ('embeddings_per_minute', '120', 'number', 'Maximum embeddings to generate per minute', 'performance'),
    
    -- Search Configuration
    ('search_vector_weight', '0.6', 'number', 'Weight for vector similarity in search', 'search'),
    ('search_bm25_weight', '0.3', 'number', 'Weight for BM25 text matching in search', 'search'),
    ('search_metadata_weight', '0.1', 'number', 'Weight for metadata matching in search', 'search'),
    ('search_cache_ttl', '3600', 'number', 'Search cache TTL in seconds', 'search'),
    ('search_default_limit', '20', 'number', 'Default number of search results', 'search'),
    
    -- Database Configuration
    ('database_max_connections', '10', 'number', 'Maximum database connections', 'database'),
    ('database_timeout', '30', 'number', 'Database timeout in seconds', 'database'),
    
    -- Docling Service
    ('docling_enabled', 'true', 'boolean', 'Enable Docling service for document processing', 'docling'),
    ('docling_service_url', 'http://localhost:8082', 'string', 'Docling service URL', 'docling'),
    ('docling_timeout', '300', 'number', 'Docling service timeout in seconds', 'docling'),
    ('docling_fallback', 'true', 'boolean', 'Use fallback extraction when Docling fails', 'docling')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description,
    category = EXCLUDED.category,
    updated_at = CURRENT_TIMESTAMP;

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_system_config_key ON system_config(config_key);
CREATE INDEX IF NOT EXISTS idx_system_config_category ON system_config(category);

-- Update database reset procedure to preserve configuration
CREATE OR REPLACE FUNCTION reset_database_preserve_config()
RETURNS void AS $$
BEGIN
    -- Store configuration temporarily
    CREATE TEMP TABLE temp_config AS SELECT * FROM system_config;
    
    -- Reset main tables (existing reset logic)
    TRUNCATE TABLE text_search CASCADE;
    TRUNCATE TABLE chunks CASCADE;
    TRUNCATE TABLE files CASCADE;
    TRUNCATE TABLE indexing_stats CASCADE;
    
    -- Recreate system_config table structure if needed
    -- (The config table itself should not be truncated)
    
    RAISE NOTICE 'Database reset completed, configuration preserved';
END;
$$ LANGUAGE plpgsql;