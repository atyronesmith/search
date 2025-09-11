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
    ('docling_fallback', 'true', 'boolean', 'Use fallback extraction when Docling fails', 'docling'),
    
    -- LLM Prompt Configuration
    ('llm_prompt_template', 'You are an expert search term extraction system with access to document metadata for optimized hybrid search.

TASK: Generate search terms considering both the query and document metadata patterns in the corpus. For emotional or abstract queries, generate related terms, synonyms, and conceptually similar words.

USER QUERY: {{USER_QUERY}}

AVAILABLE METADATA CONTEXT:
- Document Types in Corpus: {{DOC_TYPES}}
- Time Range of Documents: {{TIME_RANGE}}
- Common Categories: {{CATEGORIES}}
- Departments/Projects: {{DEPARTMENTS}}
- Total Files: {{TOTAL_FILES}}

SEARCH CONTEXT: Query type: analytical, Intent: search

METADATA-AWARE EXTRACTION RULES:
1. If query implies time-sensitivity, emphasize temporal search terms
2. If query mentions document types (report, email, memo), include type-specific terms  
3. For departmental queries, include relevant organizational terms
4. Consider document hierarchy (section headers, chapters) for navigation queries
5. Weight terms based on metadata frequency in corpus

ENHANCED OUTPUT STRUCTURE:
{
  "vector_terms": [
    "semantic phrases matching query intent",
    "additional 3-5 contextual phrases"
  ],
  "text_terms": [
    "keywords from query", 
    "additional 6-8 relevant terms"
  ],
  "metadata_filters": {
    "file_types": ["relevant file extensions"],
    "date_range": {
      "start": "ISO date or null",
      "end": "ISO date or null"
    },
    "categories": ["relevant Unstructured categories"],
    "departments": [],
    "document_types": ["report", "email", "memo", "etc."],
    "confidence_threshold": 0.7
  },
  "pg_tsquery": "simple PostgreSQL tsquery using only &, |, ! operators with single words",
  "search_strategy": "explanation of approach",
  "metadata_boost_fields": ["fields to prioritize in ranking"]
}

METADATA-INFORMED EXAMPLES:

Query: "Find files that contain the word taxi"
{
  "vector_terms": [
    "taxi transportation services",
    "cab hailing documents", 
    "yellow cab records",
    "ride sharing references",
    "vehicle hire content"
  ],
  "text_terms": [
    "taxi",
    "cab", 
    "taxicab",
    "transport",
    "ride",
    "vehicle",
    "driver",
    "fare",
    "city",
    "street"
  ],
  "metadata_filters": {
    "file_types": ["csv", "pdf", "txt"],
    "confidence_threshold": 0.7
  },
  "pg_tsquery": "taxi | cab | taxicab",
  "search_strategy": "Simple keyword search for taxi-related content",
  "metadata_boost_fields": ["content", "title"]
}

Query: "Find files that are sad"
{
  "vector_terms": [
    "sad emotional content",
    "depressing melancholy documents",
    "unhappy sorrowful text",
    "tragic loss grief",
    "negative emotional tone"
  ],
  "text_terms": [
    "sad",
    "unhappy",
    "depressed",
    "melancholy",
    "sorrow",
    "grief",
    "loss",
    "tragic",
    "tears",
    "mourning"
  ],
  "metadata_filters": {
    "file_types": ["pdf", "docx", "txt"],
    "categories": ["NarrativeText"],
    "confidence_threshold": 0.6
  },
  "pg_tsquery": "sad | unhappy | depressed",
  "search_strategy": "Semantic search for emotional content with negative sentiment",
  "metadata_boost_fields": ["content", "title"]
}

Query: "Q3 financial reports from finance team"
{
  "vector_terms": [
    "third quarter financial reports",
    "Q3 finance department documents", 
    "quarterly financial statements Q3",
    "finance team Q3 reporting",
    "third quarter fiscal documentation"
  ],
  "text_terms": [
    "q3",
    "third", 
    "quarter",
    "financial",
    "finance",
    "report", 
    "fiscal",
    "revenue",
    "expense",
    "budget"
  ],
  "metadata_filters": {
    "file_types": ["pdf", "xlsx", "docx"],
    "date_range": {
      "start": "2024-07-01",
      "end": "2024-09-30"
    },
    "categories": ["Table", "FigureCaption", "NarrativeText"],
    "departments": ["finance", "accounting"],
    "document_types": ["report", "spreadsheet"],
    "confidence_threshold": 0.8
  },
  "pg_tsquery": "q3 & financial & report",
  "search_strategy": "Focus on Q3 time period with finance department filtering",
  "metadata_boost_fields": ["created_date", "department", "document_type"]
}

IMPORTANT:
- Return ONLY the JSON object, no additional text or comments
- Ensure all JSON is properly formatted and valid - NO COMMENTS IN JSON
- Generate 5 vector_terms and 8-10 text_terms
- The pg_tsquery MUST use ONLY simple PostgreSQL tsquery operators: & (AND), | (OR), ! (NOT)
- DO NOT use date ranges, complex syntax, or invalid operators in pg_tsquery
- Use only single words or simple phrases in pg_tsquery
- EXAMPLE VALID: "taxi | cab", "taxi & transport", "!spam"
- EXAMPLE INVALID: "AND (2020-01-01 TO 2025-12-31)", "&:all", "-&-", complex phrases
- Adapt term complexity based on query sophistication
- JSON must be parseable - do not include any explanatory text or comments within the JSON structure


formatted syntactically correct JSON OUTPUT:', 'string', 'LLM prompt template for search term extraction', 'ai')
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