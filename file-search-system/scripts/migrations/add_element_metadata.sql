-- Migration: Add element metadata support for Unstructured.io chunks
-- Date: 2025-01-11
-- Description: Adds columns to support using Unstructured elements directly as chunks

BEGIN;

-- Add new chunk type for element-based chunks
ALTER TABLE chunks 
    DROP CONSTRAINT IF EXISTS chunks_chunk_type_check;

ALTER TABLE chunks 
    ADD CONSTRAINT chunks_chunk_type_check 
    CHECK (chunk_type IN ('semantic', 'code', 'table', 'list', 'sliding', 'element'));

-- Add element metadata columns if they don't exist
ALTER TABLE chunks 
    ADD COLUMN IF NOT EXISTS element_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS element_types TEXT[],
    ADD COLUMN IF NOT EXISTS category_depth INTEGER,
    ADD COLUMN IF NOT EXISTS parent_element_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS is_title BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_header BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS emphasis_score FLOAT DEFAULT 1.0;

-- Add comments for documentation
COMMENT ON COLUMN chunks.element_type IS 'Type from Unstructured: Title, NarrativeText, Table, ListItem, etc.';
COMMENT ON COLUMN chunks.element_types IS 'Array of types when multiple elements are grouped';
COMMENT ON COLUMN chunks.category_depth IS 'Hierarchy level: 0=top level, 1=section, 2=subsection';
COMMENT ON COLUMN chunks.parent_element_id IS 'Unstructured parent_id for hierarchy';
COMMENT ON COLUMN chunks.is_title IS 'Quick flag for title elements';
COMMENT ON COLUMN chunks.is_header IS 'Quick flag for header elements';
COMMENT ON COLUMN chunks.emphasis_score IS 'Scoring boost factor based on element importance';

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_chunks_element_type ON chunks(element_type);
CREATE INDEX IF NOT EXISTS idx_chunks_emphasis ON chunks(emphasis_score DESC);
CREATE INDEX IF NOT EXISTS idx_chunks_is_title ON chunks(is_title) WHERE is_title = TRUE;
CREATE INDEX IF NOT EXISTS idx_chunks_is_header ON chunks(is_header) WHERE is_header = TRUE;
CREATE INDEX IF NOT EXISTS idx_chunks_category_depth ON chunks(category_depth);

-- Update emphasis scores for existing chunks based on type (if any)
UPDATE chunks 
SET emphasis_score = CASE 
    WHEN chunk_type = 'table' THEN 1.3
    WHEN chunk_type = 'list' THEN 0.9
    ELSE 1.0
END
WHERE emphasis_score = 1.0;

-- Create a function to calculate emphasis score from element type
CREATE OR REPLACE FUNCTION calculate_emphasis_score(elem_type VARCHAR)
RETURNS FLOAT AS $$
BEGIN
    RETURN CASE elem_type
        WHEN 'Title' THEN 2.0
        WHEN 'Header' THEN 1.5
        WHEN 'Table' THEN 1.3
        WHEN 'ListItem' THEN 0.9
        WHEN 'Footer' THEN 0.7
        WHEN 'PageBreak' THEN 0.5
        ELSE 1.0
    END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Create a view for element statistics
CREATE OR REPLACE VIEW element_statistics AS
SELECT 
    f.path,
    c.element_type,
    COUNT(*) as element_count,
    AVG(LENGTH(c.content)) as avg_size,
    MIN(LENGTH(c.content)) as min_size,
    MAX(LENGTH(c.content)) as max_size,
    AVG(c.emphasis_score) as avg_emphasis
FROM chunks c
JOIN files f ON c.file_id = f.id
WHERE c.element_type IS NOT NULL
GROUP BY f.path, c.element_type
ORDER BY f.path, element_count DESC;

-- Grant permissions if needed
GRANT SELECT ON element_statistics TO PUBLIC;

COMMIT;

-- Verification query
SELECT 
    column_name, 
    data_type, 
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'chunks' 
    AND column_name IN ('element_type', 'element_types', 'category_depth', 
                        'parent_element_id', 'is_title', 'is_header', 'emphasis_score')
ORDER BY ordinal_position;